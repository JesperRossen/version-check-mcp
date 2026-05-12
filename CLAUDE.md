<!-- GSD:project-start source:PROJECT.md -->
## Project

**Version Check MCP**

A lightweight Model Context Protocol (MCP) server written in Go that gives local AI coding agents (Claude Desktop, Cursor, Windsurf, etc.) a single source of truth for software package versions across NPM, PyPI, Go Modules, GitHub Actions, and Maven. It exists to stop "version hallucination" — agents inventing or guessing version strings that lead to broken installs — by serving the exact right string every time, fast, from a single static binary.

**Core Value:** When an AI agent asks "does this package version exist?", the server returns a correct answer in under 20ms with useful alternatives if it doesn't.

### Constraints

- **Language**: Go 1.25+ — hard floor; the chosen MCP SDK (`github.com/modelcontextprotocol/go-sdk@v1.6.0`) declares `go 1.25.0` in its `go.mod` and will not compile on earlier versions. CI matrix: 1.25 (floor) + 1.26 (latest stable).
- **Tech stack**: Minimal dependency footprint is a core principle. Allowed direct dependencies in v1: `github.com/modelcontextprotocol/go-sdk` (the MCP SDK), `github.com/hashicorp/golang-lru/v2` (bounded LRU+TTL — narrow scope, mature, MPL-2.0), `golang.org/x/sync` (singleflight), `golang.org/x/mod` (semver). Test fixture comparison uses stdlib helpers, NOT a third-party golden-file library. Every additional dependency requires explicit justification in Key Decisions.
- **Performance**: Sub-20ms server startup (cold) — the agent UX depends on the server feeling instant.
- **Protocol**: MCP over JSON-RPC on stdio — stdout is sacred (protocol traffic only); logs go to stderr.
- **Network**: HTTP calls only via `net/http`; no third-party HTTP clients in v1.
- **Distribution**: Single static binary, no runtime dependency on the host — defeats the `node_modules`/venv friction the project exists to avoid.
- **Audience**: Author-only for v1 — no contribution infrastructure, issue templates, or polished docs required to ship.
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## TL;DR / Prescriptive Picks
| Concern | Pick | One-liner reason |
|---|---|---|
| MCP SDK | `github.com/modelcontextprotocol/go-sdk@v1.6.0` | The Anthropic-blessed official SDK; aligned with the spec milestone; PROJECT.md already names it. |
| Go toolchain floor | **Go 1.25.0** (not 1.22) | The official SDK's `go.mod` declares `go 1.25.0`. PROJECT.md's "1.22+" is stale and must be raised. |
| TTL cache | `github.com/hashicorp/golang-lru/v2/expirable` (v2.0.7) | Bounded LRU + TTL, tiny API, BSD-licensed, no goroutine janitor surprise. |
| Semver | `golang.org/x/mod/semver` (v0.36.0) | Stdlib-adjacent, covers parse/compare/sort. Wrap for non-`v`-prefixed input. |
| Logging | `log/slog` with `slog.NewJSONHandler(os.Stderr, ...)` (stdlib, since Go 1.21) | Zero dependency, JSON-to-stderr is one line, satisfies the "stdout sacred" rule. |
| HTTP client | `net/http` stdlib | Project constraint; sufficient. |
| Golden-file tests | `github.com/sebdah/goldie/v2@v2.8.0` | Actively maintained (Oct 2025); cupaloy is dormant since 2022. |
| Release tooling | `goreleaser/goreleaser-action@v2` with GoReleaser v2.15.4 | Standard multi-arch GitHub Releases pipeline; we extend it with a post-archive hook to emit `.mcpb`. |
| MCPB tooling | `@anthropic-ai/mcpb` CLI (`mcpb init`, `mcpb pack`) — Node CLI run in CI only | Official tool; no Go-native equivalent exists; only used at release time. |
## 1. MCP Go SDK — the load-bearing pick
### Verified facts
- **Import path:** `github.com/modelcontextprotocol/go-sdk/mcp` (the SDK lives under the `mcp` package).
- **Current tag:** **v1.6.0** (May 8, 2026 per GitHub releases page) — Context7 also lists v1.0.0 and v1.2.0 as indexed versions, with v1.6.0 being current.
- **Maintenance:** Actively maintained — 24 tagged releases, 663 commits on main, weekly cadence in 2026.
- **Go floor:** `go 1.25.0` per `go.mod` on `main`. **This contradicts PROJECT.md's "1.22+" constraint — see Action item below.**
- **Status:** Official Anthropic-owned SDK. The Go ecosystem has two camps:
### Why the official SDK over mark3labs/mcp-go
### Confirmed minimal usage shape (for the roadmapper)
### Action item for PROJECT.md
## 2. Go toolchain
| Item | Value | Notes |
|---|---|---|
| Minimum Go version | **1.25.0** | Imposed by `go-sdk` `go.mod`. |
| Recommended build version | **1.26.x** (1.26.3 current) | Latest stable; gets full security fixes. |
| Currently supported by upstream | 1.25.x, 1.26.x | 1.24 in security-only window; 1.23 EOL. |
## 3. In-memory TTL cache
### Why
- **Bounded by design** — LRU caps memory automatically; you cannot leak by caching every package the agent ever asks about.
- **TTL built-in** — no manual sweeper goroutine, no shutdown coordination, no test flakiness from timer races.
- **Tiny API** — `Add`, `Get`, `Remove`, `Purge`. Zero learning curve.
- **Generics** — type-safe `LRU[K, V]`, so registry payloads stay typed.
- **License:** MPL-2.0 (acceptable for an open-source MCP server).
### Alternatives weighed
| Option | Verdict |
|---|---|
| `patrickmn/go-cache` | **Reject.** Last release Oct 2017 — effectively abandoned. No bound on cache size; the API is fine but unbounded growth is a footgun for a long-running daemon. |
| `dgraph-io/ristretto` v2.4.0 | **Reject for v1.** Excellent for high-throughput workloads (millions of ops/s), but overkill for ~5 registries × a few thousand keys. Adds admission policy complexity and a heavier dep tree. Revisit only if profiling shows the LRU is a bottleneck (it won't). |
| `sync.Map` + manual TTL | **Reject.** You'd reimplement eviction, bounding, and sweeping — three things `expirable` already gets right. Hand-rolling is only attractive if zero deps is a hard rule; it isn't, the project already accepts the MCP SDK. |
| `stdlib map + sync.RWMutex` + ticker | Same objection as `sync.Map` — wastes engineering budget. |
## 4. Semver
### Why
- **Stdlib-adjacent.** Same authors, same release cadence, same review standards as the standard library.
- **Covers everything we need:** parse, compare, sort, prerelease detection.
- **Tiny API surface** — Compare, IsValid, Major, MajorMinor, Prerelease, Build, Sort.
- **License:** BSD-3-Clause.
### One catch
### Why not Masterminds/semver
- v3.5.0 is excellent but offers things v1 doesn't need: **constraint parsing** (`^1.2.3`, `~1.2`, ranges). The `validate_version` tool answers a yes/no question about an exact version, not a range. YAGNI applies.
- Adds another transitive dep tree and a slightly larger binary.
- Revisit only if a future tool like `find_compatible_version(pkg, constraint)` is added.
## 5. Structured logging (stderr-only)
### Why
- **Zero dependency.** Matches PROJECT.md's "stdlib where feasible" constraint exactly.
- **Stderr-only is one parameter.** `slog.NewJSONHandler(os.Stderr, ...)` — stdout is never touched, satisfying the MCP protocol constraint.
- **In stdlib since Go 1.21.** Floor is 1.25, so it's always available.
- **Structured by default.** No `fmt.Sprintf`-vs-struct debates.
- **Level filtering for free.** Wire the `--verbose` CLI flag to `LevelDebug`.
### Why not zerolog / zap
- **zerolog** (rs/zerolog) and **zap** (uber-go/zap) are both fine libraries, but they predate slog and exist mostly for benchmark-driven workloads. For a server that emits maybe 10 log lines per request and runs single-process, slog's overhead is invisible.
- Adding either violates the "stdlib + one MCP SDK" minimal-dependency posture for **no observable benefit**.
## 6. HTTP client
- Use a **single shared `*http.Client`** (not `http.Get`); set a sane `Timeout` (e.g. 5s) and reuse it across registries.
- Set a custom `User-Agent` header (`version-check-mcp/<ver> (+repo URL)`) — registries and GitHub explicitly request this and may rate-limit blank UAs more aggressively.
- Inspect `Retry-After` headers on 429/503 from npm and GitHub; surface as `rate_limited` structured error rather than blindly retrying.
## 7. Golden-file / fixture testing
### Why
- **Actively maintained** — v2.8.0 released October 2025; recent `testing.TB` interface refactor.
- **JSON-aware assertions** (`AssertJson`) keep the diff readable.
- **Functional options** (`WithFixtureDir`, `WithDiffEngine`) make CI integration straightforward.
### Why not cupaloy
- `bradleyjkemp/cupaloy@v2.8.0` is **dormant since September 2022**. Snapshot diffs use `go-spew` and are noisier for JSON payloads.
- For fixtures that mirror real registry JSON responses, you want JSON-native comparison; goldie does this natively, cupaloy treats everything as bytes.
### Why not hand-rolled
## 8. Release tooling
### Producing the `.mcpb` bundle in the same release run
## 9. MCPB (MCP Bundle) format
### What it is
- **`.mcpb`** is the MCP Bundle format — a ZIP archive containing one or more MCP server binaries plus a `manifest.json` descriptor. Conceptually equivalent to Chrome's `.crx` or VS Code's `.vsix`.
- Officially renamed from **DXT (Desktop Extensions)** in late 2025. Anthropic blog post: "Adopting the MCP Bundle format (.mcpb) for portable local servers" (Nov 2025). The `.dxt` extension and `@anthropic-ai/dxt` package are deprecated aliases.
- The spec is owned by the Model Context Protocol org: `github.com/modelcontextprotocol/mcpb` (mirrored at `github.com/anthropics/mcpb`).
### Who publishes the spec
- **Spec file:** `MANIFEST.md` in `modelcontextprotocol/mcpb`. Current `manifest_version` is **0.3**.
- **CLI:** `@anthropic-ai/mcpb` on npm — provides `mcpb init` and `mcpb pack`.
### How to build one for a Go binary
# Lay out server/ with the platform binaries from dist/
## 10. Public registry API endpoints (2026)
| Registry | Endpoint(s) | Auth | Rate limit | Notes |
|---|---|---|---|---|
| **NPM** | `GET https://registry.npmjs.org/{package}` (full packument with all versions) <br> `GET https://registry.npmjs.org/{package}/{version}` (single version) | None for read | **Not publicly documented.** npm reserves the right to 429; honor `Retry-After`. | Stable for years; no migration on the horizon. Returns the `dist-tags.latest` field — use that for "latest stable". |
| **PyPI** | `GET https://pypi.org/pypi/{project}/json` (project metadata with `releases` map) <br> `GET https://pypi.org/pypi/{project}/{version}/json` (version-only, no `releases`) | None for read | Not formally documented; the user agent string matters. | The `releases` key is preserved on the project endpoint. Some fields (`downloads`, `has_sig`) are deprecated; ignore them. |
| **Go Modules** | `GET https://proxy.golang.org/{module}/@v/list` (newline-separated versions) <br> `GET https://proxy.golang.org/{module}/@latest` (latest version + time, JSON) <br> `GET https://proxy.golang.org/{module}/@v/{version}.info` (single version JSON) | None | Generous; Google-operated CDN. | Module paths must be **lower-cased and `!`-escaped** for capitals (`github.com/Foo/Bar` → `github.com/!foo/!bar`). Important pitfall — flag in roadmap. |
| **GitHub Actions / tags** | `GET https://api.github.com/repos/{owner}/{repo}/tags` (paginated, 30/page, max 100) <br> `GET https://api.github.com/repos/{owner}/{repo}/releases` <br> `GET https://api.github.com/repos/{owner}/{repo}/releases/latest` | Optional (token boosts limit) | **60 req/hr unauthenticated**, 5,000/hr authenticated | The 60/hr limit is the dominant real-world failure mode. PROJECT.md already calls this out. Cache aggressively. |
| **Maven Central** | `GET https://search.maven.org/solrsearch/select?q=g:{group}+AND+a:{artifact}&core=gav&rows=200&wt=json` | None | Not documented | **Endpoint stability caveat:** The web UI was redirected from search.maven.org to central.sonatype.com in Feb 2023. Sonatype committed to keeping the REST API working ("API requests will continue to function ... we expect to support the API even after we've retired the web interface"). As of 2026 the legacy `/solrsearch/select` endpoint is still operational, but treat this as the **most fragile** registry — wrap the call so it can be swapped for a `central.sonatype.com` equivalent without touching call sites. |
- **NPM, PyPI, Go proxy, GitHub:** HIGH — verified against current official docs.
- **Maven:** MEDIUM — the API works but the long-term endpoint is the least settled. Build the abstraction to swap.
## Installation
# Core
# Test
# Release-time (not a Go dep)
## Alternatives Considered
| Recommended | Alternative | When to Use Alternative |
|---|---|---|
| `modelcontextprotocol/go-sdk` | `mark3labs/mcp-go` | Stuck on Go <1.25, or you specifically need features the official SDK is slow to land. Neither applies. |
| `hashicorp/golang-lru/v2` | `dgraph-io/ristretto` | High-cardinality cache with millions of keys, or sustained high-throughput workload. Doesn't apply. |
| `hashicorp/golang-lru/v2` | `patrickmn/go-cache` | Never. Unmaintained and unbounded. |
| `golang.org/x/mod/semver` | `Masterminds/semver` | If a future tool needs constraint ranges (`^1.2`, `~1.2.3`). v2+ scope. |
| `log/slog` | `rs/zerolog` / `uber-go/zap` | Hot path emits >10k log/s. Single-user MCP server never approaches this. |
| `goldie/v2` | `bradleyjkemp/cupaloy` | Never. cupaloy is dormant. |
| `goldie/v2` | hand-rolled | Never (unless zero-deps is a hard rule, which here it isn't). |
## What NOT to Use
| Avoid | Why | Use Instead |
|---|---|---|
| `patrickmn/go-cache` | Abandoned since 2017; unbounded growth in a long-running daemon | `hashicorp/golang-lru/v2/expirable` |
| `bradleyjkemp/cupaloy` | Dormant since Sept 2022; byte-only diffs over JSON | `sebdah/goldie/v2` |
| `Masterminds/semver/v3` for v1 | Pulls in constraint-range machinery you don't use; adds binary size | `golang.org/x/mod/semver` |
| `zerolog` / `zap` | No measurable benefit at this scale; violates "stdlib + 1 dep" posture | `log/slog` |
| `go-resty/resty` / `imroc/req` | Project constraint forbids; net/http suffices | `net/http` |
| Pre-v1 `go-sdk` API (`server.NewServer(...)` from `.../go-sdk/server`) | Outdated package layout from before v1.x | `mcp.NewServer(...)` from `.../go-sdk/mcp` |
| `.dxt` references in docs/manifest | Renamed to `.mcpb` Nov 2025 | `.mcpb` exclusively |
| Writing to stdout for **anything** other than MCP protocol traffic | Corrupts JSON-RPC framing; agent silently misbehaves | All logging → `os.Stderr` via slog |
## Stack Patterns by Variant
- Add `golang.org/x/oauth2` (already pulled in transitively by the MCP SDK; reuse).
- For npm `.npmrc` parsing, the simplest path is hand-rolled INI-ish parsing; no library is worth pulling in.
- Swap `golang.org/x/mod/semver` for `Masterminds/semver/v3` at that point only. The wrapping should be small and localized to the version-comparison module.
- `hashicorp/golang-lru/v2` exposes `Stats`-style methods. Stick with it; do not switch to ristretto for metrics alone.
## Version Compatibility Matrix
| Component | Version | Compatible With | Notes |
|---|---|---|---|
| Go toolchain | ≥ 1.25.0 | go-sdk v1.6.0+ | **Hard floor — PROJECT.md must be updated from 1.22+.** |
| `go-sdk` | v1.6.0 | Go 1.25+ | v1.x is API-stable. |
| `golang-lru/v2` | v2.0.7 | Go 1.18+ (generics) | Comfortably in range. |
| `x/mod/semver` | v0.36.0 | Go 1.23+ | Comfortably in range. |
| `goldie/v2` | v2.8.0 | Go 1.18+ | Comfortably in range. |
| MCPB manifest | v0.3 | Claude Desktop ≥ 1.0.0 | Verify against current MANIFEST.md before Phase 5. |
## Sources
- **`/modelcontextprotocol/go-sdk` (Context7)** — minimal stdio server snippets, v1.x API (HIGH confidence)
- **github.com/modelcontextprotocol/go-sdk** — releases page (v1.6.0 May 2026), `go.mod` declares `go 1.25.0` (HIGH)
- **github.com/mark3labs/mcp-go** — v0.52.0, alternative comparison (HIGH)
- **go.dev/doc/devel/release** — Go 1.25.x and 1.26.x supported in 2026; 1.24 in security-only; 1.23 EOL (HIGH)
- **pkg.go.dev/log/slog** — stdlib structured logging, available since 1.21 (HIGH)
- **github.com/hashicorp/golang-lru** — v2.0.7 with `expirable` subpackage (HIGH)
- **pkg.go.dev/golang.org/x/mod/semver** — v0.36.0 API (HIGH)
- **github.com/Masterminds/semver** — v3.5.0 features for the "alternative" comparison (HIGH)
- **github.com/sebdah/goldie** — v2.8.0 (Oct 2025) (HIGH)
- **github.com/bradleyjkemp/cupaloy** — dormant since Sept 2022 (HIGH)
- **github.com/modelcontextprotocol/mcpb / anthropics/mcpb** — MANIFEST.md, CLI usage (`mcpb init`, `mcpb pack`) (MEDIUM-HIGH)
- **blog.modelcontextprotocol.io / Anthropic — DXT → MCPB rename Nov 2025** (HIGH)
- **goreleaser.com / goreleaser/goreleaser-action** — v2.15.4 GoReleaser (HIGH); MCPB integration is custom glue (MEDIUM)
- **docs.npmjs.com / github.com/npm/registry** — endpoints; rate limit policy exists but is not numerically documented (HIGH on endpoints, LOW on numbers)
- **docs.pypi.org/api/json/** — `/pypi/{project}/json` and `/pypi/{project}/{version}/json` endpoints (HIGH)
- **go.dev/ref/mod#goproxy-protocol** — full GOPROXY endpoint spec (`@v/list`, `@latest`, `@v/{v}.info`) (HIGH)
- **docs.github.com REST API** — `/repos/{owner}/{repo}/tags`, `/releases`, `/releases/latest`; 60/hr unauthenticated, 5000/hr authenticated (HIGH)
- **central.sonatype.org/faq/what-happened-to-search-maven-org/** — search.maven.org REST API still operational, web UI deprecated (MEDIUM — long-term endpoint stability is the weakest claim in this document)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
