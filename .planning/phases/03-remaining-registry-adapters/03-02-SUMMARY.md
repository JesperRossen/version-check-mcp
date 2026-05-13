---
phase: 03-remaining-registry-adapters
plan: "02"
subsystem: registry-adapters
tags: [pypi, pep440, registry, cache, yanked, adapter]

dependency_graph:
  requires:
    - phase: 03-01
      provides: internal/filter.PEP440Normalize, internal/filter.FilterAndPickHighest, internal/httperr.MapHTTPStatus
  provides:
    - internal/registry/pypi.Adapter (Validate, Latest, Name)
    - internal/registry/pypi.New(*http.Client, *cache.Cache)
    - testdata/fixtures/pypi/requests.json
    - testdata/fixtures/pypi/nonexistent.json + headers sidecar
  affects:
    - main.go (wave 4 wiring — pypi.New to be registered alongside npm.New)
    - internal/mcp (tool handler wiring)

tech_stack:
  added: []
  patterns:
    - PyPI JSON API endpoint: https://pypi.org/pypi/{project}/json
    - PEP 440 normalization applied to both input and map keys before equality comparison
    - All-files-yanked heuristic (A1) for PEP 592 yanked detection
    - info.version fast path for Latest when no filter active
    - cache.Get[*pypiProject] with Op="project" key (not per-version)

key_files:
  created:
    - internal/registry/pypi/pypi.go
    - internal/registry/pypi/url.go
    - internal/registry/pypi/pypi_test.go
    - testdata/fixtures/pypi/requests.json
    - testdata/fixtures/pypi/nonexistent.json
    - testdata/fixtures/pypi/nonexistent.json.headers.json
  modified: []

key_decisions:
  - "Source constants: pypi-info-version (fast-path Latest), versions-map (normal Validate hit), pypi-yanked (yanked Validate hit), computed-highest (filtered Latest)"
  - "Yanked detection: all-files-yanked heuristic (A1) — a version is yanked iff len(files)>0 AND every file object has yanked=true. Empty file list treated as non-yanked."
  - "requests-yanked.json omitted as separate fixture — requests.json already contains a yanked release (2.32.1), satisfying the plan's allowance for a single fixture"
  - "PEP440Normalize called twice in Validate: once on input, once on each releases map key (no pre-normalization on load — normalizes at comparison time)"

requirements-completed: [REG-02, LAT-02]

duration: 15min
completed: "2026-05-13"
---

# Phase 3 Plan 2: PyPI Registry Adapter Summary

**PyPI adapter implementing registry.Registry with PEP 440 normalization, PEP 592 yanked-flag detection, and info.version fast-path for latest-stable queries.**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-05-13T10:40:00Z
- **Completed:** 2026-05-13T10:55:00Z
- **Tasks:** 2 (Task 1: fixtures; Task 2: TDD adapter implementation)
- **Files created:** 6

## Accomplishments

- PyPI adapter implementing registry.Registry with Validate, Latest, Name methods
- PEP 440 normalization applied to both input version and release map keys — enables matching "2.31.0-rc1" to fixture key "2.31.0rc1"
- Yanked release detection per PEP 592 A1 heuristic: returns Exists=true with Source="pypi-yanked"
- info.version fast path for Latest: no filter, no incPre → single-field read, no iteration
- Cache wired via cache.Get[*pypiProject] — second identical call is a cache hit (1 upstream fetch total)
- Shared filter and httperr packages from Plan 01 consumed; no NPM logic duplicated

## Task Commits

Each task was committed atomically:

1. **Task 1: Record PyPI fixtures** - `97b8089` (chore)
2. **Task 2 RED: Failing PyPI adapter tests** - `049e087` (test)
3. **Task 2 GREEN: PyPI adapter implementation** - `b39f4af` (feat)

**Plan metadata:** (docs commit follows)

_Note: Task 2 is TDD — separate test (RED) and implementation (GREEN) commits._

## Files Created/Modified

- `internal/registry/pypi/pypi.go` - Adapter struct, New, Validate, Latest, Name, Source constants, isYanked helper
- `internal/registry/pypi/url.go` - projectURL helper
- `internal/registry/pypi/pypi_test.go` - 9 fixture-replay tests covering all acceptance criteria
- `testdata/fixtures/pypi/requests.json` - Nominal PyPI project fixture (6 releases: stable, PEP 440 non-canonical key, yanked, prerelease)
- `testdata/fixtures/pypi/nonexistent.json` - Empty body for 404 fixture replay
- `testdata/fixtures/pypi/nonexistent.json.headers.json` - Status 404 sidecar

## Decisions Made

- **Source values:** `pypi-info-version` / `versions-map` / `pypi-yanked` / `computed-highest` — distinct from NPM's `dist-tags.latest` to reflect the different fast-path mechanism
- **Yanked heuristic (A1):** All-files-yanked. An empty file list is treated as non-yanked (conservative). This is consistent with the RESEARCH.md recommendation for Open Question 1 and D-PYPI-01.
- **Single fixture for yanked:** requests.json already contains 2.32.1 as a yanked release; a separate requests-yanked.json file was not needed (plan explicitly allowed this).
- **Normalize at comparison time:** PEP440Normalize is called during the Validate iteration loop rather than pre-normalizing the map on load, to keep the pypiProject struct simple and avoid mutation of cached data.

## Deviations from Plan

None - plan executed exactly as written.

## Threat Model Compliance

| Threat ID | Status |
|-----------|--------|
| T-03-pypi-01 (URL injection via pkg path) | Mitigated — host is hard-coded "pypi.org"; path segment is pkg name which Go http stack rejects for traversal |
| T-03-pypi-02 (DoS via large body) | Accepted for v1 — client Timeout bounds overall fetch; io.LimitReader deferred as noted in plan |
| T-03-pypi-03 (info disclosure in errors) | Accepted — pkg/version/registry in errs.E details is intentional for agent UX |

## Known Stubs

None.

## Threat Flags

None — no new network endpoints beyond pypi.org, no auth paths, no schema changes.

## Self-Check: PASSED

Files exist:
- internal/registry/pypi/pypi.go: FOUND
- internal/registry/pypi/url.go: FOUND
- internal/registry/pypi/pypi_test.go: FOUND
- testdata/fixtures/pypi/requests.json: FOUND
- testdata/fixtures/pypi/nonexistent.json: FOUND
- testdata/fixtures/pypi/nonexistent.json.headers.json: FOUND

Commits:
- 97b8089: chore(03-02): add PyPI fixture files
- 049e087: test(03-02): add failing PyPI adapter tests (RED phase)
- b39f4af: feat(03-02): implement PyPI adapter

Test results: 9 passed, 0 failed
go vet: clean
go build: clean
