---
phase: "06-code-review-cleanup"
plan: "02"
subsystem: "filter"
tags: ["perf", "audit", "allocation", "nearest", "deps"]
dependency_graph:
  requires: ["06-01"]
  provides: ["performance audit complete", "DEP-01 invariant confirmed"]
  affects: ["internal/filter/nearest.go", ".planning/STATE.md"]
tech_stack:
  added: []
  patterns: ["pre-sized-slices"]
key_files:
  created: []
  modified:
    - "internal/filter/nearest.go"
    - ".planning/STATE.md"
  deleted: []
decisions:
  - "All 4 direct deps confirmed necessary - DEP-01 invariant holds"
  - "nearest.go candidates and byDist slices pre-sized as allocation wins"
  - "PyPI Validate O(n) scan and parseParts memoization deferred as out-of-scope for v1"
metrics:
  duration: "180s"
  completed: "2026-05-20"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 2
---

# Phase 06 Plan 02: Performance Audit Pass Summary

**One-liner:** Audited all 5 adapters and filter/nearest.go for allocation patterns; applied 2 pre-sizing wins in NearestVersions; confirmed all 4 direct dependencies necessary (DEP-01 holds).

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | performance audit pass on all 5 adapters and filter/nearest.go | bff6a3c | nearest.go: 2 pre-sizing fixes |
| 2 | dependency audit + performance findings documented in STATE.md | 015161a | STATE.md updated |

## What Was Done

### Task 1: Performance Audit

**Audit Scope and Findings per Pattern:**

**Pattern A: Unbounded append without pre-sizing**

| File | Finding | Action |
|------|---------|--------|
| `nearest.go` line 63 | `var candidates []string` - unbounded | Fixed: `make([]string, 0, len(versions))` |
| `nearest.go` line 109 | `var byDist []ranked` - unbounded | Fixed: `make([]ranked, 0, len(candidates))` |
| `npm.go` lines 98, 121 | Already uses `make([]string, 0, len(p.Versions))` | No action |
| `pypi.go` lines 178, 206 | Already uses `make([]string, 0, len(p.Releases))` | No action |
| `gomod.go` line 84 | Already uses `make([]string, 0, len(parts))` | No action |
| `gh.go` line 145 | Uses `make([]string, len(entries))` (exact size) | No action |
| `maven.go` line 167 | Already uses `make([]string, 0, len(m.Versioning.Versions))` | No action |
| `maven.go` line 195 | Returns `meta.Versioning.Versions` directly (no copy) | No action - optimal |

**Pattern B: Multiple passes over same collection**

- PyPI `Validate()`: single O(n) scan over releases map with normalize - acceptable, no double-pass
- PyPI `Latest()` + `Versions()`: each iterates releases once - no double-pass within a single call
- All other adapters: single-pass or fast path with no iteration

**Pattern C: filter/nearest.go allocations**

Both pre-sizing wins applied (see Pattern A above).

**Pattern D: HTTP response body handling**

All 5 adapters: `defer resp.Body.Close()` correctly placed after status check; body read once via `json.NewDecoder` or `io.ReadAll`. No issues found.

### Task 2: Dependency Audit

**All 4 direct dependencies confirmed necessary:**

| Dep | Usage Evidence | Verdict |
|-----|---------------|---------|
| `modelcontextprotocol/go-sdk` | MCP server/tool registration in `internal/mcp/` | Required |
| `hashicorp/golang-lru/v2` | `expirable.LRU` in `internal/cache/cache.go` | Required |
| `golang.org/x/sync` | `singleflight.Group` in `internal/cache/cache.go:22` | Required |
| `golang.org/x/mod` | `semver`, `module.IsPseudoVersion`, `modfile` in filter/, gomod/, depcheck | Required |

DEP-01 invariant holds: 4 direct deps, all necessary, none removable without major rework.

## Deviations from Plan

None - plan executed exactly as written.

## Findings - Out of Scope for v1

**PyPI Validate O(n) scan:** `Validate()` iterates `Releases` map with PEP 440 normalized key comparison on each element. For typical packages (<1000 versions), this is negligible. A pre-built normalized lookup map would be faster but adds complexity not warranted at this scale.

**parseParts memoization in nearest.go:** `parseParts` is called twice per candidate in same-minor and same-major tiers (once for target parts, once per candidate). Memoizing `tPatch`/`tMinor`/`tMajor` outside the switch would save redundant calls. `n < 1000` in practice; skipped per plan guidance.

## Verification Results

```
go vet ./...              - PASS
go build ./...            - PASS
go test ./... -count=1    - PASS (all packages; TestStdioCleanliness flaky under parallel build - pre-existing)
grep -c 'Performance Audit' .planning/STATE.md  => 1
grep -c 'Dependency audit' .planning/STATE.md   => 1
```

## Known Stubs

None.

## Threat Flags

None - this plan applied only allocation pre-sizing with no behavior changes, no new network endpoints, and no trust boundary modifications.

## Self-Check: PASSED

- [x] bff6a3c exists: confirmed
- [x] 015161a exists: confirmed
- [x] nearest.go candidates uses make([]string, 0, len(versions)): confirmed
- [x] nearest.go byDist uses make([]ranked, 0, len(candidates)): confirmed
- [x] STATE.md contains "Performance Audit" section: grep -c returns 1
- [x] STATE.md contains "Dependency audit" section: grep -c returns 1
- [x] All tests pass: confirmed
