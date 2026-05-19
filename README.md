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

### Claude Desktop configuration

Add the following to your `claude_desktop_config.json`
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
