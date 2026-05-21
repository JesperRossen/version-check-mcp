package mcp

import (
	"context"
	"log/slog"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/version"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	validateDescription = "Check whether a specific package version exists in its registry. " +
		"Returns {exists, source, requested_version}. " +
		"Errors carry a type discriminator: 'invalid_input' (range supplied or malformed input), " +
		"'not_found' (the version does not exist), " +
		"'rate_limited' (upstream rate cap; details.reset_at hints when to retry), " +
		"'upstream_down' (registry unreachable or unexpected). " +
		"Supply the exact version string (no ranges); npm/PyPI/Maven omit the 'v' prefix, Go modules retain it. " +
		"When adding or updating a dependency, MUST call this tool. Result is authoritative — live registry data, newer than your training cutoff."

	latestDescription = "Get the latest version of a package, optionally constrained by major and minor. " +
		"Set 'major' to constrain (e.g. 17 returns latest 17.x). 'minor' requires 'major'. " +
		"Returns {version, source}. Errors carry the same four type discriminators as validate_version. " +
		"An empty filter result (no version satisfies major/minor) returns 'not_found'. " +
		"When adding or updating a dependency, MUST call this tool. Result is authoritative — live registry data, newer than your training cutoff."
)

// Server is the wrapped MCP server. It owns the SDK server, the registries
// map (per-Manager Registry implementations), the cache (passed to adapters
// in Phase 2+), and the logger.
type Server struct {
	registries map[Manager]registry.Registry
	cache      *cache.Cache
	logger     *slog.Logger
	sdk        *sdkmcp.Server
}

// NewServer wires the building blocks into a ready-to-run MCP server: SDK
// server creation, recovery middleware install, and the two tool
// registrations with locked names, LLM-readable descriptions, and reflected
// JSON schemas.
//
// Tools are registered via (*Server).AddTool (the raw ToolHandler form) so
// the handler controls StructuredContent end-to-end. The typed
// ToolHandlerFor + AddTool[In,Out] form auto-overwrites StructuredContent
// with the marshaled Out struct, which collides with the explicit error
// envelope we need to ship for UX-02 / VAL-06.
func NewServer(registries map[Manager]registry.Registry, c *cache.Cache, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	sdk := sdkmcp.NewServer(
		&sdkmcp.Implementation{
			Name:    "version-check-mcp",
			Version: version.Version,
		},
		nil,
	)
	sdk.AddReceivingMiddleware(recoverMiddleware(logger))

	s := &Server{
		registries: registries,
		cache:      c,
		logger:     logger,
		sdk:        sdk,
	}

	sdk.AddTool(
		&sdkmcp.Tool{
			Name:        "validate_version",
			Description: validateDescription,
			InputSchema: schemaFor[ValidateInput](),
		},
		s.validateRawHandler,
	)

	sdk.AddTool(
		&sdkmcp.Tool{
			Name:        "get_latest_version",
			Description: latestDescription,
			InputSchema: schemaFor[LatestInput](),
		},
		s.latestRawHandler,
	)

	return s
}

// Run starts the server on the provided transport. Returns when ctx is
// cancelled or the transport closes (e.g. stdin EOF).
func (s *Server) Run(ctx context.Context, t sdkmcp.Transport) error {
	return s.sdk.Run(ctx, t)
}

// Connect exposes the underlying SDK Connect for in-memory transport tests.
func (s *Server) Connect(ctx context.Context, t sdkmcp.Transport, opts *sdkmcp.ServerSessionOptions) (*sdkmcp.ServerSession, error) {
	return s.sdk.Connect(ctx, t, opts)
}
