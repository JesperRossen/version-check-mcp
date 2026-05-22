# Changelog

All notable changes to version-check-mcp are documented here.

## [0.0.2] - 2026-05-23

### Fixed
- GoReleaser: replace deprecated `brews` key with `homebrew_casks` (GoReleaser v2 removed the old key)
- GoReleaser: expose `HOMEBREW_TAP_GITHUB_TOKEN` to the release step env so the tap is actually updated
- README: update Homebrew install to `brew install --cask` to match the cask format GoReleaser generates
- Integration test `TestStdio_NPM_Validate_Miss`: align with design decision D-MISS-01 (version misses return a success-shaped response with `exists=false` and alternatives, not an error envelope)

## [0.0.1] - 2026-05-22

### Added
- MCP server over stdio: `validate_version` and `get_latest_version` tools with LLM-readable schemas
- NPM registry adapter: full packument fetch, scoped packages (`@scope/pkg`), `dist-tags.latest`
- PyPI adapter: PEP 440 normalization, yanked-release detection
- Go Modules adapter: `+incompatible` suffix preservation, pseudo-version prerelease classification, `golang.org/x/mod` escape for capital letters in module paths
- GitHub Actions adapter: `/tags` listing + `/releases/latest` hint, `rate_limited` on 403+`X-RateLimit-Remaining: 0`
- Maven Central adapter: `group:artifact` parsing, group-path URL encoding, SNAPSHOT filter
- Alternatives on miss: 3-5 ranked alternatives (`latest_stable`, `nearest_semver`, `latest_in_major`) on every `validate_version` miss
- In-memory LRU+TTL cache (hashicorp/golang-lru/v2/expirable) with singleflight dedup
- Multi-arch GoReleaser release: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64 - CGO_ENABLED=0 static binaries
- MCPB bundle (`version-check-mcp.mcpb`) for one-click Claude Desktop install
- Agent setup docs: Claude Desktop, Claude Code CLI, OpenCode (verified), Codex CLI, Gemini CLI (from official docs)
- GITHUB_TOKEN env var guidance for 5000 req/hr GitHub API rate limit
- Structured JSON logging to stderr (`log/slog`); stdout reserved for MCP protocol traffic only

### Fixed
- `npm.go`: HTTP response body bounded with `io.LimitReader` (32 MiB) to prevent memory exhaustion on large packuments
- `nearest.go`: `NearestVersions` result capped at 5 entries with explicit guard; doc-comment updated to match

### Notes
- Go 1.25+ required (floor set by `github.com/modelcontextprotocol/go-sdk`)
- Four direct dependencies: `modelcontextprotocol/go-sdk`, `hashicorp/golang-lru/v2`, `golang.org/x/sync`, `golang.org/x/mod`
- First public tag - validation ongoing through daily use; v1.0.0 reserved for battle-tested release
