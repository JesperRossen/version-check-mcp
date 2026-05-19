# Roadmap: Version Check MCP

**Created:** 2026-05-12
**Granularity:** standard
**Phases:** 6
**v1 Requirements:** 41 (all mapped)
**Parallelization:** enabled (Phase 3 internal adapters parallel)

## Project Reference

**Core Value:** When an AI agent asks "does this package version exist?", the server returns a correct answer in under 20ms with useful alternatives if it doesn't.

**Constraints:**

- Go 1.25+ (MCP SDK floor)
- Minimum dependency footprint (4 direct deps allowed)
- Sub-20ms cold start
- Stdout sacred (MCP protocol only); logs to stderr
- Single static binary distribution

## Phases

- [x] **Phase 1: Foundation & MCP Scaffolding** - Stdout-safe MCP server skeleton, dependency discipline, schema contract, tool stubs, cache+singleflight, registry interface, binary-level stdout integration test (completed 2026-05-12)
- [x] **Phase 2: NPM Adapter & End-to-End Spine** - First adapter end-to-end through both tools, validates the architecture under one ecosystem, fixture infrastructure (completed 2026-05-13)
- [x] **Phase 3: Remaining Registry Adapters** - PyPI, Go Modules, GitHub Actions, Maven Central adapters (mutually independent, internally parallelizable) (completed 2026-05-15)
- [x] **Phase 4: Alternatives & Response-Shape Hardening** - Cross-registry alternatives suggestion, ecosystem-native version-string verification, response shape audit (completed 2026-05-19)
- [ ] **Phase 5: Distribution** - Multi-arch GoReleaser binaries, MCPB bundle, checksums, macOS quarantine doc
- [ ] **Phase 6: Dogfooding & v1.0.0** - Wire into Claude Desktop, daily-use validation window, tag v1.0.0

## Phase Details

### Phase 1: Foundation & MCP Scaffolding

**Goal**: A stdio MCP server boots, registers two tool stubs with full LLM-readable schemas, emits zero non-protocol bytes to stdout, and has the cache/singleflight/registry-interface scaffolding all subsequent adapters plug into.

**Depends on**: Nothing (first phase)

**Requirements**: DEP-01, DEP-02, MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, MCP-06, VAL-05, VAL-06, UX-02, UX-03, CACHE-01, CACHE-02, CACHE-03, CACHE-04, TEST-03

**Success Criteria** (what must be TRUE):

1. Running the built binary and sending an MCP `initialize` request over stdin returns a valid JSON-RPC `initialize` response on stdout with zero stray bytes (verified by the binary-level integration test in CI).
2. `version-check-mcp --help` shows `--cache-ttl <duration>` and `--verbose` flags; defaults are sensible (15 min TTL, info-level logs); when run with no flags the server starts silently and all log output appears on stderr as structured JSON via `log/slog`.
3. The server exits cleanly when stdin closes; a forced panic inside a tool handler surfaces to the client as a structured MCP error (one of `rate_limited`, `not_found`, `upstream_down`, `invalid_input`) rather than corrupting stdout.
4. Listing tools via MCP returns `validate_version` and `get_latest_version` with LLM-readable description text in the schema; calling either with a semver range like `^1.2.3` returns a structured `invalid_input` error and echoes the requested version.
5. `go.mod` direct-dependency list contains exactly the four allowed deps (`modelcontextprotocol/go-sdk`, `hashicorp/golang-lru/v2`, `golang.org/x/sync`, `golang.org/x/mod`); a unit test against `cache.GetOrLoad` fires N concurrent identical loads and confirms only one loader runs (singleflight) with prerelease-aware cache keys.

**Plans**: 5 plans

- [x] 01-01-PLAN.md — Wave 0 test scaffolding: go.mod with 4 locked deps, RED tests for errs/cache/fake/mcp/integration, GREEN depcheck enforcing DEP-01/DEP-02
- [x] 01-02-PLAN.md — Wave 1: internal/errs package (D-07 structured error type + 4 constructors)
- [x] 01-03-PLAN.md — Wave 2: Registry interface (D-05 locked seam) + FakeRegistry (D-06) with programmable panic hook
- [x] 01-04-PLAN.md — Wave 2: Cache with envelope storage, singleflight dedup, tiered TTL (D-08 — CACHE-01..04)
- [x] 01-05-PLAN.md — Wave 3: MCP server (tools+errmap+recover middleware), cmd/main.go entrypoint, integration test passes

---

### Phase 2: NPM Adapter & End-to-End Spine

**Goal**: A user can invoke `validate_version` and `get_latest_version` against NPM and get correct results — including scoped packages — proving the full stack from MCP handler through Registry interface through cache through HTTP through structured-error mapping.

**Depends on**: Phase 1

**Requirements**: VAL-01, VAL-02, LAT-01, LAT-03, LAT-04, LAT-05, REG-01, TEST-01, TEST-02, TEST-04

**Success Criteria** (what must be TRUE):

1. `validate_version(manager: "npm", pkg: "react", version: "18.3.1")` returns `{exists: true, source: <how-confirmed>}` and the same call with version `"99.0.0"` returns `{exists: false, ...}` with `requested_version` echoed; both calls complete under 20ms warm and the second identical call within TTL is a cache hit.
2. `validate_version(manager: "npm", pkg: "@types/node", ...)` works end-to-end with the scoped `/` correctly percent-encoded in the upstream URL (fixture-verified).
3. `get_latest_version(manager: "npm", pkg: "react")` returns `{version, source: "dist-tags.latest"}`; with `include_prereleases: false` (default) the result is a stable version; with `include_prereleases: true` the result may be a prerelease and that is observable in the response. `get_latest_version(manager: "npm", pkg: "react", major: 17)` returns the highest 17.x release; `major: 17, minor: 0` returns the highest 17.0.x release; `minor` without `major` returns `invalid_input`; a filter that matches no version returns `not_found`.
4. The recorded-fixture test suite runs deterministically in CI against `testdata/fixtures/npm/` using only stdlib comparison helpers (no `goldie`, no `cupaloy`); an `-update` flag (or env var) regenerates fixtures; one fixture exercises the scoped-package gotcha.
5. A cold-start benchmark exists in CI and reports startup time on every build (advisory only — regression noted, not failing).

**Plans**: 5 plans

Plans:
**Wave 1**

- [x] 02-01-PLAN.md — Wave 1: fixture-replay infrastructure (`internal/testfixtures/`), NPM URL builder with scoped-package encoding, initial fixtures (react, @types/node, 404)
- [x] 02-02-PLAN.md — Wave 1: LAT-05 version-list filter as pure function — `(versions, incPre, *major, *minor) -> (highest, ok)` — independent of HTTP/cache/adapter

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-03-PLAN.md — Wave 2: NPM adapter implementing Registry interface (VAL-01, VAL-02, LAT-01, REG-01, TEST-01/02), fixture-tested against react and @types/node

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-04-PLAN.md — Wave 3: wire NPM adapter into production binary; end-to-end MCP JSON-RPC-over-stdio test for validate_version + get_latest_version
- [x] 02-05-PLAN.md — Wave 3: CI workflow (full test suite + cold-start benchmark to GITHUB_STEP_SUMMARY, advisory only) closing TEST-04

---

### Phase 3: Remaining Registry Adapters

**Goal**: PyPI, Go Modules, GitHub Actions, and Maven Central all answer `validate_version` and `get_latest_version` correctly, each handling its signature ecosystem gotcha, and each surfacing `rate_limited` distinctly from `not_found` and `upstream_down`.

**Depends on**: Phase 2

**Requirements**: LAT-02, REG-02, REG-03, REG-04, REG-05

**Parallelization**: The four adapters in this phase are mutually independent Go packages with no inter-adapter dependencies — each can be implemented, tested, and merged in parallel. The plan-phase step should emit one plan per adapter so they can run concurrently if `parallelization: true` in config.

**Success Criteria** (what must be TRUE):

1. **PyPI**: `validate_version(manager: "pypi", pkg: "requests", version: "2.31.0rc1")` matches PyPI's normalized form (PEP 440); yanked releases (PEP 592) are flagged but `exists: true` if matched exactly; `validate_version` with a non-canonical input like `1.0.0-rc1` matches a release stored as `1.0.0rc1`.
2. **Go Modules**: `get_latest_version(manager: "gomod", pkg: "github.com/aws/aws-sdk-go")` returns a version with the `+incompatible` suffix preserved verbatim; pseudo-versions (`v0.0.0-yyyymmddhhmmss-hash`) are classified as prerelease and never surface as latest-stable; output always carries the mandatory `v` prefix.
3. **GitHub Actions**: `get_latest_version(manager: "gh", pkg: "actions/checkout")` works for actions that tag without making a Release object (uses `/tags` for listing, `/releases/latest` only as the latest-stable hint); a 403 with `X-RateLimit-Remaining: 0` maps to `rate_limited` with the `X-RateLimit-Reset` hint exposed in the error so the agent can wait.
4. **Maven Central**: `validate_version(manager: "maven", pkg: "org.springframework:spring-core", version: "6.1.0")` resolves correctly with `org.springframework` converted to the `org/springframework/` URL path; `*-SNAPSHOT` versions are always filtered out of stable results; package-id parsing rejects garbage with `invalid_input`.
5. Each adapter has a recorded-fixture set under `testdata/fixtures/<adapter>/` covering its signature gotcha; each adapter documents (in code comment + test) which registry field its `get_latest_version` trusts (registry-release-pointer vs computed-highest) and reports it as `source` in the response.

**Plans**: 5 plans

Plans:
**Wave 1**

- [x] 03-01-PLAN.md — Wave 1: promote internal/filter (with vPrefixed + pseudo-exclusion + PEP440Normalize) and internal/httperr (with registryName param) packages

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-02-PLAN.md — Wave 2: PyPI adapter (PEP 440 normalization, yanked flag via Source=pypi-yanked) + fixtures
- [x] 03-03-PLAN.md — Wave 2: Go Modules adapter (@v/list + @latest, module.EscapePath for capitals, pseudo-version fallback, +incompatible preservation) + fixtures
- [x] 03-04-PLAN.md — Wave 2: GitHub Actions adapter (2-page /tags pagination, /releases/latest hint, 403+X-RateLimit-Remaining:0 override, non-semver tag validate) + fixtures

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 03-05-PLAN.md — Wave 3: Maven Central adapter (encoding/xml, <release> pointer, SNAPSHOT filter, group:artifact parsing) + main.go wiring (replace 4 fake stubs)

---

### Phase 4: Alternatives & Response-Shape Hardening

**Goal**: When a requested version doesn't exist, the response carries 3-5 useful, ecosystem-native alternatives the LLM agent can pattern-match; the response shape is consistent and reviewed across all five registries.

**Depends on**: Phase 3

**Requirements**: VAL-03, VAL-04, UX-01

**Success Criteria** (what must be TRUE):

1. `validate_version` on a miss returns `alternatives` as a 3–5 entry array of `{version, reason}` where `reason ∈ {latest_stable, nearest_semver, latest_in_major}`, with `latest_stable` always first in the array and also duplicated outside the array as a top-level field.
2. Every alternative version string is in ecosystem-native form: Go entries carry `v` prefix, NPM entries don't, GH Actions tags follow their repo's convention, Maven entries are bare version strings, PyPI entries are PEP 440 canonical. Verified by a cross-registry fixture-driven test.
3. `nearest_semver` ordering ranks by patch-distance within same minor first, then minor-distance within same major, then major-distance — verified by a unit test against synthetic version lists.
4. A schema/response-shape audit pass confirms all five adapters produce identical top-level keys and identical error-type discriminator strings; an end-to-end test exercises one miss per registry and asserts the response shape contract.

**Plans**: 3 plans

Plans:
**Wave 1**

- [x] 04-01-PLAN.md — Wave 1: NearestVersions algorithm in internal/filter/ + unit tests
- [x] 04-02-PLAN.md — Wave 1: Add Versions() to Registry interface + all 5 adapter implementations + Fake

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 04-03-PLAN.md — Wave 2: Wire alternatives into validateRawHandler miss path + cross-registry response shape audit test

---

### Phase 5: Distribution

**Goal**: Anyone (including the author) can download a single static binary or one-click MCPB bundle from GitHub Releases for any of the five supported OS/arch combos, verify checksums, and run it without installing anything else.

**Depends on**: Phase 4

**Requirements**: DIST-01, DIST-02, DIST-03, DIST-04

**Success Criteria** (what must be TRUE):

1. A `git tag v0.x.0` push triggers a GoReleaser CI workflow that produces five binaries (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64) built with `CGO_ENABLED=0`, with `_ "time/tzdata"` embedded, uploaded to a GitHub Release with SHA256 checksums.
2. Running `file <binary>` on the macOS arm64 artifact shows statically linked / no dynamic libc dependencies; running the binary on a clean machine starts up and responds to `initialize` under 20ms cold.
3. A `version-check-mcp.mcpb` bundle is produced by an `@anthropic-ai/mcpb pack` post-build step (manifest_version 0.3), attached to the same GitHub Release, and installs into Claude Desktop in one click.
4. The README documents the macOS `xattr -d com.apple.quarantine` workaround for unsigned binaries; downloading the macOS artifact and following the README produces a working server.

**Plans**: 2 plans

Plans:
- [ ] 05-01-PLAN.md — Wave 1: tzdata import + .goreleaser.yaml (5 targets, CGO=0, SHA256, prerelease:auto) + README quick-start
- [ ] 05-02-PLAN.md — Wave 2: .github/workflows/release.yml (GoReleaser + MCPB pack + gh release upload) + README MCPB section

---

### Phase 6: Dogfooding & v1.0.0

**Goal**: The author uses the released binary daily through Claude Desktop, validates the wedge against real version-hallucination workflows, and tags v1.0.0 once the dogfood window is stable.

**Depends on**: Phase 5

**Requirements**: VALD-01, VALD-02

**Success Criteria** (what must be TRUE):

1. The MCPB bundle is installed into Claude Desktop and the two tools appear in the tool list with their LLM-readable descriptions intact.
2. A defined dogfood window (e.g. ≥7 days of daily use) elapses with structured logs captured to stderr; rate-limit, not-found, and upstream-down events are observable in the logs, not just successes.
3. No P0 bugs (stdout corruption, wrong-by-default latest-stable, scoped-package failure, `+incompatible` mishandling, Maven group-path bug) are open at the end of the window.
4. A `v1.0.0` git tag is cut once the window closes stable; the corresponding GitHub Release is the first non-pre tagged release.

**Plans**: TBD

---

## Coverage Map

All 40 v1 requirements mapped to exactly one phase. No orphans, no duplicates.

| Phase | Requirements | Count |
|-------|--------------|-------|
| 1 | DEP-01, DEP-02, MCP-01, MCP-02, MCP-03, MCP-04, MCP-05, MCP-06, VAL-05, VAL-06, UX-02, UX-03, CACHE-01, CACHE-02, CACHE-03, CACHE-04, TEST-03 | 17 |
| 2 | VAL-01, VAL-02, LAT-01, LAT-03, LAT-04, LAT-05, REG-01, TEST-01, TEST-02, TEST-04 | 10 |
| 3 | LAT-02, REG-02, REG-03, REG-04, REG-05 | 5 |
| 4 | VAL-03, VAL-04, UX-01 | 3 |
| 5 | DIST-01, DIST-02, DIST-03, DIST-04 | 4 |
| 6 | VALD-01, VALD-02 | 2 |
| **Total** | | **41** |

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation & MCP Scaffolding | 0/5 | Planned | - |
| 2. NPM Adapter & End-to-End Spine | 0/? | Not started | - |
| 3. Remaining Registry Adapters | 0/? | Not started | - |
| 4. Alternatives & Response-Shape Hardening | 0/? | Not started | - |
| 5. Distribution | 0/? | Not started | - |
| 6. Dogfooding & v1.0.0 | 0/? | Not started | - |

---
*Roadmap created: 2026-05-12*
