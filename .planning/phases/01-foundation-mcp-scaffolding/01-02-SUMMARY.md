---
phase: 01-foundation-mcp-scaffolding
plan: 02
status: complete
date: 2026-05-12
---

# Plan 01-02 SUMMARY — internal/errs package

## What landed

`internal/errs/errs.go` (~85 lines):

- `type Kind string` + 4 constants (`KindRateLimited`/`KindNotFound`/`KindUpstreamDown`/`KindInvalidInput`)
  with wire-visible string values `rate_limited`/`not_found`/`upstream_down`/`invalid_input` (UX-02).
- `type E struct { Kind; Message; Details map[string]any; Wrapped error }`.
- `(*E).Error()` — single-line `"<kind>: <msg>[: <wrapped>]"` representation.
- `(*E).Unwrap()` — returns `Wrapped` so `errors.Unwrap`/`errors.Is` chains work.
- `InvalidInput(msg, details...)`, `NotFound(msg, details...)`, `RateLimited(reset, details...)`,
  `UpstreamDown(wrapped, details...)` — all constructors.
- Unexported `detailsMap(kv []any)` helper: slog-style key/value variadic; non-string keys dropped;
  odd-length variadic drops the trailing key; always returns a non-nil map.

## Tests

`go test -v ./internal/errs/...` — 5 passed:
- `TestKindsHaveCorrectStringValues`
- `TestConstructorsSetKind`
- `TestErrorsAsRecoversE`
- `TestUnwrapReturnsWrapped`
- `TestRateLimitedDetailsCarryResetTime`

`go vet ./internal/errs/...` — clean.

## Deviations from D-07

None. Constructor signatures and field layout match the locked spec verbatim.

## Dependency footprint

Stdlib only: `fmt`, `time`. No new direct deps.
