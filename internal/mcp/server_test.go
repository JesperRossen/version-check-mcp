// RED test (Wave 0). Production code lands in Wave 3 (plan 01-05).
// See .planning/phases/01-foundation-mcp-scaffolding/01-VALIDATION.md.
package mcp_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	internalmcp "github.com/JesperRossen/version-check-mcp/internal/mcp"
	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/fake"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// connectInMemory wires a client to a server over the SDK's in-memory transport
// and returns a connected client session ready for tool calls.
func connectInMemory(t *testing.T, registries map[internalmcp.Manager]registry.Registry) (*sdkmcp.ClientSession, func()) {
	t.Helper()
	c := cache.NewCache(64, 2*time.Second)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	server := internalmcp.NewServer(registries, c, logger)

	clientT, serverT := sdkmcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	cleanup := func() {
		_ = session.Close()
		c.Close()
	}
	return session, cleanup
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

func TestToolsRegistered(t *testing.T) {
	session, done := connectInMemory(t, map[internalmcp.Manager]registry.Registry{
		internalmcp.ManagerNPM: fake.New("npm"),
	})
	defer done()

	res, err := session.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"validate_version", "get_latest_version"} {
		if !got[want] {
			t.Errorf("tool %q not registered; got %v", want, res.Tools)
		}
	}
	if len(res.Tools) != 2 {
		t.Errorf("got %d tools, want exactly 2", len(res.Tools))
	}
}

func TestSchemaDescriptions(t *testing.T) {
	session, done := connectInMemory(t, map[internalmcp.Manager]registry.Registry{
		internalmcp.ManagerNPM: fake.New("npm"),
	})
	defer done()

	res, err := session.ListTools(context.Background(), &sdkmcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if strings.TrimSpace(tool.Description) == "" {
			t.Errorf("tool %q has empty Description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil InputSchema", tool.Name)
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal InputSchema for %q: %v", tool.Name, err)
		}
		var parsed struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("unmarshal schema for %q: %v", tool.Name, err)
		}
		for prop, p := range parsed.Properties {
			if strings.TrimSpace(p.Description) == "" {
				t.Errorf("tool %q property %q has empty description", tool.Name, prop)
			}
		}
	}
}

func TestValidateRejectsRanges(t *testing.T) {
	f := fake.New("npm")
	registries := map[internalmcp.Manager]registry.Registry{internalmcp.ManagerNPM: f}
	session, done := connectInMemory(t, registries)
	defer done()

	ranges := []string{"^1.2.3", "~1.2", "1.x", ">= 1.0", "*", ">1.0,<2.0"}
	for _, v := range ranges {
		args := map[string]any{"manager": "npm", "package": "react", "version": v}
		res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
			Name:      "validate_version",
			Arguments: args,
		})
		if err != nil {
			t.Errorf("CallTool(%q) transport err = %v", v, err)
			continue
		}
		if !res.IsError {
			t.Errorf("range %q: IsError=false, want true", v)
		}
		errType := extractErrorType(t, res)
		if errType != "invalid_input" {
			t.Errorf("range %q: error.type = %q, want invalid_input", v, errType)
		}
	}
	if got := f.ValidateCalls.Load(); got != 0 {
		t.Errorf("FakeRegistry.Validate was called %d times; range rejection should not reach the registry", got)
	}
}

func TestRequestedVersionEcho(t *testing.T) {
	f := fake.New("npm")
	f.ValidateResult = registry.ValidateResult{Exists: true, Source: "npm"}
	session, done := connectInMemory(t, map[internalmcp.Manager]registry.Registry{
		internalmcp.ManagerNPM: f,
	})
	defer done()

	// success path
	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "validate_version",
		Arguments: map[string]any{"manager": "npm", "package": "react", "version": "18.2.0"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if got := extractField(t, res, "requested_version"); got != "18.2.0" {
		t.Errorf("requested_version = %q, want %q", got, "18.2.0")
	}

	// range rejection path
	res2, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "validate_version",
		Arguments: map[string]any{"manager": "npm", "package": "react", "version": "^1.0.0"},
	})
	if err != nil {
		t.Fatalf("CallTool(range): %v", err)
	}
	if got := extractField(t, res2, "requested_version"); got != "^1.0.0" {
		t.Errorf("requested_version (range) = %q, want %q", got, "^1.0.0")
	}
}

func TestErrorEnvelopeShape(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"rate_limited", errs.RateLimited(time.Now().Add(time.Minute)), "rate_limited"},
		{"not_found", errs.NotFound("missing"), "not_found"},
		{"upstream_down", errs.UpstreamDown(errs.NotFound("x")), "upstream_down"},
		{"invalid_input", errs.InvalidInput("bad"), "invalid_input"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := fake.New("npm")
			f.ValidateErr = tc.err
			session, done := connectInMemory(t, map[internalmcp.Manager]registry.Registry{
				internalmcp.ManagerNPM: f,
			})
			defer done()

			res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
				Name:      "validate_version",
				Arguments: map[string]any{"manager": "npm", "package": "react", "version": "1.0.0"},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Errorf("IsError=false, want true")
			}
			if got := extractErrorType(t, res); got != tc.want {
				t.Errorf("error.type = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPanicRecoveredAsUpstreamDown(t *testing.T) {
	f := fake.New("npm")
	f.PanicOn = "validate"
	session, done := connectInMemory(t, map[internalmcp.Manager]registry.Registry{
		internalmcp.ManagerNPM: f,
	})
	defer done()

	res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "validate_version",
		Arguments: map[string]any{"manager": "npm", "package": "react", "version": "1.0.0"},
	})
	if err != nil {
		t.Fatalf("CallTool transport err (panic should not crash process): %v", err)
	}
	if !res.IsError {
		t.Error("IsError=false, want true")
	}
	if got := extractErrorType(t, res); got != "upstream_down" {
		t.Errorf("error.type = %q, want upstream_down", got)
	}
	details := extractErrorDetails(t, res)
	if v, ok := details["panic"]; !ok || v == "" {
		t.Errorf("error.details.panic = %v, want non-empty", v)
	}
}

// extractErrorType reads StructuredContent.error.type from a CallToolResult.
func extractErrorType(t *testing.T, res *sdkmcp.CallToolResult) string {
	t.Helper()
	sc := structuredContent(t, res)
	errBlock, ok := sc["error"].(map[string]any)
	if !ok {
		t.Errorf("StructuredContent missing error block: %v", sc)
		return ""
	}
	typ, _ := errBlock["type"].(string)
	return typ
}

func extractErrorDetails(t *testing.T, res *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	sc := structuredContent(t, res)
	errBlock, ok := sc["error"].(map[string]any)
	if !ok {
		return nil
	}
	d, _ := errBlock["details"].(map[string]any)
	return d
}

func extractField(t *testing.T, res *sdkmcp.CallToolResult, key string) string {
	t.Helper()
	sc := structuredContent(t, res)
	v, _ := sc[key].(string)
	return v
}

func structuredContent(t *testing.T, res *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	if res.StructuredContent == nil {
		t.Errorf("CallToolResult.StructuredContent is nil")
		return nil
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal StructuredContent: %v", err)
	}
	return out
}
