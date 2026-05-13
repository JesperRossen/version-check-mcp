---
phase: 03-remaining-registry-adapters
plan: "04"
subsystem: registry-gh
tags: [github-actions, adapter, pagination, rate-limit, non-semver, tdd]
dependency_graph:
  requires:
    - internal/filter.FilterAndPickHighest (plan 03-01)
    - internal/httperr.MapHTTPStatus (plan 03-01)
  provides:
    - internal/registry/gh.Adapter
    - internal/registry/gh.New
  affects:
    - cmd/version-check-mcp/main.go (will replace fake.New stub in wave 3)
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN per task
    - Two-page pagination capped at 200 tags (D-GH-01)
    - GitHub-specific 403+X-RateLimit-Remaining:0 override before shared httperr.MapHTTPStatus
    - /releases/latest as latest-stable hint (fast path) with tags filter as fallback
    - Exact string match validate (D-GH-04 — "v6" is valid even though not semver-valid)
key_files:
  created:
    - testdata/fixtures/gh/checkout-tags-p1.json
    - testdata/fixtures/gh/checkout-tags-p2.json
    - testdata/fixtures/gh/checkout-releases-latest.json
    - testdata/fixtures/gh/checkout-tags-p1-short.json
    - testdata/fixtures/gh/rate-limited.json
    - testdata/fixtures/gh/rate-limited.json.headers.json
    - testdata/fixtures/gh/nonexistent.json
    - testdata/fixtures/gh/nonexistent.json.headers.json
    - internal/registry/gh/url.go
    - internal/registry/gh/url_test.go
    - internal/registry/gh/gh.go
    - internal/registry/gh/gh_test.go
  modified: []
decisions:
  - "D-GH-04 validate: exact string match in tags list, not semver validation — v6 returns exists:true"
  - "Two Op cache keys: tags (combined p1+p2) and release-latest (tag_name from /releases/latest)"
  - "Latest fast path: /releases/latest used only when incPre=false AND major==nil AND minor==nil"
  - "releaseLatestFor not-found silently falls through; other errors (rate-limit, upstream-down) bubble up"
  - "mapErr: checks 403+X-RateLimit-Remaining==0 BEFORE delegating to httperr.MapHTTPStatus"
  - "reset_at stored as int64 (Unix seconds) in error Details, not as time.Time"
metrics:
  duration_seconds: 480
  completed_date: "2026-05-13"
  tasks_completed: 3
  tasks_total: 3
  files_created: 12
  files_modified: 0
  tests_added: 17
---

# Phase 3 Plan 4: GitHub Actions Adapter Summary

**One-liner:** GitHub Actions adapter with two-page tag pagination, exact-string validate (non-semver v6 support), /releases/latest fast path, and GitHub-specific 403+X-RateLimit-Remaining:0 rate-limit override.

## What Was Built

### Cache-Key Strategy

Two distinct `Op` values under `Manager:"gh"`:

| Key | Shape | Contents | When set |
|-----|-------|----------|----------|
| `Key{Manager:"gh", Pkg:repo, Op:"tags", IncPre:incPre}` | `[]string` | Combined page-1 + page-2 tag names | On first `Validate` or `Latest` call for a repo |
| `Key{Manager:"gh", Pkg:repo, Op:"release-latest", IncPre:incPre}` | `string` | `tag_name` from `/releases/latest` | On first unfiltered stable `Latest` call |

The `tags` key stores the combined list (up to 200 entries) — both `Validate` and `Latest` derive in-process from this single cached fetch, amortising the pagination cost across all subsequent calls.

### Conditional Flow: /releases/latest vs Tags Filter

```
Latest(ctx, pkg, incPre, major, minor)
  if !incPre && major == nil && minor == nil:
    tagName := releaseLatestFor(ctx, pkg)
    if tagName != "" && semver.IsValid(tagName):
      → {Version: tagName, Source: "registry-release-pointer"}
    else if err is KindNotFound:
      fall through  ← repo has no formal releases (common for actions repos)
    else:
      ↑ bubble up (rate-limit, upstream-down, etc.)
  
  tags := tagsFor(ctx, pkg)  ← fetches p1 + p2 if p1==100
  v := filter.FilterAndPickHighest(tags, vPrefixed=true, incPre, major, minor)
  → {Version: v, Source: "computed-highest"}
```

### Rate-Limit Override Placement

`mapErr` is the local function called from all non-2xx branches. It checks:

1. `resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0"` → `errs.RateLimited` with `reset_at` parsed from `X-RateLimit-Reset` as `int64` Unix seconds
2. All other cases → `httperr.MapHTTPStatus(resp, pkg, "gh")`

The override runs **before** the shared mapper so it captures GitHub's non-standard rate-limit signal (403 instead of 429).

### D-GH-04: Non-Semver Tag Validation

`Validate` iterates the tags slice with `t == version` (exact byte match). No semver parsing is attempted. This means:
- `"v6"` → `exists:true` if the repo tags with `"v6"` (actions/checkout does)
- `"v6.0.2"` → `exists:true` if the repo tags with `"v6.0.2"`
- `"V6"` → `exists:false` (case-sensitive, as GitHub tag names are)

`Latest` uses `filter.FilterAndPickHighest(vPrefixed=true)` which calls `semver.IsValid(v)` and skips non-semver entries — so `"v6"` is silently excluded from `get_latest_version` results.

## Test Coverage

| Test | Assertion |
|------|-----------|
| TestURL_Tags (2 subtests) | tagsURL output shape + host check |
| TestURL_ReleasesLatest (2 subtests) | releasesLatestURL output shape + host check |
| TestValidate_HitNonSemverV6 | "v6" → exists:true, Source="versions-list" |
| TestValidate_HitSemverV602 | "v6.0.2" → exists:true |
| TestValidate_MissVersion | "v999.0.0" → KindNotFound |
| TestValidate_RateLimited | 403+X-RateLimit-Remaining:0 → KindRateLimited, reset_at=1999999999 |
| TestValidate_NotFound404 | 404 → KindNotFound |
| TestPagination_FetchesPageTwoWhen100 | upstream calls==2 when page 1 has 100 entries |
| TestPagination_NoPageTwoWhenShort | upstream calls==1 when page 1 has <100 entries |
| TestLatest_ReleasePointer | Source="registry-release-pointer" for unfiltered stable Latest |
| TestLatest_FilterMajorSkipsNonSemver | major=6 filter → v6.0.2 (not v6); Source="computed-highest" |
| TestLatest_IncPre | incPre=true bypasses pointer fast path → Source="computed-highest" |
| TestCache_HitOnSecondCall | 0 extra upstream calls on second Validate |

**Total tests: 17**

## Deviations from Plan

None — plan executed exactly as written.

## Threat Model Compliance

| Threat ID | Status |
|-----------|--------|
| T-03-gh-01 (DoS via 60/hr rate limit) | Mitigated — aggressive caching of tags + release-latest; pagination capped at 2 pages |
| T-03-gh-02 (SSRF via repo string) | Mitigated — host hardcoded to api.github.com in url.go |
| T-03-gh-03 (rate-limited error disclosure) | Accepted — reset_at + registry name is intentional for agent UX |

## Known Stubs

None.

## Threat Flags

None — no new network endpoints introduced beyond the planned GitHub API calls. Host is hardcoded.

## Self-Check: PASSED

Files exist:
- internal/registry/gh/url.go: FOUND
- internal/registry/gh/url_test.go: FOUND
- internal/registry/gh/gh.go: FOUND
- internal/registry/gh/gh_test.go: FOUND
- testdata/fixtures/gh/checkout-tags-p1.json: FOUND (100 entries)
- testdata/fixtures/gh/checkout-tags-p2.json: FOUND (5 entries)
- testdata/fixtures/gh/checkout-releases-latest.json: FOUND
- testdata/fixtures/gh/checkout-tags-p1-short.json: FOUND (7 entries)
- testdata/fixtures/gh/rate-limited.json: FOUND
- testdata/fixtures/gh/rate-limited.json.headers.json: FOUND
- testdata/fixtures/gh/nonexistent.json: FOUND
- testdata/fixtures/gh/nonexistent.json.headers.json: FOUND

Commits:
- 06027bd: chore(03-04): add gh fixtures
- 52ffc0a: feat(03-04): implement gh URL builders
- 146e060: feat(03-04): implement gh adapter
