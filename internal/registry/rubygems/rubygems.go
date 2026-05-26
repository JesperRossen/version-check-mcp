// Package rubygems implements the registry.Registry interface for RubyGems.
package rubygems

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

var _ registry.Registry = (*Adapter)(nil)

const (
	sourceVersionsList    = "versions-list"
	sourceComputedHighest = "computed-highest"
	maxBodyBytes          = 5 << 20
)

type gemVersion struct {
	Number     string `json:"number"`
	Prerelease bool   `json:"prerelease"`
	Platform   string `json:"platform"`
}

type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

func (a *Adapter) Name() string { return "rubygems" }

func (a *Adapter) versionsFor(ctx context.Context, pkg string, incPre bool) ([]gemVersion, error) {
	if strings.ContainsAny(pkg, "/ \t\n") {
		return nil, errs.InvalidInput("rubygems package name must not contain path separators or whitespace", "pkg", pkg)
	}

	key := cache.Key{Manager: "rubygems", Pkg: pkg, Op: "versions", IncPre: incPre}
	return cache.Get[[]gemVersion](ctx, a.cache, key, func(ctx context.Context) ([]gemVersion, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsURL(pkg), nil)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "rubygems")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := a.client.Do(req)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "rubygems")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, httperr.MapHTTPStatus(resp, pkg, "rubygems")
		}

		var out []gemVersion
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&out); err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "rubygems", "reason", "malformed_body")
		}
		return out, nil
	})
}

func dedupVersions(versions []gemVersion) []gemVersion {
	seen := make(map[string]bool, len(versions))
	out := make([]gemVersion, 0, len(versions))
	for _, v := range versions {
		if seen[v.Number] {
			continue
		}
		seen[v.Number] = true
		out = append(out, v)
	}
	return out
}

func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	versions, err := a.versionsFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}

	for _, v := range dedupVersions(versions) {
		if v.Number == version {
			return registry.ValidateResult{Exists: true, Source: sourceVersionsList}, nil
		}
	}

	return registry.ValidateResult{}, errs.NotFound(
		"rubygems version not in versions list",
		"pkg", pkg, "version", version,
	)
}

func (a *Adapter) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	versions, err := a.Versions(ctx, pkg, incPre)
	if err != nil {
		return registry.LatestResult{}, err
	}

	winner, ok := filter.FilterAndPickHighest(versions, false, incPre, major, minor)
	if !ok {
		return registry.LatestResult{}, errs.NotFound(
			"no version matches filter",
			"pkg", pkg, "incPre", incPre, "major", major, "minor", minor,
		)
	}

	return registry.LatestResult{Version: winner, Source: sourceComputedHighest}, nil
}

func (a *Adapter) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
	versions, err := a.versionsFor(ctx, pkg, incPre)
	if err != nil {
		return nil, err
	}

	deduped := dedupVersions(versions)
	out := make([]string, 0, len(deduped))
	for _, v := range deduped {
		if !incPre && v.Prerelease {
			continue
		}
		out = append(out, v.Number)
	}
	return out, nil
}
