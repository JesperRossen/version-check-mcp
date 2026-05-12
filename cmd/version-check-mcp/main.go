// version-check-mcp is the MCP server entrypoint. Boots an stdio MCP
// server that exposes validate_version and get_latest_version tools.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	appmcp "github.com/JesperRossen/version-check-mcp/internal/mcp"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/fake"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	cacheTTL := flag.Duration("cache-ttl", 15*time.Minute, "TTL for cached registry responses")
	verbose := flag.Bool("verbose", false, "enable debug-level logging")
	flag.Parse()

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	c := cache.NewCache(1024, *cacheTTL)
	defer c.Close()

	// Phase 1: all five managers backed by the FakeRegistry. Real adapters
	// land in Phase 2 (NPM) and Phase 3 (PyPI/Go/GH/Maven).
	registries := map[appmcp.Manager]registry.Registry{
		appmcp.ManagerNPM:   fake.New("npm"),
		appmcp.ManagerPyPI:  fake.New("pypi"),
		appmcp.ManagerGomod: fake.New("gomod"),
		appmcp.ManagerGH:    fake.New("gh"),
		appmcp.ManagerMaven: fake.New("maven"),
	}

	server := appmcp.NewServer(registries, c, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, &sdkmcp.StdioTransport{}); err != nil && !isCleanShutdown(err) {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}

// isCleanShutdown reports whether err represents a normal shutdown of the
// stdio transport (ctx cancelled, stdin EOF, or jsonrpc2's "server is
// closing" sentinel). These all signal "the client went away" rather than
// a server fault, so we exit 0 to keep the integration test's clean-exit
// contract.
func isCleanShutdown(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}
	// jsonrpc2's ErrServerClosing isn't exported from the public mcp package,
	// so match by message — the SDK wraps it via %w + ": EOF".
	return strings.Contains(err.Error(), "server is closing")
}
