---
phase: 02-npm-adapter-end-to-end-spine
plan: 05
subsystem: infra
tags: [ci, github-actions, benchmark, cold-start, test-04]

requires:
  - phase: 02-npm-adapter-end-to-end-spine/02-04
    provides: build-tag-gated newSharedClient + NPM_FIXTURE_DIR fixture-replay transport
provides:
  - First CI workflow in the repo (push/PR triggers, Go 1.25.x + 1.26.x matrix)
  - Automated go vet + go test ./... -race + tagged integration tests on every push/PR
  - Advisory cold-start benchmark posted to $GITHUB_STEP_SUMMARY
  - Baseline cold-start measurement: 8 ms (Linux CI runner)
affects: [phase-05-release, phase-06-dogfood]

tech-stack:
  added: []
  patterns:
    - "Build-matrix on the Go-version floor + latest stable to catch toolchain-specific regressions early"
    - "Advisory benchmark step with continue-on-error to surface metrics without gating merges"
    - "GITHUB_STEP_SUMMARY-based reporting so numbers are visible on the PR page, no separate dashboard needed"

key-files:
  created:
    - .github/workflows/ci.yml
  modified: []

key-decisions:
  - "Cold-start step runs only on the 1.25.x matrix entry (if: matrix.go == '1.25.x') to keep the summary unambiguous — two competing numbers would muddy regression-tracking"
  - "Newline-delimited JSON-RPC framing in the benchmark step (matches Phase 1's binary), not the LSP Content-Length form sketched in the plan's interfaces section"
  - "Subshell-with-trailing-sleep pattern: `( printf '%s\\n' \"$REQ\"; sleep 5 ) | ./bin` keeps stdin open while the binary processes the request, then naturally ends; the kill afterwards is belt-and-suspenders"

patterns-established:
  - "Spin-wait poll on output file size at 1 ms granularity for sub-second wall-clock benchmarks in shell"
  - "Cold-start measurement deliberately uses fixture-replay (no network) so the number reflects binary startup, not registry latency"

requirements-completed:
  - TEST-04

duration: ~15min
completed: 2026-05-13
---

# Plan 02-05: CI workflow + cold-start benchmark Summary

**First CI workflow lands: go vet + race-tested unit tests + tagged integration tests on Go 1.25 and 1.26, plus an advisory cold-start benchmark that measured 8 ms on the first run.**

## Performance

- **Tasks:** 2 (workflow YAML + blocking human checkpoint)
- **Files created:** 1 (`.github/workflows/ci.yml`, ~83 lines)
- **First observed cold-start:** **8 ms** (Linux CI, ubuntu-latest)
- **Local dry-run (macOS):** 445 ms — confirms the Linux runner is ~55× faster than a cold macOS spawn, consistent with the warm-cache CI environment
- **Workflow runtime:** matrix entries completed successfully

## Accomplishments

- TEST-04 closed: cold-start time is now measured automatically on every push/PR and posted to the workflow Summary view; never fails the build (continue-on-error: true).
- First measured cold-start: **8 ms** on Linux (`ubuntu-latest` — the GitHub-managed Ubuntu image current on the day of the run, typically ubuntu-22.04 or ubuntu-24.04 depending on rollout).
- Both Go matrix entries (1.25.x + 1.26.x) ran to completion.
- The advisory cold-start step posted a `### Cold-Start Benchmark` heading with the measured ms to `$GITHUB_STEP_SUMMARY` as designed.
- Local-vs-CI drift: local `go test ./...` finishes in ~3-4 s; CI runtime is roughly comparable once the Go module cache is warm. The 437 ms gap between local cold-start (445 ms) and CI cold-start (8 ms) is the expected difference between a macOS process-spawn cold-start (slow file IO, slower process create) and a Linux runner spawning from a warm filesystem cache.

## Task Commits

1. **Task 1: `.github/workflows/ci.yml`** — `ci(02-05): add CI workflow with advisory cold-start benchmark (TEST-04)`
2. **Task 2: Human checkpoint** — verified by user (cold-start-ms=8 reported on CI; build green).

## Files Created/Modified

- `.github/workflows/ci.yml` — single-job workflow: checkout → setup-go (matrix) → vet → unit tests (race) → integration tests (testfixtures tag) → advisory cold-start

## Decisions Made

- Used newline-delimited JSON-RPC framing in the cold-start shell block. The plan's `<interfaces>` section sketched LSP-style Content-Length framing; the actual binary speaks newline-delimited (matches Phase 1's `TestStdioCleanliness`). Substituted at implementation time. No protocol regression — Phase 1's stdio_test.go uses the same framing.
- Cold-start step is gated to a single matrix entry (`if: matrix.go == '1.25.x'`). Reporting two numbers per run would dilute the regression-tracking signal.

## Deviations from Plan

### Implementation choices that differ from the plan's sketch

**1. JSON-RPC framing form**
- **Plan sketch:** LSP-style `Content-Length: N\r\n\r\n<body>` framing.
- **Actual:** newline-delimited (one JSON-RPC object per line).
- **Rationale:** Phase 1's binary speaks newline-delimited (`internal/mcp/server.go` uses `sdkmcp.StdioTransport{}` default, which is newline-delimited; Phase 1's `TestStdioCleanliness` confirms this). Sending Content-Length framing to the binary would have produced no response, and the cold-start measurement would have hit the 5 s timeout fallback every run.
- **Disposition:** Auto-fixed at implementation time. The plan's `<action>` block correctly noted "the cold-start step does NOT need a perfectly-shaped response", but in practice the binary's stdio parser requires the request to be parseable as a JSON object, which it would not be in LSP framing.

**2. Stdin-keepalive mechanism**
- **Plan sketch:** `echo -n "$REQUEST" | ... &` with a poll loop and `kill "$PID"`.
- **Actual:** `( printf '%s\n' "$REQ"; sleep 5 ) | ./bin` — the subshell keeps stdin open for 5 s, ensuring the binary doesn't see EOF before responding.
- **Rationale:** The plan's `echo -n` closes stdin immediately on the producer side, racing the binary's response. The subshell-with-trailing-sleep is the standard fix.

---

**Total deviations:** 2 implementation refinements. Zero scope creep. Both refinements are necessary to make the cold-start measurement actually work — the plan's prose-level sketch correctly identified the goal but the example shell did not match the binary's actual stdio framing.

## Issues Encountered

- None on the CI side. The human-checkpoint workflow ran green on the first push.

## Confirmation

- `.github/workflows/ci.yml` parses as valid YAML and the cold-start shell block passes `bash -n`.
- Local dry-run prior to push confirmed the script emits a valid `cold-start-ms` number and the binary responds with a well-formed JSON-RPC `initialize` result envelope.
- CI run reported: `cold-start-ms=8` — well under the <20 ms target (advisory, not enforced).
- No new direct dependencies; `internal/depcheck` test continues to pass.

## Next Phase Readiness

- Phase 2 is now fully complete (5/5 plans). Every Phase 2 requirement (VAL-01, VAL-02, LAT-01, LAT-03/04/05, REG-01, TEST-01, TEST-02, TEST-04) is implemented and verified at both the adapter unit-test level (02-03) and the binary integration-test level (02-04), with automated regression detection (02-05).
- Phase 3 (PyPI/Go/GH/Maven adapters) can now copy the Phase 2 structure: each adapter follows the `internal/registry/<pkg>/{url.go, filter.go, errmap.go, <pkg>.go, *_test.go}` layout established here; `cmd/version-check-mcp/main.go` adds a `<pkg>.New(sharedClient, c)` binding; `cmd/version-check-mcp/client_testfixtures.go` extends `mapURLToFixture` with a new prefix.
- Phase 5 (Distribution) inherits the working CI workflow as the basis for the release pipeline.

---
*Phase: 02-npm-adapter-end-to-end-spine*
*Completed: 2026-05-13*
