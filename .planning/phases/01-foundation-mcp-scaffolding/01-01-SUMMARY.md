---
phase: 01-foundation-mcp-scaffolding
plan: 01
status: complete
date: 2026-05-12
---

# Plan 01-01 SUMMARY — Module bootstrap + RED test scaffolding

## Module

- **Path:** `github.com/JesperRossen/version-check-mcp` (derived from `git remote get-url origin`)
- **Go floor:** `go 1.25.0` in `go.mod` (toolchain in use: 1.26.3)

## Direct dependencies (DEP-01, locked)

| Module | Resolved version |
|---|---|
| `github.com/modelcontextprotocol/go-sdk` | v1.6.0 |
| `github.com/hashicorp/golang-lru/v2` | v2.0.7 |
| `golang.org/x/sync` | v0.20.0 |
| `golang.org/x/mod` | v0.36.0 |

Verified by `TestDirectDepsLockedToFour` (depcheck) — passes today, fails the build the moment a fifth direct dep is added.

## RED test functions created (must turn GREEN in Waves 1–2)

`internal/errs/errs_test.go` (Wave 1, plan 01-02):
- `TestKindsHaveCorrectStringValues`
- `TestConstructorsSetKind`
- `TestErrorsAsRecoversE`
- `TestUnwrapReturnsWrapped`
- `TestRateLimitedDetailsCarryResetTime`

`internal/cache/cache_test.go` (Wave 2, plan 01-04):
- `TestKeyStringIsDeterministicAndCollisionFree`
- `TestSingleflightDedupes` (50-goroutine concurrency assertion + IncPre collision)
- `TestExpires`
- `TestTieredTTL` (success / not_found / upstream_down sub-cases)

`internal/registry/fake/fake_test.go` (Wave 2, plan 01-03):
- `TestFakeReturnsConfiguredValidateResult`
- `TestFakeReturnsConfiguredLatestResult`
- `TestFakeReturnsConfiguredError`
- `TestFakePanicHookFires`
- `TestFakeNameMatchesConstructor`

`internal/mcp/server_test.go` (Wave 3, plan 01-05):
- `TestToolsRegistered`
- `TestSchemaDescriptions`
- `TestValidateRejectsRanges` (also asserts FakeRegistry.Validate counter stays at 0)
- `TestRequestedVersionEcho`
- `TestErrorEnvelopeShape`
- `TestPanicRecoveredAsUpstreamDown`

`test/integration/stdio_test.go` (Wave 3, plan 01-05):
- `TestStdioCleanliness`
- `TestStderrIsJSON`
- `TestCacheTTLFlag`
- `TestHelpOutput`

## GREEN test

`internal/depcheck/depcheck_test.go`:
- `TestDirectDepsLockedToFour` — passes (DEP-01)
- `TestNoForbiddenFixtureLibs` — passes (DEP-02)

`go test ./internal/depcheck/...` exits 0.

## Friction notes

- **Toolchain upgrade required.** Local Go was 1.24.4 at the start; the MCP SDK declares `go 1.25.0` so the user upgraded to 1.26.3 before module init.
- **`go mod tidy` cannot run yet** because the RED tests reference internal packages that don't exist (the desired Wave-0 state). Used `go get golang.org/x/mod/modfile` to pull the transitive deps needed by the depcheck test instead. `go mod tidy` will become clean automatically once Wave 1 plan 01-02 introduces real `internal/errs` source.
- **`go.mod` got bumped from `go 1.25` to `go 1.25.0` by `go get`** (auto-bump to a more specific patch). Acceptable — `^go 1\.25` regex still matches.

## Files written

- `go.mod` (4 direct deps, `go 1.25.0`)
- `go.sum` (populated)
- `.gitignore` (+ `tmp/`, `/version-check-mcp`)
- `internal/errs/errs_test.go`
- `internal/cache/cache_test.go`
- `internal/registry/fake/fake_test.go`
- `internal/mcp/server_test.go`
- `test/integration/stdio_test.go`
- `internal/depcheck/depcheck_test.go`
