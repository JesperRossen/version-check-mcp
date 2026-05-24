package crate_test

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
	"github.com/JesperRossen/version-check-mcp/internal/registry/crate"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

var _ registry.Registry = (*crate.Adapter)(nil)

func fixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	return filepath.Join(moduleRoot, "testdata", "fixtures", "crate")
}

func newAdapter(t *testing.T, callCount *atomic.Int64) *crate.Adapter {
	t.Helper()
	dir := fixtureDir(t)
	urlToFile := func(reqURL string) string {
		switch reqURL {
		case "https://crates.io/api/v1/crates/serde":
			return "serde.json"
		case "https://crates.io/api/v1/crates/nonexistent":
			return "nonexistent.json"
		case "https://crates.io/api/v1/crates/yanked-crate":
			return "yanked-crate.json"
		default:
			t.Fatalf("unexpected fixture URL: %s", reqURL)
			return ""
		}
	}
	client := testfixtures.Client(t, dir, urlToFile, callCount)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(c.Close)
	return crate.New(client, c)
}

func intPtr(i int) *int { return &i }

func TestValidate_Exists(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), "serde", "1.0.228", false)
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

func TestValidate_Yanked(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), "yanked-crate", "1.0.0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Exists {
		t.Fatal("expected Exists=true")
	}
	if res.Source != "crate-yanked" {
		t.Fatalf("Source=%q, want %q", res.Source, "crate-yanked")
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
	_, err := a.Validate(context.Background(), "serde", "99.0.0", false)
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
	res, err := a.Latest(context.Background(), "serde", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "1.0.228" {
		t.Fatalf("Version=%q, want %q", res.Version, "1.0.228")
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source=%q, want %q", res.Source, "computed-highest")
	}
}

func TestLatest_IncludePrerelease(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "serde", true, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "2.0.0-beta.1" {
		t.Fatalf("Version=%q, want %q", res.Version, "2.0.0-beta.1")
	}
}

func TestLatest_WithMajor(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "serde", false, intPtr(1), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "1.0.228" {
		t.Fatalf("Version=%q, want %q", res.Version, "1.0.228")
	}
}

func TestVersions(t *testing.T) {
	a := newAdapter(t, nil)
	versions, err := a.Versions(context.Background(), "serde", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]bool{
		"2.0.0-beta.1": true,
		"1.0.228":      true,
		"1.0.227":      true,
		"1.0.0":        true,
	}
	if len(versions) != len(want) {
		t.Fatalf("len(versions)=%d, want %d (%v)", len(versions), len(want), versions)
	}
	for _, v := range versions {
		if !want[v] {
			t.Fatalf("unexpected version %q in %v", v, versions)
		}
	}
}

func TestCache_HitOnSecondCall(t *testing.T) {
	var calls atomic.Int64
	a := newAdapter(t, &calls)
	ctx := context.Background()
	if _, err := a.Validate(ctx, "serde", "1.0.228", false); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := a.Validate(ctx, "serde", "1.0.228", false); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls=%d, want 1", got)
	}
}
