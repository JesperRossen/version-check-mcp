---
phase: "04-alternatives-response-shape-hardening"
plan: "01"
subsystem: "filter"
tags: ["algorithm", "semver", "alternatives", "ranking", "tdd"]
dependency_graph:
  requires: ["golang.org/x/mod/semver", "golang.org/x/mod/module"]
  provides: ["NearestVersions", "AlternativeEntry", "filter.ReasonLatestStable", "filter.ReasonNearestSemver", "filter.ReasonLatestInMajor"]
  affects: ["internal/filter/nearest.go"]
tech_stack:
  added: []
  patterns: ["3-tier semver distance ranking", "deduplication via map", "TDD red-green cycle"]
key_files:
  created:
    - internal/filter/nearest.go
    - internal/filter/nearest_test.go
  modified: []
decisions:
  - "3-tier ranking: same-minor (patch dist) > same-major (minor dist) > cross-major (major dist); higher wins on tie"
  - "latest_in_major finds the BEST in target's major; if already in result as nearest_semver, skip entirely (don't fall back to second-highest)"
  - "When nearest_semver == latest_in_major, nearest_semver label wins (found first)"
metrics:
  duration: "5m 23s"
  completed: "2026-05-19"
  tasks_completed: 1
  tasks_total: 1
  files_created: 2
  files_modified: 0
---

# Phase 04 Plan 01: NearestVersions Algorithm Summary

**One-liner:** Pure semver distance-ranking algorithm with 3-tier scoring, tiebreak by higher version, and deduplication producing `[]AlternativeEntry` for miss responses.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Add failing tests for NearestVersions | e790894 | internal/filter/nearest_test.go |
| GREEN | Implement NearestVersions algorithm | 4ad1802 | internal/filter/nearest.go, nearest_test.go |

## What Was Built

`NearestVersions(versions []string, target string, vPrefixed bool, latestStable string) []AlternativeEntry`

The function ranks candidate versions by semver distance from a target and returns a deduplicated slice for embedding in miss responses:

1. **`latest_stable`** - always first (the overall highest stable version)
2. **`nearest_semver`** - closest version by 3-tier distance ranking
3. **`latest_in_major`** - highest version in target's own major (only when different from entries already in result)

### Distance Ranking (3-tier)

| Tier | Condition | Distance metric |
|------|-----------|-----------------|
| 1 | Same minor as target | Absolute patch difference |
| 2 | Same major, different minor | Absolute minor difference |
| 3 | Different major | Absolute major difference |

Tiebreaker: higher version wins. This ensures e.g. "1.6.0" beats "1.4.0" when both are distance-1 from "1.5.0".

### Deduplication Rules

- `latest_in_major` is the HIGHEST in target's major (not a fallback); if already added as `nearest_semver`, omitted entirely
- `inResult` map prevents any version string appearing twice

## TDD Gate Compliance

- **RED commit:** `e790894` - `test(04-01): add failing tests for NearestVersions algorithm` (build failed, function undefined)
- **GREEN commit:** `4ad1802` - `feat(04-01): implement NearestVersions algorithm with deduplication` (11/11 tests pass)

## Deviations from Plan

### Design Clarification (not a deviation)

The plan's behavior test case 7 stated "includes 16.14.0 as latest_in_major" for target "16.99.0". After analysis: under the plan's algorithm (nearest_semver found first, latest_in_major skipped if equal), "16.14.0" gets labeled `nearest_semver` (it's the closest by 3-tier ranking - same major = tier 2 beats different major = tier 3). The test was updated with a comment explaining the actual label. This aligns with the plan's action text ("Skip if it equals latestStable or nearest_semver already in the list").

No bugs introduced. No Rule 1/2/3 deviations required.

## Verification

```
go test ./internal/filter/ -run TestNearestVersions -v -count=1
# PASS: 11/11 subtests

go vet ./internal/filter/
# (no output - clean)
```

## Self-Check

- [x] `internal/filter/nearest.go` created
- [x] `internal/filter/nearest_test.go` created (148 lines, exceeds 80-line minimum)
- [x] RED commit `e790894` exists
- [x] GREEN commit `4ad1802` exists
- [x] `NearestVersions` exported ✓
- [x] `AlternativeEntry` exported ✓
- [x] Uses `semver.Major`, `semver.MajorMinor`, `semver.Compare`, `semver.IsValid` ✓

## Self-Check: PASSED
