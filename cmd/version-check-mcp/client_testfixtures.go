//go:build testfixtures

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// fixtureHeaders mirrors internal/testfixtures.fixtureHeaders. Duplicated here
// rather than imported so the production binary's main package never depends
// on a test-helper package, even under the testfixtures build tag.
type fixtureHeaders struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
}

// fixtureRoundTripper resolves outbound requests by mapping req.URL.String()
// to a file under NPM_FIXTURE_DIR (currently NPM-only; extends naturally as
// more adapter fixtures land). 16 MiB hard cap mirrors the testfixtures
// helper to bound adversarial fixture content.
type fixtureRoundTripper struct {
	npmDir string
}

const maxFixtureBodyBytes = 16 << 20

func (t fixtureRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	u := r.URL.String()
	rel := mapURLToFixture(u)
	if rel == "" {
		return nil, &url2FixtureErr{u: u}
	}
	cleanDir := filepath.Clean(t.npmDir)
	full := filepath.Join(t.npmDir, rel)
	clean := filepath.Clean(full)
	if clean != cleanDir && !strings.HasPrefix(clean, cleanDir+string(filepath.Separator)) {
		return nil, &url2FixtureErr{u: u}
	}
	body, err := os.ReadFile(clean)
	if err != nil {
		return nil, err
	}
	status := http.StatusOK
	header := http.Header{"Content-Type": []string{"application/json"}}
	if hb, herr := os.ReadFile(clean + ".headers.json"); herr == nil {
		var fh fixtureHeaders
		if jerr := json.Unmarshal(hb, &fh); jerr == nil {
			if fh.Status != 0 {
				status = fh.Status
			}
			if len(fh.Headers) > 0 {
				header = http.Header{}
				for k, v := range fh.Headers {
					header.Set(k, v)
				}
			}
		}
	}
	limited := io.LimitReader(bytes.NewReader(body), maxFixtureBodyBytes)
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     header,
		Body:       io.NopCloser(limited),
		Request:    r,
	}, nil
}

type url2FixtureErr struct{ u string }

func (e *url2FixtureErr) Error() string { return "fixture: unmapped or escaping URL: " + e.u }

// mapURLToFixture returns the filename (relative to NPM_FIXTURE_DIR) that
// holds the response body for url, or "" if the URL is unmapped.
func mapURLToFixture(u string) string {
	const base = "https://registry.npmjs.org/"
	if !strings.HasPrefix(u, base) {
		return ""
	}
	pkg := strings.TrimPrefix(u, base)
	switch pkg {
	case "react":
		return "react.json"
	case "@types%2Fnode":
		return "types-node.json"
	case "nonexistent":
		return "nonexistent.json"
	default:
		return ""
	}
}

// newSharedClient returns the fixture-replay client when NPM_FIXTURE_DIR is
// set; otherwise the production client. The build-tag gating means this
// implementation never lands in the released binary.
func newSharedClient() *http.Client {
	if dir := os.Getenv("NPM_FIXTURE_DIR"); dir != "" {
		return &http.Client{
			Timeout:   5 * time.Second,
			Transport: fixtureRoundTripper{npmDir: dir},
		}
	}
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: uaTransport{ua: userAgent(), next: http.DefaultTransport},
	}
}
