// Package errs is the canonical structured error type for the project.
// Every Registry method, the panic-recovery middleware, the MCP errmap,
// and every adapter flows through *E. The four Kind discriminator strings
// are wire-visible (they appear in MCP error envelopes) — do not rename.
package errs

import (
	"fmt"
	"time"
)

type Kind string

const (
	KindRateLimited  Kind = "rate_limited"
	KindNotFound     Kind = "not_found"
	KindUpstreamDown Kind = "upstream_down"
	KindInvalidInput Kind = "invalid_input"
)

type E struct {
	Kind    Kind
	Message string
	Details map[string]any
	Wrapped error
}

func (e *E) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("%s: %s: %s", e.Kind, e.Message, e.Wrapped.Error())
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

func (e *E) Unwrap() error { return e.Wrapped }

func InvalidInput(msg string, details ...any) *E {
	return &E{Kind: KindInvalidInput, Message: msg, Details: detailsMap(details)}
}

func NotFound(msg string, details ...any) *E {
	return &E{Kind: KindNotFound, Message: msg, Details: detailsMap(details)}
}

func RateLimited(reset time.Time, details ...any) *E {
	d := detailsMap(details)
	if _, set := d["reset_at"]; !set {
		d["reset_at"] = reset
	}
	return &E{
		Kind:    KindRateLimited,
		Message: "upstream rate limited; see details.reset_at",
		Details: d,
	}
}

func UpstreamDown(wrapped error, details ...any) *E {
	msg := "upstream unavailable"
	if wrapped != nil {
		msg = wrapped.Error()
	}
	return &E{
		Kind:    KindUpstreamDown,
		Message: msg,
		Details: detailsMap(details),
		Wrapped: wrapped,
	}
}

// detailsMap converts a slog-style variadic key/value sequence into a map.
// Non-string keys are silently dropped. A trailing key with no value is
// dropped. The returned map is always non-nil so callers can read keys
// without nil-map panics.
func detailsMap(kv []any) map[string]any {
	m := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		m[k] = kv[i+1]
	}
	return m
}
