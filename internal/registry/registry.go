// Package registry defines the locked seam (D-05) every package-version
// adapter implements: NPM (Phase 2), PyPI/Go/GitHub/Maven (Phase 3). The
// interface is stable across Phases 2–6 — drift breaks every downstream
// adapter.
//
// Error contract: all errors returned from Validate, Latest, and any future
// method MUST be *errs.E (wrapping permitted; errors.As must recover the
// *errs.E). The cache layer, the MCP errmap, and the panic-recovery
// middleware all depend on this. The contract is enforced by callers via
// errors.As, not by the interface signature — that's why this package does
// not import internal/errs.
package registry

import "context"

// ValidateResult is the answer to "does this exact version exist?".
// Source describes how existence was confirmed (e.g. "dist-tags.latest",
// "computed-highest", or "fake" in the Phase-1 test double).
type ValidateResult struct {
	Exists bool
	Source string
}

// LatestResult is the answer to "what is the latest version of this package?".
// Version is ecosystem-native (Go retains the "v" prefix, NPM does not — see
// D-05). No normalisation happens at the interface boundary.
type LatestResult struct {
	Version string
	Source  string
}

// Registry is the locked seam from D-05. All adapters implement these three
// methods. The major/minor *int parameters on Latest distinguish a true "no
// filter" (nil) from "filter to major 0" (&zero) — D-05 rationale.
type Registry interface {
	Validate(ctx context.Context, pkg, version string, incPre bool) (ValidateResult, error)
	Latest(ctx context.Context, pkg string, incPre bool, major, minor *int) (LatestResult, error)
	// Versions returns all known version strings for the package.
	// The returned strings are in ecosystem-native form (v-prefixed for Go/GH,
	// unprefixed for NPM/PyPI/Maven). Results come from the adapter's internal
	// cache (populated by Validate or Latest calls) so no extra HTTP call occurs
	// if the cache is warm. Per D-IF-02.
	Versions(ctx context.Context, pkg string, incPre bool) ([]string, error)
	// Name returns the manager identifier this registry serves: "npm",
	// "pypi", "gomod", "gh", "maven", "crate", "rubygems", or "fake"
	// (test double only).
	Name() string
}
