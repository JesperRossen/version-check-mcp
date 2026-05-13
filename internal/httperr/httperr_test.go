package httperr_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/httperr"
)

func mkResp(status int, h http.Header) *http.Response {
	if h == nil {
		h = http.Header{}
	}
	return &http.Response{StatusCode: status, Header: h}
}

func asErrsE(t *testing.T, err error) *errs.E {
	t.Helper()
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected *errs.E, got %T: %v", err, err)
	}
	return e
}

// TestMapHTTPStatus_404_NPM verifies 404 → NotFound, pkg=react, registry=npm in details.
func TestMapHTTPStatus_404_NPM(t *testing.T) {
	e := asErrsE(t, httperr.MapHTTPStatus(mkResp(404, nil), "react", "npm"))
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
	if got := e.Details["pkg"]; got != "react" {
		t.Fatalf("Details[pkg] = %v, want %q", got, "react")
	}
	if got := e.Details["registry"]; got != "npm" {
		t.Fatalf("Details[registry] = %v, want %q", got, "npm")
	}
}

// TestMapHTTPStatus_404_PyPI verifies the not-found message references pypi, not npm.
func TestMapHTTPStatus_404_PyPI(t *testing.T) {
	e := asErrsE(t, httperr.MapHTTPStatus(mkResp(404, nil), "requests", "pypi"))
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
	if got := e.Details["registry"]; got != "pypi" {
		t.Fatalf("Details[registry] = %v, want %q", got, "pypi")
	}
	// Message must mention the registry name.
	if !strings.Contains(e.Message, "pypi") {
		t.Fatalf("Message %q does not contain registry name %q", e.Message, "pypi")
	}
}

// TestMapHTTPStatus_429_NumericRetryAfter verifies 429 with delta-seconds header.
func TestMapHTTPStatus_429_NumericRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "60")
	e := asErrsE(t, httperr.MapHTTPStatus(mkResp(429, h), "react", "npm"))
	if e.Kind != errs.KindRateLimited {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindRateLimited)
	}
	resetAny, ok := e.Details["reset_at"]
	if !ok {
		t.Fatalf("Details[reset_at] missing")
	}
	reset, ok := resetAny.(time.Time)
	if !ok {
		t.Fatalf("Details[reset_at] is %T, want time.Time", resetAny)
	}
	want := time.Now().Add(60 * time.Second)
	delta := reset.Sub(want)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Fatalf("reset_at = %v, want ~%v (±5s)", reset, want)
	}
	if got := e.Details["registry"]; got != "npm" {
		t.Fatalf("Details[registry] = %v, want %q", got, "npm")
	}
}

// TestMapHTTPStatus_429_HTTPDateRetryAfter verifies 429 with HTTP-date header.
func TestMapHTTPStatus_429_HTTPDateRetryAfter(t *testing.T) {
	const httpDate = "Wed, 21 Oct 2099 07:28:00 GMT"
	want, err := http.ParseTime(httpDate)
	if err != nil {
		t.Fatalf("reference parse failed: %v", err)
	}
	h := http.Header{}
	h.Set("Retry-After", httpDate)
	e := asErrsE(t, httperr.MapHTTPStatus(mkResp(429, h), "react", "npm"))
	if e.Kind != errs.KindRateLimited {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindRateLimited)
	}
	resetAny, ok := e.Details["reset_at"]
	if !ok {
		t.Fatalf("Details[reset_at] missing")
	}
	reset, ok := resetAny.(time.Time)
	if !ok {
		t.Fatalf("Details[reset_at] is %T, want time.Time", resetAny)
	}
	if !reset.Equal(want) {
		t.Fatalf("reset_at = %v, want %v", reset, want)
	}
}

// TestMapHTTPStatus_500 verifies 500 → UpstreamDown with status=500 in details.
func TestMapHTTPStatus_500(t *testing.T) {
	e := asErrsE(t, httperr.MapHTTPStatus(mkResp(500, nil), "react", "npm"))
	if e.Kind != errs.KindUpstreamDown {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindUpstreamDown)
	}
	if got := e.Details["status"]; got != 500 {
		t.Fatalf("Details[status] = %v, want 500", got)
	}
	if got := e.Details["registry"]; got != "npm" {
		t.Fatalf("Details[registry] = %v, want %q", got, "npm")
	}
}

// TestMapHTTPStatus_502_503 verifies 502 and 503 → UpstreamDown.
func TestMapHTTPStatus_502_503(t *testing.T) {
	for _, code := range []int{502, 503} {
		e := asErrsE(t, httperr.MapHTTPStatus(mkResp(code, nil), "react", "npm"))
		if e.Kind != errs.KindUpstreamDown {
			t.Fatalf("%d: Kind = %q, want %q", code, e.Kind, errs.KindUpstreamDown)
		}
	}
}

// TestMapHTTPStatus_UnexpectedNon2xx verifies unexpected status (e.g. 418) → UpstreamDown with unexpected_status=true.
func TestMapHTTPStatus_UnexpectedNon2xx(t *testing.T) {
	e := asErrsE(t, httperr.MapHTTPStatus(mkResp(418, nil), "react", "npm"))
	if e.Kind != errs.KindUpstreamDown {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindUpstreamDown)
	}
	if got := e.Details["unexpected_status"]; got != true {
		t.Fatalf("Details[unexpected_status] = %v, want true", got)
	}
}

// TestParseRetryAfter_Empty verifies empty → ~30s fallback window.
func TestParseRetryAfter_Empty(t *testing.T) {
	got := httperr.ParseRetryAfter("")
	if got.IsZero() {
		t.Fatal("ParseRetryAfter(\"\") returned zero time")
	}
	want := time.Now().Add(30 * time.Second)
	delta := got.Sub(want)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Fatalf("ParseRetryAfter(\"\") = %v, want ~%v (±5s)", got, want)
	}
}

// TestParseRetryAfter_Numeric verifies numeric delta-seconds.
func TestParseRetryAfter_Numeric(t *testing.T) {
	got := httperr.ParseRetryAfter("30")
	want := time.Now().Add(30 * time.Second)
	delta := got.Sub(want)
	if delta < -5*time.Second || delta > 5*time.Second {
		t.Fatalf("ParseRetryAfter(\"30\") = %v, want ~%v (±5s)", got, want)
	}
}

// TestParseRetryAfter_HTTPDate verifies HTTP-date parsing.
func TestParseRetryAfter_HTTPDate(t *testing.T) {
	const httpDate = "Wed, 21 Oct 2099 07:28:00 GMT"
	want, err := http.ParseTime(httpDate)
	if err != nil {
		t.Fatalf("reference parse failed: %v", err)
	}
	got := httperr.ParseRetryAfter(httpDate)
	if !got.Equal(want) {
		t.Fatalf("ParseRetryAfter(%q) = %v, want %v", httpDate, got, want)
	}
}
