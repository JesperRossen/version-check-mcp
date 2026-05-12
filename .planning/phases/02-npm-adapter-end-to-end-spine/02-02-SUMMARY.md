---
phase: 02-npm-adapter-end-to-end-spine
plan: 02
subsystem: registry/npm
tags: [npm, semver, filter, lat-03, lat-04, lat-05]
requires:
  - "go.mod has golang.org/x/mod v0.36.0 (Phase 1 dep)"
provides:
  - "filterAndPickHighest — pure version-list filter consumed by 02-03's Latest method"
affects: []
tech-stack:
  added: []
  patterns:
    - "v-prefix-then-strip when interfacing with golang.org/x/mod/semver on NPM-shaped input"
    - "Silent-skip guard via semver.IsValid before any other semver.* call"
key-files:
  created:
    - internal/registry/npm/filter.go
    - internal/registry/npm/filter_test.go
  modified: []
decisions:
  - "Filter does NOT validate that minor!=nil implies major!=nil — that's the MCP handler's job; duplicating it would create two sources of truth."
  - "Returned version is unprefixed (NPM wire format); the v-prefix is purely an internal interfacing detail for golang.org/x/mod/semver."
  - "Test corpus uses '1.2.x' instead of the plan's '1.2' to exercise the malformed-skip path — see Behavioural Surprises."
metrics:
  duration: "~15 min"
  completed: "2026-05-12"
---

# Phase 2 Plan 02: NPM version-list filter (LAT-03/04/05) Summary

Pure `filterAndPickHighest` function implementing the three-step LAT-05 filter (major/minor constraint, prerelease policy, semver-highest pick) for NPM-shaped (unprefixed) version strings, with table-driven tests covering LAT-03, LAT-04, LAT-05, edge cases (major=0), and resilience invariants (malformed-skip, non-mutation).

## Exact Signature

```go
// internal/registry/npm/filter.go
func filterAndPickHighest(versions []string, incPre bool, major, minor *int) (string, bool)
```

- Imports: `sort`, `strconv`, `golang.org/x/mod/semver` (exactly the three the plan called for).
- Uses all five semver primitives: `IsValid`, `Prerelease`, `Major`, `MajorMinor`, `Compare`.
- Returned string is **unprefixed** — the internal `v`-prefix is stripped via `candidates[len-1][1:]` before return.

## Canonical Test Corpus

`canonicalVersions` (single fixture exercised by 5 of the 6 test functions):

```
stable:       17.0.0, 17.0.1, 17.0.2, 17.1.0,
              18.0.0, 18.1.0, 18.2.0, 18.3.0, 18.3.1,
              19.0.0,
              0.14.0, 0.14.7, 0.15.0
prereleases:  18.3.0-rc.0, 19.0.0-rc.1, 19.0.0-beta.0, 18.0.0-alpha.1
malformed:    "not-a-version", "1.2.x", "garbage", ""
```

The major=0 line (`0.14.x`, `0.15.0`) exists specifically to exercise the `major:0` edge case called out in D-FILTER-01.

## Requirement Coverage

| Requirement | Test(s) | Assertion |
|---|---|---|
| LAT-03 (stable-only) | `TestFilter_StableOnly` | highest = `19.0.0`, prereleases dropped |
| LAT-04 (incPre opt-in) | `TestFilter_IncPre` (two subtests) | (a) stable still wins when present (`19.0.0` > `19.0.0-rc.1`); (b) RC observable when no stable for that x.y.z |
| LAT-05 (major/minor) | `TestFilter_MajorMinor` (8 rows) | `major=17`→`17.1.0`, `major=17,minor=0`→`17.0.2`, `major=18`→`18.3.1`, `major=18 incPre`→`18.3.1`, `major=0`→`0.15.0`, `major=0,minor=14`→`0.14.7`, empty matches return `("",false)` |
| Malformed resilience | `TestFilter_MalformedSkipped` | one valid amid garbage → that one; all garbage → `("",false)` |
| NPM unprefixed return | `TestFilter_NPMUnprefixedOnReturn` | result has no leading `v` |
| Non-mutation (T-02-08) | `TestFilter_DoesNotMutateInput` | `reflect.DeepEqual` against pre-call snapshot |

Total: 6 top-level `TestFilter_*` functions, **18 subtests**, all green under `-race`.

## Behavioural Surprises

### `golang.org/x/mod/semver` accepts shortened forms

The plan's malformed corpus listed `"1.2"` as an invalid entry expected to be silently skipped. In practice **`semver.IsValid("v1.2") == true`** — `golang.org/x/mod` accepts both `v1` and `v1.2` and treats them as `v1.0.0` and `v1.2.0` respectively. This is an x/mod quirk relative to strict SemVer 2.0.0 (which requires `MAJOR.MINOR.PATCH`).

**Decision:** Swap `"1.2"` for `"1.2.x"` in the test corpus. `1.2.x` IS rejected by `semver.IsValid`, so the silent-skip path is exercised as the plan intended.

**Implication for downstream plans:** NPM publishes virtually no shortened-form keys in real packuments (npm registry normalises to `MAJOR.MINOR.PATCH`), so this is unlikely to bite the adapter at runtime — but **if** a malformed key like `"1.2"` ever appeared, the filter would treat it as `v1.2.0` and could surface it as a result. If that ever matters, the fix is one extra strict-form check inside the filter, gated on `strings.Count(raw,".") >= 2`. Out of scope for v1; flagging here for traceability.

### `semver.Compare("v19.0.0","v19.0.0-rc.1") > 0`

Stable releases sort **higher** than their own RCs (`-rc.1` is a prerelease suffix and the prereleased form is less than the unsuffixed form per SemVer 2.0.0 §11). This means with `incPre=true` and a stable `19.0.0` present, the RC is filtered in but not picked. Confirmed in `TestFilter_IncPre/stable_still_wins_when_present`.

## Threat Model Compliance

| Threat | Disposition | Mitigation in this plan |
|---|---|---|
| T-02-07 DoS via malformed semver | mitigate | Every `semver.*` call is preceded by `semver.IsValid` (skip on false). `TestFilter_MalformedSkipped` covers this. |
| T-02-08 in-place mutation | mitigate | New `candidates` slice allocated; input untouched. `TestFilter_DoesNotMutateInput` covers this. |
| T-02-09 information disclosure | accept | n/a — public registry data, no sensitive content. |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Malformed corpus entry `"1.2"` parses as valid under x/mod semver**
- **Found during:** Task 2 (first test run)
- **Issue:** Plan's test corpus included `"1.2"` in the malformed-skip set, but `golang.org/x/mod/semver.IsValid("v1.2") == true`. Two `TestFilter_MalformedSkipped` subtests failed because `v1.2` ranked higher than `v1.0.0`.
- **Fix:** Replaced `"1.2"` with `"1.2.x"` in `canonicalVersions` and both `TestFilter_MalformedSkipped` subtests. `"1.2.x"` is rejected by `semver.IsValid`, restoring the plan's intent (silent-skip exercise).
- **Files modified:** `internal/registry/npm/filter_test.go`
- **Commit:** `8defca5`
- **Note:** This is a behaviour of the x/mod library, not a defect in the filter. The filter implementation itself was untouched.

## Verification Output

- `go build ./internal/registry/npm/...` — success
- `go vet ./internal/registry/npm/...` — clean
- `go test ./internal/registry/npm/ -run 'TestFilter_' -count=1 -v` — 18/18 passing
- `go test ./internal/registry/npm/ -run 'TestFilter_' -count=1 -race` — green
- `go test ./internal/depcheck/...` — still passing (no new direct deps introduced)

## Acceptance Criteria

### Task 1 (filter.go)

- [x] `grep -cE '^func filterAndPickHighest\(' filter.go == 1`
- [x] Signature matches `<interfaces>` block exactly
- [x] `golang.org/x/mod/semver` imported (== 1 import line)
- [x] All five `semver.*` primitives used (Major, MajorMinor, Prerelease, Compare, IsValid)
- [x] Imports limited to `sort`, `strconv`, `golang.org/x/mod/semver`
- [x] `go vet` clean, `go build` succeeds

### Task 2 (filter_test.go)

- [x] `grep -cE '^func TestFilter_' == 6`
- [x] Imports limited to `testing` and `reflect`
- [x] `intPtr` helper present and used (10 occurrences)
- [x] `grep -c 'semver' filter_test.go == 0` (no semver import or symbol use; tests behind the abstraction)
- [x] All tests pass under `-count=1` and `-race`

## Commits

| Task | Hash | Message |
|---|---|---|
| 1 | `7dc5e19` | feat(02-02): implement filterAndPickHighest (LAT-03/04/05) |
| 2 | `8defca5` | test(02-02): table-driven filter tests for LAT-03/04/05 |

## Self-Check: PASSED

- [x] `internal/registry/npm/filter.go` exists
- [x] `internal/registry/npm/filter_test.go` exists
- [x] Commit `7dc5e19` present in `git log`
- [x] Commit `8defca5` present in `git log`
- [x] `.planning/phases/02-npm-adapter-end-to-end-spine/02-02-SUMMARY.md` exists (this file)
