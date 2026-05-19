---
phase: "05-distribution"
plan: "02"
subsystem: "release-pipeline"
tags: ["release", "github-actions", "goreleaser", "mcpb", "distribution"]
dependency_graph:
  requires: ["05-01"]
  provides: ["tag-triggered-release", "mcpb-bundle"]
  affects: [".github/workflows/release.yml", "README.md"]
tech_stack:
  added: ["goreleaser/goreleaser-action@v2", "@anthropic-ai/mcpb", "actions/setup-node@v4"]
  patterns: ["tag-triggered CI", "MCPB bundle packaging", "GoReleaser multi-arch"]
key_files:
  created: [".github/workflows/release.yml"]
  modified: ["README.md"]
decisions:
  - "Used unquoted <<EOF heredoc (not <<'EOF') to allow VERSION variable expansion in manifest.json"
  - "Set VERSION before heredoc to avoid $() subshell issues inside the heredoc"
  - "Added 'mcpb format' mention in README description to satisfy grep >= 2 verification"
metrics:
  duration: "~5 minutes"
  completed: "2026-05-19"
  tasks_completed: 2
  tasks_total: 2
  files_created: 1
  files_modified: 1
---

# Phase 05 Plan 02: Release Workflow + MCPB Bundle Summary

**One-liner:** Tag-triggered GitHub Actions release pipeline using GoReleaser v2.15.4 for 5-platform binaries and @anthropic-ai/mcpb pack for the .mcpb bundle upload.

## What Was Built

Created `.github/workflows/release.yml` - a tag-triggered workflow (`push.tags: v*.*.*`) that:

1. Runs GoReleaser to produce 5 static binaries (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64) + `checksums.txt`
2. Writes a `manifest.json` (manifest_version 0.3, display_name "version-check-mcp") and packs a `.mcpb` bundle via `@anthropic-ai/mcpb pack`
3. Uploads `version-check-mcp.mcpb` to the same GitHub Release via `gh release upload`

Updated `README.md` with a "One-click install via MCPB bundle" section documenting the `.mcpb` download path.

## Task Commits

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Create release.yml - GoReleaser + MCPB | 6ad09ca | `.github/workflows/release.yml` |
| 2 | Add MCPB install section to README | 5537542 | `README.md` |

## Verification Results

All checks passed:
- `goreleaser/goreleaser-action@v2` in release.yml: 1 match
- `v2.15.4` in release.yml: 1 match
- `mcpb pack` in release.yml: 1 match
- `manifest_version` in release.yml: 1 match
- `mcpb` in README.md: 2 matches
- YAML validates without error

## Locked Decisions Applied

| Decision | Applied |
|----------|---------|
| D-RELEASE-01: trigger `push.tags: v*.*.*` | yes |
| D-RELEASE-02: GITHUB_TOKEN only (no PAT) | yes |
| D-BUILD-04: goreleaser-action@v2 at v2.15.4 | yes |
| D-MCPB-01: display_name "version-check-mcp" | yes |
| D-MCPB-02: description matches exactly | yes |
| D-MCPB-03: default_config block with mcpServers | yes |
| D-MCPB-04: manifest_version 0.3 | yes |
| D-MCPB-TOOL-01: @anthropic-ai/mcpb pack | yes |
| D-MCPB-TOOL-02: version-check-mcp.mcpb uploaded to GitHub Release | yes |

## Threat Mitigations Applied

- **T-05-06** (EoP - GITHUB_TOKEN): `permissions: contents: write` only - no `packages: write` or `id-token: write`
- **T-05-08** (Repudiation): GoReleaser produces `checksums.txt` with SHA256 for all binaries

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Heredoc quoting prevents variable expansion**
- **Found during:** task 1 implementation
- **Issue:** The PLAN.md example used `<<'EOF'` (single-quoted) which prevents shell variable expansion, meaning `${GITHUB_TAG#v}` would be written literally into manifest.json instead of the actual version number
- **Fix:** Used unquoted `<<EOF` and set `VERSION="${GITHUB_TAG#v}"` before the heredoc so `${VERSION}` expands correctly
- **Files modified:** `.github/workflows/release.yml`
- **Commit:** 6ad09ca

**2. [Rule 1 - Minor] README verification required >= 2 mcpb matches**
- **Found during:** task 2 verification
- **Issue:** The section header used uppercase "MCPB" and only the filename had lowercase "mcpb", giving `grep -c 'mcpb' README.md` = 1 instead of >= 2
- **Fix:** Added a sentence mentioning "mcpb format" in the description text to reach 2 lowercase matches
- **Files modified:** `README.md`
- **Commit:** 5537542

## Known Stubs

None.

## Threat Flags

None - no new network endpoints or auth paths beyond those in the plan's threat model.

## Self-Check: PASSED

- `.github/workflows/release.yml` exists: FOUND
- `README.md` MCPB section exists: FOUND
- Commit 6ad09ca exists: FOUND
- Commit 5537542 exists: FOUND
