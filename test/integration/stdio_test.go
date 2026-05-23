// RED test (Wave 0). Production code lands in Wave 3 (plan 01-05).
// See .planning/phases/01-foundation-mcp-scaffolding/01-VALIDATION.md.
package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
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
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	respCh := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, readErr := readOneJSONResponse(stdout)
		if readErr != nil {
			errCh <- readErr
			return
		}
		respCh <- resp
	}()

	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"0.0.0"}}}` + "\n"
	if _, err := io.WriteString(stdin, req); err != nil {
		t.Fatalf("write stdin: %v", err)
	}

	var resp map[string]any
	select {
	case resp = <-respCh:
	case readErr := <-errCh:
		t.Fatalf("read initialize response: %v; stderr=%q", readErr, stderr.String())
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for initialize response; stderr=%q", stderr.String())
	}

	if err := stdin.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("wait: %v; stderr=%q", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("binary did not exit after stdin close; stderr=%q", stderr.String())
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
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	_, _ = io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`+"\n")
	if _, err := readOneJSONResponse(stdout); err != nil {
		t.Fatalf("read initialize response: %v", err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}

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
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("args=%v StdoutPipe: %v", args, err)
		}
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("args=%v StdinPipe: %v", args, err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatalf("start %v: %v", args, err)
		}
		_, _ = io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`+"\n")
		if _, err := readOneJSONResponse(stdout); err != nil {
			t.Fatalf("args=%v read initialize response: %v", args, err)
		}
		if err := stdin.Close(); err != nil {
			t.Fatalf("args=%v close stdin: %v", args, err)
		}

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
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("help command failed: %v", err)
		}
		if code := exitErr.ExitCode(); code != 0 && code != 2 {
			t.Fatalf("help exit code = %d, want 0 or 2", code)
		}
	}
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

func readOneJSONResponse(r io.Reader) (map[string]any, error) {
	dec := json.NewDecoder(bufio.NewReader(r))
	var resp map[string]any
	if err := dec.Decode(&resp); err != nil {
		return nil, err
	}
	return resp, nil
}
