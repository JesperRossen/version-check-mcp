package gomod

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

// Source enum values for Go Modules adapter.
const (
	sourceProxyLatest    = "proxy-latest"     // @latest endpoint trusted verbatim
	sourceComputedHighest = "computed-highest" // filtered highest from @v/list
	sourceVersionsList   = "versions-list"    // membership check in @v/list
)

// latestResponse is the JSON body shape from proxy.golang.org/{mod}/@latest.
type latestResponse struct {
	Version string `json:"Version"`
	Time    string `json:"Time"`
}

// Adapter implements registry.Registry for proxy.golang.org.
// It uses the GOPROXY protocol: @v/list for the full version list and
// @latest as a stable-hint. Module paths with capital letters are escaped
// via module.EscapePath before URL construction (D-GOMOD-01, Pitfall 1).
type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

// New constructs a gomod Adapter. The caller provides the shared *http.Client
// (UA-injecting transport) and shared *cache.Cache.
func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

// Name returns "gomod".
func (a *Adapter) Name() string { return "gomod" }

// listFor fetches and caches the @v/list for mod. The list is a
// newline-delimited text body from proxy.golang.org/{escaped_mod}/@v/list.
// Results are cached under Key{Manager:"gomod", Pkg:mod, Op:"list", IncPre:incPre}.
func (a *Adapter) listFor(ctx context.Context, mod string, incPre bool) ([]string, error) {
	key := cache.Key{Manager: "gomod", Pkg: mod, Op: "list", IncPre: incPre}
	return cache.Get[[]string](ctx, a.cache, key, func(ctx context.Context) ([]string, error) {
		u, err := ListURL(mod)
		if err != nil {
			// module.EscapePath rejected the path.
			return nil, errs.UpstreamDown(err, "pkg", mod, "registry", "gomod", "reason", "invalid_module_path")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", mod, "registry", "gomod")
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", mod, "registry", "gomod")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, httperr.MapHTTPStatus(resp, mod, "gomod")
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", mod, "registry", "gomod", "reason", "read_body")
		}
		// Split on newline; TrimRight drops the trailing newline; filter empties.
		raw := strings.TrimRight(string(body), "\n")
		if raw == "" {
			return []string{}, nil
		}
		parts := strings.Split(raw, "\n")
		versions := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				versions = append(versions, p)
			}
		}
		return versions, nil
	})
}

// latestFor fetches and caches the @latest response for mod.
// Returns the Version field from the JSON body.
// Results are cached under Key{Manager:"gomod", Pkg:mod, Op:"latest", IncPre:incPre}.
func (a *Adapter) latestFor(ctx context.Context, mod string, incPre bool) (string, error) {
	key := cache.Key{Manager: "gomod", Pkg: mod, Op: "latest", IncPre: incPre}
	return cache.Get[string](ctx, a.cache, key, func(ctx context.Context) (string, error) {
		u, err := LatestURL(mod)
		if err != nil {
			return "", errs.UpstreamDown(err, "pkg", mod, "registry", "gomod", "reason", "invalid_module_path")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return "", errs.UpstreamDown(err, "pkg", mod, "registry", "gomod")
		}
		resp, err := a.client.Do(req)
		if err != nil {
			return "", errs.UpstreamDown(err, "pkg", mod, "registry", "gomod")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", httperr.MapHTTPStatus(resp, mod, "gomod")
		}
		var lr latestResponse
		if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
			return "", errs.UpstreamDown(err, "pkg", mod, "registry", "gomod", "reason", "malformed_body")
		}
		return lr.Version, nil
	})
}

// Validate answers "does this exact version exist for this module?".
// It fetches the @v/list and checks membership by exact string equality.
// Per D-GOMOD-03: an explicit pseudo-version present in the list returns
// Exists=true regardless of incPre (validate is an existence check, not a filter).
func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	versions, err := a.listFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}
	for _, v := range versions {
		if v == version {
			return registry.ValidateResult{Exists: true, Source: sourceVersionsList}, nil
		}
	}
	return registry.ValidateResult{}, errs.NotFound(
		"gomod version not in @v/list",
		"pkg", pkg, "version", version,
	)
}

// Latest answers "what is the latest version of this module?".
//
// Fast path (Source="proxy-latest"): when incPre=false and no major/minor
// filter is set and @latest returns a non-pseudo, non-prerelease version,
// return @latest verbatim.
//
// Fallback (Source="computed-highest"): when @latest returns a pseudo-version
// or prerelease (and incPre=false), or when major/minor filter is set,
// compute highest from the @v/list via filter.FilterAndPickHighest.
func (a *Adapter) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	// Try @latest fast path when no filter is constraining the result.
	if major == nil && minor == nil {
		latest, err := a.latestFor(ctx, pkg, incPre)
		if err == nil {
			// Determine whether @latest is usable as the stable pointer.
			isPseudo := module.IsPseudoVersion(latest)
			isPre := semver.Prerelease(latest) != ""
			// incPre=true admits pseudo and prerelease versions verbatim.
			if incPre || (!isPseudo && !isPre) {
				return registry.LatestResult{Version: latest, Source: sourceProxyLatest}, nil
			}
			// @latest is pseudo/prerelease and incPre=false — fall through to computed.
		}
		// @latest failed or is unusable; fall through to computed-highest.
	}

	// Computed-highest path: fetch the list and apply the filter.
	versions, err := a.listFor(ctx, pkg, incPre)
	if err != nil {
		return registry.LatestResult{}, err
	}
	highest, ok := filter.FilterAndPickHighest(versions, true /* vPrefixed */, incPre, major, minor)
	if !ok {
		return registry.LatestResult{}, errs.NotFound(
			"no gomod version matches filter",
			"pkg", pkg, "incPre", incPre, "major", major, "minor", minor,
		)
	}
	return registry.LatestResult{Version: highest, Source: sourceComputedHighest}, nil
}

// Versions returns all known version strings for the module. The list comes
// from the proxy @v/list endpoint, which is already cached after the first
// Validate or Latest call. Versions are v-prefixed (Go ecosystem-native).
func (a *Adapter) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
	return a.listFor(ctx, pkg, incPre)
}
