---
slug: homebrew-mise-distribution
date: 2026-05-22
status: complete
---

# Quick Task: Homebrew tap + mise/ubi distribution

Add frictionless install methods for non-Claude-Desktop users so they don't have to manually download and path a binary.

## Scope

- Add Homebrew tap support via GoReleaser `brews:` stanza
- Add mise/ubi install instructions (zero publishing cost - uses existing GitHub Releases)
- Add `--version` flag to binary (required for `brew test`)
- Update README: new Installation section with all three paths (MCPB, Homebrew, mise), Agent Setup intro explaining PATH assumption, replace all `/path/to/version-check-mcp` with `version-check-mcp`
- Scaffold `JesperRossen/homebrew-tap` repo (main branch, Formula/ dir, README)

## Files changed

- `cmd/version-check-mcp/main.go` - added `--version` flag
- `.goreleaser.yaml` - added `brews:` stanza pointing to `JesperRossen/homebrew-tap`
- `README.md` - restructured Installation + Agent Setup sections

## Remaining manual step

Add `HOMEBREW_TAP_GITHUB_TOKEN` (PAT with `repo` scope on `JesperRossen/homebrew-tap`) to the main repo's GitHub Actions secrets.
