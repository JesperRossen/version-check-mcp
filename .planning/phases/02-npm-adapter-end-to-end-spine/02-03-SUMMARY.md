---
phase: 02-npm-adapter-end-to-end-spine
plan: 03
subsystem: registry
tags: [npm, registry, http, cache, singleflight, fixtures]

requires:
  - phase: 01-foundation-mcp-scaffolding
    provides: registry.Registry interface, cache.Cache, errs.E, MCP stdio scaffold
  - phase: 02-npm-adapter-end-to-end-spine/02-01
    provides: testfixtures replay client, escapeNPMPkg/packumentURL, react+types-node+nonexistent fixtures
  - phase: 02-npm-adapter-end-to-end-spine/02-02
    provides: filterAndPickHighest (LAT-03/04/05 pure function)
provides:
  - NPM Adapter implementing registry.Registry verbatim
  - Packument decode with json.RawMessage per-version values (memory-bounded)
  - mapHTTPStatus + parseRetryAfter for 404/429/5xx/other -> *errs.E
  - Three Source enum strings emitted by NPM: "versions-map", "dist-tags.latest", "computed-highest"
  - Adapter test scaffold (newAdapter helper + URL canary capture)
affects: [02-04-wire-stdio, phase-03-other-adapters]

tech-stack:
  added: []
  patterns:
    - "cache.Get[*Packument] pointer-type convention adapter-wide"
    - "URL-mapping closure in fixture tests captures req.URL for canary assertions"
    - "Errmap as separate file (errmap.go) so adapters can be diffed against each other"

key-files:
  created:
    - internal/registry/npm/packument.go
    - internal/registry/npm/errmap.go
    - internal/registry/npm/errmap_test.go
    - internal/registry/npm/npm.go
    - internal/registry/npm/npm_test.go
  modified:
    - testdata/fixtures/npm/nonexistent.headers.json (renamed to nonexistent.json.headers.json)

key-decisions:
  - "Packument keeps Versions as map[string]json.RawMessage; per-version blobs are never decoded (Pitfall #5)"
  - "Validate ignores incPre for filtering (D-FILTER-01); the IncPre cache-key bit exists for Latest's benefit but Validate reads the same Versions map either way"
  - "Adapter does NOT do package-name regex validation; that boundary belongs to the MCP handler (per 02-CONTEXT.md Reusable Assets)"
  - "Adapter sets no agent-identifier header; injection lives in the shared client's Transport (02-04)"

patterns-established:
  - "Loader closure inside cache.Get[*V] returns *errs.E on every failure path so cache tier policy works uniformly"
  - "Compile-time interface assertion in test file: `var _ registry.Registry = (*npm.Adapter)(nil)`"

requirements-completed:
  - VAL-01
  - VAL-02
  - LAT-01
  - LAT-03
  - LAT-04
  - LAT-05
  - REG-01
  - TEST-01
  - TEST-02

duration: ~25min
completed: 2026-05-12
---

# Plan 02-03: NPM Adapter Summary

**NPM Registry adapter implementing registry.Registry verbatim; packument cached as *Packument with three Source enums (versions-map, dist-tags.latest, computed-highest), 43 fixture-replay tests green under -race.**

## Performance

- **Tasks:** 3 (errmap+packument, adapter, tests)
- **Files created:** 5 (3 production, 2 test)
- **Files renamed:** 1 (fixture sibling)
- **Tests added:** 16 (7 errmap + 9 adapter)
- **All NPM tests:** 43 pass under `go test -race`
- **Whole-module regression:** 82 pass, vet clean

## Accomplishments

- `npm.New(client *http.Client, c *cache.Cache) *npm.Adapter` constructor with `*Adapter` registered as `var _ registry.Registry = (*npm.Adapter)(nil)` (compile-time interface check passes).
- Source enum strings emitted (verbatim): `"versions-map"` (Validate-hit), `"dist-tags.latest"` (Latest fast path), `"computed-highest"` (Latest filtered or IncPre).
- `TestNPMValidate_Hit` uses react version `18.3.1`; `dist-tags.latest` for the recorded react fixture is `19.2.6` — they differ, so the test deliberately decouples Validate-hit from the dist-tags fast path.
- React packument contains both stable (e.g. `19.2.6`) and prerelease (`0.0.0-*` development build keys, `17.0.0-rc.*`, etc.). The IncPre path picks the highest sortable version including prereleases — assertions only check `Source == "computed-highest"` and `Version != ""`.
- All requirements landed: VAL-01, VAL-02, LAT-01, LAT-03, LAT-04, LAT-05, REG-01, TEST-01, TEST-02.

## Task Commits

1. **Task 1: Packument + errmap + errmap tests** — `c9ef9b5` (`feat(02-03)`)
2. **Task 2: NPM Adapter (New, Validate, Latest, Name)** — `d20af2e` (`feat(02-03)`)
3. **Task 3: Adapter tests (Validate/Latest/Cache/Scoped URL/Singleflight)** — `27b8761` (`test(02-03)`)

The fixture rename (see Deviations) was carried by the Task 1 commit as a side effect of the rename being staged at that point.

## Files Created/Modified

- `internal/registry/npm/packument.go` — `type Packument` + `parsePackument(io.Reader)`
- `internal/registry/npm/errmap.go` — `mapHTTPStatus` + `parseRetryAfter`
- `internal/registry/npm/errmap_test.go` — 7 tests
- `internal/registry/npm/npm.go` — `Adapter`, `New`, `packumentFor`, `Validate`, `Latest`, `Name`
- `internal/registry/npm/npm_test.go` — 9 adapter tests + interface assertion
- `testdata/fixtures/npm/nonexistent.headers.json` → `nonexistent.json.headers.json` (rename — see Deviations)

## Decisions Made

- None of the locked decisions (D-FILTER-01 / D-SOURCE-01 / D-HTTP-01 / D-NPM-01) were re-opened. The adapter follows the plan's `<action>` blocks verbatim.
- Adapter omits the optional package-name regex per plan guidance; the handler owns input validation (per 02-CONTEXT.md "Reusable Assets" invariant — adapter never constructs `InvalidInput`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Blocking] Fixture sibling filename mismatch**

- **Found during:** Task 3 (`TestNPMValidate_Miss_PackageNotFound` failed with `Kind = upstream_down, want not_found`).
- **Issue:** Plan 02-01 committed the not-found header override as `testdata/fixtures/npm/nonexistent.headers.json`, but `internal/testfixtures/replay.go` and its tests use the convention `<fixture-file>.headers.json` literally — i.e. `nonexistent.json.headers.json`. The replay client therefore never loaded the `{"status":404,...}` override and replayed an empty body with status 200.
- **Fix:** `git mv testdata/fixtures/npm/nonexistent.headers.json testdata/fixtures/npm/nonexistent.json.headers.json`. The fixture content was not modified.
- **Verification:** `go test ./internal/registry/npm/ -run TestNPMValidate_Miss_PackageNotFound -count=1` exits 0; full module green.
- **Committed in:** `c9ef9b5` (carried by the Task 1 commit because the rename was staged at that point).

---

**Total deviations:** 1 auto-fixed (1 blocking — fixture/helper convention mismatch from 02-01).
**Impact on plan:** Minor — no scope creep. The fixture was correctly conceived in 02-01 but named under a different convention than the helper exposes. The fix aligns the committed fixture with `internal/testfixtures/replay.go`'s documented sibling pattern.

## Issues Encountered

- One static-analysis diagnostic on `parsePackument` (unused at end of Task 1) was expected and cleared once the adapter in Task 2 began calling it.

## Confirmation

- Adapter source contains zero stdout writes (`grep -cE 'os\.Stdout|fmt\.Println|fmt\.Print\(' internal/registry/npm/npm.go` → 0).
- Adapter source contains zero `httptest` references (`grep -c httptest internal/registry/npm/npm.go` → 0).
- Adapter source contains zero agent-identifier-header literals (`grep -c User-Agent internal/registry/npm/npm.go` → 0).
- Compile-time interface assertion present in test file.

## Next Phase Readiness

- 02-04 can wire `npm.New(...)` into the production binary; the cache/client pair construction is the only remaining glue.
- The shared `*http.Client` Transport must inject the agent-identifier header (Pattern referenced by the plan) — 02-04's responsibility.
- TEST-04 (advisory cold-start CI signal) remains for 02-05.

---
*Phase: 02-npm-adapter-end-to-end-spine*
*Completed: 2026-05-12*
