package gomod_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/cache"
	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
	"github.com/JesperRossen/version-check-mcp/internal/registry/gomod"
	"github.com/JesperRossen/version-check-mcp/internal/testfixtures"
)

// Compile-time assertion: *gomod.Adapter satisfies the registry.Registry seam.
var _ registry.Registry = (*gomod.Adapter)(nil)

const (
	gomodFixturesPath = "../../../testdata/fixtures/gomod"

	awsModule   = "github.com/aws/aws-sdk-go"
	azureModule = "github.com/Azure/azure-sdk-for-go"
	noModule    = "github.com/nonexistent/nonexistent"

	// URLs used in the fixture map (escaped form for Azure)
	awsListURL      = "https://proxy.golang.org/github.com/aws/aws-sdk-go/@v/list"
	awsLatestURL    = "https://proxy.golang.org/github.com/aws/aws-sdk-go/@latest"
	azureListURL    = "https://proxy.golang.org/github.com/!azure/azure-sdk-for-go/@v/list"
	azureLatestURL  = "https://proxy.golang.org/github.com/!azure/azure-sdk-for-go/@latest"
	noListURL       = "https://proxy.golang.org/github.com/nonexistent/nonexistent/@v/list"
	noLatestURL     = "https://proxy.golang.org/github.com/nonexistent/nonexistent/@latest"
)

func intPtr(i int) *int { return &i }

// newAdapter builds an Adapter wired to a standard fixture-replay client.
// callCount tracks upstream RoundTrip invocations.
func newAdapter(t *testing.T, callCount *atomic.Int64) *gomod.Adapter {
	t.Helper()
	mapURL := func(u string) string {
		switch u {
		case awsListURL:
			return "aws-sdk-go.list"
		case awsLatestURL:
			return "aws-sdk-go-latest.json"
		case azureListURL:
			return "azure-sdk.list"
		case azureLatestURL:
			return "azure-sdk-latest.json"
		case noListURL:
			return "nonexistent.list"
		case noLatestURL:
			return "nonexistent.list" // 404 handled by headers file on list
		default:
			t.Fatalf("unmapped fixture URL: %q", u)
			return ""
		}
	}
	client := testfixtures.FixtureClient(t, gomodFixturesPath, mapURL, callCount)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(func() { c.Close() })
	return gomod.New(client, c)
}

// newAdapterWithPseudoLatest builds an adapter where @latest for aws-sdk-go
// returns a pseudo-version — used for TestLatest_PseudoFallback.
func newAdapterWithPseudoLatest(t *testing.T) *gomod.Adapter {
	t.Helper()
	mapURL := func(u string) string {
		switch u {
		case awsListURL:
			return "aws-sdk-go.list"
		case awsLatestURL:
			return "aws-sdk-go-latest-pseudo.json"
		default:
			t.Fatalf("unmapped fixture URL: %q", u)
			return ""
		}
	}
	client := testfixtures.FixtureClient(t, gomodFixturesPath, mapURL, nil)
	c := cache.NewCache(64, 5*time.Minute)
	t.Cleanup(func() { c.Close() })
	return gomod.New(client, c)
}

// TestValidate_Hit checks that a known version in the list returns Exists=true.
func TestValidate_Hit(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), awsModule, "v1.50.0", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false, want true")
	}
	if res.Source != "versions-list" {
		t.Fatalf("Source = %q, want \"versions-list\"", res.Source)
	}
}

// TestValidate_Incompatible checks that a version with +incompatible suffix
// is matched verbatim and the suffix is preserved in the source confirmation.
func TestValidate_Incompatible(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), awsModule, "v1.55.7+incompatible", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false for v1.55.7+incompatible, want true")
	}
	if res.Source != "versions-list" {
		t.Fatalf("Source = %q, want \"versions-list\"", res.Source)
	}
}

// TestValidate_PseudoExplicit verifies D-GOMOD-03: an explicit pseudo-version
// present in the list returns Exists=true even when incPre=false.
// (validate is an existence check, not a filter).
func TestValidate_PseudoExplicit(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Validate(context.Background(), awsModule, "v0.0.0-20230101120000-abc123def456", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !res.Exists {
		t.Fatalf("Exists = false for explicit pseudo-version with incPre=false, want true (D-GOMOD-03)")
	}
}

// TestValidate_MissVersion checks that a version absent from the list returns
// KindNotFound.
func TestValidate_MissVersion(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), awsModule, "v99.0.0", false)
	if err == nil {
		t.Fatal("Validate err = nil, want KindNotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// TestValidate_MissModule404 checks that a 404 response for @v/list returns
// KindNotFound (via httperr.MapHTTPStatus).
func TestValidate_MissModule404(t *testing.T) {
	a := newAdapter(t, nil)
	_, err := a.Validate(context.Background(), noModule, "v1.0.0", false)
	if err == nil {
		t.Fatal("Validate err = nil, want KindNotFound")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("err type = %T, want *errs.E", err)
	}
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
}

// TestLatest_ProxyLatest checks that Latest returns Source="proxy-latest" when
// @latest returns a non-pseudo, non-prerelease version.
func TestLatest_ProxyLatest(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), awsModule, false, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "proxy-latest" {
		t.Fatalf("Source = %q, want \"proxy-latest\"", res.Source)
	}
	if res.Version == "" {
		t.Fatal("Version is empty")
	}
}

// TestLatest_PseudoFallback checks that when @latest returns a pseudo-version
// and incPre=false, Latest falls back to filter.FilterAndPickHighest and
// returns Source="computed-highest".
func TestLatest_PseudoFallback(t *testing.T) {
	a := newAdapterWithPseudoLatest(t)
	res, err := a.Latest(context.Background(), awsModule, false, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want \"computed-highest\" (pseudo @latest should trigger fallback)", res.Source)
	}
	if res.Version == "" {
		t.Fatal("Version is empty")
	}
}

// TestLatest_IncPre checks that incPre=true trusts @latest verbatim even
// when it is a pseudo-version (Source="proxy-latest").
func TestLatest_IncPre(t *testing.T) {
	a := newAdapterWithPseudoLatest(t)
	res, err := a.Latest(context.Background(), awsModule, true, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if res.Source != "proxy-latest" {
		t.Fatalf("Source = %q, want \"proxy-latest\" (incPre=true trusts @latest)", res.Source)
	}
}

// TestLatest_FilterMajor checks that a major filter bypasses @latest and uses
// computed-highest.
func TestLatest_FilterMajor(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), awsModule, false, intPtr(1), nil)
	if err != nil {
		t.Fatalf("Latest(major=1) err = %v", err)
	}
	if res.Source != "computed-highest" {
		t.Fatalf("Source = %q, want \"computed-highest\"", res.Source)
	}
	if res.Version == "" {
		t.Fatal("Version is empty")
	}
}

// TestLatest_EscapedPath verifies that the Azure module request hits the
// !azure-escaped URL — proven by the fixture map routing.
func TestLatest_EscapedPath(t *testing.T) {
	a := newAdapter(t, nil)
	res, err := a.Latest(context.Background(), azureModule, false, nil, nil)
	if err != nil {
		t.Fatalf("Latest(azure) err = %v", err)
	}
	if res.Version == "" {
		t.Fatal("Version is empty")
	}
	// If the URL was not !-escaped, the fixture client would fatal with
	// "unmapped fixture URL" — the fact we reach here proves routing works.
}

// TestCache_HitOnSecondCall verifies that two identical Validate calls trigger
// exactly one upstream @v/list fetch (cache hit on second call).
func TestCache_HitOnSecondCall(t *testing.T) {
	var calls atomic.Int64
	a := newAdapter(t, &calls)
	ctx := context.Background()

	if _, err := a.Validate(ctx, awsModule, "v1.50.0", false); err != nil {
		t.Fatalf("first Validate err = %v", err)
	}
	if _, err := a.Validate(ctx, awsModule, "v1.50.0", false); err != nil {
		t.Fatalf("second Validate err = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1 (cache hit on second call)", got)
	}
}
