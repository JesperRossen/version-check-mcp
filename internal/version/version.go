// Package version exposes the build-time version string. Phase 5 overrides
// this via -ldflags "-X github.com/JesperRossen/version-check-mcp/internal/version.Version=v1.0.0".
package version

var Version = "dev"
