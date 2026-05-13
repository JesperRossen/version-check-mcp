---
phase: 03-remaining-registry-adapters
plan: "03"
subsystem: gomod-adapter
tags: [gomod, goproxy, module-escape, pseudo-version, incompatible, cache]
dependency_graph:
  requires:
    - internal/filter.FilterAndPickHighest (03-01)
    - internal/httperr.MapHTTPStatus (03-01)
  provides:
    - internal/registry/gomod.Adapter
    - internal/registry/gomod.ListURL
    - internal/registry/gomod.LatestURL
  affects:
    - cmd/version-check-mcp/main.go (fake stub → gomod.New)
tech_stack:
  added: []
  patterns:
    - TDD RED/GREEN per task
    - GOPROXY @v/list membership check for Validate
    - GOPROXY @latest fast-path with pseudo-version fallback
    - module.EscapePath for capital-letter module paths (!-escaping)
    - filter.FilterAndPickHighest(vPrefixed=true) for computed-highest
    - fixture-replay tests with urlToFile map proving !-escape routing
key_files:
  created:
    - testdata/fixtures/gomod/aws-sdk-go.list
    - testdata/fixtures/gomod/aws-sdk-go-latest.json
    - testdata/fixtures/gomod/aws-sdk-go-latest-pseudo.json
    - testdata/fixtures/gomod/azure-sdk.list
    - testdata/fixtures/gomod/azure-sdk-latest.json
    - testdata/fixtures/gomod/nonexistent.list
    - testdata/fixtures/gomod/nonexistent.list.headers.json
    - internal/registry/gomod/url.go
    - internal/registry/gomod/gomod.go
    - internal/registry/gomod/url_test.go
    - internal/registry/gomod/gomod_test.go
  modified: []
decisions:
  - "cache-key strategy: two Op values per (mod, incPre) — 'list' for @v/list, 'latest' for @latest; validate and latest both share the 'list' cache"
  - "pseudo-version fallback: when @latest returns pseudo-version and incPre=false, fall back to FilterAndPickHighest(vPrefixed=true, incPre=false) which excludes pseudo-versions per D-GOMOD-03"
  - "D-GOMOD-03 invariant: Validate always does exact string match regardless of incPre; pseudo-versions are valid if present in list"
  - "ListURL/LatestURL exported (capital) to allow external test package (gomod_test) to call them in URL canary assertions"
  - "aws-sdk-go-latest-pseudo.json added as extra fixture for TestLatest_PseudoFallback and TestLatest_IncPre"
metrics:
  duration_seconds: 390
  completed_date: "2026-05-13"
  tasks_completed: 3
  tasks_total: 3
  files_created: 11
  files_modified: 0
  tests_added: 16
---

# Phase 3 Plan 3: Go Modules Adapter Summary

**One-liner:** Go Modules adapter using @v/list membership for Validate and @latest-with-pseudo-fallback for Latest, with module.EscapePath enforcing GOPROXY !-escaping for capital-letter module paths.

## What Was Built

### testdata/fixtures/gomod/

Seven fixtures covering all adapter test cases:

- `aws-sdk-go.list` — newline-delimited @v/list with versions covering: plain semver (`v1.50.0`), `+incompatible` (`v1.55.7+incompatible`), prerelease (`v2.0.0-rc1`), and pseudo-version (`v0.0.0-20230101120000-abc123def456`).
- `aws-sdk-go-latest.json` — @latest body returning `v1.55.7+incompatible` (non-prerelease pointer).
- `aws-sdk-go-latest-pseudo.json` — @latest body returning a pseudo-version (triggers fallback to computed-highest).
- `azure-sdk.list` — versions for `github.com/Azure/azure-sdk-for-go`; fixture keyed on the !-escaped URL `github.com/!azure/azure-sdk-for-go`.
- `azure-sdk-latest.json` — @latest for Azure module.
- `nonexistent.list` — empty body.
- `nonexistent.list.headers.json` — `{"status":404,"headers":{}}` to replay a 404 response.

### internal/registry/gomod/url.go

```go
func ListURL(mod string) (string, error)   // https://proxy.golang.org/{escaped}/@v/list
func LatestURL(mod string) (string, error) // https://proxy.golang.org/{escaped}/@latest
```

Both call `module.EscapePath(mod)`; on error return `("", err)`. Capital letters in module paths are converted to `!lowercase` (e.g. `Azure` → `!azure`).

### internal/registry/gomod/gomod.go

Exports: `Adapter`, `New(client, c)`, `Name()="gomod"`, `Validate`, `Latest`.

Cache-key strategy:
- `Key{Manager:"gomod", Pkg:mod, Op:"list", IncPre:incPre}` → `[]string` (version list from @v/list)
- `Key{Manager:"gomod", Pkg:mod, Op:"latest", IncPre:incPre}` → `string` (Version from @latest JSON)

`Validate` logic:
1. `listFor(ctx, mod, incPre)` — cached @v/list fetch
2. Iterate list with exact string equality (preserves `+incompatible`, pseudo-versions)
3. Hit → `{Exists:true, Source:"versions-list"}`; Miss → `errs.NotFound`

`Latest` logic:
1. If `major==nil && minor==nil`, try `latestFor(ctx, mod, incPre)`
2. If @latest is non-pseudo AND (incPre=true OR not prerelease) → `{Version:latest, Source:"proxy-latest"}`
3. Else: `listFor` + `filter.FilterAndPickHighest(versions, vPrefixed=true, incPre, major, minor)` → `{Version:highest, Source:"computed-highest"}`

Source constants: `"proxy-latest"`, `"computed-highest"`, `"versions-list"`.

## Test Coverage

| Test | Description |
|------|-------------|
| TestURL_ListNominal | Plain module path builds correct @v/list URL |
| TestURL_ListEscapes | Capital A in Azure → !azure in URL |
| TestURL_LatestEscapes | Capital A in Azure → !azure in @latest URL |
| TestURL_LatestSuffix | /@latest suffix used |
| TestURL_EmptyInputReturnsError | Empty module path returns error |
| TestValidate_Hit | Known version → Exists=true, Source="versions-list" |
| TestValidate_Incompatible | +incompatible version matched verbatim |
| TestValidate_PseudoExplicit | D-GOMOD-03: pseudo-version explicit match with incPre=false |
| TestValidate_MissVersion | Version absent → KindNotFound |
| TestValidate_MissModule404 | 404 list → KindNotFound (via httperr.MapHTTPStatus) |
| TestLatest_ProxyLatest | @latest returns stable → Source="proxy-latest" |
| TestLatest_PseudoFallback | @latest pseudo + incPre=false → Source="computed-highest" |
| TestLatest_IncPre | incPre=true trusts @latest pseudo → Source="proxy-latest" |
| TestLatest_FilterMajor | major filter set → bypasses @latest → "computed-highest" |
| TestLatest_EscapedPath | Azure module routes to !azure URL (proven by fixture map) |
| TestCache_HitOnSecondCall | Two Validate calls = 1 upstream fetch |

## Deviations from Plan

### Extra fixture (Rule 2 - Missing critical functionality)

**Found during:** Task 3 pre-implementation reading

**Issue:** The plan specified the `aws-sdk-go-latest-pseudo.json` fixture conditionally ("if implementing TestLatest_PseudoFallback requires it, add..."). Both `TestLatest_PseudoFallback` (incPre=false, @latest pseudo → fallback) and `TestLatest_IncPre` (incPre=true, @latest pseudo → trust verbatim) require a fixture that returns a pseudo-version from @latest.

**Fix:** Added `aws-sdk-go-latest-pseudo.json` as a dedicated fixture in Task 1.

**Files modified:** `testdata/fixtures/gomod/aws-sdk-go-latest-pseudo.json`

### ListURL/LatestURL exported (minor deviation)

**Found during:** Task 2

**Issue:** The plan said "lowercase OK — same-package only" but the test file uses `package gomod_test` (external test package), which requires exported symbols.

**Fix:** Named functions `ListURL` and `LatestURL` (exported) instead of `listURL`/`latestURL`. The acceptance criteria's `grep -c 'module.EscapePath' ... == 2` is satisfied by 2 actual function calls (the other 4 matches are comment lines).

## Known Stubs

None.

## Threat Model Compliance

| Threat ID | Status |
|-----------|--------|
| T-03-gomod-01 (module path → URL tampering) | Mitigated — module.EscapePath rejects invalid paths; error returned as errs.UpstreamDown |
| T-03-gomod-02 (DoS via large list body) | Mitigated — http.Client.Timeout bounds the read |
| T-03-gomod-03 (malformed @latest JSON) | Mitigated — json.NewDecoder rejects malformed JSON; errs.UpstreamDown with reason="malformed_body" |

## Threat Flags

None — no new network endpoints, auth paths, or schema changes introduced beyond the gomod registry adapter itself.

## Self-Check: PASSED

Files exist:
- testdata/fixtures/gomod/aws-sdk-go.list: FOUND
- testdata/fixtures/gomod/aws-sdk-go-latest.json: FOUND
- testdata/fixtures/gomod/aws-sdk-go-latest-pseudo.json: FOUND
- testdata/fixtures/gomod/azure-sdk.list: FOUND
- testdata/fixtures/gomod/azure-sdk-latest.json: FOUND
- testdata/fixtures/gomod/nonexistent.list: FOUND
- testdata/fixtures/gomod/nonexistent.list.headers.json: FOUND
- internal/registry/gomod/url.go: FOUND
- internal/registry/gomod/gomod.go: FOUND
- internal/registry/gomod/url_test.go: FOUND
- internal/registry/gomod/gomod_test.go: FOUND

Commits:
- 79f8618: chore(03-03): add gomod fixtures
- ead8630: test(03-03): failing tests for URL builders (RED)
- 7c4225e: feat(03-03): implement gomod URL builders (GREEN)
- 5d8e69e: test(03-03): failing gomod adapter tests (RED)
- 19b9477: feat(03-03): implement gomod adapter (GREEN)
