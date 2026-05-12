---
phase: 02-npm-adapter-end-to-end-spine
plan: 01
subsystem: testfixtures, registry/npm, cache, testdata
tags: [scaffolding, fixtures, http-replay, npm-url]
requires:
  - 01-04 (cache.Key type — comment-only refresh)
provides:
  - internal/testfixtures.FixtureClient / RecordingClient / Client / UpdateMode
  - internal/registry/npm.escapeNPMPkg / packumentURL (unexported; consumed in 02-03)
  - testdata/fixtures/npm/{react,types-node,nonexistent[+.headers]}.json
  - testdata/README.md (fixture convention reference)
affects:
  - internal/cache/key.go (Op doc comment only)
tech-stack:
  added: []
  patterns:
    - "RoundTripper-as-seam (D-HTTP-01): no httpFetcher interface wrapper"
    - "UPDATE_FIXTURES=1 env switch flips Client between replay and recording"
    - "Path-traversal guard via filepath.Clean + prefix check"
    - "16 MiB io.LimitReader bounds replayed body size"
key-files:
  created:
    - internal/testfixtures/replay.go
    - internal/testfixtures/replay_test.go
    - internal/registry/npm/url.go
    - internal/registry/npm/url_test.go
    - testdata/fixtures/npm/react.json
    - testdata/fixtures/npm/types-node.json
    - testdata/fixtures/npm/nonexistent.json
    - testdata/fixtures/npm/nonexistent.headers.json
    - testdata/README.md
  modified:
    - internal/cache/key.go
decisions:
  - "D-FIX-01 honored: fixtures are literal upstream response bodies (no pretty-print, no field strip)"
  - "D-FIX-02 honored: missing fixture calls t.Fatalf with UPDATE_FIXTURES=1 hint — no skip"
  - "D-HTTP-01 honored: roundTripperFunc is unexported; tests build *http.Client directly"
  - "D-NPM-01 honored: cache.Key.Op comment widened to include 'packument'"
  - "T-02-01 (SSRF) mitigation: packumentURL hard-codes host; no baseURL parameter"
  - "T-02-02 (path traversal) mitigation: filepath.Clean + prefix check inside FixtureClient"
  - "T-02-05 (oversized fixture DoS) mitigation: 16 MiB io.LimitReader on response body"
metrics:
  duration: ~10 min (recorded)
  completed: 2026-05-12
---

# Phase 02 Plan 01: NPM Adapter Spine — Fixture Scaffolding Summary

Built the cross-cutting fixture-replay package, the NPM scoped-pkg URL helper, and the three initial NPM fixtures that 02-02 and 02-03 will consume. All changes are stdlib-only; zero new direct dependencies; DEP-01 invariant preserved.

## Final API

### `internal/testfixtures` (package `testfixtures`)

```go
type roundTripperFunc func(*http.Request) (*http.Response, error) // unexported
func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error)

func UpdateMode() bool

func FixtureClient(t testing.TB, fixtureDir string,
    urlToFile func(reqURL string) string, callCount *atomic.Int64) *http.Client

func RecordingClient(t testing.TB, fixtureDir string,
    urlToFile func(string) string) *http.Client

func Client(t testing.TB, fixtureDir string,
    urlToFile func(string) string, callCount *atomic.Int64) *http.Client
```

Imports: `bytes`, `encoding/json`, `io`, `net/http`, `os`, `path/filepath`, `strings`, `sync/atomic`, `testing`. All stdlib.

### `internal/registry/npm` (package `npm`)

```go
func escapeNPMPkg(pkg string) string   // unexported
func packumentURL(pkg string) string   // unexported
```

Imports: `strings` only.

### Op comment now in `internal/cache/key.go`

```
//   - Op is one of "validate" | "latest" | "packument"; key.go does not enforce this.
```

Comment-only edit; struct definition unchanged.

## Recorded fixture sizes

```
       60 testdata/fixtures/npm/nonexistent.headers.json
        0 testdata/fixtures/npm/nonexistent.json
  6650648 testdata/fixtures/npm/react.json
 11004480 testdata/fixtures/npm/types-node.json
 17655188 total
```

## Fixture content checks (drives downstream test assertions)

| Fixture | dist-tags.latest | versions | 17.x count | prerelease count |
|---|---|---|---|---|
| react.json | `"19.2.6"` | 2804 | 7 | 1021 |
| types-node.json | `"25.7.0"` | (large) | n/a | n/a |
| nonexistent.headers.json | n/a | n/a | n/a | n/a (status=404) |

**Yes**, the react packument contains prerelease versions (1021 keys matching `(alpha|beta|rc|canary|pre)`), so the 02-02 filter test for `IncPre` toggling has plenty of natural fodder without requiring a hand-crafted fixture.

## Verification

- `go test ./internal/testfixtures/... -race -count=1` — 6/6 pass
- `go test ./internal/registry/npm/ -run 'TestEscapeNPMPkg|TestPackumentURL_' -count=1` — 9/9 pass (TestEscapeNPMPkg with 5 subtests + 3 TestPackumentURL_* + 1 host-pin assertion across 5 inputs)
- `go test ./internal/cache/... -count=1` — 7/7 pass (comment-only edit, no behavior break)
- `go test ./... -count=1` — 48/48 pass across 11 packages
- `go vet ./...` — clean
- `internal/depcheck` test still passes (no new direct deps)

## Commits

| Task | Commit | Description |
|---|---|---|
| 1 | `58a3b84` | feat(02-01): add testfixtures replay+recording HTTP client |
| 2 | `ed3f6a0` | feat(02-01): add escapeNPMPkg + packumentURL scoped-pkg encoder |
| 3 | `c1a3d57` | feat(02-01): refresh cache.Key.Op comment + record initial NPM fixtures |

## Deviations from Plan

**One micro-deviation, comment-only, zero behavioural impact:**

**1. [Rule 3 — Blocking] Removed literal `url.PathEscape` token from `url.go` doc comment**

- **Found during:** Task 2 acceptance-criteria check
- **Issue:** The doc comment originally cited `url.PathEscape` and `url.QueryEscape` by name to explain why the hand-rolled escaper exists. The plan's acceptance criterion is `grep -c 'url.PathEscape' internal/registry/npm/url.go == 0` (treating that identifier as a forbidden anti-pattern token even in comments). Initial draft matched 2.
- **Fix:** Rephrased the comment to describe the stdlib path/query escaper behaviour without naming the symbol verbatim. The pedagogical content is preserved.
- **Files modified:** `internal/registry/npm/url.go`
- **Commit:** rolled into `ed3f6a0` (Task 2 commit) — the rewrite happened before commit.

No deviations from D-FIX-01, D-FIX-02, D-HTTP-01, or D-NPM-01.

## Threat Flags

None. All surface introduced by this plan is enumerated in the plan's `<threat_model>` (T-02-01 through T-02-06) and the mitigations are present in code:
- T-02-01 SSRF: `packumentURL` host hard-coded; verified by `TestPackumentURL_NeverChangesHost`.
- T-02-02 path traversal: `filepath.Clean` + prefix check in `FixtureClient`; verified by `TestFixtureClient_PathTraversalGuard`.
- T-02-05 oversized body DoS: `io.LimitReader(body, 16<<20)`.

## Self-Check: PASSED

Verified on disk:
- `internal/testfixtures/replay.go` — FOUND
- `internal/testfixtures/replay_test.go` — FOUND
- `internal/registry/npm/url.go` — FOUND
- `internal/registry/npm/url_test.go` — FOUND
- `internal/cache/key.go` — MODIFIED (comment refreshed)
- `testdata/fixtures/npm/react.json` — FOUND (6,650,648 bytes)
- `testdata/fixtures/npm/types-node.json` — FOUND (11,004,480 bytes)
- `testdata/fixtures/npm/nonexistent.json` — FOUND (0 bytes, intentional)
- `testdata/fixtures/npm/nonexistent.headers.json` — FOUND (60 bytes)
- `testdata/README.md` — FOUND

Verified in git log:
- Commit `58a3b84` — FOUND
- Commit `ed3f6a0` — FOUND
- Commit `c1a3d57` — FOUND
