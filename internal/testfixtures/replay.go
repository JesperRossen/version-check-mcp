// Package testfixtures provides a stdlib-only fixture-replay HTTP client for
// use in adapter tests. The production binary must never import this package;
// it is intentionally internal/ and test-oriented.
//
// Design references:
//   - D-HTTP-01: net/http.RoundTripper IS the seam — no custom interface
//     wrapper. We adapt with an unexported function type.
//   - D-FIX-01: committed fixtures are literal HTTP response bodies, one per
//     file under testdata/fixtures/<adapter>/.
//   - D-FIX-02: a missing fixture must fail the test loudly (no skip).
//
// Recording: set UPDATE_FIXTURES=1 in the environment to swap the replay
// transport for a recording transport that writes upstream bodies to disk.
// Recording mode is dev-only; never run in CI.
package testfixtures

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// roundTripperFunc adapts a plain function to the http.RoundTripper interface.
// Unexported: callers receive a fully built *http.Client; the stdlib
// RoundTripper IS the seam (D-HTTP-01), so we do not export a wrapper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper.
func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// maxFixtureBodyBytes bounds the bytes we are willing to replay from a
// fixture file. 16 MiB is well above any real registry response and bounds
// adversarial fixture content (T-02-05).
const maxFixtureBodyBytes = 16 << 20

// fixtureHeaders is the on-disk schema for an optional `<fixture>.headers.json`
// sibling file. When present, it overrides the default 200 status and JSON
// Content-Type header.
type fixtureHeaders struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// UpdateMode reports whether the test process is in fixture-recording mode,
// gated by UPDATE_FIXTURES=1.
func UpdateMode() bool {
	return os.Getenv("UPDATE_FIXTURES") == "1"
}

// FixtureClient returns an *http.Client whose Transport replays HTTP response
// bodies from disk. The urlToFile callback maps a request URL string to a path
// relative to fixtureDir. If callCount is non-nil, it is incremented on every
// RoundTrip — useful for cache-hit assertions (loader-called-exactly-once).
//
// A missing fixture file causes t.Fatalf with a regenerate hint (D-FIX-02).
// A path-traversal attempt by urlToFile is rejected with t.Fatalf (T-02-02).
// The response body is wrapped in an io.LimitReader bounding it to 16 MiB
// (T-02-05).
func FixtureClient(t testing.TB, fixtureDir string, urlToFile func(reqURL string) string, callCount *atomic.Int64) *http.Client {
	t.Helper()
	cleanDir := filepath.Clean(fixtureDir)
	tr := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if callCount != nil {
			callCount.Add(1)
		}
		rel := urlToFile(req.URL.String())
		full := filepath.Join(fixtureDir, rel)
		clean := filepath.Clean(full)
		// Path-traversal guard: the resolved path must lie inside fixtureDir.
		if clean != cleanDir && !strings.HasPrefix(clean, cleanDir+string(filepath.Separator)) {
			t.Fatalf("fixture path escapes fixture dir: %q", clean)
			return nil, nil
		}

		body, err := os.ReadFile(clean)
		if err != nil {
			if os.IsNotExist(err) {
				t.Fatalf("missing fixture %q for URL %q (regenerate with UPDATE_FIXTURES=1)", clean, req.URL.String())
				return nil, nil
			}
			t.Fatalf("read fixture %q: %v", clean, err)
			return nil, nil
		}

		status := http.StatusOK
		header := http.Header{"Content-Type": []string{"application/json"}}

		if hb, err := os.ReadFile(clean + ".headers.json"); err == nil {
			var fh fixtureHeaders
			if jerr := json.Unmarshal(hb, &fh); jerr != nil {
				t.Fatalf("decode headers file %q: %v", clean+".headers.json", jerr)
				return nil, nil
			}
			if fh.Status != 0 {
				status = fh.Status
			}
			if len(fh.Headers) > 0 {
				header = http.Header{}
				for k, v := range fh.Headers {
					header.Set(k, v)
				}
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("read headers file %q: %v", clean+".headers.json", err)
			return nil, nil
		}

		limited := io.LimitReader(bytes.NewReader(body), maxFixtureBodyBytes)
		return &http.Response{
			StatusCode: status,
			Status:     http.StatusText(status),
			Header:     header,
			Body:       io.NopCloser(limited),
			Request:    req,
		}, nil
	})
	return &http.Client{Transport: tr}
}

// RecordingClient returns an *http.Client whose Transport performs real HTTP
// calls (via http.DefaultTransport) and writes each response body to
// fixtureDir/urlToFile(req.URL). For non-200 responses or responses carrying a
// Retry-After header, a sibling <file>.headers.json is also written so the
// FixtureClient can replay the same status code later.
//
// Only intended for dev use under UPDATE_FIXTURES=1.
func RecordingClient(t testing.TB, fixtureDir string, urlToFile func(string) string) *http.Client {
	t.Helper()
	tr := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		resp, err := http.DefaultTransport.RoundTrip(req)
		if err != nil {
			t.Fatalf("recording transport: real request failed: %v", err)
			return nil, nil
		}
		buf, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("recording transport: read body: %v", err)
			return nil, nil
		}

		rel := urlToFile(req.URL.String())
		full := filepath.Join(fixtureDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("recording transport: mkdir: %v", err)
			return nil, nil
		}
		if err := os.WriteFile(full, buf, 0o644); err != nil {
			t.Fatalf("recording transport: write fixture: %v", err)
			return nil, nil
		}

		if resp.StatusCode != http.StatusOK || resp.Header.Get("Retry-After") != "" {
			hdrs := fixtureHeaders{
				Status:  resp.StatusCode,
				Headers: map[string]string{},
			}
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				hdrs.Headers["Retry-After"] = ra
			}
			out, jerr := json.Marshal(hdrs)
			if jerr != nil {
				t.Fatalf("recording transport: marshal headers: %v", jerr)
				return nil, nil
			}
			if err := os.WriteFile(full+".headers.json", out, 0o644); err != nil {
				t.Fatalf("recording transport: write headers: %v", err)
				return nil, nil
			}
		}

		resp.Body = io.NopCloser(bytes.NewReader(buf))
		return resp, nil
	})
	return &http.Client{Transport: tr}
}

// Client returns the recording client when UPDATE_FIXTURES=1, otherwise the
// replay client. Adapter tests should always call Client; the env switch
// selects the mode.
func Client(t testing.TB, fixtureDir string, urlToFile func(string) string, callCount *atomic.Int64) *http.Client {
	t.Helper()
	if UpdateMode() {
		return RecordingClient(t, fixtureDir, urlToFile)
	}
	return FixtureClient(t, fixtureDir, urlToFile, callCount)
}
