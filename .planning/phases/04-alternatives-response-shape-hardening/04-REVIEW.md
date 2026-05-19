---
phase: 04
status: findings
depth: standard
reviewed_files: 11
critical: 0
major: 4
minor: 3
---

# Phase 04: Code Review Report

**Reviewed:** 2026-05-19T00:00:00Z
**Depth:** standard
**Files Reviewed:** 11
**Status:** findings

## Summary

Phase 04 introduces the `NearestVersions` algorithm in `internal/filter/nearest.go`, wires it into the miss-path in `internal/mcp/tools.go`, adds a `Versions` method to the `Registry` interface, and implements it across all five adapters. The shape audit test in `tools_shape_test.go` exercises the cross-registry invariants.

The core algorithm is well-structured and the test coverage for the happy path is solid. However, several correctness issues exist: `NearestVersions` can silently produce wrong answers when the target version is invalid-semver but non-empty, the `latest_in_major` candidates pool incorrectly excludes `latestStable` from consideration which means the highest in the target major can be silently dropped, the PyPI `Versions` method ignores yanked releases while the filter does not, and the wrong argument key (`"package"` vs `"pkg"`) is used in the shape-audit test, meaning the test never exercises the package it claims to validate.

---

## Critical Issues

_None._

---

## Major Issues

### MJ-01: Shape-audit test sends `"package"` key; handler expects `"pkg"` - tests always pass vacuously

**File:** `internal/mcp/tools_shape_test.go:138-140` and `172-174`

**Issue:** Both the hit and miss `CallTool` invocations pass `"package": tc.pkg` in the `Arguments` map. The `ValidateInput` struct field is tagged `json:"pkg"`, so `decodeArgs` silently ignores the `"package"` key and leaves `in.Pkg` as an empty string `""`. The handler then calls `reg.Validate(ctx, "", ...)` — and the Fake does not check the pkg argument, so it always returns `ValidateResult{Exists:true}` / `ValidateErr` regardless of `pkg`. This means the test proves that the shape is correct when `pkg=""`, not that the real handler logic is wired correctly for the configured packages.

**Fix:**
```go
// Change:
"package": tc.pkg,
// To:
"pkg": tc.pkg,
```
Apply to both the hit sub-test (line ~138) and the miss sub-test (line ~172).

---

### MJ-02: `NearestVersions` silently returns only `latest_stable` when `target` is invalid semver and `latestStable` is empty - but the real gap is: non-semver target with a valid `latestStable` skips all distance ranking, even if `versions` is non-empty and useful

**File:** `internal/filter/nearest.go:53-56`

**Issue:** If `normTarget` is not valid semver (e.g., a Go pseudo-version or a PEP 440 epoch version like `"1!2.0.0"`), the function returns only `[{latestStable, "latest_stable"}]` and skips distance ranking entirely. This is documented in the `parseParts` contract but is not reflected in the function's doc comment, which says "Returns nil when versions is empty or latestStable is empty" - omitting the silent short-circuit for invalid targets. More importantly, when `NearestVersions` is called from `buildMissResponse` with a target like `"1!2.0.0"` (PyPI epoch) or a pseudo-version (Go modules), the caller has already confirmed the version does not exist; an agent asking for an invalid epoch version will receive only `latestStable` with no other context. This is a user-experience correctness issue (UX-01): the caller could reasonably expect the full 3-entry result.

The missing test cases are: (a) invalid-semver target with otherwise rankable versions, (b) PyPI epoch versions.

**Fix:** Document the limitation explicitly in the function's doc comment and add a test case. If broader coverage is desired, call `filter.FilterAndPickHighest` directly for the `nearest_semver` slot when `normTarget` is invalid.

---

### MJ-03: `latest_in_major` pool excludes `latestStable` from consideration, causing the wrong version to appear (or no entry) when the highest in the target major IS `latestStable`

**File:** `internal/filter/nearest.go:163-187`

**Issue:** The `latest_in_major` loop skips `v == normLatest` (line 165). Consider a scenario where `target="v1.9.0"` (does not exist), `versions=["v1.8.0","v2.0.0"]`, and `latestStable="v2.0.0"`. The loop correctly skips `v2.0.0` (different major), so `highestInMajor=""` and no `latest_in_major` entry is emitted. So far correct. But consider `target="v2.9.0"` (does not exist), `versions=["v2.0.0","v2.8.0"]`, `latestStable="v2.8.0"`. The loop skips `v2.8.0` because `v == normLatest`, so `highestInMajor="v2.0.0"`. But `v2.0.0` is already either the nearest_semver or will be added as `latest_in_major` - and `v2.8.0` (the true latest in the major) is silently omitted. The agent receives a stale `v2.0.0` as `latest_in_major` when `v2.8.0` exists and is in the same major.

The intent is to avoid emitting `latestStable` twice, but the skip should be applied at emission time (via `inResult` check), not during candidate selection. The bug means the `latest_in_major` candidate pool is wrong whenever `latestStable` is in the target's major.

**Fix:**
```go
// Remove the normLatest guard from the loop:
for _, v := range candidates {
    // Don't skip normLatest here - the inResult check at emit time handles dedup
    if semver.Major(v) != targetMajor {
        continue
    }
    if highestInMajor == "" || semver.Compare(v, highestInMajor) > 0 {
        highestInMajor = v
    }
}
// The existing inResult[highestInMajor] check at line 178 already prevents
// emitting latestStable a second time.
```

---

### MJ-04: PyPI `Versions` returns yanked releases; `NearestVersions` will include yanked versions as alternatives

**File:** `internal/registry/pypi/pypi.go:200-209`

**Issue:** `Versions` returns all keys from `proj.Releases` without filtering out yanked releases (PEP 592). When a PyPI package's `Versions` list is passed to `NearestVersions` and `FilterAndPickHighest`, yanked releases can surface as `nearest_semver` or `latest_in_major` alternatives. The `Latest` method already correctly excludes yanked releases (line 180-183), but `Versions` does not. An agent receiving a yanked version as an alternative and subsequently installing it would get a broken package.

**Fix:**
```go
func (a *Adapter) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
    proj, err := a.projectFor(ctx, pkg, incPre)
    if err != nil {
        return nil, err
    }
    keys := make([]string, 0, len(proj.Releases))
    for k, files := range proj.Releases {
        if isYanked(files) {
            continue // exclude yanked per PEP 592 - D-PYPI-01
        }
        keys = append(keys, k)
    }
    return keys, nil
}
```

---

## Minor Issues

### MN-01: `NearestVersions` capped at 5 per doc comment but no cap is enforced

**File:** `internal/filter/nearest.go:25-27`

**Issue:** The doc comment states "at most 5 alternative versions" but the function has no enforced cap. Currently the function can return at most 3 entries (latestStable + nearest_semver + latest_in_major), so this is not a live bug, but the documented contract and the implementation disagree. If a future contributor adds more sources, the cap will silently be exceeded.

**Fix:** Either add a `result = result[:min(5, len(result))]` guard or update the doc comment to accurately reflect the current maximum of 3.

---

### MN-02: `buildMissResponse` silently swallows `Latest` errors; an empty `latestStable` causes `NearestVersions` to return `nil`

**File:** `internal/mcp/tools.go:131-135`

**Issue:** If `reg.Latest(...)` returns an error, `latestStable` is set to `""` and the error is discarded. `NearestVersions` then returns `nil` (per its early-exit at line 44). The response shape will contain `"alternatives": null`, which fails the shape-audit assertion at line 209 (`if altsRaw == nil { t.Fatal(...) }`). This means the production code can silently produce a response that fails the test's invariant contract when the registry is temporarily unavailable.

**Fix:** Propagate the `Latest` error (or at minimum log it via `slog`) and return a degraded-but-valid response shape:
```go
latestRes, err := reg.Latest(ctx, in.Pkg, false, nil, nil)
latestStable := ""
if err != nil {
    slog.WarnContext(ctx, "latest lookup failed during miss-path", "pkg", in.Pkg, "err", err)
    // latestStable stays "", alts will be nil — use empty slice to maintain shape
}
if err == nil {
    latestStable = latestRes.Version
}
alts := filter.NearestVersions(versions, in.Version, vPrefixed, latestStable)
if alts == nil {
    alts = []filter.AlternativeEntry{} // maintain non-nil shape guarantee
}
```

---

### MN-03: `parseParts` recomputes `semver.Major` / `semver.MajorMinor` redundantly inside the ranking loop

**File:** `internal/filter/nearest.go:107-135`

**Issue:** For every candidate in `byDist`, `parseParts(normTarget)` is called inside the `switch` cases, re-parsing the target on every iteration. The target does not change between iterations. This is a minor efficiency issue (not a correctness bug), but it makes the code harder to read.

**Fix:** Extract target parts before the loop:
```go
tMaj, tMin, tPat := parseParts(normTarget)
// ... then use tMaj, tMin, tPat in the switch cases
```

---

_Reviewed: 2026-05-19T00:00:00Z_
_Reviewer: OpenCode (gsd-code-reviewer)_
_Depth: standard_
