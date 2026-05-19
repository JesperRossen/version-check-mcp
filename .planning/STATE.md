---
gsd_state_version: 1.0
milestone: v1.0.0
milestone_name: "**Goal**: The author uses the released binary daily through Claude Desktop, validates the wedge against real version-hallucination workflows, and tags v1.0.0 once the dogfood window is stable."
status: executing
last_updated: "2026-05-19T12:02:32.826Z"
progress:
  total_phases: 6
  completed_phases: 2
  total_plans: 5
  completed_plans: 5
  percent: 33
---

# State: Version Check MCP

**Updated:** 2026-05-12 (initial)

## Project Reference

**Core Value:** When an AI agent asks "does this package version exist?", the server returns a correct answer in under 20ms with useful alternatives if it doesn't.

**Current Focus:** Phase 04 — alternatives-response-shape-hardening

## Current Position

Phase: 04 (alternatives-response-shape-hardening) — EXECUTING
Plan: 3 of 3 (COMPLETE)

- **Milestone:** v1
- **Phase:** 4
- **Plan:** 04-03 complete
- **Status:** Phase 04 all 3 plans complete
- **Progress:** `[░░░░░░░░░░] 0% (0/6 phases)`

## Performance Metrics

| Metric | Target | Current |
|--------|--------|---------|
| Cold start | <20ms | not measured |
| Direct dependencies | 4 | 0 (no code yet) |
| v1 requirement coverage | 40/40 | 40/40 in roadmap, 0/40 implemented |
| Phases complete | 6 | 0 |

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
- [ ] Run `/gsd-plan-phase 1` to decompose Phase 1 into plans

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

- **Last action:** Phase 04, Plan 03 complete — alternatives wired into miss path + cross-registry shape audit test
- **Next action:** `/gsd-plan-phase 5` — plan Phase 5 (NPM + PyPI + GoMod registry adapters or similar)
- **Files of record:**
  - `.planning/PROJECT.md`
  - `.planning/REQUIREMENTS.md`
  - `.planning/ROADMAP.md`
  - `.planning/research/` (STACK, FEATURES, ARCHITECTURE, PITFALLS, SUMMARY)

---
*State initialized: 2026-05-12*
