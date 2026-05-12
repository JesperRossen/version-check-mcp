package testfixtures

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

// fakeTB satisfies testing.TB just enough to capture Fatalf calls without
// aborting the surrounding test goroutine. We embed *testing.T so unused
// interface methods inherit a real implementation; we then override the
// failure path.
type fakeTB struct {
	*testing.T
	fatalCount int
	lastMsg    string
}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.fatalCount++
	f.lastMsg = fmt.Sprintf(format, args...)
	// Do NOT call runtime.Goexit; we want execution to continue so the test
	// can observe fatalCount. The RoundTripper that called Fatalf returns
	// (nil, nil) afterwards which the http.Client will surface as an error.
	_ = runtime.NumGoroutine() // keep import used
}

func (f *fakeTB) Helper() {}

func TestFixtureClient_ReplaysBody(t *testing.T) {
	dir := t.TempDir()
	body := []byte(`{"hello":"world"}`)
	if err := os.WriteFile(filepath.Join(dir, "ok.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	client := FixtureClient(t, dir, func(_ string) string { return "ok.json" }, nil)
	resp, err := client.Get("https://example.test/whatever")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	got, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != string(body) {
		t.Fatalf("body mismatch: got %q want %q", got, body)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: got %q want application/json", ct)
	}
}

func TestFixtureClient_MissingFails(t *testing.T) {
	dir := t.TempDir()
	ftb := &fakeTB{T: t}
	client := FixtureClient(ftb, dir, func(_ string) string { return "missing.json" }, nil)
	_, _ = client.Get("https://example.test/x")
	if ftb.fatalCount == 0 {
		t.Fatalf("expected Fatalf to be invoked for missing fixture")
	}
	if ftb.lastMsg == "" || !contains(ftb.lastMsg, "UPDATE_FIXTURES=1") {
		t.Fatalf("expected hint to regenerate via UPDATE_FIXTURES=1, got %q", ftb.lastMsg)
	}
}

func TestFixtureClient_CountsCalls(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ok.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var counter atomic.Int64
	client := FixtureClient(t, dir, func(_ string) string { return "ok.json" }, &counter)
	for i := 0; i < 3; i++ {
		resp, err := client.Get("https://example.test/x")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		_ = resp.Body.Close()
	}
	if got := counter.Load(); got != 3 {
		t.Fatalf("callCount: got %d want 3", got)
	}
}

func TestFixtureClient_HeadersOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.json"), []byte(``), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.json.headers.json"),
		[]byte(`{"status":404,"headers":{"Content-Type":"application/json"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	client := FixtureClient(t, dir, func(_ string) string { return "x.json" }, nil)
	resp, err := client.Get("https://example.test/x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status: got %d want 404", resp.StatusCode)
	}
}

func TestFixtureClient_PathTraversalGuard(t *testing.T) {
	dir := t.TempDir()
	ftb := &fakeTB{T: t}
	client := FixtureClient(ftb, dir, func(_ string) string { return "../../etc/passwd" }, nil)
	_, _ = client.Get("https://example.test/x")
	if ftb.fatalCount == 0 {
		t.Fatalf("expected Fatalf for path-traversal attempt")
	}
	if !contains(ftb.lastMsg, "escapes fixture dir") {
		t.Fatalf("expected escape-fixture message, got %q", ftb.lastMsg)
	}
}

func TestUpdateMode_Env(t *testing.T) {
	t.Setenv("UPDATE_FIXTURES", "1")
	if !UpdateMode() {
		t.Fatalf("UpdateMode() should be true when UPDATE_FIXTURES=1")
	}
	t.Setenv("UPDATE_FIXTURES", "")
	if UpdateMode() {
		t.Fatalf("UpdateMode() should be false when UPDATE_FIXTURES is unset/empty")
	}
}

// Ensure roundTripperFunc satisfies http.RoundTripper (compile-time check).
var _ http.RoundTripper = roundTripperFunc(nil)

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
