// Package httperr provides shared HTTP-status-to-structured-error mapping for
// all registry adapters.
//
// D-HTTPERR-PROMOTE: promoted from internal/registry/npm/errmap.go so that
// PyPI, Go Modules, GitHub, and Maven adapters can reuse the same mapping
// without duplicating code. The added registryName parameter ensures error
// messages and details are registry-specific rather than hardcoded to "npm".
//
// Security (T-03-01): ParseRetryAfter never propagates a raw header value into
// the error message. A malformed header silently falls back to 30 seconds.
package httperr

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
)

// MapHTTPStatus converts a non-2xx upstream response into a typed *errs.E.
//
// Mapping:
//   - 404 → errs.NotFound with pkg and registry details.
//   - 429 → errs.RateLimited with Retry-After parsed via ParseRetryAfter and
//     registry in details.
//   - 5xx → errs.UpstreamDown with status and registry in details.
//   - Any other non-2xx → errs.UpstreamDown with unexpected_status=true.
//
// The not-found message includes registryName so callers see e.g.
// "pypi package not found" rather than a hardcoded "npm" string.
func MapHTTPStatus(resp *http.Response, pkg, registryName string) error {
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errs.NotFound(
			fmt.Sprintf("%s package not found", registryName),
			"pkg", pkg,
			"registry", registryName,
			"status", resp.StatusCode,
		)
	case resp.StatusCode == http.StatusTooManyRequests:
		reset := ParseRetryAfter(resp.Header.Get("Retry-After"))
		return errs.RateLimited(reset, "pkg", pkg, "registry", registryName)
	case resp.StatusCode >= 500:
		return errs.UpstreamDown(nil, "pkg", pkg, "registry", registryName, "status", resp.StatusCode)
	default:
		return errs.UpstreamDown(nil, "pkg", pkg, "registry", registryName, "status", resp.StatusCode, "unexpected_status", true)
	}
}

// ParseRetryAfter parses an RFC 7231 §7.1.3 Retry-After header value, in
// either delta-seconds or HTTP-date form, and returns the absolute time at
// which the caller may retry.
//
// Empty or unparseable values fall back to 30 seconds in the future
// (Pitfall #7: never return a zero time.Time, which downstream consumers
// would interpret as "retry immediately").
//
// ParseRetryAfter is exported (capitalised) so adapters such as the GitHub
// adapter can reuse it directly for X-RateLimit-Reset header parsing.
func ParseRetryAfter(h string) time.Time {
	const fallback = 30 * time.Second
	if h == "" {
		return time.Now().Add(fallback)
	}
	if secs, err := strconv.Atoi(h); err == nil {
		return time.Now().Add(time.Duration(secs) * time.Second)
	}
	if t, err := http.ParseTime(h); err == nil {
		return t
	}
	return time.Now().Add(fallback)
}
