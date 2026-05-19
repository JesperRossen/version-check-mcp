// Package fake provides a programmable in-memory Registry implementation
// used across Phase-1 tests (cache, handler, recovery). One fake, many
// tests — D-06.
package fake

import (
	"context"
	"sync/atomic"

	"github.com/JesperRossen/version-check-mcp/internal/registry"
)

// Fake is a Registry test double. Configure fields directly before passing
// to the system under test. Fields are intentionally exported for direct
// mutation — there is no setter API.
//
// Concurrency: the call counters are atomic so the SDK in-memory transport
// (which dispatches handlers on its own goroutines) can safely observe
// them. Other fields (ValidateResult, LatestResult, ValidateErr, etc.) are
// expected to be configured before the system under test is started; tests
// should not mutate them mid-flight.
type Fake struct {
	name string

	// ValidateResult / LatestResult are returned on the happy path.
	ValidateResult registry.ValidateResult
	LatestResult   registry.LatestResult

	// ValidateErr / LatestErr, if non-nil, are returned (per-method) AFTER
	// the panic check. Tests assign *errs.E values here.
	ValidateErr error
	LatestErr   error

	// PanicOn controls the panic hook used by the recovery test:
	//   "validate"  → Validate panics
	//   "latest"    → Latest   panics
	//   "versions"  → Versions panics
	//   "any"       → all panic
	//   ""          → disabled
	// Panic value is PanicMessage if non-empty, else "fake panic".
	PanicOn      string
	PanicMessage string

	// VersionsList / VersionsErr configure the Versions method response.
	VersionsList []string
	VersionsErr  error

	// ValidateCalls / LatestCalls / VersionsCalls count invocations. Used by
	// handler tests to assert "Registry was NOT called" when input was
	// range-rejected.
	ValidateCalls atomic.Int64
	LatestCalls   atomic.Int64
	VersionsCalls atomic.Int64
}

// New constructs a Fake with sensible defaults: Source="fake",
// LatestResult.Version="v0.0.0".
func New(name string) *Fake {
	return &Fake{
		name:           name,
		ValidateResult: registry.ValidateResult{Source: "fake"},
		LatestResult:   registry.LatestResult{Version: "v0.0.0", Source: "fake"},
	}
}

func (f *Fake) Name() string { return f.name }

func (f *Fake) Validate(ctx context.Context, pkg, version string, incPre bool) (registry.ValidateResult, error) {
	f.ValidateCalls.Add(1)
	if f.PanicOn == "validate" || f.PanicOn == "any" {
		panic(f.panicValue())
	}
	if f.ValidateErr != nil {
		return registry.ValidateResult{}, f.ValidateErr
	}
	return f.ValidateResult, nil
}

func (f *Fake) Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (registry.LatestResult, error) {
	f.LatestCalls.Add(1)
	if f.PanicOn == "latest" || f.PanicOn == "any" {
		panic(f.panicValue())
	}
	if f.LatestErr != nil {
		return registry.LatestResult{}, f.LatestErr
	}
	return f.LatestResult, nil
}

func (f *Fake) panicValue() any {
	if f.PanicMessage != "" {
		return f.PanicMessage
	}
	return "fake panic"
}

func (f *Fake) Versions(ctx context.Context, pkg string, incPre bool) ([]string, error) {
	f.VersionsCalls.Add(1)
	if f.PanicOn == "versions" || f.PanicOn == "any" {
		panic(f.panicValue())
	}
	if f.VersionsErr != nil {
		return nil, f.VersionsErr
	}
	return f.VersionsList, nil
}

// Compile-time interface conformance check (D-06). The build fails if Fake
// drifts from the D-05 Registry contract.
var _ registry.Registry = (*Fake)(nil)
