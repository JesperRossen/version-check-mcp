---
phase: 01-foundation-mcp-scaffolding
plan: 04
status: complete
date: 2026-05-12
---

# Plan 01-04 SUMMARY — cache + singleflight

## What landed

### `internal/cache/key.go`

- `type Key struct { Manager, Pkg, Op string; IncPre bool }`
- `(Key).String()` — `url.QueryEscape` every string field, then `|`-join.
  Eliminates Assumption A2 (pkg-name special-char collision) by construction.
- Documents that `Op` is locked to `"validate"` or `"latest"`.

### `internal/cache/cache.go`

- Internal `entry` envelope: `{ value any, kind string, expiresAt time.Time }` —
  the chosen workaround for `expirable.LRU`'s single-global-TTL limitation
  (RESEARCH.md Pitfall #2).
- `type Cache struct` with `*expirable.LRU[string, entry]`,
  `singleflight.Group`, `fullTTL`, `shortTTL`.
- `func NewCache(size int, fullTTL time.Duration) *Cache` — derives shortTTL
  via `clamp(fullTTL/30, [1s, 30s])`. size<=0 defaults to 1024.
- `func NewCacheWithShortTTL(size int, fullTTL, shortTTL time.Duration) *Cache` —
  test-only variant that bypasses the 1s floor (recommended in the plan).
- `func (*Cache) Close()` — calls `lru.Purge()` (the expirable LRU's resource
  release path).
- `type Loader[V any] func(ctx context.Context) (V, error)`.
- `func Get[V any](ctx, c, k, load) (V, error)` — singleflight per `k.String()`,
  tier policy applied (success=fullTTL, NotFound=shortTTL, everything else=no-cache).
  Uses `errors.As` to detect `*errs.E` with `KindNotFound`.

## Tier semantics (CACHE-04)

| Loader returns | Cached? | TTL |
|---|---|---|
| `nil` (success) | yes | `fullTTL` |
| `*errs.E` with `KindNotFound` | yes (negative cache) | `shortTTL` |
| `*errs.E` with `KindUpstreamDown` | no | — |
| `*errs.E` with `KindRateLimited` | no | — |
| non-`*errs.E` error | no | — |

## Tests

`go test -race ./internal/cache/...` — 7 passed (with race detector):

- `TestKeyStringIsDeterministicAndCollisionFree`
- `TestSingleflightDedupes` — 50 concurrent goroutines, identical key →
  loader called exactly once. **Phase 1 success criterion #5 satisfied.**
  Second sub-assertion flips `IncPre` and verifies the loader fires again.
- `TestExpires` — entries evicted after `fullTTL`.
- `TestTieredTTL/success cached at fullTTL`
- `TestTieredTTL/not_found cached at shortTTL`
- `TestTieredTTL/upstream_down not cached`

`go vet ./internal/cache/...` — clean.

Cumulative: 19 GREEN tests across 5 packages.

## Deviations from RESEARCH.md § Pattern 4

- **Added `NewCacheWithShortTTL` test-only constructor.** The plan explicitly
  recommended this to let `TestTieredTTL` use a sub-second shortTTL without
  lowering the production floor. The test was updated to call it.
- **`Close()` calls `Purge()`** rather than a hypothetical `lru.Close()`. The
  `hashicorp/golang-lru/v2/expirable` LRU does not expose a `Close()`
  method — its sweeper goroutine lifecycle is internal. `Purge()` is the
  documented way to release entries; the sweeper exits when the LRU is
  unreferenced. (Pitfall #7 in RESEARCH.md described a defensive pattern,
  not a specific API call.)
- **Type-assertion failure in fast path falls through to reload** instead of
  panicking. Two adapters using different `V` for the same `Key` is a
  programmer bug, but the safe behaviour is to refetch.

## Dependency footprint

Already-direct deps used by this plan:
- `github.com/hashicorp/golang-lru/v2/expirable`
- `golang.org/x/sync/singleflight`
- `github.com/JesperRossen/version-check-mcp/internal/errs`

No new direct deps added.
