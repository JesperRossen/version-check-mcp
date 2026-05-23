// RED test (Wave 0). Production code lands in Wave 2 (plan 01-04).
// See .planning/phases/01-foundation-mcp-scaffolding/01-VALIDATION.md.
package cache_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
)

func TestKeyStringIsDeterministicAndCollisionFree(t *testing.T) {
	a := cache.Key{Manager: "npm", Pkg: "react", Op: "validate", IncPre: false}
	b := cache.Key{Manager: "npm", Pkg: "react", Op: "validate", IncPre: false}
	if a.String() != b.String() {
		t.Errorf("equal Keys produced different String(): %q vs %q", a.String(), b.String())
	}

	c := a
	c.IncPre = true
	if a.String() == c.String() {
		t.Errorf("Keys differing only on IncPre produced equal String(): %q", a.String())
	}

	// Sanity: pkg name with embedded separator-like chars must not collide with a
	// pkg whose name resembles a concatenation of fields.
	k1 := cache.Key{Manager: "npm", Pkg: "@types/node", Op: "validate", IncPre: false}
	k2 := cache.Key{Manager: "npm", Pkg: "@types|node", Op: "validate", IncPre: false}
	if k1.String() == k2.String() {
		t.Errorf("@types/node collides with @types|node in String(): %q", k1.String())
	}
}

func TestSingleflightDedupes(t *testing.T) {
	c := cache.NewCache(64, 2*time.Second)
	defer c.Close()

	const N = 50
	var calls atomic.Int64
	var wg sync.WaitGroup
	wg.Add(N)
	loaderCh := make(chan struct{})

	k := cache.Key{Manager: "npm", Pkg: "react", Op: "latest", IncPre: false}
	loader := func(ctx context.Context) (string, error) {
		calls.Add(1)
		<-loaderCh
		return "1.0.0", nil
	}

	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			v, err := cache.Get[string](context.Background(), c, k, loader)
			if err != nil {
				t.Errorf("Get err = %v", err)
			}
			if v != "1.0.0" {
				t.Errorf("Get value = %q, want %q", v, "1.0.0")
			}
		}()
	}
	close(loaderCh)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("loader call count = %d, want 1 (singleflight should dedupe)", got)
	}

	// CACHE-02: flipping IncPre yields a different key → second loader invocation.
	k2 := k
	k2.IncPre = true
	if _, err := cache.Get[string](context.Background(), c, k2, loader); err != nil {
		t.Errorf("Get(k2) err = %v", err)
	}
	if got := calls.Load(); got < 2 {
		t.Errorf("loader call count after IncPre flip = %d, want >=2", got)
	}
}

func TestExpires(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	c := cache.NewCacheWithClock(64, 50*time.Millisecond, 50*time.Millisecond, func() time.Time {
		return now
	})
	defer c.Close()

	k := cache.Key{Manager: "npm", Pkg: "react", Op: "latest", IncPre: false}

	v1, err := cache.Get[string](context.Background(), c, k, func(ctx context.Context) (string, error) {
		return "v", nil
	})
	if err != nil || v1 != "v" {
		t.Fatalf("first Get: v=%q err=%v", v1, err)
	}

	var calls atomic.Int64
	v2, err := cache.Get[string](context.Background(), c, k, func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "w", nil
	})
	if err != nil {
		t.Fatalf("second Get err = %v", err)
	}
	if v2 != "v" {
		t.Fatalf("second Get value = %q, want %q", v2, "v")
	}
	if calls.Load() != 0 {
		t.Fatalf("loader called %d times before expiry, want 0", calls.Load())
	}

	now = now.Add(60 * time.Millisecond)
	v3, err := cache.Get[string](context.Background(), c, k, func(ctx context.Context) (string, error) {
		calls.Add(1)
		return "w", nil
	})
	if err != nil {
		t.Fatalf("third Get err = %v", err)
	}
	if v3 != "w" {
		t.Fatalf("third Get value = %q, want %q", v3, "w")
	}
	if calls.Load() != 1 {
		t.Fatalf("loader was called %d times after expiry, want 1", calls.Load())
	}
}

func TestTieredTTL(t *testing.T) {
	// Use the test-only constructor so shortTTL can be set below the
	// production 1s floor. fullTTL=2s, shortTTL=50ms keeps the test fast.
	c := cache.NewCacheWithShortTTL(64, 2*time.Second, 50*time.Millisecond)
	defer c.Close()

	t.Run("success cached at fullTTL", func(t *testing.T) {
		k := cache.Key{Manager: "npm", Pkg: "ok-pkg", Op: "validate", IncPre: false}
		var calls atomic.Int64
		loader := func(ctx context.Context) (string, error) {
			calls.Add(1)
			return "ok", nil
		}
		_, _ = cache.Get[string](context.Background(), c, k, loader)
		_, _ = cache.Get[string](context.Background(), c, k, loader)
		if calls.Load() != 1 {
			t.Errorf("success loader called %d times, want 1", calls.Load())
		}
	})

	t.Run("not_found cached at shortTTL", func(t *testing.T) {
		k := cache.Key{Manager: "npm", Pkg: "missing-pkg", Op: "validate", IncPre: false}
		var calls atomic.Int64
		loader := func(ctx context.Context) (string, error) {
			calls.Add(1)
			return "", errs.NotFound("not found")
		}
		_, err := cache.Get[string](context.Background(), c, k, loader)
		if err == nil {
			t.Fatal("first Get returned nil err, want not_found")
		}
		_, err = cache.Get[string](context.Background(), c, k, loader)
		if err == nil {
			t.Fatal("second Get returned nil err, want not_found (cached)")
		}
		var e *errs.E
		if !errors.As(err, &e) || e.Kind != errs.KindNotFound {
			t.Errorf("recovered err Kind = %v, want %v", e, errs.KindNotFound)
		}
		if calls.Load() != 1 {
			t.Errorf("not_found loader called %d times, want 1 (should be cached)", calls.Load())
		}
	})

	t.Run("upstream_down not cached", func(t *testing.T) {
		k := cache.Key{Manager: "npm", Pkg: "flaky-pkg", Op: "validate", IncPre: false}
		var calls atomic.Int64
		loader := func(ctx context.Context) (string, error) {
			calls.Add(1)
			return "", errs.UpstreamDown(errors.New("502"))
		}
		_, _ = cache.Get[string](context.Background(), c, k, loader)
		_, _ = cache.Get[string](context.Background(), c, k, loader)
		if calls.Load() < 2 {
			t.Errorf("upstream_down loader called %d times, want >=2 (must not cache)", calls.Load())
		}
	})
}
