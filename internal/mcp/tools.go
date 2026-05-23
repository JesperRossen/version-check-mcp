package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/registry"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Manager is the locked package-manager enum (D-01). The five string values
// are wire-visible (they appear in tool schemas and JSON-RPC payloads) and
// must not change.
type Manager string

const (
	ManagerNPM   Manager = "npm"
	ManagerPyPI  Manager = "pypi"
	ManagerGomod Manager = "gomod"
	ManagerGH    Manager = "gh"
	ManagerMaven Manager = "maven"
)

type ValidateInput struct {
	Manager            Manager `json:"manager" jsonschema:"package manager: one of npm, pypi, gomod, gh, maven"`
	Pkg                string  `json:"pkg" jsonschema:"package identifier in ecosystem-native form (e.g. 'react', 'requests', 'github.com/foo/bar', 'actions/checkout', 'org.springframework:spring-core')"`
	Version            string  `json:"version" jsonschema:"exact version string (no ranges); ecosystem-native form (Go retains 'v' prefix, NPM does not)"`
	IncludePrereleases bool    `json:"include_prereleases,omitempty" jsonschema:"if true, prereleases considered valid; default false"`
}

type LatestInput struct {
	Manager            Manager `json:"manager" jsonschema:"package manager: one of npm, pypi, gomod, gh, maven"`
	Pkg                string  `json:"pkg" jsonschema:"package identifier in ecosystem-native form"`
	IncludePrereleases bool    `json:"include_prereleases,omitempty" jsonschema:"if true, prereleases are considered; default false"`
	Major              *int    `json:"major,omitempty" jsonschema:"optional integer constraining the result to that major version (e.g. 17 returns latest 17.x)"`
	Minor              *int    `json:"minor,omitempty" jsonschema:"optional integer constraining the result to that minor (requires major); e.g. major=17,minor=0 returns latest 17.0.x"`
}

// validateInputSchema returns a JSON schema object for ValidateInput.
func validateInputSchema() json.RawMessage {
	return mustSchema(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"manager", "pkg", "version"},
		"properties": map[string]any{
			"manager": map[string]any{
				"type":        "string",
				"description": "package manager: one of npm, pypi, gomod, gh, maven",
				"enum":        []string{"npm", "pypi", "gomod", "gh", "maven"},
			},
			"pkg": map[string]any{
				"type":        "string",
				"description": "package identifier in ecosystem-native form (e.g. 'react', 'requests', 'github.com/foo/bar', 'actions/checkout', 'org.springframework:spring-core')",
			},
			"version": map[string]any{
				"type":        "string",
				"description": "exact version string (no ranges); ecosystem-native form (Go retains 'v' prefix, NPM does not)",
			},
			"include_prereleases": map[string]any{
				"type":        "boolean",
				"description": "if true, prereleases considered valid; default false",
			},
		},
	})
}

// latestInputSchema returns a JSON schema object for LatestInput.
func latestInputSchema() json.RawMessage {
	return mustSchema(map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"manager", "pkg"},
		"properties": map[string]any{
			"manager": map[string]any{
				"type":        "string",
				"description": "package manager: one of npm, pypi, gomod, gh, maven",
				"enum":        []string{"npm", "pypi", "gomod", "gh", "maven"},
			},
			"pkg": map[string]any{
				"type":        "string",
				"description": "package identifier in ecosystem-native form",
			},
			"include_prereleases": map[string]any{
				"type":        "boolean",
				"description": "if true, prereleases are considered; default false",
			},
			"major": map[string]any{
				"type":        "integer",
				"description": "optional integer constraining the result to that major version (e.g. 17 returns latest 17.x)",
			},
			"minor": map[string]any{
				"type":        "integer",
				"description": "optional integer constraining the result to that minor (requires major); e.g. major=17,minor=0 returns latest 17.0.x",
			},
		},
	})
}

func mustSchema(schema map[string]any) json.RawMessage {
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("marshal tool schema: %v", err))
	}
	return raw
}

// isRangeLike returns true for any version string that looks like a range
// or wildcard rather than an exact pinned version (VAL-05 / D-03). Exact
// versions across npm/pypi/gomod/gh/maven use only [0-9A-Za-z._+-] plus an
// optional leading 'v' for Go. Anything else is suspicious.
func isRangeLike(v string) bool {
	if v == "" || v == "*" {
		return true
	}
	if strings.ContainsAny(v, "^~*<>=, ") {
		return true
	}
	if strings.Contains(v, "||") {
		return true
	}
	if strings.ContainsAny(v, "xX") {
		return true
	}
	return false
}

// validateRawHandler is the raw ToolHandler form: it decodes arguments
// itself and returns the explicit *CallToolResult so the SDK does not
// overwrite StructuredContent (which it does in the typed handler form).
func (s *Server) validateRawHandler(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	var in ValidateInput
	if err := decodeArgs(req, &in); err != nil {
		return toCallToolResult(
			errs.InvalidInput("malformed arguments", "error", err.Error()),
			"",
		), nil
	}

	in.Pkg = strings.TrimSpace(in.Pkg)
	in.Version = strings.TrimSpace(in.Version)
	if err := validateValidateInput(in); err != nil {
		return toCallToolResult(err, in.Version), nil
	}

	if isRangeLike(in.Version) {
		return toCallToolResult(
			errs.InvalidInput("version must be exact, not a range", "requested_version", in.Version),
			in.Version,
		), nil
	}

	reg, ok := s.registries[in.Manager]
	if !ok {
		return toCallToolResult(
			errs.InvalidInput("unknown manager", "manager", string(in.Manager), "requested_version", in.Version),
			in.Version,
		), nil
	}

	res, err := reg.Validate(ctx, in.Pkg, in.Version, in.IncludePrereleases)
	if err != nil {
		var e *errs.E
		if errors.As(err, &e) && e.Kind == errs.KindNotFound {
			return s.buildMissResponse(ctx, reg, in)
		}
		return toCallToolResult(err, in.Version), nil
	}

	return successResult(in.Version, map[string]any{
		"exists":            res.Exists,
		"source":            res.Source,
		"requested_version": in.Version,
	}), nil
}

// buildMissResponse assembles the success-shaped alternatives response (D-MISS-01).
func (s *Server) buildMissResponse(ctx context.Context, reg registry.Registry, in ValidateInput) (*sdkmcp.CallToolResult, error) {
	// Determine if this registry uses v-prefixed versions.
	vPrefixed := reg.Name() == "gomod" || reg.Name() == "gh"

	// Get version list from cache (no HTTP — cache was populated by Validate call).
	versions, err := reg.Versions(ctx, in.Pkg, in.IncludePrereleases)
	if err != nil {
		return toCallToolResult(err, in.Version), nil
	}

	// Get latest stable (also a cache hit).
	latestRes, err := reg.Latest(ctx, in.Pkg, false, nil, nil)
	latestStable := ""
	if err == nil {
		latestStable = latestRes.Version
	}

	// Compute alternatives.
	alts := filter.NearestVersions(versions, in.Version, vPrefixed, latestStable)

	return successResult(in.Version, map[string]any{
		"exists":            false,
		"requested_version": in.Version,
		"latest_stable":     latestStable,
		"alternatives":      alts,
	}), nil
}

func (s *Server) latestRawHandler(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
	var in LatestInput
	if err := decodeArgs(req, &in); err != nil {
		return toCallToolResult(
			errs.InvalidInput("malformed arguments", "error", err.Error()),
			"",
		), nil
	}

	in.Pkg = strings.TrimSpace(in.Pkg)
	if err := validateLatestInput(in); err != nil {
		return toCallToolResult(err, ""), nil
	}

	reg, ok := s.registries[in.Manager]
	if !ok {
		return toCallToolResult(
			errs.InvalidInput("unknown manager", "manager", string(in.Manager)),
			"",
		), nil
	}

	res, err := reg.Latest(ctx, in.Pkg, in.IncludePrereleases, in.Major, in.Minor)
	if err != nil {
		return toCallToolResult(err, ""), nil
	}

	return successResult("", map[string]any{
		"version": res.Version,
		"source":  res.Source,
	}), nil
}

func validateValidateInput(in ValidateInput) error {
	if strings.TrimSpace(string(in.Manager)) == "" {
		return errs.InvalidInput("manager is required", "manager", "")
	}
	if in.Pkg == "" {
		return errs.InvalidInput("pkg is required", "pkg", "")
	}
	if in.Version == "" {
		return errs.InvalidInput("version is required", "version", "")
	}
	return nil
}

func validateLatestInput(in LatestInput) error {
	if strings.TrimSpace(string(in.Manager)) == "" {
		return errs.InvalidInput("manager is required", "manager", "")
	}
	if in.Pkg == "" {
		return errs.InvalidInput("pkg is required", "pkg", "")
	}
	if in.Minor != nil && in.Major == nil {
		return errs.InvalidInput("minor filter requires major", "minor", *in.Minor)
	}
	if in.Major != nil && *in.Major < 0 {
		return errs.InvalidInput("major must be >= 0", "major", *in.Major)
	}
	if in.Minor != nil && *in.Minor < 0 {
		return errs.InvalidInput("minor must be >= 0", "minor", *in.Minor)
	}
	return nil
}

// decodeArgs unmarshals the raw arguments from a CallToolRequest into the
// target struct. CallToolParamsRaw.Arguments is json.RawMessage; an empty
// argument set produces a zero-value struct rather than an error.
func decodeArgs(req *sdkmcp.CallToolRequest, target any) error {
	if req == nil || req.Params == nil {
		return nil
	}
	raw := req.Params.Arguments
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, target)
}

// successResult builds a CallToolResult for the happy path with the
// StructuredContent envelope set explicitly. We mirror the error path's
// shape so tests probe both via the same JSON paths.
func successResult(requestedVersion string, payload map[string]any) *sdkmcp.CallToolResult {
	sc := map[string]any{}
	for k, v := range payload {
		sc[k] = v
	}
	if _, set := sc["requested_version"]; !set && requestedVersion != "" {
		sc["requested_version"] = requestedVersion
	}
	textBlock, _ := json.Marshal(sc)
	return &sdkmcp.CallToolResult{
		Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: string(textBlock)}},
		StructuredContent: sc,
	}
}
