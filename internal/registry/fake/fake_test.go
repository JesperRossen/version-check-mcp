// RED test (Wave 0). Production code lands in Wave 2 (plan 01-03).
// See .planning/phases/01-foundation-mcp-scaffolding/01-VALIDATION.md.
package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

func TestFakeReturnsConfiguredValidateResult(t *testing.T) {
	f := New("npm")
	f.ValidateResult = registry.ValidateResult{Exists: true, Source: "npm"}
	got, err := f.Validate(context.Background(), "react", "18.2.0", false)
	if err != nil {
		t.Fatalf("Validate err = %v", err)
	}
	if !got.Exists || got.Source != "npm" {
		t.Errorf("Validate = %+v, want {Exists:true Source:npm}", got)
	}
}

func TestFakeReturnsConfiguredLatestResult(t *testing.T) {
	f := New("npm")
	f.LatestResult = registry.LatestResult{Version: "18.2.0", Source: "npm"}
	got, err := f.Latest(context.Background(), "react", false, nil, nil)
	if err != nil {
		t.Fatalf("Latest err = %v", err)
	}
	if got.Version != "18.2.0" || got.Source != "npm" {
		t.Errorf("Latest = %+v, want {Version:18.2.0 Source:npm}", got)
	}
}

func TestFakeReturnsConfiguredError(t *testing.T) {
	f := New("npm")
	f.ValidateErr = errs.NotFound("nope")
	_, err := f.Validate(context.Background(), "no-such-pkg", "1.0.0", false)
	if err == nil {
		t.Fatal("Validate returned nil err, want *errs.E")
	}
	var e *errs.E
	if !errors.As(err, &e) || e.Kind != errs.KindNotFound {
		t.Errorf("err = %v, want *errs.E with KindNotFound", err)
	}
}

func TestFakePanicHookFires(t *testing.T) {
	f := New("npm")
	f.PanicOn = "validate"
	defer func() {
		if r := recover(); r == nil {
			t.Error("Validate did not panic; expected panic via PanicOn hook")
		}
	}()
	_, _ = f.Validate(context.Background(), "react", "18.2.0", false)
}

func TestFakeNameMatchesConstructor(t *testing.T) {
	if got := New("npm").Name(); got != "npm" {
		t.Errorf("Name() = %q, want %q", got, "npm")
	}
}
