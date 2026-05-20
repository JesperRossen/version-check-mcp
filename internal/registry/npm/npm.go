package npm

import (
	"context"
	"net/http"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

// Adapter implements registry.Registry against https://registry.npmjs.org.
// Construction is via New; the constructor takes the shared *http.Client
// (the agent identifier header is injected by its Transport — the adapter
// itself sets no such header) and the shared *cache.Cache.
type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

// New constructs an NPM Adapter.
func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

// Name returns "npm".
func (a *Adapter) Name() string { return "npm" }

// packumentFor fetches the packument for pkg, going through cache.Get so a
// single upstream hop is amortised across the TTL window (and concurrent
// callers collapse via singleflight). Pitfall #6: V is *Packument, never
// Packument — mixing pointer and value would split the cache by type identity.
func (a *Adapter) packumentFor(ctx context.Context, pkg string, incPre bool) (*Packument, error) {
	key := cache.Key{Manager: "npm", Pkg: pkg, Op: "packument", IncPre: incPre}
	return cache.Get[*Packument](ctx, a.cache, key, func(ctx context.Context) (*Packument, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, packumentURL(pkg), nil)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg)
		}
		req.Header.Set("Accept", "application/json")
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, httperr.MapHTTPStatus(resp, pkg, "npm")
		}
		p, err := parsePackument(resp.Body)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "reason", "malformed_body")
		}
		return p, nil
	})
}

// Validate answers "does this exact version exist for this package?". Per
// D-FILTER-01 / D-SOURCE-01, Validate does NOT filter prereleases — it is an
// existence question, not a filter question. The IncPre bit in the cache key
// is for Latest's benefit; both Validate calls (IncPre=true and IncPre=false)
// read the same Versions map, so the result is identical whichever slot they
// land in. The boolean discrimination keeps the cache contract uniform across
// Validate and Latest.
func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	p, err := a.packumentFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}
	if _, ok := p.Versions[version]; !ok {
		return registry.ValidateResult{}, errs.NotFound(
			"npm version not in versions map",
			"pkg", pkg, "version", version,
		)
	}
	return registry.ValidateResult{Exists: true, Source: "versions-map"}, nil
}

// Latest answers "what is the latest version?". Three Source enum strings are
// possible (D-SOURCE-01): "dist-tags.latest" (fast path: unfiltered +
// stable-only), or "computed-highest" (everything else — IncPre=true, or any
// major/minor filter).
func (a *Adapter) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	p, err := a.packumentFor(ctx, pkg, incPre)
	if err != nil {
		return registry.LatestResult{}, err
	}

	// Fast path: unfiltered + stable-only → dist-tags.latest verbatim.
	if !incPre && major == nil && minor == nil {
		if v, ok := p.DistTags["latest"]; ok && v != "" {
			return registry.LatestResult{Version: v, Source: "dist-tags.latest"}, nil
		}
		// Fallthrough: rare; packument missing dist-tags.latest.
	}

	keys := make([]string, 0, len(p.Versions))
	for k := range p.Versions {
		keys = append(keys, k)
	}
	// vPrefixed=false: npm versions are unprefixed (no "v" prefix on the wire).
	highest, ok := filter.FilterAndPickHighest(keys, false, incPre, major, minor)
	if !ok {
		return registry.LatestResult{}, errs.NotFound(
			"no version matches filter",
			"pkg", pkg, "incPre", incPre, "major", major, "minor", minor,
		)
	}
	return registry.LatestResult{Version: highest, Source: "computed-highest"}, nil
}

// Versions returns all known version strings for the package. The list comes
// from the packument's Versions map, which is already cached after the first
// Validate or Latest call.
func (a *Adapter) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
	p, err := a.packumentFor(ctx, pkg, incPre)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(p.Versions))
	for k := range p.Versions {
		keys = append(keys, k)
	}
	return keys, nil
}
