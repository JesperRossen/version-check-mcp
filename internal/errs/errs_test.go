// RED test (Wave 0). Production code lands in Wave 1 (plan 01-02).
// See .planning/phases/01-foundation-mcp-scaffolding/01-VALIDATION.md.
package errs

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

func TestKindsHaveCorrectStringValues(t *testing.T) {
	cases := []struct {
		k    Kind
		want string
	}{
		{KindRateLimited, "rate_limited"},
		{KindNotFound, "not_found"},
		{KindUpstreamDown, "upstream_down"},
		{KindInvalidInput, "invalid_input"},
	}
	for _, c := range cases {
		if string(c.k) != c.want {
			t.Errorf("Kind = %q, want %q", string(c.k), c.want)
		}
	}
}

func TestConstructorsSetKind(t *testing.T) {
	if got := InvalidInput("bad").Kind; got != KindInvalidInput {
		t.Errorf("InvalidInput Kind = %q, want %q", got, KindInvalidInput)
	}
	if got := NotFound("missing").Kind; got != KindNotFound {
		t.Errorf("NotFound Kind = %q, want %q", got, KindNotFound)
	}
	if got := RateLimited(time.Now().Add(time.Minute)).Kind; got != KindRateLimited {
		t.Errorf("RateLimited Kind = %q, want %q", got, KindRateLimited)
	}
	if got := UpstreamDown(io.EOF).Kind; got != KindUpstreamDown {
		t.Errorf("UpstreamDown Kind = %q, want %q", got, KindUpstreamDown)
	}
}

func TestErrorsAsRecoversE(t *testing.T) {
	original := NotFound("pkg foo not found")
	wrapped := fmt.Errorf("registry call failed: %w", original)
	var target *E
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As failed to recover *E from wrapped error: %v", wrapped)
	}
	if target.Kind != KindNotFound {
		t.Errorf("recovered Kind = %q, want %q", target.Kind, KindNotFound)
	}
}

func TestUnwrapReturnsWrapped(t *testing.T) {
	e := UpstreamDown(io.EOF)
	if got := errors.Unwrap(e); got != io.EOF {
		t.Errorf("Unwrap = %v, want io.EOF", got)
	}
}

func TestRateLimitedDetailsCarryResetTime(t *testing.T) {
	reset := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	e := RateLimited(reset)
	if e.Details == nil {
		t.Fatal("RateLimited Details is nil, want populated")
	}
	got, ok := e.Details["reset_at"]
	if !ok {
		t.Fatal("Details missing reset_at key")
	}
	// Accept either time.Time or its formatted/marshaled form.
	switch v := got.(type) {
	case time.Time:
		if !v.Equal(reset) {
			t.Errorf("reset_at time = %v, want %v", v, reset)
		}
	case string:
		if v == "" {
			t.Error("reset_at string is empty")
		}
	default:
		t.Errorf("reset_at unexpected type %T: %v", v, v)
	}
}
