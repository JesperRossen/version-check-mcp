package gh_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/gh"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

// Compile-time assertion that *gh.Adapter satisfies the registry.Registry
// seam locked in D-05.
var _ registry.Registry = (*gh.Adapter)(nil)

const (
	ghFixturesPath = "../../../testdata/fixtures/gh"

	// Repo names used in tests.
	repoCheckout    = "actions/checkout"
	repoShort       = "actions/short"
	repoRateLimited = "actions/ratelimited"
	repoNonexistent = "actions/nonexistent"

	// Fixture URL constants (computed from tagsURL / releasesLatestURL shapes).
	urlCheckoutTagsP1   = "https://api.github.com/repos/actions/checkout/tags?per_page=100&page=1"
	urlCheckoutTagsP2   = "https://api.github.com/repos/actions/checkout/tags?per_page=100&page=2"
	urlCheckoutRelLatest = "https://api.github.com/repos/actions/checkout/releases/latest"

	urlShortTagsP1 = "https://api.github.com/repos/actions/short/tags?per_page=100&page=1"

	urlRateLimitedTagsP1 = "https://api.github.com/repos/actions/ratelimited/tags?per_page=100&page=1"
	urlNonexistentTagsP1 = "https://api.github.com/repos/actions/nonexistent/tags?per_page=100&page=1"
)

func intPtr(i int) *int { return &i }

// newAdapter builds an Adapter wired to a fixture-replay client.
// callCount reflects upstream-fetch invocations.
func newAdapter(t *testing.T, callCount *atomic.Int64) *gh.Adapter {
	t.Helper()
	if callCount == nil {
		callCount = &atomic.Int64{}
	}
	mapURL := func(u string) string {
		switch u {
		case urlCheckoutTagsP1:
			return "checkout-tags-p1.json"
		case urlCheckoutTagsP2:
			return "checkout-tags-p2.json"
		case urlCheckoutRelLatest:
			return "checkout-releases-latest.json"
		case urlShortTagsP1:
			return "checkout-tags-p1-short.json"
		case urlRateLimitedTagsP1:
			return "rate-limited.json"
		case urlNonexistentTagsP1:
			return "nonexistent.json"
		default:
			t.Fatalf("unmapped fixture URL: %q", u)
			return ""
		}
	}
	client := testfixtures.FixtureClient(t, ghFixturesPath, mapURL, callCount)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(func() { c.Close() })
	return gh.New(client, c)
}

// TestValidate_HitNonSemverV6 proves D-GH-04: validate uses exact string match,
// so "v6" (which is not semver-valid) must return exists:true.
func TestValidate_HitNonSemverV6(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), repoCheckout, "v6", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false, want true for non-semver tag 'v6'")
	}
	if res.Source != "versions-list" {
		t.Fatalf("Source = %q, want %q", res.Source, "versions-list")
	}
}

// TestValidate_HitSemverV602 proves a semver-valid tag returns exists:true.
func TestValidate_HitSemverV602(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), repoCheckout, "v6.0.2", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false, want true for semver tag 'v6.0.2'")
	}
	if res.Source != "versions-list" {
		t.Fatalf("Source = %q, want %q", res.Source, "versions-list")
	}
}

// TestValidate_MissVersion proves a non-existent version returns KindNotFound.
func TestValidate_MissVersion(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), repoCheckout, "v999.0.0", false)
	if err == nil {
		t.Fatalf("Validate err = nil, want NotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err is %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// TestValidate_RateLimited proves 403 + X-RateLimit-Remaining:0 maps to
// KindRateLimited with reset_at detail matching the fixture's Unix timestamp.
func TestValidate_RateLimited(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), repoRateLimited, "v1.0.0", false)
	if err == nil {
		t.Fatalf("Validate err = nil, want RateLimited")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err is %T, want *errs.E", err)
	}
	if e.Kind != errs.KindRateLimited {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindRateLimited)
	}
	// Fixture X-RateLimit-Reset is "1999999999"
	resetAt, ok := e.Details["reset_at"]
	if !ok {
		t.Fatalf("Details missing reset_at key; details = %v", e.Details)
	}
	// reset_at should be int64(1999999999)
	resetInt, ok := resetAt.(int64)
	if !ok {
		t.Fatalf("reset_at type = %T, want int64; value = %v", resetAt, resetAt)
	}
	if resetInt != 1999999999 {
		t.Fatalf("reset_at = %d, want 1999999999", resetInt)
	}
}

// TestValidate_NotFound404 proves 404 response maps to KindNotFound.
func TestValidate_NotFound404(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), repoNonexistent, "v1.0.0", false)
	if err == nil {
		t.Fatalf("Validate err = nil, want NotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err is %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// TestPagination_FetchesPageTwoWhen100 proves D-GH-01: when page 1 has exactly
// 100 entries, page 2 is also fetched. The 100-entry fixture causes 2 HTTP
// calls on first invocation.
func TestPagination_FetchesPageTwoWhen100(t *testing.T) {
	var calls atomic.Int64
	a := newAdapter(t, &calls)
	_, err := a.Validate(context.Background(), repoCheckout, "v6.0.2", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want 2 (page1 + page2 for 100-entry fixture)", got)
	}
}

// TestPagination_NoPageTwoWhenShort proves D-GH-01: when page 1 has fewer than
// 100 entries, page 2 is NOT fetched.
func TestPagination_NoPageTwoWhenShort(t *testing.T) {
	var calls atomic.Int64
	a := newAdapter(t, &calls)
	_, err := a.Validate(context.Background(), repoShort, "v6.0.2", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (no page2 for short fixture)", got)
	}
}

// TestLatest_ReleasePointer proves that when incPre=false and no major/minor
// filter is set, /releases/latest is used as the hint (Source="registry-release-pointer").
func TestLatest_ReleasePointer(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), repoCheckout, false, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "registry-release-pointer" {
		t.Fatalf("Source = %q, want %q", res.Source, "registry-release-pointer")
	}
	if res.Version == "" {
		t.Fatalf("Version is empty")
	}
}

// TestLatest_FilterMajorSkipsNonSemver proves that when a major filter is set,
// the computed-highest path is used and non-semver tags like "v6" are skipped.
// The highest semver-valid v6.x.y tag is returned.
func TestLatest_FilterMajorSkipsNonSemver(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), repoCheckout, false, intPtr(6), nil)
	if err != nil {
		t.Fatalf("Latest(major=6) err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want %q", res.Source, "computed-highest")
	}
	// v6 is NOT semver-valid; v6.0.2 is the highest semver-valid v6.x.y tag.
	if res.Version == "v6" {
		t.Fatalf("Version = 'v6' (non-semver); should have been filtered out by FilterAndPickHighest")
	}
	if res.Version == "" {
		t.Fatalf("Version is empty")
	}
	// Must be a semver-valid tag starting with v6.
	if res.Version != "v6.0.2" {
		t.Fatalf("Version = %q, want v6.0.2 (highest semver-valid v6.x)", res.Version)
	}
}

// TestLatest_IncPre proves that when incPre=true, the computed-highest path is
// used (the /releases/latest hint is bypassed, as it only reports stable).
func TestLatest_IncPre(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), repoCheckout, true, nil, nil)
	if err != nil {
		t.Fatalf("Latest(incPre=true) err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want %q", res.Source, "computed-highest")
	}
	if res.Version == "" {
		t.Fatalf("Version is empty")
	}
}

// TestCache_HitOnSecondCall proves the tags list is cached: two consecutive
// Validate calls trigger exactly 2 upstream calls (p1+p2) on the first
// invocation and 0 additional calls on the second.
func TestCache_HitOnSecondCall(t *testing.T) {
	var calls atomic.Int64
	a := newAdapter(t, &calls)
	ctx := context.Background()
	if _, err := a.Validate(ctx, repoCheckout, "v6.0.2", false); err != nil {
		t.Fatalf("first Validate err = %v", err)
	}
	beforeSecond := calls.Load()
	if _, err := a.Validate(ctx, repoCheckout, "v6.0.2", false); err != nil {
		t.Fatalf("second Validate err = %v", err)
	}
	if after := calls.Load(); after != beforeSecond {
		t.Fatalf("second Validate caused %d extra upstream calls, want 0", after-beforeSecond)
	}
}
