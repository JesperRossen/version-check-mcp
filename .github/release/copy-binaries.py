#!/usr/bin/env python3
"""Copy GoReleaser build binaries into the MCPB bundle staging directory.

Reads dist/artifacts.json (written by GoReleaser) to locate the exact binary
paths, avoiding platform-specific glob issues with BSD vs GNU find.

Usage: python3 copy-binaries.py
Working directory must be the repo root (dist/ must exist there).
"""
import json
import shutil
import sys

DEST = "mcpb-staging/server"

TARGETS = {
    ("darwin", "amd64"): f"{DEST}/version-check-mcp-darwin-amd64",
    ("darwin", "arm64"): f"{DEST}/version-check-mcp-darwin-arm64",
    ("linux",  "amd64"): f"{DEST}/version-check-mcp-linux-amd64",
    ("linux",  "arm64"): f"{DEST}/version-check-mcp-linux-arm64",
    ("windows","amd64"): f"{DEST}/version-check-mcp-windows.exe",
}

with open("dist/artifacts.json") as f:
    artifacts = json.load(f)

copied = 0
for a in artifacts:
    if a["type"] != "Binary":
        continue
    key = (a.get("goos", ""), a.get("goarch", ""))
    if key in TARGETS:
        shutil.copy(a["path"], TARGETS[key])
        copied += 1

if copied != len(TARGETS):
    print(f"ERROR: expected {len(TARGETS)} binaries, copied {copied}", file=sys.stderr)
    sys.exit(1)
