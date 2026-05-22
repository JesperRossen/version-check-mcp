# version-check-mcp
A Lightweight Version Validator for AI Coding Agents

## Installation

### Claude Desktop: one-click MCPB bundle (recommended)

Download `version-check-mcp.mcpb` from
[GitHub Releases](https://github.com/JesperRossen/version-check-mcp/releases/latest)
and open it - Claude Desktop installs the server automatically with a pre-filled
`mcpServers` configuration entry. No PATH setup required. The mcpb format bundles
all platform binaries into a single portable archive.

### Homebrew (macOS / Linux)

```sh
brew install JesperRossen/tap/version-check-mcp
```

This puts `version-check-mcp` on your PATH. Use `version-check-mcp` directly in
agent configs - no path required.

### mise (macOS / Linux / Windows)

If you use [mise](https://mise.jdx.dev):

```sh
mise use -g ubi:JesperRossen/version-check-mcp
```

Downloads the right binary for your platform directly from GitHub Releases and adds
it to your PATH via mise's shims. Works on all platforms including Windows. No
Homebrew required.

### Manual: download binary

Download the binary for your platform from
[GitHub Releases](https://github.com/JesperRossen/version-check-mcp/releases/latest),
place it somewhere on your PATH, and use the full path in agent configs.

### Verify checksum (recommended)

```sh
sha256sum -c checksums.txt
```

## Agent Setup

The server speaks MCP over stdio. Every agent that supports stdio MCP servers can use it.

The configs below use `version-check-mcp` directly, which works if the binary is on your
PATH (Homebrew or mise install). If you downloaded the binary manually, replace
`version-check-mcp` with the full path to the binary. On macOS, clear quarantine first:

```sh
xattr -d com.apple.quarantine /path/to/version-check-mcp
```

### Claude Desktop

Install via MCPB bundle (one-click, recommended) - see the [installation section](#claude-desktop-one-click-mcpb-bundle-recommended) above.

For manual configuration, add the following to your `claude_desktop_config.json`
(`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "version-check-mcp": {
      "command": "version-check-mcp"
    }
  }
}
```

Replace `version-check-mcp` with the full path if you installed the binary manually.

> **Optional:** Increase GitHub rate limit - Claude Desktop reads `GITHUB_TOKEN` from
> your shell environment automatically. Set it in your shell profile (e.g. `~/.zshrc`):
> `export GITHUB_TOKEN=ghp_...`
>
> **Warning:** Do not commit token values to `.mcp.json`, `opencode.json`, or any
> version-controlled config file. Use a secrets manager or environment-level injection instead.

**Verification status:** Verified.

---

### Claude Code CLI

Project-scoped config at `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "version-check-mcp": {
      "command": "version-check-mcp",
      "type": "stdio"
    }
  }
}
```

Global alternative: `~/.claude/claude.json` (same schema).

> **Optional:** Increase GitHub rate limit - add an `env` field to pass `GITHUB_TOKEN`:
>
> ```json
> {
>   "mcpServers": {
>     "version-check-mcp": {
>       "command": "version-check-mcp",
>       "type": "stdio",
>       "env": { "GITHUB_TOKEN": "ghp_..." }
>     }
>   }
> }
> ```
>
> **Warning:** Do not commit token values to `.mcp.json`, `opencode.json`, or any
> version-controlled config file. Use a secrets manager or environment-level injection instead.

**Verification status:** Verified (confirmed from `.mcp.json` in this repo).

---

### OpenCode

Config at `opencode.json` or `opencode.jsonc` at your project root, or
`~/.config/opencode/opencode.json` globally:

```json
{
  "mcp": {
    "version-check-mcp": {
      "type": "local",
      "command": ["version-check-mcp"]
    }
  }
}
```

> **Optional:** Increase GitHub rate limit - add an `env` field:
>
> ```json
> "version-check-mcp": {
>   "type": "local",
>   "command": ["version-check-mcp"],
>   "env": { "GITHUB_TOKEN": "ghp_..." }
> }
> ```
>
> **Warning:** Do not commit token values to `.mcp.json`, `opencode.json`, or any
> version-controlled config file. Use a secrets manager or environment-level injection instead.

**Verification status:** Verified (confirmed from author's `~/.config/opencode/opencode.jsonc`).

---

### Codex CLI

> **Note:** Based on official Codex CLI docs - not personally tested by the author.

Config at `~/.codex/config.toml` (global) or `.codex/config.toml` (project-scoped, trusted projects only):

```toml
[mcp_servers.version-check-mcp]
command = "version-check-mcp"
```

CLI shortcut:

```sh
codex mcp add version-check-mcp -- version-check-mcp
```

> **Optional:** Increase GitHub rate limit - add an env section:
>
> ```toml
> [mcp_servers.version-check-mcp.env]
> GITHUB_TOKEN = "ghp_..."
> ```
>
> **Warning:** Do not commit token values to `.mcp.json`, `opencode.json`, or any
> version-controlled config file. Use a secrets manager or environment-level injection instead.

**Verification status:** Based on official Codex CLI docs - not personally tested by the author.

---

### Gemini CLI

> **Note:** Based on official Gemini CLI docs - not personally tested by the author.

Config at `~/.gemini/settings.json`:

```json
{
  "mcpServers": {
    "version-check-mcp": {
      "command": "version-check-mcp"
    }
  }
}
```

CLI shortcut:

```sh
gemini mcp add version-check-mcp version-check-mcp
```

> **Optional:** Increase GitHub rate limit - add an `env` field. Gemini CLI supports
> shell variable expansion (`$VAR`); other agents set literal values:
>
> ```json
> "env": { "GITHUB_TOKEN": "$GITHUB_TOKEN" }
> ```
>
> **Warning:** Do not commit token values to `.mcp.json`, `opencode.json`, or any
> version-controlled config file. Use a secrets manager or environment-level injection instead.

**Verification status:** Based on official Gemini CLI docs - not personally tested by the author.
