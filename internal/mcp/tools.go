package mcp

import (
	"bytes"
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

// Manager is the locked package-manager enum (D-01). The seven string values
// are wire-visible (they appear in tool schemas and JSON-RPC payloads) and
// must not change.
type Manager string

const (
	ManagerNPM      Manager = "npm"
	ManagerPyPI     Manager = "pypi"
	ManagerGomod    Manager = "gomod"
	ManagerGH       Manager = "gh"
	ManagerMaven    Manager = "maven"
	ManagerCrate    Manager = "crate"
	ManagerRubygems Manager = "rubygems"
)

type ValidateInput struct {
	Manager            Manager `json:"manager" jsonschema:"package manager: one of npm, pypi, gomod, gh, maven, crate, rubygems"`
	Pkg                string  `json:"pkg" jsonschema:"package identifier in ecosystem-native form (e.g. 'react', 'requests', 'github.com/foo/bar', 'actions/checkout', 'org.springframework:spring-core', 'serde', 'rails')"`
	Version            string  `json:"version" jsonschema:"exact version string (no ranges); ecosystem-native form (Go retains 'v' prefix, NPM does not)"`
	IncludePrereleases bool    `json:"include_prereleases,omitempty" jsonschema:"if true, prereleases considered valid; default false"`
}

type LatestInput struct {
	Manager            Manager `json:"manager" jsonschema:"package manager: one of npm, pypi, gomod, gh, maven, crate, rubygems"`
	Pkg                string  `json:"pkg" jsonschema:"package identifier in ecosystem-native form (e.g. 'react', 'requests', 'github.com/foo/bar', 'actions/checkout', 'org.springframework:spring-core', 'serde', 'rails')"`
	IncludePrereleases bool    `json:"include_prereleases,omitempty" jsonschema:"if true, prereleases are considered; default false"`
	Major              *int    `json:"major,omitempty" jsonschema:"optional integer constraining the result to that major version (e.g. 17 returns latest 17.x)"`
	Minor              *int    `json:"minor,omitempty" jsonschema:"optional integer constraining the result to that minor (requires major); e.g. major=17,minor=0 returns latest 17.0.x"`
}

// validateInputSchema returns a JSON schema object for ValidateInput.
func validateInputSchema() json.RawMessage {
	return mustSchema(map[string]any{
		"type":                 "object",
		"description":          "Validate one exact version for a package in the selected ecosystem. Required fields: manager, pkg, version. Optional include_prereleases defaults to false.",
		"additionalProperties": false,
		"required":             []string{"manager", "pkg", "version"},
		"properties": map[string]any{
			"manager": map[string]any{
				"type":        "string",
				"description": "Registry family for lookup. Allowed values: npm (NPM), pypi (Python), gomod (Go modules), gh (GitHub tags/releases), maven (Maven Central), crate (Crates.io), rubygems (RubyGems).",
				"enum":        []string{"npm", "pypi", "gomod", "gh", "maven", "crate", "rubygems"},
			},
			"pkg": map[string]any{
				"type":        "string",
				"description": "Package identifier in ecosystem-native syntax. Examples: react, requests, github.com/foo/bar, actions/checkout, org.springframework:spring-core, serde, rails.",
			},
			"version": map[string]any{
				"type":        "string",
				"description": "Exact version string to validate. Do not pass ranges like ^1.2, ~1.2, >=1.0, or x wildcards.",
			},
			"include_prereleases": map[string]any{
				"type":        "boolean",
				"description": "If true, prerelease versions (alpha, beta, rc, etc.) are eligible. If omitted, defaults to false.",
			},
		},
	})
}

// latestInputSchema returns a JSON schema object for LatestInput.
func latestInputSchema() json.RawMessage {
	return mustSchema(map[string]any{
		"type":                 "object",
		"description":          "Resolve newest version for a package, optionally constrained by major/minor. Required fields: manager, pkg.",
		"additionalProperties": false,
		"required":             []string{"manager", "pkg"},
		"properties": map[string]any{
			"manager": map[string]any{
				"type":        "string",
				"description": "Registry family for lookup. Allowed values: npm (NPM), pypi (Python), gomod (Go modules), gh (GitHub tags/releases), maven (Maven Central), crate (Crates.io), rubygems (RubyGems).",
				"enum":        []string{"npm", "pypi", "gomod", "gh", "maven", "crate", "rubygems"},
			},
			"pkg": map[string]any{
				"type":        "string",
				"description": "Package identifier in ecosystem-native syntax. Examples: react, requests, github.com/foo/bar, actions/checkout, org.springframework:spring-core, serde, rails.",
			},
			"include_prereleases": map[string]any{
				"type":        "boolean",
				"description": "If true, prerelease versions (alpha, beta, rc, etc.) are eligible. If omitted, defaults to false.",
			},
			"major": map[string]any{
				"type":        "integer",
				"description": "Optional non-negative major filter. Example: 17 returns the newest 17.x release.",
			},
			"minor": map[string]any{
				"type":        "integer",
				"description": "Optional non-negative minor filter. Requires major. Example: major=17, minor=0 returns newest 17.0.x.",
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
// versions across npm/pypi/gomod/gh/maven/crate/rubygems use only [0-9A-Za-z._+-] plus an
// optional leading 'v' for Go. Anything else is suspicious.
func isRangeLike(v string) bool {
	if v == "" || v == "*" {
		return true
	}
	if strings.ContainsAny(v, "^~<>=, ") || strings.Contains(v, "||") {
		return true
	}
	for _, part := range strings.Split(v, ".") {
		if part == "x" || part == "X" || part == "*" {
			return true
		}
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

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
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
