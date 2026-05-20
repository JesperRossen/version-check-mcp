---
gsd_state_version: 1.0
milestone: v1.0.0
milestone_name: "**Goal**: The author uses the released binary daily through Claude Desktop, validates the wedge against real version-hallucination workflows, and tags v1.0.0 once the dogfood window is stable."
status: executing
last_updated: "2026-05-19T00:00:00.000Z"
progress:
  total_phases: 7
  completed_phases: 5
  total_plans: 20
  completed_plans: 20
  percent: 71
---

# State: Version Check MCP

**Updated:** 2026-05-20

## Project Reference

**Core Value:** When an AI agent asks "does this package version exist?", the server returns a correct answer in under 20ms with useful alternatives if it doesn't.

**Current Focus:** Phase 5 — distribution

## Current Position

Phase: 06 (code-review-cleanup) — COMPLETE
Plan: 2 of 2 (COMPLETE)

- **Milestone:** v1
- **Phase:** 6 (complete — 2 plans)
- **Plan:** 2 complete, 0 remaining
- **Status:** Executing (Phase 7 next)
- **Progress:** [█████████░] 85%

## Performance Metrics

| Metric | Target | Current |
|--------|--------|---------|
| Cold start | <20ms | not measured |
| Direct dependencies | 4 | 0 (no code yet) |
| v1 requirement coverage | 40/40 | 40/40 in roadmap, 0/40 implemented |
| Phases complete | 6 | 5 |

## Accumulated Context

### Decisions (from PROJECT.md, pending implementation)

- Go 1.25+ floor (MCP SDK requirement, supersedes earlier 1.22 in PROJECT.md — needs update)
- Minimum dependency footprint: `modelcontextprotocol/go-sdk`, `hashicorp/golang-lru/v2`, `golang.org/x/sync`, `golang.org/x/mod`
- Stdlib-only test fixture comparison (no `goldie`, `cupaloy`)
- Maven IDs as `group:artifact` strings
- Single global cache TTL via CLI flag
- Prereleases hidden by default, per-call opt-in
- KindNotFound intercepted in validateRawHandler; routes to success-shaped miss with alternatives (D-MISS-01)
- buildMissResponse uses Versions+Latest (cache hits) then filter.NearestVersions for alternatives assembly
- vPrefixed detection: reg.Name() == "gomod" || reg.Name() == "gh"

### Open Todos

- [x] Update PROJECT.md Go version constraint from `1.22+` to `1.25+` — DONE (PROJECT.md already correct; confirmed Go 1.25+ in Constraints section)
- [x] Fix code review MJ-01: `tools_shape_test.go` sends `"package"` key but handler expects `"pkg"` — CONFIRMED FIXED (test already uses "pkg"; grep confirms no "package" key in tools_shape_test.go)
- [x] Fix code review MJ-03: `latest_in_major` candidate loop skips `v == normLatest` too early — CONFIRMED FIXED (inResult dedup handles it; all 13 TestNearestVersions sub-tests pass)
- [x] Fix code review MJ-04: PyPI `Versions()` includes yanked releases — CONFIRMED FIXED (Versions() already filters yanked per PEP 592; TestLatest_YankedVersionSkipped passes)

### Blockers

None.

### Key Pitfalls to Remember (from research)

- **Stdout pollution** = silent MCP disconnect. Single chokepoint + binary-level test in Phase 1.
- **PEP 440 normalization** for PyPI — string equality is wrong.
- **Go `+incompatible` / pseudo-versions** — preserve verbatim, classify pseudo as prerelease.
- **GitHub 60/hr unauthenticated rate limit** — cache aggressively, surface `rate_limited` distinctly.
- **NPM scoped packages** — URL-encode the `/` inside the name segment (`@types%2Fnode`).
- **Maven group path** — dots become slashes in URL (`org.springframework` → `org/springframework/`).
- **Cache must be tiered** — successes full TTL, 404s short TTL, 5xx never cached as success.

### Performance Audit (Phase 6)

**Date:** 2026-05-20
**Scope:** All 5 adapters + internal/filter/nearest.go

**Findings applied:**
- `nearest.go NearestVersions`: pre-sized `candidates` slice with `make([]string, 0, len(versions))` (was unbounded `var candidates []string`)
- `nearest.go NearestVersions`: pre-sized `byDist` slice with `make([]ranked, 0, len(candidates))` (was unbounded `var byDist []ranked`)

**Findings documented (out-of-scope for v1):**
- PyPI `Validate()` iterates `Releases` map with normalized key comparison (O(n) scan) — acceptable for typical package sizes (<1000 versions); a lookup map could speed this up but adds complexity not warranted at this scale
- `nearest.go` `parseParts` is called 2x per candidate in same-minor/same-major tiers (once for target, once per candidate) — memoizing target parse could help but n < 1000 in practice

**Dependency audit:**
- go-sdk: Required (MCP protocol — removing requires hand-rolling JSON-RPC + MCP spec)
- golang-lru/v2: Required (bounded LRU+TTL cache, CACHE-01..04 — stdlib alternatives require hand-rolling eviction)
- x/sync: Required (singleflight in cache.GetOrLoad — confirmed by `internal/cache/cache.go:22`)
- x/mod: Required (semver comparison, module.IsPseudoVersion in gomod/filter, modfile in depcheck — confirmed by grep)
- All 4 direct deps confirmed necessary — DEP-01 invariant holds

## Session Continuity

- **Last action:** Phase 06 complete — code review PASS_WITH_NOTES (2 warnings noted in 06-REVIEW.md); all tests pass; ROADMAP.md updated
- **Next action:** Phase 07 (Dogfooding & v1.0.0)

### Phase 06 Code Review Notes (PASS_WITH_NOTES)

See `.planning/phases/06-code-review-cleanup/06-REVIEW.md` for full findings.

- **WARNING** `npm.go:51` — Unbounded HTTP response body passed to JSON decoder; large packages (e.g. `@types/node` ~10 MiB) could exhaust memory. Fix: wrap with `io.LimitReader(resp.Body, 32<<20)` before passing to `parsePackument`. Deferred to Phase 7 or hotfix.
- **WARNING** `nearest.go:27` — Doc-comment says "at most 5" but result is never trimmed. Currently capped at 3 in practice; tighten doc-comment or add `if len(result) > 5 { result = result[:5] }`. Deferred to Phase 7 or hotfix.

### Roadmap Evolution

- Phase 6 added: Code Review & Cleanup (inserted before Dogfooding; old Phase 6 renumbered to Phase 7)
- **Files of record:**
  - `.planning/PROJECT.md`
  - `.planning/REQUIREMENTS.md`
  - `.planning/ROADMAP.md`
  - `.planning/research/` (STACK, FEATURES, ARCHITECTURE, PITFALLS, SUMMARY)

---
*State initialized: 2026-05-12*
