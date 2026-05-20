---
phase: "06-code-review-cleanup"
plan: "01"
subsystem: "npm-adapter"
tags: ["refactor", "cleanup", "npm", "httperr", "filter"]
dependency_graph:
  requires: []
  provides: ["npm-adapter uses shared httperr + filter packages"]
  affects: ["internal/registry/npm"]
tech_stack:
  added: []
  patterns: ["shared-httperr-pattern", "shared-filter-pattern"]
key_files:
  created: []
  modified:
    - "internal/registry/npm/npm.go"
    - ".planning/STATE.md"
  deleted:
    - "internal/registry/npm/errmap.go"
    - "internal/registry/npm/errmap_test.go"
    - "internal/registry/npm/filter.go"
    - "internal/registry/npm/filter_test.go"
decisions:
  - "npm adapter now uses httperr.MapHTTPStatus (same as pypi, gomod, gh, maven)"
  - "npm adapter now uses filter.FilterAndPickHighest with vPrefixed=false (same as pypi, maven)"
  - "All 4 STATE.md open todos confirmed resolved"
metrics:
  duration: "168s"
  completed: "2026-05-20"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 6
---

# Phase 06 Plan 01: npm adapter migration to shared httperr + filter Summary

**One-liner:** Deleted npm's private errmap.go + filter.go and wired npm.go to use httperr.MapHTTPStatus("npm") and filter.FilterAndPickHighest(vPrefixed=false), matching the pattern already used by pypi, gomod, gh, and maven adapters.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | migrate npm to shared httperr + shared filter, delete private files | a98d602 | npm.go modified; errmap.go, errmap_test.go, filter.go, filter_test.go deleted |
| 2 | close STATE.md open todos and fix PROJECT.md Go version | 8f57261 | STATE.md updated |

## What Was Done

### Task 1: npm adapter migration

`internal/registry/npm/npm.go` had two private helper call sites:
- `mapHTTPStatus(resp, pkg)` - private function in errmap.go
- `filterAndPickHighest(keys, incPre, major, minor)` - private function in filter.go

Both were replaced with their shared package equivalents:
- `httperr.MapHTTPStatus(resp, pkg, "npm")` - adds `"npm"` as registryName
- `filter.FilterAndPickHighest(keys, false, incPre, major, minor)` - `false` = vPrefixed=false (npm versions are unprefixed)

The 4 private files were deleted. The npm adapter now follows the exact same pattern as pypi, gomod, gh, and maven - all use the shared `internal/httperr` and `internal/filter` packages.

### Task 2: STATE.md todos resolved

All 4 open todos confirmed resolved:
- **PROJECT.md Go version**: Already says `Go 1.25+` in Constraints - no change needed
- **MJ-01**: `tools_shape_test.go` already uses `"pkg"` key (confirmed via grep; no `"package"` key found)
- **MJ-03**: `latest_in_major` inResult dedup is correct; all 13 TestNearestVersions sub-tests pass
- **MJ-04**: PyPI `Versions()` already filters yanked releases; TestLatest_YankedVersionSkipped passes

## Deviations from Plan

**1. [Rule 1 - Minor observation] PROJECT.md Go version was already correct**
- The plan said to find `Go 1.22` and change to `Go 1.25` in PROJECT.md
- Found: PROJECT.md Constraints section already read `Go 1.25+` (updated in a prior phase)
- The Key Decisions table has historical text "Floor raised from 1.22 to 1.25" - intentionally preserved as historical context
- Action: No change to PROJECT.md needed; STATE.md todo marked as "already correct"

## Verification Results

```
go vet ./...          - PASS (no issues)
go build ./...        - PASS
go test ./... -count=1 - PASS (14 packages, all ok)

grep -c 'httperr.MapHTTPStatus' internal/registry/npm/npm.go  => 1
grep -c 'filter.FilterAndPickHighest' internal/registry/npm/npm.go => 1
test ! -f internal/registry/npm/errmap.go  => errmap.go deleted OK
test ! -f internal/registry/npm/filter.go  => filter.go deleted OK
grep 'Go 1.25' .planning/PROJECT.md => found (already correct)
```

## Known Stubs

None.

## Threat Flags

None - this plan modified only internal code paths with no new network endpoints or trust boundaries.

## Self-Check: PASSED

- [x] a98d602 exists: `git log --oneline | grep a98d602` - confirmed
- [x] 8f57261 exists: `git log --oneline | grep 8f57261` - confirmed
- [x] internal/registry/npm/errmap.go does not exist - confirmed
- [x] internal/registry/npm/filter.go does not exist - confirmed
- [x] internal/registry/npm/npm.go uses httperr.MapHTTPStatus - confirmed
- [x] internal/registry/npm/npm.go uses filter.FilterAndPickHighest - confirmed
- [x] All tests pass - confirmed
