package npm_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/npm"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

// Compile-time assertion that *npm.Adapter satisfies the registry.Registry
// seam locked in D-05.
var _ registry.Registry = (*npm.Adapter)(nil)

const (
	// reactHitVersion is a stable, real-but-not-latest version of react that
	// is known to exist in testdata/fixtures/npm/react.json — used as the
	// canonical Validate-hit input to avoid coupling to dist-tags.latest.
	reactHitVersion = "18.3.1"
	// typesNodeHitVersion is a version known to exist in types-node.json.
	typesNodeHitVersion = "22.0.0"

	npmReactURL     = "https://registry.npmjs.org/react"
	npmScopedURL    = "https://registry.npmjs.org/@types%2Fnode"
	npmNotFoundURL  = "https://registry.npmjs.org/nonexistent"
	npmFixturesPath = "../../../testdata/fixtures/npm"
)

func intPtr(i int) *int { return &i }

// newAdapter builds an Adapter wired to a fixture-replay client. callCount
// reflects upstream-fetch invocations (one increment per RoundTrip). seenURL,
// if non-nil, captures the most recent request URL for URL-canary assertions.
func newAdapter(t *testing.T, seenURL *string) (*npm.Adapter, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	mapURL := func(u string) string {
		if seenURL != nil {
			*seenURL = u
		}
		switch u {
		case npmReactURL:
			return "react.json"
		case npmScopedURL:
			return "types-node.json"
		case npmNotFoundURL:
			return "nonexistent.json"
		default:
			t.Fatalf("unmapped fixture URL: %q", u)
			return ""
		}
	}
	client := testfixtures.FixtureClient(t, npmFixturesPath, mapURL, &calls)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(func() { c.Close() })
	return npm.New(client, c), &calls
}

func TestNPMValidate_Hit(t *testing.T) {
	a, calls := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), "react", reactHitVersion, false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false, want true")
	}
	if res.Source != "versions-map" {
		t.Fatalf("Source = %q, want %q", res.Source, "versions-map")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

func TestNPMValidate_Miss_VersionAbsent(t *testing.T) {
	a, calls := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), "react", "99.0.0", false)
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
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

func TestNPMValidate_Miss_PackageNotFound(t *testing.T) {
	a, calls := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), "nonexistent", "1.0.0", false)
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
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

// TEST-02 canary + REG-01: the exact upstream URL for "@types/node" must be
// "https://registry.npmjs.org/@types%2Fnode" — verified via the captured
// req.URL.String() in the fixture client's urlToFile callback.
func TestNPMValidate_ScopedPackage_URLEncoding(t *testing.T) {
	var seen string
	a, _ := newAdapter(t, &seen)
	res, err := a.Validate(context.Background(), "@types/node", typesNodeHitVersion, false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if seen != npmScopedURL {
		t.Fatalf("upstream URL = %q, want %q", seen, npmScopedURL)
	}
	if !res.Exists {
		t.Fatalf("Exists = false, want true")
	}
}

// LAT-01 fast path: unfiltered + IncPre=false → dist-tags.latest verbatim.
// The react fixture's dist-tags.latest is whatever was current when 02-01
// recorded the fixture; the assertion couples to the fixture, not a literal.
func TestNPMLatest_DistTags(t *testing.T) {
	a, _ := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "react", false, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "dist-tags.latest" {
		t.Fatalf("Source = %q, want %q", res.Source, "dist-tags.latest")
	}
	if res.Version == "" {
		t.Fatalf("Version is empty")
	}
}

// LAT-04: IncPre=true overrides the dist-tags fast path.
func TestNPMLatest_IncPre(t *testing.T) {
	a, _ := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "react", true, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want %q", res.Source, "computed-highest")
	}
	if res.Version == "" {
		t.Fatalf("Version is empty")
	}
}

// LAT-05: major filter forces the computed-highest branch.
func TestNPMLatest_MajorFilter(t *testing.T) {
	a, _ := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "react", false, intPtr(17), nil)
	if err != nil {
		t.Fatalf("Latest(major=17) err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want %q", res.Source, "computed-highest")
	}
	if !strings.HasPrefix(res.Version, "17.") {
		t.Fatalf("Version = %q, want 17.* prefix", res.Version)
	}

	_, err = a.Latest(context.Background(), "react", false, intPtr(99), nil)
	if err == nil {
		t.Fatalf("Latest(major=99) err = nil, want NotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err is %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// Phase 2 success criterion #1: second sequential Validate on same adapter
// hits the cache — upstream RoundTripper called exactly once.
func TestNPMCache_HitOnSecondCall(t *testing.T) {
	a, calls := newAdapter(t, nil)
	ctx := context.Background()
	if _, err := a.Validate(ctx, "react", reactHitVersion, false); err != nil {
		t.Fatalf("first Validate err = %v", err)
	}
	if _, err := a.Validate(ctx, "react", reactHitVersion, false); err != nil {
		t.Fatalf("second Validate err = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

// CACHE-03 / singleflight: 50 concurrent Validates collapse to one fetch.
func TestNPMCache_SingleflightDedupes(t *testing.T) {
	a, calls := newAdapter(t, nil)
	ctx := context.Background()
	const N = 50
	var wg sync.WaitGroup
	start := make(chan struct{})
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			<-start
			_, _ = a.Validate(ctx, "react", reactHitVersion, false)
		}()
	}
	close(start)
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (singleflight)", got)
	}
}
