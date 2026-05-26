// Package mcp_test — cross-registry response shape audit (Phase 4, Plan 03).
// Verifies that hit and miss paths produce the correct key sets for all 7
// supported registries, and that miss responses satisfy the ecosystem-native
// version format and latest_stable-first invariants.
package mcp_test

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
	internalmcp "github.com/JesperRossen/version-check-mcp/internal/mcp"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/fake"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// stringsEqual reports whether two string slices contain the same elements
// (both sorted before comparing).
func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sorted := func(s []string) []string {
		cp := append([]string(nil), s...)
		sort.Strings(cp)
		return cp
	}
	sa, sb := sorted(a), sorted(b)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

type shapeCase struct {
	name      string
	manager   string
	pkg       string
	version   string // existing version for hit
	missVer   string // non-existent version for miss
	vPrefixed bool
	versions  []string // synthetic version list for the fake
	latest    string   // latest stable version
}

var shapeCases = []shapeCase{
	{
		name: "npm", manager: "npm", pkg: "react",
		version: "18.2.0", missVer: "99.0.0", vPrefixed: false,
		versions: []string{"16.0.0", "17.0.0", "18.0.0", "18.1.0", "18.2.0"},
		latest:   "18.2.0",
	},
	{
		name: "pypi", manager: "pypi", pkg: "requests",
		version: "2.31.0", missVer: "99.0.0", vPrefixed: false,
		versions: []string{"2.28.0", "2.29.0", "2.30.0", "2.31.0", "2.32.0"},
		latest:   "2.32.0",
	},
	{
		name: "gomod", manager: "gomod", pkg: "github.com/foo/bar",
		version: "v1.2.0", missVer: "v99.0.0", vPrefixed: true,
		versions: []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0", "v2.0.0"},
		latest:   "v2.0.0",
	},
	{
		name: "gh", manager: "gh", pkg: "actions/checkout",
		version: "v4.1.0", missVer: "v99.0.0", vPrefixed: true,
		versions: []string{"v3.0.0", "v3.6.0", "v4.0.0", "v4.1.0", "v4.2.0"},
		latest:   "v4.2.0",
	},
	{
		name: "maven", manager: "maven", pkg: "org.example:lib",
		version: "1.0.0", missVer: "99.0.0", vPrefixed: false,
		versions: []string{"1.0.0", "1.1.0", "2.0.0", "2.1.0", "3.0.0"},
		latest:   "3.0.0",
	},
	{
		name: "crate", manager: "crate", pkg: "serde",
		version: "1.0.228", missVer: "99.0.0", vPrefixed: false,
		versions: []string{"1.0.0", "1.0.227", "1.0.228", "2.0.0-beta.1"},
		latest:   "1.0.228",
	},
	{
		name: "rubygems", manager: "rubygems", pkg: "rails",
		version: "8.1.3", missVer: "99.0.0", vPrefixed: false,
		versions: []string{"7.2.3", "8.0.0", "8.1.3", "8.2.0-beta.1"},
		latest:   "8.1.3",
	},
}

// buildRegistries constructs one fake per shape case and returns the map
// keyed by Manager. Each fake is configured for BOTH hit and miss — the
// same Fake is reused in sub-tests by toggling ValidateErr.
func buildFakesForShape(t *testing.T) map[string]*fake.Fake {
	t.Helper()
	fakes := make(map[string]*fake.Fake, len(shapeCases))
	for _, tc := range shapeCases {
		f := fake.New(tc.manager)
		f.VersionsList = tc.versions
		f.LatestResult = registry.LatestResult{Version: tc.latest, Source: "fake"}
		fakes[tc.manager] = f
	}
	return fakes
}

func TestResponseShapeAudit(t *testing.T) {
	// We need a separate server per sub-test because Fake.ValidateErr is
	// toggled between hit (nil) and miss (NotFound). Build one server per
	// test case to keep state clean.

	for _, tc := range shapeCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// --- HIT path ---
			t.Run("hit_keys", func(t *testing.T) {
				fHit := fake.New(tc.manager)
				fHit.ValidateResult = registry.ValidateResult{Exists: true, Source: "test"}
				fHit.VersionsList = tc.versions
				fHit.LatestResult = registry.LatestResult{Version: tc.latest, Source: "fake"}

				registries := map[internalmcp.Manager]registry.Registry{
					internalmcp.Manager(tc.manager): fHit,
				}
				session, done := connectInMemory(t, registries)
				defer done()

				res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
					Name: "validate_version",
					Arguments: map[string]any{
						"manager": tc.manager,
						"pkg":     tc.pkg,
						"version": tc.version,
					},
				})
				if err != nil {
					t.Fatalf("hit CallTool: %v", err)
				}
				if res.IsError {
					t.Fatalf("hit IsError=true, want false: %v", extractSC(t, res))
				}

				sc := extractSC(t, res)
				gotKeys := sortedKeys(sc)
				wantKeys := []string{"exists", "requested_version", "source"}
				if !stringsEqual(gotKeys, wantKeys) {
					t.Errorf("hit keys = %v, want %v (registry=%s)", gotKeys, wantKeys, tc.manager)
				}
			})

			// --- MISS path ---
			t.Run("miss_keys", func(t *testing.T) {
				fMiss := fake.New(tc.manager)
				fMiss.ValidateErr = errs.NotFound("synthetic miss")
				fMiss.VersionsList = tc.versions
				fMiss.LatestResult = registry.LatestResult{Version: tc.latest, Source: "fake"}

				registries := map[internalmcp.Manager]registry.Registry{
					internalmcp.Manager(tc.manager): fMiss,
				}
				session, done := connectInMemory(t, registries)
				defer done()

				res, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
					Name: "validate_version",
					Arguments: map[string]any{
						"manager": tc.manager,
						"pkg":     tc.pkg,
						"version": tc.missVer,
					},
				})
				if err != nil {
					t.Fatalf("miss CallTool: %v", err)
				}
				if res.IsError {
					t.Fatalf("miss IsError=true, want false (should be success-shaped): %v", extractSC(t, res))
				}

				sc := extractSC(t, res)
				gotKeys := sortedKeys(sc)
				wantKeys := []string{"alternatives", "exists", "latest_stable", "requested_version"}
				if !stringsEqual(gotKeys, wantKeys) {
					t.Errorf("miss keys = %v, want %v (registry=%s)", gotKeys, wantKeys, tc.manager)
				}

				// exists must be false
				if exists, _ := sc["exists"].(bool); exists {
					t.Errorf("miss exists=true, want false")
				}

				// latest_stable must match configured latest
				if ls, _ := sc["latest_stable"].(string); ls != tc.latest {
					t.Errorf("latest_stable=%q, want %q", ls, tc.latest)
				}

				// requested_version must echo the miss version
				if rv, _ := sc["requested_version"].(string); rv != tc.missVer {
					t.Errorf("requested_version=%q, want %q", rv, tc.missVer)
				}

				// alternatives shape
				altsRaw := sc["alternatives"]
				if altsRaw == nil {
					t.Fatal("alternatives is nil")
				}
				altsJSON, _ := json.Marshal(altsRaw)
				var alts []map[string]any
				if err := json.Unmarshal(altsJSON, &alts); err != nil {
					t.Fatalf("alternatives unmarshal: %v", err)
				}
				if len(alts) == 0 {
					t.Fatal("alternatives is empty")
				}

				validReasons := map[string]bool{
					"latest_stable":   true,
					"nearest_semver":  true,
					"latest_in_major": true,
				}
				for i, alt := range alts {
					// Each entry must have exactly {version, reason}
					altKeys := sortedKeys(alt)
					wantAltKeys := []string{"reason", "version"}
					if !stringsEqual(altKeys, wantAltKeys) {
						t.Errorf("alt[%d] keys = %v, want %v", i, altKeys, wantAltKeys)
					}

					reason, _ := alt["reason"].(string)
					if !validReasons[reason] {
						t.Errorf("alt[%d] reason=%q not in closed enum", i, reason)
					}
				}

				// latest_stable must be first alternative (D-NEAREST-04)
				if first, _ := alts[0]["reason"].(string); first != "latest_stable" {
					t.Errorf("alternatives[0].reason=%q, want latest_stable", first)
				}

				// Ecosystem-native version format (UX-01)
				for i, alt := range alts {
					ver, _ := alt["version"].(string)
					if tc.vPrefixed {
						if !strings.HasPrefix(ver, "v") {
							t.Errorf("alt[%d] version=%q should have v-prefix for registry=%s", i, ver, tc.manager)
						}
					} else {
						if strings.HasPrefix(ver, "v") {
							t.Errorf("alt[%d] version=%q should NOT have v-prefix for registry=%s", i, ver, tc.manager)
						}
					}
				}
			})
		})
	}
}

// extractSC marshals and unmarshals StructuredContent for shape inspection.
func extractSC(t *testing.T, res *sdkmcp.CallToolResult) map[string]any {
	t.Helper()
	if res.StructuredContent == nil {
		t.Fatal("StructuredContent is nil")
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
