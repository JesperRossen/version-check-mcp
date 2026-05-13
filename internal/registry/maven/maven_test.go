package maven_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/maven"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

// Compile-time assertion: maven.Adapter must implement registry.Registry.
var _ registry.Registry = (*maven.Adapter)(nil)

// fixtureDir returns the absolute path to testdata/fixtures/maven relative to
// this test file. This is invariant to go test working-directory changes.
func fixtureDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile is .../internal/registry/maven/maven_test.go
	// We need to go up 4 levels to the module root, then into testdata/fixtures/maven.
	moduleRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	return filepath.Join(moduleRoot, "testdata", "fixtures", "maven")
}

// newAdapter builds an Adapter using a fixture-replay HTTP client. The
// urlToFile function maps request URLs to relative fixture file names.
func newAdapter(t *testing.T, callCount *atomic.Int64) *maven.Adapter {
	t.Helper()
	dir := fixtureDir(t)
	urlToFile := func(reqURL string) string {
		switch reqURL {
		case maven.MetadataURL("org.springframework", "spring-core"):
			return "spring-core-metadata.xml"
		case maven.MetadataURL("com.example", "snapshot-lib"):
			return "snapshot-metadata.xml"
		case maven.MetadataURL("com.example", "nonexistent"):
			return "nonexistent.xml"
		default:
			t.Fatalf("unexpected fixture URL: %s", reqURL)
			return ""
		}
	}
	client := testfixtures.Client(t, dir, urlToFile, callCount)
	c := cache.NewCache(64, 0)
	t.Cleanup(c.Close)
	return maven.New(client, c)
}

// TestValidate_Hit verifies that a version present in <versions> returns
// Exists:true, Source:"versions-list".
func TestValidate_Hit(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), "org.springframework:spring-core", "6.1.0", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Exists {
		t.Error("expected Exists=true")
	}
	if res.Source != "versions-list" {
		t.Errorf("expected Source=%q, got %q", "versions-list", res.Source)
	}
}

// TestValidate_HitSnapshotExplicit verifies that validate is a membership check:
// querying an explicit SNAPSHOT version that is present in <versions> returns
// Exists:true regardless of incPre flag (validate is not a stability filter).
func TestValidate_HitSnapshotExplicit(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), "org.springframework:spring-core", "7.0.7-SNAPSHOT", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Exists {
		t.Error("expected Exists=true for explicit SNAPSHOT membership check")
	}
}

// TestValidate_MissVersion verifies that a version absent from <versions>
// returns a KindNotFound error.
func TestValidate_MissVersion(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), "org.springframework:spring-core", "99.0.0", false)
	if err == nil {
		t.Fatal("expected error for missing version")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.E, got %T: %v", err, err)
	}
	if e.Kind != errs.KindNotFound {
		t.Errorf("expected Kind=KindNotFound, got %q", e.Kind)
	}
}

// TestValidate_NotFound404 verifies that a 404 response for the metadata XML
// maps to KindNotFound.
func TestValidate_NotFound404(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), "com.example:nonexistent", "1.0.0", false)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.E, got %T: %v", err, err)
	}
	if e.Kind != errs.KindNotFound {
		t.Errorf("expected Kind=KindNotFound for 404, got %q", e.Kind)
	}
}

// TestValidate_InvalidPkg verifies that inputs without exactly one colon return
// KindInvalidInput (D-MAVEN-05).
func TestValidate_InvalidPkg(t *testing.T) {
	a := newAdapter(t, nil)

	for _, bad := range []string{"no-colon", "a:b:c", "", ":"} {
		_, err := a.Validate(context.Background(), bad, "1.0.0", false)
		if err == nil {
			t.Errorf("input %q: expected error, got nil", bad)
			continue
		}
		var e *errs.E
		if !errors.As(err, &e) {
			t.Errorf("input %q: expected *errs.E, got %T: %v", bad, err, err)
			continue
		}
		if e.Kind != errs.KindInvalidInput {
			t.Errorf("input %q: expected Kind=KindInvalidInput, got %q", bad, e.Kind)
		}
	}
}

// TestLatest_ReleasePointer verifies that Latest with incPre=false and no
// major/minor filter trusts the <release> element (D-MAVEN-03).
func TestLatest_ReleasePointer(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "org.springframework:spring-core", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version != "7.0.7" {
		t.Errorf("expected Version=%q, got %q", "7.0.7", res.Version)
	}
	if res.Source != "registry-release-pointer" {
		t.Errorf("expected Source=%q, got %q", "registry-release-pointer", res.Source)
	}
}

// TestLatest_DoesNotTrustLatestElement verifies that the adapter ignores the
// <latest> XML element (which points to a SNAPSHOT) and returns the <release>
// value instead (D-MAVEN-03 pitfall).
func TestLatest_DoesNotTrustLatestElement(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "org.springframework:spring-core", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The fixture's <latest> is 7.0.7-SNAPSHOT; the adapter must return 7.0.7.
	if res.Version == "7.0.7-SNAPSHOT" {
		t.Error("adapter must NOT trust <latest>; returned the SNAPSHOT value")
	}
	if res.Version != "7.0.7" {
		t.Errorf("expected Version=%q (the <release> value), got %q", "7.0.7", res.Version)
	}
}

// TestLatest_SnapshotFiltered verifies that when the stable fallback path
// (computed-highest) is used (here triggered by a major filter that bypasses
// the <release> fast path), SNAPSHOT versions are not returned.
func TestLatest_SnapshotFiltered(t *testing.T) {
	a := newAdapter(t, nil)
	major := 7
	res, err := a.Latest(context.Background(), "org.springframework:spring-core", false, &major, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Version) > 0 && res.Version[len(res.Version)-len("-SNAPSHOT"):] == "-SNAPSHOT" {
		t.Errorf("stable result must not end with -SNAPSHOT, got %q", res.Version)
	}
	if res.Source != "computed-highest" {
		t.Errorf("expected Source=%q when filter is active, got %q", "computed-highest", res.Source)
	}
}

// TestLatest_IncPreAdmitsSnapshot verifies that incPre=true bypasses <release>
// and may return a SNAPSHOT as the computed highest.
func TestLatest_IncPreAdmitsSnapshot(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "org.springframework:spring-core", true, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With incPre=true the SNAPSHOT is in play. It should be the highest.
	if res.Version != "7.0.7-SNAPSHOT" {
		t.Errorf("expected highest with incPre=true to be %q, got %q", "7.0.7-SNAPSHOT", res.Version)
	}
	if res.Source != "computed-highest" {
		t.Errorf("expected Source=%q, got %q", "computed-highest", res.Source)
	}
}

// TestLatest_FilterMajor verifies that a major constraint filters versions
// correctly in the computed-highest path.
func TestLatest_FilterMajor(t *testing.T) {
	a := newAdapter(t, nil)
	major := 6
	res, err := a.Latest(context.Background(), "org.springframework:spring-core", false, &major, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Stable versions with major=6 in the fixture: 6.0.0, 6.1.0. Highest is 6.1.0.
	if res.Version != "6.1.0" {
		t.Errorf("expected Version=%q for major=6 filter, got %q", "6.1.0", res.Version)
	}
}

// TestLatest_SnapshotMetadata_ReleasePointer verifies that on the
// snapshot-metadata.xml fixture (where highest version is a SNAPSHOT),
// incPre=false still returns the <release> pointer (1.5.0), not a SNAPSHOT.
// This is D-MAVEN-04 pitfall 4.
func TestLatest_SnapshotMetadata_ReleasePointer(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), "com.example:snapshot-lib", false, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Version == "2.0.0-SNAPSHOT" {
		t.Error("stable result must not be a SNAPSHOT; <release> pointer should be used")
	}
	if res.Version != "1.5.0" {
		t.Errorf("expected Version=%q (the <release> value), got %q", "1.5.0", res.Version)
	}
	if res.Source != "registry-release-pointer" {
		t.Errorf("expected Source=%q, got %q", "registry-release-pointer", res.Source)
	}
}

// TestCache_HitOnSecondCall verifies that two consecutive identical Validate
// calls trigger exactly 1 upstream HTTP request (cache hit on second call).
func TestCache_HitOnSecondCall(t *testing.T) {
	var count atomic.Int64
	a := newAdapter(t, &count)

	ctx := context.Background()
	_, err := a.Validate(ctx, "org.springframework:spring-core", "6.1.0", false)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	_, err = a.Validate(ctx, "org.springframework:spring-core", "6.1.0", false)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("expected 1 upstream call, got %d", count.Load())
	}
}
