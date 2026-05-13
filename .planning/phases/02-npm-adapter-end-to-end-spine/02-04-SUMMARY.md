---
phase: 02-npm-adapter-end-to-end-spine
plan: 04
subsystem: infra
tags: [http, transport, integration-test, build-tags, npm, mcp-stdio]

requires:
  - phase: 01-foundation-mcp-scaffolding
    provides: appmcp.NewServer + Run, MCP errmap, stdio cleanliness invariants, internal/version
  - phase: 02-npm-adapter-end-to-end-spine/02-03
    provides: npm.New constructor + Registry impl
provides:
  - Production binary wired to the real NPM adapter
  - Shared *http.Client (5s timeout) with User-Agent injecting transport
  - Build-tag-gated fixture-replay transport (NPM_FIXTURE_DIR) for deterministic CI
  - Six binary-level stdio integration tests proving end-to-end MCP path against NPM
affects: [phase-03-other-adapters, phase-05-release]

tech-stack:
  added: []
  patterns:
    - "Build-tag-gated test transport (//go:build testfixtures) keeps test helpers out of the shipped binary"
    - "uaTransport pattern: shared *http.Client with a UA-injecting RoundTripper wrapping http.DefaultTransport"
    - "Integration tests spawn the binary as a subprocess and exchange newline-delimited JSON-RPC over stdio"

key-files:
  created:
    - cmd/version-check-mcp/client.go
    - cmd/version-check-mcp/client_testfixtures.go
    - test/integration/npm_stdio_test.go
  modified:
    - cmd/version-check-mcp/main.go

key-decisions:
  - "Chose Option A (build-tag-gated newSharedClient) over Option B (env-var hook in production code). The test-only file `client_testfixtures.go` carries the fixture-replay implementation; the production binary built without `-tags testfixtures` has no awareness of NPM_FIXTURE_DIR."
  - "Pre-existing `internal/version.Version` (default `\"dev\"`, overridable via ldflags in Phase 5) is used for the User-Agent — no fallback literal was needed."
  - "Integration tests do not import `internal/testfixtures` even from `_test.go`; the fixture transport is reimplemented inline in the build-tagged client file so the cmd package never carries a test-helper import edge."

patterns-established:
  - "When a test needs to override a production behavior, prefer a build tag + parallel file pair over runtime env-var branches in production code"
  - "JSON-RPC over stdio is newline-delimited in this binary (no Content-Length framing) — integration tests read with bufio.Reader.ReadString('\\n')"

requirements-completed:
  - VAL-01
  - VAL-02
  - LAT-01
  - REG-01

duration: ~20min
completed: 2026-05-13
---

# Plan 02-04: NPM stdio wiring Summary

**Real NPM adapter is now wired into the production binary behind a 5s-timeout shared *http.Client with UA injection; six binary-level stdio integration tests prove validate_version + get_latest_version end-to-end with deterministic fixtures via a build-tag-gated transport override.**

## Performance

- **Tasks:** 2 (main.go wiring + integration tests)
- **Files created:** 3 (`client.go`, `client_testfixtures.go`, `npm_stdio_test.go`)
- **Files modified:** 1 (`main.go`)
- **Integration tests:** 6 named TestStdio_NPM_* cases (all green)
- **Whole-module regression:** default tags 82 pass, `-tags testfixtures` 88 pass, vet clean
- **Binary smoke-test:** `./version-check-mcp --help` shows --cache-ttl and --verbose

## Accomplishments

- User-Agent emitted on every NPM request: `version-check-mcp/dev (+https://github.com/JesperRossen/version-check-mcp)` (the `dev` token comes from the pre-existing `internal/version.Version` and is rewritten by ldflags at release).
- Chosen override mechanism: **Option A — build-tag-gated `newSharedClient()`**. The production binary `client.go` (no build tag) ships the 5s-timeout client wrapping `http.DefaultTransport` in `uaTransport`. The test build `client_testfixtures.go` (`//go:build testfixtures`) ships an alternate `newSharedClient` that, when `NPM_FIXTURE_DIR` is set, returns a fixture-loading transport.
- Version for `TestStdio_NPM_Validate_Hit`: `18.3.1` for `react` (chosen as a known-present non-latest version; encoded as a const at the top of the test file rather than discovered via `jq` at runtime — keeps tests hermetic and fast).
- The version package `internal/version` was found from Phase 1 (it ships `Version = "dev"`); no hard-coded fallback was needed.
- `cmd.Env` set in the integration test helper: `append(os.Environ(), "NPM_FIXTURE_DIR=<repo>/testdata/fixtures/npm")`.
- No Phase 1 main.go variable renames were forced; `c` (cache), `logger`, and `registries` keep their identifiers.

## Task Commits

1. **Task 1: main.go wiring + uaTransport + build-tagged client factory** — `feat(02-04)`
2. **Task 2: Six binary-level stdio integration tests** — `test(02-04)`

## Files Created/Modified

- `cmd/version-check-mcp/main.go` — Added `uaTransport`, `userAgent()`, swapped `fake.New("npm")` for `npm.New(sharedClient, c)`, replaced inline client construction with `newSharedClient()`
- `cmd/version-check-mcp/client.go` — `//go:build !testfixtures`, production `newSharedClient()`
- `cmd/version-check-mcp/client_testfixtures.go` — `//go:build testfixtures`, fixture-replay transport gated by `NPM_FIXTURE_DIR`
- `test/integration/npm_stdio_test.go` — Six TestStdio_NPM_* cases, helpers `spawn`, `callTool`, `readJSONResponse`, `expectSuccess`, `expectError`

## Decisions Made

- Option A (build-tag gating) chosen over Option B (env-var in production code). Rationale: production binaries shipped via GoReleaser in Phase 5 build without `-tags testfixtures` and therefore have zero awareness of `NPM_FIXTURE_DIR` — eliminates threat T-02-18 (env-var redirects HTTP to local files in production) at compile time.
- Integration tests rely on **newline-delimited** JSON-RPC framing (matching Phase 1's existing `stdio_test.go`); no Content-Length headers are needed. The MCP SDK's stdio transport accepts both forms but emits newline-delimited.

## Deviations from Plan

### Minor structural deviation

**1. [Plan acceptance criterion] `http.DefaultTransport` lives in `client.go` and `client_testfixtures.go`, not literally in `main.go`**

- **Found during:** Task 1 verification (`grep -c 'http.DefaultTransport' cmd/version-check-mcp/main.go` returned 0).
- **Plan expectation:** the criterion listed `cmd/version-check-mcp/main.go` as the search target.
- **Actual:** `http.DefaultTransport` is referenced in the build-tagged client files (`client.go` and `client_testfixtures.go`) — both still inside `package main` of `cmd/version-check-mcp/`. The semantic intent ("the production binary uses `http.DefaultTransport`") is satisfied; the literal `grep` of `main.go` only is not.
- **Disposition:** Accepted as a clean structural improvement. The plan's `<action>` block explicitly recommended Option A (factoring the client into a build-tagged file pair) — the grep criterion did not catch up to that recommendation. No behavioral regression.

---

**Total deviations:** 1 (cosmetic — file-level location of `http.DefaultTransport` reference). No scope creep, no security implication.

## Issues Encountered

- 1Password SSH-signing agent locked mid-session during commit; the user unlocked it and commits proceeded normally.
- One benign LSP warning ("No packages found for open file") on the build-tagged files; this is a known gopls limitation when working with build-tagged files outside a configured `buildFlags` set and does not affect compilation or test runs.

## Confirmation

- `TestStdioCleanliness` and `TestStderrIsJSON` (Phase 1 binary-level invariants) still pass.
- `internal/depcheck` test still passes — no new direct dependencies.
- `cmd/version-check-mcp/main.go` does not import `internal/testfixtures` (verified — count is 0).
- Production binary (no build tag) has zero awareness of `NPM_FIXTURE_DIR`.

## Next Phase Readiness

- Plan 02-05 (CI workflow with cold-start measurement) is unblocked.
- The build-tag pattern established here naturally extends to Phase 3 PyPI/Go/GH/Maven adapter integration tests: add a `<pkg>_FIXTURE_DIR` env var, extend `mapURLToFixture`, and add `<pkg>_stdio_test.go`.

---
*Phase: 02-npm-adapter-end-to-end-spine*
*Completed: 2026-05-13*
