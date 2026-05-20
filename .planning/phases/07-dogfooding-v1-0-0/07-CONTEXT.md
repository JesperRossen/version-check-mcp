# Phase 7 Context: Dogfooding, Multi-Agent Setup & v1.0.0

**Discussed:** 2026-05-20
**Phase goal:** Wire the released binary into Claude Desktop + Claude Code CLI + OpenCode; document setup for Codex CLI and Gemini CLI; run a ≥7-day dogfood window; tag v1.0.0.

---

## Locked Decisions

### A: Documentation structure

**Decision:** Expand README in-place — no `docs/` subdirectory.

Add a new top-level section `## Agent Setup` with subsections per agent:
- `### Claude Desktop` (MCPB one-click + manual JSON config — already present, verify current)
- `### Claude Code CLI`
- `### OpenCode`
- `### Codex CLI`
- `### Gemini CLI`
- Optional shared subsection: `## GitHub rate limits (optional)` or inline in each agent section.

The existing README "Claude Desktop configuration" section can be folded into the new Agent Setup structure.

---

### B: Agent config formats (all verified from official docs / local config)

**Claude Desktop:**
- Install via MCPB bundle (one-click, recommended) — already documented in README.
- Manual config in `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS):
  ```json
  {
    "mcpServers": {
      "version-check-mcp": {
        "command": "/path/to/version-check-mcp"
      }
    }
  }
  ```

**Claude Code CLI (verified from `.mcp.json` in this repo):**
- Project-scoped config at `.mcp.json` in project root.
- Format: `{"mcpServers": {"name": {"command": "...", "args": [...], "type": "stdio"}}}`
- Example:
  ```json
  {
    "mcpServers": {
      "version-check-mcp": {
        "command": "/path/to/version-check-mcp",
        "type": "stdio"
      }
    }
  }
  ```
- Global config alternative: `~/.claude/claude.json` (same schema).
- Verification status: **Verified** (confirmed from `.mcp.json` in this repo).

**OpenCode (verified from `~/.config/opencode/opencode.jsonc`):**
- Config at `opencode.json` or `opencode.jsonc` at project root, or `~/.config/opencode/opencode.json` globally.
- Format: `{"mcp": {"name": {"type": "local", "command": ["binary", "args..."]}}}`
- Example:
  ```json
  {
    "mcp": {
      "version-check-mcp": {
        "type": "local",
        "command": ["/path/to/version-check-mcp"]
      }
    }
  }
  ```
- Verification status: **Verified** (confirmed from author's `~/.config/opencode/opencode.jsonc`).

**Codex CLI (from official docs — not personally tested by author):**
- Config at `~/.codex/config.toml` (global) or `.codex/config.toml` (project-scoped, trusted projects only).
- Format: TOML.
- Example:
  ```toml
  [mcp_servers.version-check-mcp]
  command = "/path/to/version-check-mcp"
  ```
- CLI shortcut: `codex mcp add version-check-mcp -- /path/to/version-check-mcp`
- MCP support: Full, production-quality as of 2026.
- Verification status: **Unverified by author** — based on official Codex CLI docs. Note this in README.

**Gemini CLI (from official docs — not personally tested by author):**
- Config at `~/.gemini/settings.json` (global).
- Format: JSON.
- Example:
  ```json
  {
    "mcpServers": {
      "version-check-mcp": {
        "command": "/path/to/version-check-mcp"
      }
    }
  }
  ```
- CLI shortcut: `gemini mcp add version-check-mcp /path/to/version-check-mcp`
- MCP support: Full, production-quality as of 2026.
- Verification status: **Unverified by author** — based on official Gemini CLI docs. Note this in README.

---

### C: Phase 6 deferred code fixes

Both deferred items from the Phase 6 code review must be fixed in the 07-01 preflight plan, before proceeding with the dogfood window.

**Fix 1 — `npm.go:51` (safety, unbounded HTTP body):**
- Problem: Response body passed directly to JSON decoder; large packages (~10 MiB) can exhaust memory.
- Fix: Wrap with `io.LimitReader(resp.Body, 32<<20)` before passing to `parsePackument`.
- Priority: Real correctness/safety issue — fix before v1.0.0.

**Fix 2 — `nearest.go:27` (correctness, doc-comment mismatch):**
- Problem: Doc-comment says "at most 5" but result is never trimmed; currently capped at 3 in practice.
- Fix: Either tighten the doc-comment to say "at most 3" or add `if len(result) > 5 { result = result[:5] }`.
- Priority: Low-risk cosmetic, but dishonest API contract — fix before v1.0.0.

---

### D: GITHUB_TOKEN env var guidance

Include a `> **Optional:** Increase GitHub rate limit` callout **within each agent's config section** in the README.

GitHub Actions adapter is 60 req/hr unauthenticated, 5000/hr with a token. Show how to pass the token as an env var for each agent:

- **Claude Code CLI** (`.mcp.json`):
  ```json
  {
    "mcpServers": {
      "version-check-mcp": {
        "command": "/path/to/version-check-mcp",
        "type": "stdio",
        "env": { "GITHUB_TOKEN": "ghp_..." }
      }
    }
  }
  ```
- **OpenCode** (`opencode.json`):
  ```json
  "version-check-mcp": {
    "type": "local",
    "command": ["/path/to/version-check-mcp"],
    "env": { "GITHUB_TOKEN": "ghp_..." }
  }
  ```
- **Codex CLI** (`config.toml`):
  ```toml
  [mcp_servers.version-check-mcp.env]
  GITHUB_TOKEN = "ghp_..."
  ```
- **Gemini CLI** (`settings.json`):
  ```json
  "env": { "GITHUB_TOKEN": "$GITHUB_TOKEN" }
  ```

Note: Gemini CLI supports shell variable expansion (`$VAR`), others set literal values. Warn users not to commit token values to `.mcp.json` or `opencode.json` — prefer a secrets manager or environment-level injection.

---

### E: Plan renumbering strategy

The 4 existing PLAN.md files predate the scope expansion and must be renumbered before re-planning.

**Pre-plan cleanup (do before `/gsd-plan-phase 7` runs):**
1. Rename `07-03-PLAN.md` (old dogfood window plan) → `07-04-PLAN.md`
2. Rename `07-04-PLAN.md` (old v1.0.0 release plan) → `07-05-PLAN.md`
3. Overwrite/replace `07-03-PLAN.md` with the new multi-agent setup docs plan (produced by the planner).
4. Update internal `plan:` frontmatter numbers in the renamed files.

**Final 5-plan structure:**
- `07-01-PLAN.md` — Pre-flight: fix deferred items (C), verify tests + build smoke test, DOGFOOD.md template
- `07-02-PLAN.md` — Install MCPB into Claude Desktop, verify both tools visible (human checkpoint)
- `07-03-PLAN.md` — Write multi-agent setup docs (Claude Code CLI, OpenCode, Codex, Gemini), update README
- `07-04-PLAN.md` — Dogfood window ≥7 days, P0 tracker clean, author sign-off (human checkpoint)
- `07-05-PLAN.md` — CHANGELOG.md, push v1.0.0 tag, verify release pipeline (human checkpoint)

---

## What the Planner Should Know

### In scope
- README expansion with Agent Setup section (all 4 CLI agents + Claude Desktop)
- GITHUB_TOKEN guidance inline in each agent section
- Pre-flight code fixes (npm.go LimitReader, nearest.go doc-comment)
- 07-01 must verify all fixes and run full test suite before any dogfood work starts
- 07-03 is the net-new plan that didn't exist before - it writes all the README content
- DOGFOOD.md template for daily log entries

### Out of scope
- A `docs/` subdirectory — README only
- A global OpenCode config file — project-scoped is sufficient for docs example
- Per-agent GITHUB_TOKEN secrets management beyond the callout note
- Testing Codex CLI or Gemini CLI personally - doc them from official sources with a verification note
- Any new functionality / registry adapters

### Constraints reminder
- No new dependencies
- All tests must pass after preflight fixes
- stdout is sacred — zero non-protocol bytes (already guaranteed by existing infra)

---

## Deferred Ideas (not in scope for Phase 7)

- Cursor IDE MCP setup (uses same JSON format as Claude Desktop but in `.cursor/mcp.json`)
- Windsurf MCP setup
- Per-agent test scripts to verify MCP connectivity
- GITHUB_TOKEN secret injection via CI-style env management
- v1.1.0 scope items

---

*Context written: 2026-05-20*
