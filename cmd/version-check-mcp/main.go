// version-check-mcp is the MCP server entrypoint. Boots an stdio MCP
// server that exposes validate_version and get_latest_version tools.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	appmcp "github.com/JesperRossen/version-check-mcp/internal/mcp"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/gh"
	"github.com/JesperRossen/version-check-mcp/internal/registry/gomod"
	"github.com/JesperRossen/version-check-mcp/internal/registry/maven"
	"github.com/JesperRossen/version-check-mcp/internal/registry/npm"
	"github.com/JesperRossen/version-check-mcp/internal/registry/pypi"
	"github.com/JesperRossen/version-check-mcp/internal/version"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// uaTransport wraps an http.RoundTripper and injects a User-Agent header on
// any outbound request that does not already carry one. The shared client's
// Transport is set to a uaTransport so every adapter's request is identified
// to upstream registries (npm and others request a non-blank UA and may
// rate-limit blank UAs more aggressively).
type uaTransport struct {
	ua   string
	next http.RoundTripper
}

func (t uaTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Header.Get("User-Agent") == "" {
		r2 := r.Clone(r.Context())
		r2.Header.Set("User-Agent", t.ua)
		r = r2
	}
	return t.next.RoundTrip(r)
}

func userAgent() string {
	return "version-check-mcp/" + version.Version + " (+https://github.com/JesperRossen/version-check-mcp)"
}

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

	sharedClient := newSharedClient()

	// Phase 3: all five registries use real adapters.
	registries := map[appmcp.Manager]registry.Registry{
		appmcp.ManagerNPM:   npm.New(sharedClient, c),
		appmcp.ManagerPyPI:  pypi.New(sharedClient, c),
		appmcp.ManagerGomod: gomod.New(sharedClient, c),
		appmcp.ManagerGH:    gh.New(sharedClient, c),
		appmcp.ManagerMaven: maven.New(sharedClient, c),
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
