package npm

import (
	"net/http"
	"strconv"
	"time"

	"github.com/JesperRossen/version-check-mcp/internal/errs"
)

// mapHTTPStatus converts a non-2xx upstream response into a typed *errs.E.
// Pattern 4 from 02-RESEARCH.md: 404 → NotFound, 429 → RateLimited (with a
// parsed Retry-After), 5xx → UpstreamDown, anything else non-2xx →
// UpstreamDown with unexpected_status=true.
func mapHTTPStatus(resp *http.Response, pkg string) error {
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errs.NotFound("npm package not found", "pkg", pkg, "status", resp.StatusCode)
	case resp.StatusCode == http.StatusTooManyRequests:
		reset := parseRetryAfter(resp.Header.Get("Retry-After"))
		return errs.RateLimited(reset, "pkg", pkg)
	case resp.StatusCode >= 500:
		return errs.UpstreamDown(nil, "pkg", pkg, "status", resp.StatusCode)
	default:
		return errs.UpstreamDown(nil, "pkg", pkg, "status", resp.StatusCode, "unexpected_status", true)
	}
}

// parseRetryAfter parses an RFC 7231 §7.1.3 Retry-After header value, in
// either delta-seconds or HTTP-date form, and returns the absolute time at
// which the caller may retry. Empty or unparseable values fall back to
// 30 seconds in the future (Pitfall #7: never return a zero time.Time, which
// downstream consumers would interpret as "retry immediately").
func parseRetryAfter(h string) time.Time {
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
