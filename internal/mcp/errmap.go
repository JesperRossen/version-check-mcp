package mcp

import (
	"errors"

	"github.com/JesperRossen/version-check-mcp/internal/errs"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// toCallToolResult is the single chokepoint that converts an error from a
// tool handler into the explicit *CallToolResult envelope shape the client
// sees. Nil error → nil result (caller falls through to typed Out path).
//
// The envelope shape is:
//
//	StructuredContent: {
//	    "error": {
//	        "type":    <KindString>,   // wire-visible UX-02 discriminator
//	        "message": <e.Message>,
//	        "details": <e.Details>,    // includes caller-set fields like reset_at
//	    },
//	    "requested_version": <verbatim echo>,
//	}
func toCallToolResult(e error, requestedVersion string) *sdkmcp.CallToolResult {
	if e == nil {
		return nil
	}

	var ee *errs.E
	if !errors.As(e, &ee) {
		// Non-errs.E error crossed the seam — defensively wrap so the
		// envelope still has the expected shape (D-07 invariant).
		ee = errs.UpstreamDown(e, "wrapped", e.Error())
	}

	details := ee.Details
	if details == nil {
		details = map[string]any{}
	}

	sc := map[string]any{
		"error": map[string]any{
			"type":    string(ee.Kind),
			"message": ee.Message,
			"details": details,
		},
	}
	if requestedVersion != "" {
		sc["requested_version"] = requestedVersion
	}

	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: ee.Error()}},
		StructuredContent: sc,
	}
}
