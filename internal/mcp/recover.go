package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/JesperRossen/version-check-mcp/internal/errs"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// recoverMiddleware wraps every receiving method dispatch in a deferred
// recover. Panics are logged to stderr (via the supplied logger) and
// converted into an MCP error envelope so the client sees an
// `upstream_down` structured tool error instead of a transport-level crash.
//
// Critically: this is the ONLY path that handles panics. The SDK does NOT
// auto-recover panics (RESEARCH.md Pitfall #3). The recover MUST swallow
// the panic — never re-panic; never print runtime stack to stdout.
func recoverMiddleware(logger *slog.Logger) sdkmcp.Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (result sdkmcp.Result, err error) {
			defer func() {
				r := recover()
				if r == nil {
					return
				}
				panicStr := fmt.Sprintf("%v", r)
				logger.Error("handler panic",
					"method", method,
					"panic", panicStr,
					"stack", string(debug.Stack()),
				)
				e := errs.UpstreamDown(fmt.Errorf("handler panic: %s", panicStr), "panic", panicStr)

				if method == "tools/call" {
					// Surface as a structured CallToolResult so the client
					// sees the panic as a tool-level upstream_down error,
					// not a transport-level JSON-RPC failure.
					result = &sdkmcp.CallToolResult{
						IsError: true,
						Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: e.Error()}},
						StructuredContent: map[string]any{
							"error": map[string]any{
								"type":    string(e.Kind),
								"message": e.Message,
								"details": map[string]any{
									"panic": panicStr,
								},
							},
						},
					}
					err = nil
					return
				}
				// Other methods (initialize, tools/list, ping, ...) have no
				// CallToolResult shape — hand the *errs.E to the SDK and
				// let its default error envelope render it.
				result = nil
				err = e
			}()
			return next(ctx, method, req)
		}
	}
}
