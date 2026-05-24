// Package crate implements the registry.Registry interface for crates.io.
package crate

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
	sourceCrateYanked     = "crate-yanked"
	sourceComputedHighest = "computed-highest"
	maxBodyBytes          = 5 << 20
)

type crateResponse struct {
	Versions []crateVersion `json:"versions"`
}

type crateVersion struct {
	Num    string `json:"num"`
	Yanked bool   `json:"yanked"`
}

type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

func (a *Adapter) Name() string { return "crate" }

func (a *Adapter) crateFor(ctx context.Context, pkg string, incPre bool) (*crateResponse, error) {
	if strings.ContainsAny(pkg, "/ \t\n") {
		return nil, errs.InvalidInput("crate name must not contain path separators or whitespace", "pkg", pkg)
	}

	key := cache.Key{Manager: "crate", Pkg: pkg, Op: "crate-meta", IncPre: incPre}
	return cache.Get[*crateResponse](ctx, a.cache, key, func(ctx context.Context) (*crateResponse, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, crateURL(pkg), nil)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "crate")
		}
		req.Header.Set("Accept", "application/json")

		resp, err := a.client.Do(req)
		if err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "crate")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, httperr.MapHTTPStatus(resp, pkg, "crate")
		}

		var out crateResponse
		if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&out); err != nil {
			return nil, errs.UpstreamDown(err, "pkg", pkg, "registry", "crate", "reason", "malformed_body")
		}
		return &out, nil
	})
}

func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	data, err := a.crateFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}

	for _, v := range data.Versions {
		if v.Num != version {
			continue
		}
		if v.Yanked {
			return registry.ValidateResult{Exists: true, Source: sourceCrateYanked}, nil
		}
		return registry.ValidateResult{Exists: true, Source: sourceVersionsList}, nil
	}

	return registry.ValidateResult{Exists: false, Source: ""}, nil
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
	data, err := a.crateFor(ctx, pkg, incPre)
	if err != nil {
		return nil, err
	}

	versions := make([]string, 0, len(data.Versions))
	for _, v := range data.Versions {
		if v.Yanked {
			continue
		}
		versions = append(versions, v.Num)
	}
	return versions, nil
}
