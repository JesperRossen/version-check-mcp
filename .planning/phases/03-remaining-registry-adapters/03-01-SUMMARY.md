---
phase: 03-remaining-registry-adapters
plan: "01"
subsystem: shared-utilities
tags: [filter, httperr, semver, pep440, go-modules, shared-packages]
dependency_graph:
  requires: []
  provides:
    - internal/filter.FilterAndPickHighest
    - internal/filter.VPrefix
    - internal/filter.StripV
    - internal/filter.PEP440Normalize
    - internal/httperr.MapHTTPStatus
    - internal/httperr.ParseRetryAfter
  affects:
    - internal/registry/pypi (wave 2 — can now import filter + httperr)
    - internal/registry/gomod (wave 2 — can now import filter + httperr)
    - internal/registry/gh (wave 2 — can now import filter + httperr)
    - internal/registry/maven (wave 2 — can now import filter + httperr)
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN per task
    - Table-driven tests with t.Parallel()
    - golang.org/x/mod/semver for semver comparison
    - golang.org/x/mod/module for IsPseudoVersion (D-GOMOD-03)
    - regexp-based PEP 440 normalization with longest-match-first alternation
key_files:
  created:
    - internal/filter/filter.go
    - internal/filter/filter_test.go
    - internal/filter/pep440.go
    - internal/filter/pep440_test.go
    - internal/httperr/httperr.go
    - internal/httperr/httperr_test.go
  modified: []
decisions:
  - "regex alternation order: longest alternatives first (alpha|beta|preview|pre|rc|a|b|c) to prevent alpha matching a+lpha"
  - "D-GOMOD-03: pseudo-version exclusion placed after prerelease check, only when vPrefixed=true"
  - "httperr.ParseRetryAfter exported with capital P so GitHub adapter can reuse it directly"
  - "not-found message uses fmt.Sprintf with registryName rather than format string to keep errs.NotFound signature clean"
metrics:
  duration_seconds: 236
  completed_date: "2026-05-13"
  tasks_completed: 3
  tasks_total: 3
  files_created: 6
  files_modified: 0
  tests_added: 54
---

# Phase 3 Plan 1: Shared Utilities Promotion Summary

**One-liner:** Promoted NPM adapter's filter and httperr utilities into standalone packages with ecosystem-aware parameters (vPrefixed, registryName, PEP 440 normalization).

## What Was Built

### internal/filter/filter.go

Exported API:

```go
func FilterAndPickHighest(versions []string, vPrefixed bool, incPre bool, major, minor *int) (string, bool)
func VPrefix(v string) string
func StripV(v string) string
```

Key changes from `internal/registry/npm/filter.go`:
- Added `vPrefixed bool` parameter: when false (NPM/PyPI/Maven), the function adds "v" prefix for semver ops and strips it on return. When true (Go/GH), versions are used verbatim.
- Added `module.IsPseudoVersion` exclusion when `incPre=false && vPrefixed=true` (D-GOMOD-03).
- Added `VPrefix` and `StripV` helper exports.

### internal/filter/pep440.go

Exported API:

```go
func PEP440Normalize(v string) string
```

Implements PEP 440 normalization rules 1-8:
1. TrimSpace
2. Lowercase
3. Strip leading "v"
4. Alias pre-release labels (alpha→a, beta→b, c/preview/pre→rc)
5. Remove separator before pre-release label (dash/underscore/dot)
6. Ensure dot separator before post/dev
7. Append "0" when pre-release label has no trailing number
8. Strip leading zeros in numeric segments

**Key implementation note:** Regex alternation order matters. The pattern is `alpha|beta|preview|pre|rc|a|b|c` (longest first) so that "alpha" matches before "a" prevents "a0lpha1" corruption.

### internal/httperr/httperr.go

Exported API:

```go
func MapHTTPStatus(resp *http.Response, pkg, registryName string) error
func ParseRetryAfter(h string) time.Time
```

Key changes from `internal/registry/npm/errmap.go`:
- Added `registryName string` parameter to `MapHTTPStatus`.
- Not-found message uses `registryName` (e.g. "pypi package not found").
- All `errs.*` calls include `"registry", registryName` in details.
- `parseRetryAfter` promoted to exported `ParseRetryAfter` for GitHub adapter reuse.
- T-03-01 mitigated: malformed Retry-After falls back to 30s, never propagated raw into error.

## Test Coverage

| Package | Test function | Cases |
|---------|--------------|-------|
| filter | TestFilterAndPickHighest | 12 |
| filter | TestVPrefix | 3 |
| filter | TestStripV | 3 |
| filter | TestPEP440Normalize | 29 |
| httperr | TestMapHTTPStatus_404_NPM | 1 |
| httperr | TestMapHTTPStatus_404_PyPI | 1 |
| httperr | TestMapHTTPStatus_429_NumericRetryAfter | 1 |
| httperr | TestMapHTTPStatus_429_HTTPDateRetryAfter | 1 |
| httperr | TestMapHTTPStatus_500 | 1 |
| httperr | TestMapHTTPStatus_502_503 | 1 |
| httperr | TestMapHTTPStatus_UnexpectedNon2xx | 1 |
| httperr | TestParseRetryAfter_Empty | 1 |
| httperr | TestParseRetryAfter_Numeric | 1 |
| httperr | TestParseRetryAfter_HTTPDate | 1 |
| **Total** | | **54** |

## Deviations from Plan

None — plan executed exactly as written.

## Threat Model Compliance

| Threat ID | Status |
|-----------|--------|
| T-03-01 (ParseRetryAfter malformed input) | Mitigated — fallback to 30s, never raw propagation |
| T-03-02 (httperr info disclosure) | Accepted — status+registry name is intentional |
| T-03-03 (PEP440Normalize input tampering) | Mitigated — regex anchored to known labels, no eval/exec |

## Known Stubs

None.

## Threat Flags

None — no new network endpoints, auth paths, or schema changes introduced. Both packages are pure utility with no I/O.

## Self-Check: PASSED

Files exist:
- internal/filter/filter.go: FOUND
- internal/filter/filter_test.go: FOUND
- internal/filter/pep440.go: FOUND
- internal/filter/pep440_test.go: FOUND
- internal/httperr/httperr.go: FOUND
- internal/httperr/httperr_test.go: FOUND

Commits:
- 52ff329: feat(03-01): promote filter package with vPrefixed param and pseudo-version exclusion
- 18ae04a: feat(03-01): add PEP 440 normalization helper to filter package
- 0f3ec84: feat(03-01): promote httperr package with registryName param and ParseRetryAfter export
