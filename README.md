# version-check-mcp
A Lightweight Version Validator for AI Coding Agents

## Installation

Download the latest binary for your platform from
[GitHub Releases](https://github.com/JesperRossen/version-check-mcp/releases/latest).

### One-click install via MCPB bundle (recommended)

Download `version-check-mcp.mcpb` from
[GitHub Releases](https://github.com/JesperRossen/version-check-mcp/releases/latest)
and open it - Claude Desktop installs the server automatically with a pre-filled
`mcpServers` configuration entry. No PATH setup required. The mcpb format bundles
all platform binaries into a single portable archive.

### Verify checksum (recommended)

```sh
sha256sum -c checksums.txt
```

### macOS: remove quarantine flag

macOS Gatekeeper blocks unsigned binaries. After downloading, run:

```sh
xattr -d com.apple.quarantine ./version-check-mcp
chmod +x ./version-check-mcp
```

## Agent Setup

The server speaks MCP over stdio. Every agent that supports stdio MCP servers can use it. Add the binary path after downloading and (on macOS) clearing quarantine.

### Claude Desktop

Install via MCPB bundle (one-click, recommended) - see the [One-click install](#one-click-install-via-mcpb-bundle-recommended) section above.

For manual configuration, add the following to your `claude_desktop_config.json`
(`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "version-check-mcp": {
      "command": "/path/to/version-check-mcp"
    }
  }
}
```

Replace `/path/to/version-check-mcp` with the absolute path to the downloaded binary.

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
      "command": "/path/to/version-check-mcp",
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
>       "command": "/path/to/version-check-mcp",
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
      "command": ["/path/to/version-check-mcp"]
    }
  }
}
```

> **Optional:** Increase GitHub rate limit - add an `env` field:
>
> ```json
> "version-check-mcp": {
>   "type": "local",
>   "command": ["/path/to/version-check-mcp"],
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
command = "/path/to/version-check-mcp"
```

CLI shortcut:

```sh
codex mcp add version-check-mcp -- /path/to/version-check-mcp
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
      "command": "/path/to/version-check-mcp"
    }
  }
}
```

CLI shortcut:

```sh
gemini mcp add version-check-mcp /path/to/version-check-mcp
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
