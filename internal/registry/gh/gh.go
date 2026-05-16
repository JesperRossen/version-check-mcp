// Package gh implements the GitHub Actions registry adapter (REG-04).
//
// Version list: GET /repos/{owner}/{repo}/tags?per_page=100 (up to 2 pages,
// D-GH-01). Cached under Key{Manager:"gh", Pkg:repo, Op:"tags", IncPre:incPre}.
//
// Latest-stable hint: GET /repos/{owner}/{repo}/releases/latest — used when
// incPre=false and no major/minor filter is set (D-GH-02). Source value is
// "registry-release-pointer". Falls back to filter.FilterAndPickHighest (source
// "computed-highest") when the hint is bypassed.
//
// Validate: exact string match in the cached tags list (D-GH-04 — no semver
// validation on validate; "v6" is a valid match if the repo tags it).
//
// Rate-limit override (D-GH-03): 403 + X-RateLimit-Remaining: 0 maps to
// errs.RateLimited with reset_at = X-RateLimit-Reset (Unix timestamp). This
// is the GitHub-specific override; all other non-2xx statuses are delegated to
// httperr.MapHTTPStatus.
package gh

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/filter"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
	"github.com/JesperRossen/version-check-mcp/internal/registry"

	"golang.org/x/mod/semver"
)

// Source constants used in ValidateResult.Source and LatestResult.Source.
const (
	sourceVersionsList    = "versions-list"
	sourceReleasePointer  = "registry-release-pointer"
	sourceComputedHighest = "computed-highest"
)

// Compile-time assertion that *Adapter satisfies the registry.Registry seam.
var _ registry.Registry = (*Adapter)(nil)

// Adapter implements registry.Registry against https://api.github.com.
type Adapter struct {
	client *http.Client
	cache  *cache.Cache
}

// New constructs a GitHub Actions Adapter.
func New(client *http.Client, c *cache.Cache) *Adapter {
	return &Adapter{client: client, cache: c}
}

// Name returns "gh".
func (a *Adapter) Name() string { return "gh" }

// tagEntry is the shape of one element in the GitHub /tags response array.
type tagEntry struct {
	Name string `json:"name"`
}

// releaseLatest is the shape of the GitHub /releases/latest response.
type releaseLatest struct {
	TagName string `json:"tag_name"`
}

// mapErr converts a non-2xx GitHub response into a typed *errs.E.
//
// GitHub-specific override (D-GH-03): a 403 response with
// X-RateLimit-Remaining: "0" is a rate-limit, not a forbidden error. We parse
// X-RateLimit-Reset (Unix timestamp) and return errs.RateLimited with reset_at.
//
// All other non-2xx statuses are delegated to httperr.MapHTTPStatus.
func mapErr(resp *http.Response, pkg string) error {
	if resp.StatusCode == http.StatusForbidden &&
		resp.Header.Get("X-RateLimit-Remaining") == "0" {
		reset := time.Now().Add(30 * time.Second) // safe fallback when header absent
		if v := resp.Header.Get("X-RateLimit-Reset"); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				reset = time.Unix(n, 0)
			}
		}
		return errs.RateLimited(reset,
			"pkg", pkg,
			"registry", "gh",
		)
	}
	return httperr.MapHTTPStatus(resp, pkg, "gh")
}

// tagsFor fetches the full tags list for the repository, going through the
// cache+singleflight layer. D-GH-01: a second page is fetched only when the
// first page returns exactly 100 entries.
func (a *Adapter) tagsFor(ctx context.Context, repo string, incPre bool) ([]string, error) {
	key := cache.Key{Manager: "gh", Pkg: repo, Op: "tags", IncPre: incPre}
	return cache.Get[[]string](ctx, a.cache, key, func(ctx context.Context) ([]string, error) {
		page1, err := a.fetchTagsPage(ctx, repo, 1)
		if err != nil {
			return nil, err
		}
		out := page1
		if len(page1) == 100 {
			page2, err := a.fetchTagsPage(ctx, repo, 2)
			if err != nil {
				return nil, err
			}
			out = append(out, page2...)
			if len(page2) == 100 {
				// More than 200 tags exist; results are incomplete.
				// Return an error rather than silently producing wrong answers.
				return nil, errs.UpstreamDown(nil,
					"pkg", repo, "registry", "gh",
					"reason", "too_many_tags_truncated",
				)
			}
		}
		return out, nil
	})
}

// fetchTagsPage performs a single GET /repos/{repo}/tags?per_page=100&page=N
// and returns the tag name strings.
func (a *Adapter) fetchTagsPage(ctx context.Context, repo string, page int) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL(repo, page), nil)
	if err != nil {
		return nil, errs.UpstreamDown(err, "pkg", repo, "registry", "gh")
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, errs.UpstreamDown(err, "pkg", repo, "registry", "gh")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, mapErr(resp, repo)
	}
	var entries []tagEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, errs.UpstreamDown(err, "pkg", repo, "registry", "gh", "reason", "malformed_tags_body")
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

// releaseLatestFor fetches the /releases/latest tag_name for the repository,
// going through the cache+singleflight layer.
func (a *Adapter) releaseLatestFor(ctx context.Context, repo string, incPre bool) (string, error) {
	key := cache.Key{Manager: "gh", Pkg: repo, Op: "release-latest", IncPre: incPre}
	return cache.Get[string](ctx, a.cache, key, func(ctx context.Context) (string, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesLatestURL(repo), nil)
		if err != nil {
			return "", errs.UpstreamDown(err, "pkg", repo, "registry", "gh")
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := a.client.Do(req)
		if err != nil {
			return "", errs.UpstreamDown(err, "pkg", repo, "registry", "gh")
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", mapErr(resp, repo)
		}
		var rel releaseLatest
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
			return "", errs.UpstreamDown(err, "pkg", repo, "registry", "gh", "reason", "malformed_release_body")
		}
		return rel.TagName, nil
	})
}

// Validate answers "does this exact version exist for this repository?".
//
// D-GH-04: uses exact string match against the cached tags list. "v6" is a
// valid match even though it is not semver-valid — validate is an existence
// check, not a semver validation.
func (a *Adapter) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	tags, err := a.tagsFor(ctx, pkg, incPre)
	if err != nil {
		return registry.ValidateResult{}, err
	}
	for _, t := range tags {
		if t == version {
			return registry.ValidateResult{Exists: true, Source: sourceVersionsList}, nil
		}
	}
	return registry.ValidateResult{}, errs.NotFound(
		"gh tag not in tags list",
		"pkg", pkg,
		"version", version,
		"registry", "gh",
	)
}

// Latest answers "what is the latest version?".
//
// Fast path (D-GH-02): when incPre=false and no major/minor filter is set,
// attempt /releases/latest. If it returns a semver-valid tag_name, return it
// as Source="registry-release-pointer". If the releases endpoint returns
// not-found (repo has no formal releases), silently fall through to the tag
// filter. Other errors bubble up.
//
// Fallback: filter.FilterAndPickHighest over the tags list with vPrefixed=true.
// Non-semver tags like "v6" are silently skipped by the filter. Source="computed-highest".
func (a *Adapter) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	// Fast path: unfiltered stable-only → try /releases/latest pointer.
	if !incPre && major == nil && minor == nil {
		tagName, err := a.releaseLatestFor(ctx, pkg, incPre)
		if err == nil && semver.IsValid(tagName) {
			return registry.LatestResult{Version: tagName, Source: sourceReleasePointer}, nil
		}
		if err != nil {
			// If the repo has no formal releases, the endpoint returns 404 →
			// KindNotFound. Fall through silently to the tags filter.
			var e *errs.E
			if errors.As(err, &e) && e.Kind == errs.KindNotFound {
				// fall through
			} else {
				// Any other error (rate-limit, upstream-down, etc.) bubbles up.
				return registry.LatestResult{}, err
			}
		}
	}

	// Fallback / filtered path: compute from the tags list.
	tags, err := a.tagsFor(ctx, pkg, incPre)
	if err != nil {
		return registry.LatestResult{}, err
	}
	v, ok := filter.FilterAndPickHighest(tags, true, incPre, major, minor)
	if !ok {
		return registry.LatestResult{}, errs.NotFound(
			"no gh tag matches filter",
			"pkg", pkg,
			"incPre", incPre,
			"major", major,
			"minor", minor,
			"registry", "gh",
		)
	}
	return registry.LatestResult{Version: v, Source: sourceComputedHighest}, nil
}
