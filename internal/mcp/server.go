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
	validateDescription = "Use this tool to verify one exact dependency version before writing it to a manifest. " +
		"When to use: validating a pinned version (package.json, pyproject.toml, go.mod, pom.xml, action tag). " +
		"When not to use: finding newest versions, use get_latest_version for that. " +
		"Required inputs: manager, pkg, version. Optional: include_prereleases (default false). " +
		"Version must be exact, not a range. " +
		"Manager enum values: npm, pypi, gomod, gh, maven, crate, rubygems. " +
		"Response on hit: {exists:true, source:string, requested_version:string}. " +
		"Response on miss: {exists:false, requested_version:string, latest_stable:string, alternatives:[{version:string, reason:latest_stable|nearest_semver|latest_in_major}]}. " +
		"Error envelope: {error:{type:invalid_input|not_found|rate_limited|upstream_down, message:string, details:object}, requested_version?:string}. " +
		"Version formatting: Go/GitHub tags usually keep 'v' prefix, npm/PyPI/Maven/Crates.io/RubyGems usually do not. " +
		"Result is authoritative live registry data."

	latestDescription = "Use this tool to look up the newest available version for a package before proposing an upgrade. " +
		"When to use: selecting a target upgrade version. " +
		"When not to use: checking whether an already chosen version exists, use validate_version. " +
		"Required inputs: manager, pkg. Optional: include_prereleases (default false), major, minor. " +
		"Filter semantics: major=17 means latest 17.x; minor requires major and means latest 17.0.x style within that branch. " +
		"Manager enum values: npm, pypi, gomod, gh, maven, crate, rubygems. " +
		"Success response: {version:string, source:string}. " +
		"If no version matches filters, returns error.type=not_found. " +
		"Error envelope: {error:{type:invalid_input|not_found|rate_limited|upstream_down, message:string, details:object}}. " +
		"Result is authoritative live registry data."
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
			InputSchema: validateInputSchema(),
		},
		s.validateRawHandler,
	)

	sdk.AddTool(
		&sdkmcp.Tool{
			Name:        "get_latest_version",
			Description: latestDescription,
			InputSchema: latestInputSchema(),
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
