// Package pypi implements the registry.Registry interface for PyPI
// (https://pypi.org). It fetches the project-level JSON endpoint once per
// (pkg, incPre) tuple, caches the parsed body, and serves both Validate and
// Latest from that cache.
//
// REG-02: PEP 440 normalization is applied to both the input version and every
// key in the releases map before equality comparison.
//
// D-PYPI-01: Yanked releases (PEP 592) are detected and surfaced via
// Source="pypi-yanked" while still returning Exists=true.
package pypi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

// Source value constants (D-SOURCE-01). These strings are wire-visible in
// LatestResult.Source and ValidateResult.Source — do not rename without a
// protocol change.
const (
	// SourceInfoVersion is returned by Latest when the fast path is taken:
	// !incPre && major==nil && minor==nil. The info.version field is the
	// registry's canonical "latest stable" pointer and is trusted directly.
	SourceInfoVersion = "pypi-info-version"

	// SourceVersionsMap is returned by Validate when a non-yanked match is
	// found in the releases map.
	SourceVersionsMap = "versions-map"

	// SourceYanked is returned by Validate when the matching release is
	// yanked (PEP 592). Exists is still true per D-PYPI-01.
	SourceYanked = "pypi-yanked"

	// SourceComputed is returned by Latest when FilterAndPickHighest is
	// used (incPre=true, or a major/minor filter is active).
	SourceComputed = "computed-highest"
)

// pypiFile is the minimum file-info shape from the releases map. We only need
// the yanked field; other fields are ignored.
type pypiFile struct {
	Yanked bool `json:"yanked"`
}

// pypiProject is the parsed shape of the PyPI project JSON endpoint response.
// Only the fields we actually use are decoded; the full response is much larger.
type pypiProject struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Releases map[string][]pypiFile `json:"releases"`
}

// Adapter implements registry.Registry against https://pypi.org.
// Construction is via New; the constructor takes the shared *http.Client and
// the shared *cache.Cache.
type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

// New constructs a PyPI Adapter.
func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

// Name returns "pypi".
func (a *Adapter) Name() string { return "pypi" }

// projectFor fetches the PyPI project JSON for pkg, going through cache.Get so
// a single upstream hop is amortised across the TTL window. Pitfall #6: V is
// *pypiProject, never pypiProject — mixing pointer/value would split the cache
// by type identity.
func (a *Adapter) projectFor(ctx context.Context, pkg string, incPre bool) (*pypiProject, error) {
	if strings.ContainsAny(pkg, "/ \t\n") {
		return nil, errs.InvalidInput("pypi package name must not contain path separators or whitespace", "pkg", pkg)
	}
	key := cache.Key{Manager: "pypi", Pkg: pkg, Op: "project", IncPre: incPre}
	return cache.Get[*pypiProject](ctx, a.cache, key, func(ctx context.Context) (*pypiProject, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, projectURL(pkg), nil)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "pypi")
		}
		req.Header.Set("Accept", "application/json")
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "pypi")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, httperr.MapHTTPStatus(resp, pkg, "pypi")
		}
		var p pypiProject
		if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "pypi", "reason", "malformed_body")
		}
		return &p, nil
	})
}

// isYanked reports whether the version identified by key is yanked per PEP 592.
// A version is yanked if: len(files) > 0 AND all file objects have Yanked=true
// (Pitfall 4 algorithm / A1 heuristic). An empty file list is treated as
// non-yanked (conservative default for future-proofing).
func isYanked(files []pypiFile) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !f.Yanked {
			return false
		}
	}
	return true
}

// Validate answers "does this exact version exist for this package?". Per
// REG-02, both the input version and every key in the releases map are
// normalized via filter.PEP440Normalize before equality comparison.
//
// A yanked release returns Exists=true with Source="pypi-yanked" (D-PYPI-01).
func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	p, err := a.projectFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}

	// Normalise the input version for PEP 440 comparison.
	normInput := filter.PEP440Normalize(version)

	for key, files := range p.Releases {
		// Normalise the map key before comparing.
		if filter.PEP440Normalize(key) == normInput {
			if isYanked(files) {
				return registry.ValidateResult{Exists: true, Source: SourceYanked}, nil
			}
			return registry.ValidateResult{Exists: true, Source: SourceVersionsMap}, nil
		}
	}

	return registry.ValidateResult{}, errs.NotFound(
		"pypi version not in releases map",
		"pkg", pkg, "version", version,
	)
}

// Latest answers "what is the latest version?".
//
// Fast path: when !incPre && major==nil && minor==nil, the info.version field
// is trusted as the canonical latest-stable pointer (Source="pypi-info-version").
//
// Otherwise, release keys are collected and filtered via
// filter.FilterAndPickHighest (Source="computed-highest"). The keys are
// unprefixed (vPrefixed=false).
func (a *Adapter) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	p, err := a.projectFor(ctx, pkg, incPre)
	if err != nil {
		return registry.LatestResult{}, err
	}

	// Fast path: unfiltered + stable-only → info.version verbatim.
	if !incPre && major == nil && minor == nil {
		if p.Info.Version != "" {
			return registry.LatestResult{Version: p.Info.Version, Source: SourceInfoVersion}, nil
		}
		// Fallthrough: rare; info.version missing.
	}

	keys := make([]string, 0, len(p.Releases))
	for k, files := range p.Releases {
		if isYanked(files) {
			continue
		}
		keys = append(keys, k)
	}

	// vPrefixed=false: PyPI versions are not v-prefixed.
	highest, ok := filter.FilterAndPickHighest(keys, false, incPre, major, minor)
	if !ok {
		return registry.LatestResult{}, errs.NotFound(
			"no version matches filter",
			"pkg", pkg, "incPre", incPre, "major", major, "minor", minor,
		)
	}
	return registry.LatestResult{Version: highest, Source: SourceComputed}, nil
}

// Versions returns all known non-yanked version strings for the package. The
// list comes from the project's Releases map keys, filtered via isYanked per
// PEP 592 (D-PYPI-01), and is already cached after the first Validate or
// Latest call.
func (a *Adapter) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
	proj, err := a.projectFor(ctx, pkg, incPre)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(proj.Releases))
	for k, files := range proj.Releases {
		if isYanked(files) {
			continue // exclude yanked per PEP 592 - D-PYPI-01
		}
		keys = append(keys, k)
	}
	return keys, nil
}
