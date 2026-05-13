package pypi_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/pypi"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

// Compile-time assertion that *pypi.Adapter satisfies the registry.Registry
// seam locked in D-05.
var _ registry.Registry = (*pypi.Adapter)(nil)

const (
	pypiRequestsURL    = "https://pypi.org/pypi/requests/json"
	pypiNonexistentURL = "https://pypi.org/pypi/nonexistent/json"
	pypiFixturesPath   = "../../../testdata/fixtures/pypi"
)

func intPtr(i int) *int { return &i }

// newAdapter builds a PyPI Adapter wired to a fixture-replay client. callCount
// reflects upstream-fetch invocations (one increment per RoundTrip).
func newAdapter(t *testing.T) (*pypi.Adapter, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	mapURL := func(u string) string {
		switch u {
		case pypiRequestsURL:
			return "requests.json"
		case pypiNonexistentURL:
			return "nonexistent.json"
		default:
			t.Fatalf("unmapped fixture URL: %q", u)
			return ""
		}
	}
	client := testfixtures.FixtureClient(t, pypiFixturesPath, mapURL, &calls)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(func() { c.Close() })
	return pypi.New(client, c), &calls
}

// TestValidate_Hit: exact match on a stable, non-yanked release.
func TestValidate_Hit(t *testing.T) {
	a, calls := newAdapter(t)
	res, err := a.Validate(context.Background(), "requests", "2.32.5", false)
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

// TestValidate_PEP440Match: input "2.31.0-rc1" must match canonical release key
// "2.31.0rc1" after PEP 440 normalization of both sides.
func TestValidate_PEP440Match(t *testing.T) {
	a, _ := newAdapter(t)
	res, err := a.Validate(context.Background(), "requests", "2.31.0-rc1", false)
	if err != nil {
		t.Fatalf("Validate(2.31.0-rc1) err = %v, want nil (PEP 440 match)", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false, want true (PEP 440 normalization must match 2.31.0rc1)")
	}
}

// TestValidate_Yanked: a yanked release returns Exists=true with Source="pypi-yanked".
func TestValidate_Yanked(t *testing.T) {
	a, _ := newAdapter(t)
	res, err := a.Validate(context.Background(), "requests", "2.32.1", false)
	if err != nil {
		t.Fatalf("Validate(yanked) err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false for yanked release, want true (D-PYPI-01)")
	}
	if res.Source != "pypi-yanked" {
		t.Fatalf("Source = %q, want %q", res.Source, "pypi-yanked")
	}
}

// TestValidate_MissVersion: version not in releases map → NotFound.
func TestValidate_MissVersion(t *testing.T) {
	a, _ := newAdapter(t)
	_, err := a.Validate(context.Background(), "requests", "99.0.0", false)
	if err == nil {
		t.Fatalf("Validate(99.0.0) err = nil, want NotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err is %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// TestValidate_MissPackage404: non-existent package (404) → NotFound.
func TestValidate_MissPackage404(t *testing.T) {
	a, _ := newAdapter(t)
	_, err := a.Validate(context.Background(), "nonexistent", "1.0.0", false)
	if err == nil {
		t.Fatalf("Validate(nonexistent) err = nil, want NotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err is %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// TestLatest_InfoVersion: fast path when no filter → info.version returned.
func TestLatest_InfoVersion(t *testing.T) {
	a, _ := newAdapter(t)
	res, err := a.Latest(context.Background(), "requests", false, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "pypi-info-version" {
		t.Fatalf("Source = %q, want %q", res.Source, "pypi-info-version")
	}
	if res.Version != "2.32.5" {
		t.Fatalf("Version = %q, want %q", res.Version, "2.32.5")
	}
}

// TestLatest_FilterMajor: major=2, minor=nil → highest 2.x via FilterAndPickHighest.
func TestLatest_FilterMajor(t *testing.T) {
	a, _ := newAdapter(t)
	res, err := a.Latest(context.Background(), "requests", false, intPtr(2), nil)
	if err != nil {
		t.Fatalf("Latest(major=2) err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want %q", res.Source, "computed-highest")
	}
	// 2.32.5 is the highest non-prerelease 2.x in the fixture.
	if res.Version != "2.32.5" {
		t.Fatalf("Version = %q, want %q", res.Version, "2.32.5")
	}
}

// TestLatest_IncPre: incPre=true skips the info.version fast path.
func TestLatest_IncPre(t *testing.T) {
	a, _ := newAdapter(t)
	res, err := a.Latest(context.Background(), "requests", true, nil, nil)
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

// TestCache_HitOnSecondCall: two consecutive identical Validate calls trigger
// exactly 1 upstream fetch (cache hit on second).
func TestCache_HitOnSecondCall(t *testing.T) {
	a, calls := newAdapter(t)
	ctx := context.Background()
	if _, err := a.Validate(ctx, "requests", "2.32.5", false); err != nil {
		t.Fatalf("first Validate err = %v", err)
	}
	if _, err := a.Validate(ctx, "requests", "2.32.5", false); err != nil {
		t.Fatalf("second Validate err = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (cache hit)", got)
	}
}
