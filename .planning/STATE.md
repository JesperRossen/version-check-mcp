---
gsd_state_version: 1.0
milestone: v1.0.0
milestone_name: "**Goal**: The author uses the released binary daily through Claude Desktop, validates the wedge against real version-hallucination workflows, and tags v1.0.0 once the dogfood window is stable."
status: executing
last_updated: "2026-05-19T00:00:00.000Z"
progress:
  total_phases: 6
  completed_phases: 5
  total_plans: 20
  completed_plans: 20
  percent: 83
---

# State: Version Check MCP

**Updated:** 2026-05-19

## Project Reference

**Core Value:** When an AI agent asks "does this package version exist?", the server returns a correct answer in under 20ms with useful alternatives if it doesn't.

**Current Focus:** Phase 5 — distribution

## Current Position

Phase: 05 (distribution) — COMPLETE
Plan: 2 of 2 (COMPLETE)

- **Milestone:** v1
- **Phase:** 6 (not yet planned)
- **Plan:** Not started
- **Status:** Ready to plan
- **Progress:** [████████░░] 83%

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

- [ ] Update PROJECT.md Go version constraint from `1.22+` to `1.25+`
- [ ] Fix code review MJ-01: `tools_shape_test.go` sends `"package"` key but handler expects `"pkg"`
- [ ] Fix code review MJ-03: `latest_in_major` candidate loop skips `v == normLatest` too early
- [ ] Fix code review MJ-04: PyPI `Versions()` includes yanked releases

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

## Session Continuity

- **Last action:** Phase 05 complete — GoReleaser config (5 targets, CGO=0, SHA256), tzdata embed, release.yml (tag-triggered GoReleaser+MCPB), README install+MCPB sections
- **Next action:** Plan Phase 6 with `/gsd-plan-phase 6`
- **Files of record:**
  - `.planning/PROJECT.md`
  - `.planning/REQUIREMENTS.md`
  - `.planning/ROADMAP.md`
  - `.planning/research/` (STACK, FEATURES, ARCHITECTURE, PITFALLS, SUMMARY)

---
*State initialized: 2026-05-12*
