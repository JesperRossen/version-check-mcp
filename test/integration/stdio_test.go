// RED test (Wave 0). Production code lands in Wave 3 (plan 01-05).
// See .planning/phases/01-foundation-mcp-scaffolding/01-VALIDATION.md.
package integration_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildBinary builds the version-check-mcp binary into a t.TempDir and returns
// the absolute path. Failing builds fail the test (Wave-0 RED state).
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "version-check-mcp")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/version-check-mcp")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(bin); statErr != nil {
		t.Fatalf("built binary missing at %s: %v", bin, statErr)
	}
	return bin
}

// repoRoot walks up from CWD until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for {
		if _, err := exec.Command("test", "-f", filepath.Join(dir, "go.mod")).CombinedOutput(); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate go.mod walking up from cwd")
		}
		dir = parent
	}
}

func TestStdioCleanliness(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.0.0"}}}` + "\n"
	if _, err := io.WriteString(stdin, req); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	// Brief delay before closing stdin so the SDK's dispatch goroutine has
	// time to write the response before the read-loop sees EOF and tears
	// the session down. Without this, a stdin.Close immediately after the
	// write races the dispatch and the response is silently dropped. Real
	// MCP clients keep stdin open across multiple calls, so this race is
	// only visible in artificial single-shot tests like this one.
	time.Sleep(500 * time.Millisecond)
	_ = stdin.Close()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("binary did not exit after stdin close; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		t.Fatalf("stdout empty; want one JSON-RPC frame. stderr=%q bin=%s", stderr.String(), bin)
	}
	dec := json.NewDecoder(strings.NewReader(out))
	var resp map[string]any
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("stdout not a JSON object: %v\nraw=%q", err, out)
	}
	if dec.More() {
		t.Errorf("stdout contains more than one JSON value")
	}
	if got := resp["jsonrpc"]; got != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", got)
	}
	if got := resp["id"]; got != float64(1) {
		t.Errorf("id = %v (%T), want 1", got, got)
	}
}

func TestStderrIsJSON(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	stdin, _ := cmd.StdinPipe()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _ = io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`+"\n")
	time.Sleep(500 * time.Millisecond)
	_ = stdin.Close()
	_ = cmd.Wait()

	// Stderr may be empty (server silent at info level); that satisfies "no non-JSON lines".
	// If non-empty, every non-empty line must parse as a JSON object.
	for _, line := range strings.Split(stderr.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("stderr line is not JSON: %q (err=%v)", line, err)
		}
	}
}

func TestCacheTTLFlag(t *testing.T) {
	bin := buildBinary(t)

	for _, args := range [][]string{{}, {"--cache-ttl=2h"}} {
		cmd := exec.Command(bin, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		stdin, _ := cmd.StdinPipe()
		if err := cmd.Start(); err != nil {
			t.Fatalf("start %v: %v", args, err)
		}
		_, _ = io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`+"\n")
		time.Sleep(500 * time.Millisecond)
		_ = stdin.Close()

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("args=%v wait err: %v; stderr=%s", args, err, stderr.String())
			}
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Errorf("args=%v: binary did not exit", args)
		}

		if strings.TrimSpace(stdout.String()) == "" {
			t.Errorf("args=%v: no stdout response", args)
		}
	}
}

func TestHelpOutput(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	// Go's flag package handles --help by printing usage and calling os.Exit(0)
	// (exit 2 only fires when an unknown/malformed flag is supplied). We
	// permit either outcome; the substantive assertion is that the help text
	// names both flags.
	_ = err
	combined := stdout.String() + stderr.String()
	// Go's flag package prints single-dash form in usage even though both
	// `-flag` and `--flag` are accepted on the CLI. We assert on substrings
	// that match either form (`-cache-ttl` is the prefix of `--cache-ttl`).
	for _, want := range []string{"-cache-ttl", "-verbose"} {
		if !strings.Contains(combined, want) {
			t.Errorf("help output missing %q; got:\n%s", want, combined)
		}
	}
}
