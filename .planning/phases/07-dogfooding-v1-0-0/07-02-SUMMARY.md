---
phase: 07-dogfooding-v1-0-0
plan: "02"
subsystem: documentation
tags: [readme, agent-setup, mcp-config, github-token]
dependency_graph:
  requires: [07-01]
  provides: [multi-agent-setup-docs]
  affects: [README.md]
tech_stack:
  added: []
  patterns: [mcpb-install, stdio-mcp, json-config, toml-config]
key_files:
  created: []
  modified:
    - README.md
decisions:
  - "Agent Setup section added in-place in README (no docs/ subdirectory per CONTEXT.md §A)"
  - "Old 'Claude Desktop configuration' section folded into new Agent Setup structure (no duplicate)"
  - "Codex CLI and Gemini CLI marked as unverified-by-author throughout"
  - "GITHUB_TOKEN warning appears in every agent section (threat T-07-02-01 mitigated)"
metrics:
  duration: "~5 minutes"
  completed: "2026-05-21"
  tasks_completed: 1
  tasks_total: 1
  files_changed: 1
---

# Phase 07 Plan 02: Agent Setup Documentation Summary

**One-liner:** Complete multi-agent README setup docs covering Claude Desktop, Claude Code CLI, OpenCode, Codex CLI, and Gemini CLI with GITHUB_TOKEN guidance and verification status labels.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Write ## Agent Setup section in README.md | fd15dfa | README.md |

## What Was Built

Added a complete `## Agent Setup` section to README.md with five agent subsections:

- **Claude Desktop** - MCPB one-click reference + manual `claude_desktop_config.json` config. Verified.
- **Claude Code CLI** - Project `.mcp.json` config + global `~/.claude/claude.json` alternative. Verified from repo's own `.mcp.json`.
- **OpenCode** - `opencode.json`/`opencode.jsonc` project and global config formats. Verified from author's config.
- **Codex CLI** - TOML config at `~/.codex/config.toml` + `codex mcp add` shortcut. **Unverified by author** - based on official docs.
- **Gemini CLI** - JSON config at `~/.gemini/settings.json` + `gemini mcp add` shortcut. **Unverified by author** - based on official docs.

Each section includes:
- Exact config block from CONTEXT.md §B
- GITHUB_TOKEN env var callout from CONTEXT.md §D
- Explicit "Do not commit token values" warning
- Verification status label

The old standalone "Claude Desktop configuration" section was removed and its content merged into the new structure (no duplicate).

## Verification Results

| Check | Expected | Actual | Pass? |
|-------|----------|--------|-------|
| `grep -c "## Agent Setup" README.md` | 1 | 1 | YES |
| `grep -c "### Claude Desktop" README.md` | 1 | 1 | YES |
| `grep -c "### Claude Code CLI" README.md` | 1 | 1 | YES |
| `grep -c "### OpenCode" README.md` | 1 | 1 | YES |
| `grep -c "### Codex CLI" README.md` | 1 | 1 | YES |
| `grep -c "### Gemini CLI" README.md` | 1 | 1 | YES |
| `grep -c "GITHUB_TOKEN" README.md` | >=5 | 7 | YES |
| `grep -c "not personally tested" README.md` | >=2 | 4 | YES |
| `grep -c "Do not commit" README.md` | >=1 | 5 | YES |
| `grep -c "Claude Desktop configuration" README.md` | 0 | 0 | YES |

## Deviations from Plan

None - plan executed exactly as written.

## Threat Mitigations Applied

| Threat ID | Mitigation |
|-----------|------------|
| T-07-02-01 | "Do not commit token values" warning added to every agent section (5 occurrences) |

## Known Stubs

None.

## Self-Check: PASSED

- README.md: exists and contains all required sections
- Commit fd15dfa: exists in git log
- All acceptance criteria verified with grep commands
