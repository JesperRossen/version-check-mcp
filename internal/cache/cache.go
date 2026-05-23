// Package cache implements the locked D-08 caching layer used by every
// registry adapter (Phases 2+). It bakes in:
//
//   - composite cache key (Manager|Pkg|Op|IncPre) — see key.go
//   - singleflight per-key dedup (CACHE-03) so 50 concurrent callers for
//     the same key trigger exactly one upstream load
//   - tiered TTL (CACHE-04): success cached for fullTTL; not_found cached
//     for shortTTL; upstream_down and rate_limited are never cached
//   - generic-in-V Get so adapters get typed values back
//
// The envelope-in-one-LRU pattern (RESEARCH.md Pitfall #2, Pattern 4) is
// the chosen workaround for expirable.LRU's single-global-TTL limitation.
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"
)

// entry is the in-LRU envelope that lets the tiered TTL coexist with the
// LRU's single global TTL. kind is "ok" or "not_found" (internal, distinct
// from errs.Kind); expiresAt enforces the per-tier deadline.
type entry struct {
	value     any
	kind      string
	expiresAt time.Time
}

type Cache struct {
	lru      *expirable.LRU[string, entry]
	sf       singleflight.Group
	fullTTL  time.Duration
	shortTTL time.Duration
	now      func() time.Time
}

// NewCache constructs a Cache with the standard shortTTL derivation:
// shortTTL = clamp(fullTTL/30, [1s, 30s]). For test scenarios that need
// arbitrarily small shortTTLs, use NewCacheWithShortTTL.
//
// size <= 0 defaults to 1024.
func NewCache(size int, fullTTL time.Duration) *Cache {
	shortTTL := fullTTL / 30
	if shortTTL > 30*time.Second {
		shortTTL = 30 * time.Second
	}
	if shortTTL < time.Second {
		shortTTL = time.Second
	}
	return NewCacheWithClock(size, fullTTL, shortTTL, time.Now)
}

// NewCacheWithShortTTL is a test-friendly variant of NewCache that exposes
// shortTTL directly. Production code should call NewCache.
func NewCacheWithShortTTL(size int, fullTTL, shortTTL time.Duration) *Cache {
	return NewCacheWithClock(size, fullTTL, shortTTL, time.Now)
}

// NewCacheWithClock is a test-friendly constructor that allows controlling
// the current time used for entry expiration checks.
func NewCacheWithClock(size int, fullTTL, shortTTL time.Duration, now func() time.Time) *Cache {
	if size <= 0 {
		size = 1024
	}
	if now == nil {
		now = time.Now
	}
	return &Cache{
		lru:      expirable.NewLRU[string, entry](size, nil, fullTTL),
		fullTTL:  fullTTL,
		shortTTL: shortTTL,
		now:      now,
	}
}

// Close shuts down the LRU's background sweeper goroutine. Production code
// should `defer c.Close()` in main; tests should defer it in t.Cleanup or
// alongside NewCache.
func (c *Cache) Close() {
	if c == nil || c.lru == nil {
		return
	}
	c.lru.Purge()
}

// Loader is the user-supplied function that fetches a value when the cache
// misses. Implementations should respect ctx and return *errs.E on failure.
type Loader[V any] func(ctx context.Context) (V, error)

// Get returns the cached value for k or invokes load to fetch it. Multiple
// concurrent calls for the same key collapse to a single loader invocation
// via singleflight. Tier policy:
//
//   - success         → cached for fullTTL
//   - *errs.E NotFound → cached for shortTTL (negative caching)
//   - everything else → not cached; subsequent calls retry
func Get[V any](ctx context.Context, c *Cache, k Key, load Loader[V]) (V, error) {
	keyStr := k.String()
	var zero V

	// Fast path: serve from cache if entry exists and hasn't expired.
	if e, ok := c.lru.Get(keyStr); ok && c.now().Before(e.expiresAt) {
		switch e.kind {
		case "not_found":
			return zero, errs.NotFound("cached miss", "key", keyStr)
		case "ok":
			if v, ok := e.value.(V); ok {
				return v, nil
			}
			// Type assertion failed (caller bug — two adapters using
			// different V for the same key). Fall through and re-load.
		}
	}

	// Slow path: singleflight ensures only one loader runs per key.
		raw, err, _ := c.sf.Do(keyStr, func() (any, error) {
			v, lerr := load(ctx)
			if lerr == nil {
				c.lru.Add(keyStr, entry{
					value:     v,
					kind:      "ok",
					expiresAt: c.now().Add(c.fullTTL),
				})
				return v, nil
			}

		var e *errs.E
		if errors.As(lerr, &e) && e.Kind == errs.KindNotFound {
			c.lru.Add(keyStr, entry{
				kind:      "not_found",
				expiresAt: c.now().Add(c.shortTTL),
			})
		}
		// upstream_down / rate_limited / non-errs.E errors are not cached.
		return zero, lerr
	})
	if err != nil {
		return zero, err
	}
	if v, ok := raw.(V); ok {
		return v, nil
	}
	return zero, nil
}
