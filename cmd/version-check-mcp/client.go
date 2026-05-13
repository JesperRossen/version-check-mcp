//go:build !testfixtures

package main

import (
	"net/http"
	"time"
)

// newSharedClient returns the production HTTP client used by every registry
// adapter: a 5-second-timeout client wrapping http.DefaultTransport in a
// uaTransport so a non-blank User-Agent is injected on every outbound call.
//
// The testfixtures build tag swaps this implementation for a fixture-replay
// variant that loads HTTP responses from disk based on NPM_FIXTURE_DIR.
func newSharedClient() *http.Client {
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: uaTransport{ua: userAgent(), next: http.DefaultTransport},
	}
}
