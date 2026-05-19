# Requirements: Version Check MCP

**Defined:** 2026-05-12
**Core Value:** When an AI agent asks "does this package version exist?", the server returns a correct answer in under 20ms with useful alternatives if it doesn't.

## v1 Requirements

### Dependency Discipline

- [x] **DEP-01**: `go.mod` direct-dependency list in v1 limited to: `github.com/modelcontextprotocol/go-sdk`, `github.com/hashicorp/golang-lru/v2`, `golang.org/x/sync`, `golang.org/x/mod`. Any addition requires a Key Decisions entry justifying it.
- [x] **DEP-02**: Test fixture comparison uses stdlib only (no `goldie`, no `cupaloy`, no `testify` for fixture comparison).

### MCP Server Core

- [x] **MCP-01**: Server starts as a stdio MCP server using `github.com/modelcontextprotocol/go-sdk@v1.6.0` and accepts JSON-RPC traffic over stdin/stdout
- [x] **MCP-02**: Server registers two tools (`validate_version`, `get_latest_version`) with full MCP tool-description schemas (LLM-readable docs)
- [x] **MCP-03**: Server emits zero non-protocol bytes to stdout; default Go logger redirected to stderr; top-level recover maps panics to structured MCP errors
- [x] **MCP-04**: Server exits cleanly when stdin closes
- [x] **MCP-05**: All logging goes to stderr as structured JSON via `log/slog`; `--verbose` flag enables debug level
- [x] **MCP-06**: `--cache-ttl` CLI flag accepts a duration string and sets the cache TTL; sensible default (15 min)

### Validation Tool

- [ ] **VAL-01**: `validate_version(manager, pkg, version, include_prereleases?)` returns `{exists: bool, source: string, alternatives: [{version, reason}, ...]}` on miss
- [ ] **VAL-02**: When the requested version exists, response is `{exists: true, source: <how-confirmed>}`
- [ ] **VAL-03**: When the requested version does not exist, `alternatives[]` contains 3–5 entries shaped as `{version, reason}` with `reason ∈ {latest_stable, nearest_semver, latest_in_major}`
- [ ] **VAL-04**: `latest_stable` entry comes first in `alternatives[]`; `latest_stable` is also duplicated outside the array for convenience
- [x] **VAL-05**: `validate_version` accepts exact versions only (no semver ranges); ranges return `invalid_input` error
- [x] **VAL-06**: `requested_version` is echoed in the response so the agent can pattern-match

### Latest Tool

- [ ] **LAT-01**: `get_latest_version(manager, pkg, include_prereleases?, major?, minor?)` returns `{version, source}` where `source ∈ {dist-tags.latest, registry-release-pointer, computed-highest}`
- [ ] **LAT-02**: Each registry adapter documents and uses the registry's own "latest" pointer when available; falls back to computed-highest only when missing
- [ ] **LAT-03**: When `include_prereleases=false` (default), prereleases are filtered out of the result
- [ ] **LAT-04**: When `include_prereleases=true`, prereleases are included; the latest may be a prerelease
- [ ] **LAT-05**: Optional integer `major` and `minor` filter parameters constrain the result to that major (or major+minor) component — works uniformly across all 5 registries; `minor` requires `major`; an invalid combination returns `invalid_input`; no version matching the filter returns `not_found`. Not a semver-range parser — see TOOL-V2-01 for true range support.

### Registries

- [ ] **REG-01**: NPM (`registry.npmjs.org`) — adapter with full validate + latest; scoped packages (`@org/pkg`) URL-encoded correctly
- [ ] **REG-02**: PyPI (`pypi.org`) — adapter with PEP 440 normalization for version comparison; honor `yanked` flag (PEP 592)
- [ ] **REG-03**: Go Modules (`proxy.golang.org`) — adapter handling `+incompatible`, pseudo-versions classified as prerelease, mandatory `v` prefix on output
- [ ] **REG-04**: GitHub Actions — adapter using tags-and-releases policy (prefer `/tags` for listing, `/releases/latest` only as latest-stable hint); surfaces `rate_limited` with `X-RateLimit-Reset` so agent can wait
- [ ] **REG-05**: Maven Central — adapter with `group:artifact` package-id parsing; uses `repo1.maven.org/maven2/.../maven-metadata.xml` for listings; SNAPSHOT versions filtered out

### Response Shape & UX

- [ ] **UX-01**: Ecosystem-native version strings in responses (Go: `v1.2.3`, NPM: `1.2.3`, etc.) so agent pastes them verbatim
- [x] **UX-02**: Structured MCP errors with type discriminator: `rate_limited`, `not_found`, `upstream_down`, `invalid_input`
- [x] **UX-03**: All tool schemas include description text written for LLM consumption (per MCP best practice — the schema *is* the agent's documentation)

### Caching & Concurrency

- [x] **CACHE-01**: In-memory cache via `hashicorp/golang-lru/v2/expirable`, single global TTL configurable via `--cache-ttl`
- [x] **CACHE-02**: Cache key includes `(manager, pkg, op, include_prereleases)` so prerelease and non-prerelease results don't collide
- [x] **CACHE-03**: Singleflight (`golang.org/x/sync/singleflight`) protects against cache stampede; one concurrent loader per key
- [x] **CACHE-04**: Tiered TTL behavior — successes cached at full TTL; 404s cached at short TTL; 5xx never cached as success (brief circuit-breaker)

### Testing

- [ ] **TEST-01**: Recorded-fixture (golden-file) test suite using stdlib helpers (no third-party fixture library); one fixture set per registry; an `-update` flag (or env var) regenerates fixtures
- [ ] **TEST-02**: Each registry fixture set covers its signature gotcha (scoped NPM pkg, PEP 440 normalized version, Go `+incompatible`, GH tags-only repo, Maven group-as-path)
- [x] **TEST-03**: Binary-level integration test in CI that spawns the built binary, sends an `initialize` message, asserts stdout is JSON-RPC-only (zero stray bytes)
- [ ] **TEST-04**: Cold-start benchmark in CI (advisory only — measured each build, regression noted but not failing)

### Distribution

- [ ] **DIST-01**: Single static binary, `CGO_ENABLED=0`, `import _ "time/tzdata"` embedded
- [x] **DIST-02**: GoReleaser-built binaries for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64 published to GitHub Releases with SHA256 checksums
- [x] **DIST-03**: MCPB bundle (`.mcpb`, manifest_version 0.3) produced via `@anthropic-ai/mcpb pack` in a post-build CI step and attached to the GitHub Release
- [x] **DIST-04**: README documents macOS `xattr -d com.apple.quarantine` workaround for unsigned binary

### Validation & Release

- [ ] **VALD-01**: Wired into Claude Desktop and used daily by author over a defined dogfood window before tagging v1.0.0
- [ ] **VALD-02**: v1.0.0 tag cut once dogfood window is stable

## v2 Requirements

### Tooling Surface

- **TOOL-V2-01**: `validate_range(manager, pkg, range)` as a separate tool — per-registry range grammars (npm `^/~`, PEP 440 `~=`, Go MVS, Maven brackets) handled distinctly; response shape `{resolves, best_match, candidates}`
- **TOOL-V2-02**: `is_latest(manager, pkg, version)` convenience tool
- **TOOL-V2-03**: Batched lookup — array of specs in one call
- **TOOL-V2-04**: `package_not_found` vs `version_not_found` discriminator split in the error type

### Registry Surface

- **REG-V2-01**: Crates.io (Rust) support
- **REG-V2-02**: Private-registry authentication (`.npmrc`-style config, private PyPI/Maven tokens)
- **REG-V2-03**: `GITHUB_TOKEN` env var to raise the GitHub API rate limit from 60/hr → 5000/hr
- **REG-V2-04**: Per-registry cache TTL configuration

### Metadata

- **META-V2-01**: Package metadata fields (description, license, homepage) on `get_latest_version` response
- **META-V2-02**: Deprecation flag (npm `deprecated`, PyPI `yanked`, Go retract directives)
- **META-V2-03**: Package-name typo / fuzzy-name detection on `not_found`

### Distribution

- **DIST-V2-01**: Homebrew formula
- **DIST-V2-02**: `go install` support documented
- **DIST-V2-03**: macOS code-signing via Apple Developer ID

## Out of Scope

| Feature | Reason |
|---------|--------|
| Package installation / env management | Specialist tool — turns lookup into code execution; defeats the wedge |
| Vulnerability scanning / CVE / advisory integration | Adds a 6th upstream and changes semantics from "valid" to "safe"; needs separate design |
| Dependency-tree introspection | Different problem; response-size explosion |
| Multi-tool-per-ecosystem surface (e.g. `check_npm_versions`) | Inflates LLM tool-discovery cost; specialist wedge prefers two tools that dispatch by `manager` |
| Live registry calls in PR CI | Fixtures only in PR pipeline — keeps tests deterministic; live integration can run nightly later |
| Contribution infrastructure (issue templates, contributing guide, etc.) | Author-only audience for v1; no contribution surface needed |
| Crates.io / NuGet / RubyGems / Packagist | Deferred — not in the daily AI-hallucination hot path for this author |
| GITHUB_TOKEN env var in v1 | Defer; revisit if 60/hr limit bites during dogfooding |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| DEP-01 | Phase 1 | Complete |
| DEP-02 | Phase 1 | Complete |
| MCP-01 | Phase 1 | Complete |
| MCP-02 | Phase 1 | Complete |
| MCP-03 | Phase 1 | Complete |
| MCP-04 | Phase 1 | Complete |
| MCP-05 | Phase 1 | Complete |
| MCP-06 | Phase 1 | Complete |
| VAL-01 | Phase 2 | Pending |
| VAL-02 | Phase 2 | Pending |
| VAL-03 | Phase 4 | Pending |
| VAL-04 | Phase 4 | Pending |
| VAL-05 | Phase 1 | Complete |
| VAL-06 | Phase 1 | Complete |
| LAT-01 | Phase 2 | Pending |
| LAT-02 | Phase 3 | Pending |
| LAT-03 | Phase 2 | Pending |
| LAT-04 | Phase 2 | Pending |
| LAT-05 | Phase 2 | Pending |
| REG-01 | Phase 2 | Pending |
| REG-02 | Phase 3 | Pending |
| REG-03 | Phase 3 | Pending |
| REG-04 | Phase 3 | Pending |
| REG-05 | Phase 3 | Pending |
| UX-01 | Phase 4 | Pending |
| UX-02 | Phase 1 | Complete |
| UX-03 | Phase 1 | Complete |
| CACHE-01 | Phase 1 | Complete |
| CACHE-02 | Phase 1 | Complete |
| CACHE-03 | Phase 1 | Complete |
| CACHE-04 | Phase 1 | Complete |
| TEST-01 | Phase 2 | Pending |
| TEST-02 | Phase 2 | Pending |
| TEST-03 | Phase 1 | Complete |
| TEST-04 | Phase 2 | Pending |
| DIST-01 | Phase 5 | Pending |
| DIST-02 | Phase 5 | Complete |
| DIST-03 | Phase 5 | Complete |
| DIST-04 | Phase 5 | Complete |
| VALD-01 | Phase 6 | Pending |
| VALD-02 | Phase 6 | Pending |

**Coverage:**
- v1 requirements: 41 total
- Mapped to phases: 41
- Unmapped: 0

---
*Requirements defined: 2026-05-12*
*Last updated: 2026-05-12 after roadmap creation*
