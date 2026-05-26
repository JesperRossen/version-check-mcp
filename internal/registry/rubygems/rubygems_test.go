package rubygems_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/rubygems"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

var _ registry.Registry = (*rubygems.Adapter)(nil)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(moduleRoot, "testdata", "fixtures", "rubygems")
}

func newAdapter(t *testing.T, callCount *atomic.Int64) *rubygems.Adapter {
	t.Helper()
	dir := fixtureDir(t)
	urlToFile := func(reqURL string) string {
		switch reqURL {
		case "https://rubygems.org/api/v1/versions/rails.json":
			return "rails.json"
		case "https://rubygems.org/api/v1/versions/nonexistent.json":
			return "nonexistent.json"
		case "https://rubygems.org/api/v1/versions/prerelease-gem.json":
			return "prerelease-gem.json"
		default:
			t.Fatalf("unexpected fixture URL: %s", reqURL)
			return ""
		}
	}
	client := testfixtures.Client(t, dir, urlToFile, callCount)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(c.Close)
	return rubygems.New(client, c)
}

func intPtr(i int) *int { return &i }

func TestValidate_Exists(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), "rails", "8.1.3", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Exists {
		t.Fatal("expected Exists=true")
	}
	if res.Source != "versions-list" {
		t.Fatalf("Source=%q, want %q", res.Source, "versions-list")
	}
}

func TestValidate_NotFoundPackage(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), "nonexistent", "1.0.0", false)
	if err == nil {
		t.Fatal("expected error")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.E, got %T: %v", err, err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind=%q, want %q", e.Kind, errs.KindNotFound)
	}
}

func TestValidate_NotFoundVersion(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), "rails", "99.0.0", false)
	if err == nil {
		t.Fatal("expected error")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.E, got %T: %v", err, err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind=%q, want %q", e.Kind, errs.KindNotFound)
	}
}

func TestLatest_Stable(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "rails", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "8.1.3" {
		t.Fatalf("Version=%q, want %q", res.Version, "8.1.3")
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source=%q, want %q", res.Source, "computed-highest")
	}
}

func TestLatest_IncludePrerelease(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "prerelease-gem", true, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "2.0.0-beta.1" {
		t.Fatalf("Version=%q, want %q", res.Version, "2.0.0-beta.1")
	}
}

func TestLatest_WithMajor(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "rails", false, intPtr(7), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "7.2.3" {
		t.Fatalf("Version=%q, want %q", res.Version, "7.2.3")
	}
}

func TestVersions_ExcludesPrerelease(t *testing.T) {
	a := newAdapter(t, nil)
	versions, err := a.Versions(context.Background(), "rails", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, v := range versions {
		if v == "8.2.0-beta.1" {
			t.Fatalf("unexpected prerelease in stable-only list: %v", versions)
		}
	}
}

func TestVersions_Deduplicated(t *testing.T) {
	a := newAdapter(t, nil)
	versions, err := a.Versions(context.Background(), "rails", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	count := 0
	for _, v := range versions {
		if v == "8.0.0" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("8.0.0 appeared %d times, want 1 (%v)", count, versions)
	}
}

func TestCache_HitOnSecondCall(t *testing.T) {
	var calls atomic.Int64
	a := newAdapter(t, &calls)
	ctx := context.Background()
	if _, err := a.Validate(ctx, "rails", "8.1.3", false); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := a.Validate(ctx, "rails", "8.1.3", false); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls=%d, want 1", got)
	}
}
