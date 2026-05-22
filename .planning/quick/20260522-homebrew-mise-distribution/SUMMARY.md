---
slug: homebrew-mise-distribution
date: 2026-05-22
status: complete
---

# Summary: Homebrew tap + mise/ubi distribution

## What was done

Added two frictionless install paths for non-Claude-Desktop users.

**`cmd/version-check-mcp/main.go`**
- Added `--version` bool flag that prints `version.Version` to stdout and exits 0
- Required for `brew test` to verify the installed binary

**`.goreleaser.yaml`**
- Added `brews:` stanza targeting `JesperRossen/homebrew-tap` repo, `Formula/` directory
- Uses `{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}` for push auth
- `install: bin.install "version-check-mcp"`
- `test: assert_match version.to_s, shell_output("#{bin}/version-check-mcp --version")`

**`README.md`**
- Restructured Installation section: MCPB (Claude Desktop), Homebrew, mise, Manual
- New Agent Setup intro explaining PATH assumption and xattr quarantine note
- Replaced all `/path/to/version-check-mcp` references with `version-check-mcp`
- Fixed stale anchor link in Claude Desktop agent section

**`JesperRossen/homebrew-tap` (external repo)**
- Created and pushed main branch with `Formula/.gitkeep` and README

## Pending

- Add `HOMEBREW_TAP_GITHUB_TOKEN` to main repo GitHub Actions secrets (PAT, `repo` scope on tap repo)
- Formula will be auto-published on next GoReleaser release
