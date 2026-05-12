---
phase: 01-foundation-mcp-scaffolding
plan: 03
status: complete
date: 2026-05-12
---

# Plan 01-03 SUMMARY — Registry interface + FakeRegistry

## What landed

### `internal/registry/registry.go`

- `type ValidateResult struct { Exists bool; Source string }`
- `type LatestResult struct { Version string; Source string }`
- `type Registry interface { Validate(...); Latest(...); Name() }` — signatures match D-05 verbatim
- Package doc states the *errs.E error contract (callers verify via `errors.As`)
- Imports: stdlib `context` only

### `internal/registry/fake/fake.go`

- `type Fake struct { ... }` — programmable Registry test double (D-06)
- `func New(name string) *Fake` — constructs with Source="fake", LatestResult.Version="v0.0.0"
- Compile-time conformance: `var _ registry.Registry = (*Fake)(nil)`

#### Exported Fake fields (quick reference for plan 05 tests)

| Field | Type | Purpose |
|---|---|---|
| `ValidateResult` | `registry.ValidateResult` | Happy-path return for Validate |
| `LatestResult` | `registry.LatestResult` | Happy-path return for Latest |
| `ValidateErr` | `error` | If non-nil, Validate returns this (after panic check) |
| `LatestErr` | `error` | If non-nil, Latest returns this (after panic check) |
| `PanicOn` | `string` | "validate" / "latest" / "any" / "" |
| `PanicMessage` | `string` | Value passed to `panic()`; defaults to "fake panic" |
| `ValidateCalls` | `atomic.Int64` | Invocation counter |
| `LatestCalls` | `atomic.Int64` | Invocation counter |

## Deviations from D-06

**Per-method error fields and atomic counters** instead of the single `Err`
field and `int` counters described in the plan:

- The recovery/envelope tests in `internal/mcp/server_test.go` cycle through
  four different `*errs.E` returns and only need one side of the Registry to
  fail at a time. Per-method `ValidateErr` / `LatestErr` is cleaner than a
  global `Err` that hits both methods.
- The SDK in-memory transport dispatches handlers on its own goroutines, so
  `ValidateCalls.Load()` from a test goroutine racing against an
  in-progress handler is undefined behaviour with a plain `int`. `atomic.Int64`
  is the correct primitive.

Interface contract is unchanged; this is purely the Fake's surface shape.

## Tests

`go test ./internal/registry/...` — 5 passed:
- `TestFakeReturnsConfiguredValidateResult`
- `TestFakeReturnsConfiguredLatestResult`
- `TestFakeReturnsConfiguredError`
- `TestFakePanicHookFires`
- `TestFakeNameMatchesConstructor`

`go vet ./internal/registry/...` — clean.

Cumulative GREEN tests after this plan: 12 across 4 packages (`depcheck`, `errs`, `registry`, `registry/fake`).

## Dependency footprint

Stdlib only: `context`, `sync/atomic`. No new direct deps.
