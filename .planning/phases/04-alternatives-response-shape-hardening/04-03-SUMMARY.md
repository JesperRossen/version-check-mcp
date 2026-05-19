---
phase: "04-alternatives-response-shape-hardening"
plan: "03"
subsystem: "mcp-handler"
tags: [alternatives, miss-path, shape-audit, cross-registry]
dependency_graph:
  requires: [04-01, 04-02]
  provides: [miss-path-with-alternatives, cross-registry-shape-contract]
  affects: [internal/mcp/tools.go, internal/mcp/tools_shape_test.go]
tech_stack:
  patterns: [filter.NearestVersions, successResult-miss-shape, KindNotFound-intercept]
key_files:
  modified:
    - internal/mcp/tools.go
    - internal/mcp/server_test.go
  created:
    - internal/mcp/tools_shape_test.go
decisions:
  - "KindNotFound intercepted before toCallToolResult; routes to buildMissResponse"
  - "buildMissResponse calls Versions + Latest (both cache hits) then filter.NearestVersions"
  - "TestErrorEnvelopeShape/not_found removed — that case no longer produces an error envelope"
  - "New TestNotFoundProducesMissShape verifies D-MISS-01 contract"
metrics:
  duration: "~15 minutes"
  completed: "2026-05-19"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
---

# Phase 04 Plan 03: Alternatives Wire-up and Shape Audit Summary

**One-liner:** `validateRawHandler` miss path now calls `filter.NearestVersions` and returns `{exists:false, requested_version, latest_stable, alternatives:[{version,reason}]}`; cross-registry shape audit confirms identical key sets across all 5 registries.

## Tasks Completed

| # | Task | Commit | Key Files |
|---|------|--------|-----------|
| 1 | Wire alternatives into validateRawHandler miss path | `9c31430` | `internal/mcp/tools.go`, `internal/mcp/server_test.go` |
| 2 | Cross-registry response shape audit test | `7af423f` | `internal/mcp/tools_shape_test.go` |

## What Was Built

### Task 1 — Miss Path in `validateRawHandler`

`tools.go` now imports `internal/filter` and `internal/registry`. The `validateRawHandler` intercepts `KindNotFound` errors via `errors.As`:

```go
var e *errs.E
if errors.As(err, &e) && e.Kind == errs.KindNotFound {
    return s.buildMissResponse(ctx, reg, in)
}
```

`buildMissResponse` (new helper method on `*Server`):
1. Calls `reg.Versions()` - cache hit, no HTTP
2. Calls `reg.Latest()` to get `latestStable` - cache hit, no HTTP
3. Calls `filter.NearestVersions(versions, in.Version, vPrefixed, latestStable)`
4. Returns `successResult` with `{exists:false, requested_version, latest_stable, alternatives}`

v-prefix detection: `vPrefixed := reg.Name() == "gomod" || reg.Name() == "gh"`

### Task 2 — Cross-Registry Shape Audit Test

`tools_shape_test.go` contains `TestResponseShapeAudit` — 5 parallel top-level sub-tests (one per registry), each with `hit_keys` and `miss_keys` sub-tests (10 sub-tests total):

**Hit path asserts:** keys exactly `{exists, requested_version, source}`

**Miss path asserts:**
- Keys exactly `{alternatives, exists, latest_stable, requested_version}`
- `exists == false`
- `latest_stable` matches configured latest
- `requested_version` echoes the miss version
- Each `alternatives[]` entry has exactly `{version, reason}`
- `reason` in closed enum `{latest_stable, nearest_semver, latest_in_major}`
- `alternatives[0].reason == "latest_stable"` (D-NEAREST-04)
- v-prefix: gomod/gh versions start with `"v"`, npm/pypi/maven do not (UX-01)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated TestErrorEnvelopeShape for new KindNotFound routing**
- **Found during:** Task 1 verification
- **Issue:** Existing `TestErrorEnvelopeShape/not_found` expected an error envelope for `KindNotFound`, but the new code routes it to the success-shaped miss response
- **Fix:** Removed `not_found` case from `TestErrorEnvelopeShape`; added `TestNotFoundProducesMissShape` that verifies the correct D-MISS-01 behavior
- **Files modified:** `internal/mcp/server_test.go`
- **Commit:** `9c31430`

## Known Stubs

None. The miss path is fully wired end-to-end.

## Threat Flags

None. New code paths operate on registry data already in cache; no new network endpoints or trust boundaries introduced. Threat model T-04-03 (alternatives in response) accepted as planned.

## Self-Check

- [x] `internal/mcp/tools.go` modified with buildMissResponse
- [x] `internal/mcp/tools_shape_test.go` created (278 lines)
- [x] `internal/mcp/server_test.go` updated (TestErrorEnvelopeShape, TestNotFoundProducesMissShape)
- [x] Commit `9c31430` exists
- [x] Commit `7af423f` exists
- [x] `go test ./...` all pass
- [x] `go vet ./...` clean

## Self-Check: PASSED
