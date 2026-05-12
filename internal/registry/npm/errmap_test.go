package npm

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
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

func TestMapHTTPStatus_404(t *testing.T) {
	e := asErrsE(t, mapHTTPStatus(mkResp(404, nil), "react"))
	if e.Kind != errs.KindNotFound {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindNotFound)
	}
	if got := e.Details["pkg"]; got != "react" {
		t.Fatalf("Details[pkg] = %v, want %q", got, "react")
	}
}

func TestMapHTTPStatus_429_WithRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "60")
	e := asErrsE(t, mapHTTPStatus(mkResp(429, h), "react"))
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
}

func TestMapHTTPStatus_429_NoRetryAfter(t *testing.T) {
	e := asErrsE(t, mapHTTPStatus(mkResp(429, nil), "react"))
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
	if reset.IsZero() {
		t.Fatalf("reset_at is zero — violates Pitfall #7")
	}
	if !reset.After(time.Now()) {
		t.Fatalf("reset_at = %v, want > now", reset)
	}
}

func TestMapHTTPStatus_5xx(t *testing.T) {
	for _, code := range []int{500, 502, 503} {
		e := asErrsE(t, mapHTTPStatus(mkResp(code, nil), "react"))
		if e.Kind != errs.KindUpstreamDown {
			t.Fatalf("%d: Kind = %q, want %q", code, e.Kind, errs.KindUpstreamDown)
		}
		if got := e.Details["status"]; got != code {
			t.Fatalf("%d: Details[status] = %v, want %d", code, got, code)
		}
	}
}

func TestMapHTTPStatus_UnexpectedStatus(t *testing.T) {
	e := asErrsE(t, mapHTTPStatus(mkResp(418, nil), "react"))
	if e.Kind != errs.KindUpstreamDown {
		t.Fatalf("Kind = %q, want %q", e.Kind, errs.KindUpstreamDown)
	}
	if got := e.Details["unexpected_status"]; got != true {
		t.Fatalf("Details[unexpected_status] = %v, want true", got)
	}
}

func TestParseRetryAfter_HTTPDate(t *testing.T) {
	const httpDate = "Wed, 21 Oct 2026 07:28:00 GMT"
	want, err := http.ParseTime(httpDate)
	if err != nil {
		t.Fatalf("reference parse failed: %v", err)
	}
	got := parseRetryAfter(httpDate)
	if !got.Equal(want) {
		t.Fatalf("parseRetryAfter(%q) = %v, want %v", httpDate, got, want)
	}
}

func TestParseRetryAfter_EmptyOrInvalid(t *testing.T) {
	for _, in := range []string{"", "not-a-date"} {
		got := parseRetryAfter(in)
		if got.IsZero() {
			t.Fatalf("parseRetryAfter(%q) returned zero time", in)
		}
		if !got.After(time.Now()) {
			t.Fatalf("parseRetryAfter(%q) = %v, want > now", in, got)
		}
	}
}
