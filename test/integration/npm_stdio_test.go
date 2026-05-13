//go:build testfixtures

// Binary-level NPM integration tests. The production binary is rebuilt with
// the testfixtures build tag so NPM_FIXTURE_DIR redirects outbound NPM HTTP
// to disk-loaded responses — CI is fully deterministic, no live registry.
//
// Run via: go test -tags testfixtures ./test/integration/... -count=1
package integration_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	npmReactPresentVersion     = "18.3.1"
	npmReactDistTagsLatest     = "19.2.6"
	npmTypesNodePresentVersion = "22.0.0"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// buildBinaryTagged builds the binary with `-tags testfixtures` exactly once
// per test process. Returns the absolute path or a fatal failure.
func buildBinaryTagged(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "vcmcp-fixt-")
		if err != nil {
			buildErr = err
			return
		}
		bin := filepath.Join(dir, "version-check-mcp")
		cmd := exec.Command("go", "build", "-tags", "testfixtures", "-o", bin, "./cmd/version-check-mcp")
		cmd.Dir = repoRoot(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildErr = err
			t.Logf("build output: %s", out)
			return
		}
		builtBin = bin
	})
	if buildErr != nil {
		t.Fatalf("build binary: %v", buildErr)
	}
	return builtBin
}

func fixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "testdata", "fixtures", "npm")
}

type session struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *bytes.Buffer
}

func (s *session) close() {
	_ = s.stdin.Close()
	_ = s.cmd.Wait()
}

func spawn(t *testing.T) *session {
	t.Helper()
	bin := buildBinaryTagged(t)
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"NPM_FIXTURE_DIR="+fixtureDir(t),
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	sess := &session{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		stderr: &stderr,
	}
	t.Cleanup(sess.close)

	// Drive `initialize` once, drain the response.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}` + "\n"
	if _, err := io.WriteString(stdin, initReq); err != nil {
		t.Fatalf("write init: %v", err)
	}
	if _, err := readJSONResponse(t, sess.stdout, 5*time.Second); err != nil {
		t.Fatalf("read init: %v; stderr=%s", err, sess.stderr.String())
	}
	return sess
}

// callTool issues a JSON-RPC tools/call request and returns the decoded
// response object.
func callTool(t *testing.T, sess *session, id int, toolName string, args map[string]any) map[string]any {
	t.Helper()
	params := map[string]any{
		"name":      toolName,
		"arguments": args,
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/call",
		"params":  params,
	}
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal req: %v", err)
	}
	if _, err := sess.stdin.Write(append(body, '\n')); err != nil {
		t.Fatalf("write req: %v", err)
	}
	resp, err := readJSONResponse(t, sess.stdout, 10*time.Second)
	if err != nil {
		t.Fatalf("read resp: %v; stderr=%s", err, sess.stderr.String())
	}
	return resp
}

// readJSONResponse reads exactly one newline-delimited JSON-RPC frame from r
// with a deadline.
func readJSONResponse(t *testing.T, r *bufio.Reader, timeout time.Duration) (map[string]any, error) {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := r.ReadString('\n')
		ch <- result{line: line, err: err}
	}()
	select {
	case res := <-ch:
		if res.err != nil && res.line == "" {
			return nil, res.err
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(res.line)), &obj); err != nil {
			return nil, err
		}
		return obj, nil
	case <-time.After(timeout):
		return nil, &timeoutErr{}
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string { return "read timeout" }

// expectSuccess returns the StructuredContent of a non-error tools/call result.
func expectSuccess(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'result': %v", resp)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("response is error envelope: %v", result)
	}
	sc, ok := result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("result missing structuredContent: %v", result)
	}
	return sc
}

// expectError returns the error type/discriminator from a tools/call error envelope.
func expectError(t *testing.T, resp map[string]any) (errType string, sc map[string]any) {
	t.Helper()
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'result': %v", resp)
	}
	if isErr, _ := result["isError"].(bool); !isErr {
		t.Fatalf("response missing isError=true: %v", result)
	}
	sc, ok = result["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("error result missing structuredContent: %v", result)
	}
	errBlock, ok := sc["error"].(map[string]any)
	if !ok {
		t.Fatalf("error result missing error block: %v", sc)
	}
	errType, _ = errBlock["type"].(string)
	return errType, sc
}

func TestStdio_NPM_Validate_Hit(t *testing.T) {
	sess := spawn(t)
	resp := callTool(t, sess, 2, "validate_version", map[string]any{
		"manager": "npm",
		"pkg":     "react",
		"version": npmReactPresentVersion,
	})
	sc := expectSuccess(t, resp)
	if got := sc["exists"]; got != true {
		t.Errorf("exists = %v, want true", got)
	}
	if got := sc["source"]; got != "versions-map" {
		t.Errorf("source = %v, want %q", got, "versions-map")
	}
	if got := sc["requested_version"]; got != npmReactPresentVersion {
		t.Errorf("requested_version = %v, want %q", got, npmReactPresentVersion)
	}
}

func TestStdio_NPM_Validate_Miss(t *testing.T) {
	sess := spawn(t)
	resp := callTool(t, sess, 2, "validate_version", map[string]any{
		"manager": "npm",
		"pkg":     "react",
		"version": "99.0.0",
	})
	errType, sc := expectError(t, resp)
	if errType != "not_found" {
		t.Errorf("error type = %q, want %q", errType, "not_found")
	}
	if got := sc["requested_version"]; got != "99.0.0" {
		t.Errorf("requested_version = %v, want %q", got, "99.0.0")
	}
}

func TestStdio_NPM_Validate_ScopedPackage(t *testing.T) {
	sess := spawn(t)
	resp := callTool(t, sess, 2, "validate_version", map[string]any{
		"manager": "npm",
		"pkg":     "@types/node",
		"version": npmTypesNodePresentVersion,
	})
	sc := expectSuccess(t, resp)
	if got := sc["exists"]; got != true {
		t.Errorf("exists = %v, want true", got)
	}
	if got := sc["source"]; got != "versions-map" {
		t.Errorf("source = %v, want %q", got, "versions-map")
	}
}

func TestStdio_NPM_Latest_DistTags(t *testing.T) {
	sess := spawn(t)
	resp := callTool(t, sess, 2, "get_latest_version", map[string]any{
		"manager": "npm",
		"pkg":     "react",
	})
	sc := expectSuccess(t, resp)
	if got := sc["source"]; got != "dist-tags.latest" {
		t.Errorf("source = %v, want %q", got, "dist-tags.latest")
	}
	if got := sc["version"]; got != npmReactDistTagsLatest {
		t.Errorf("version = %v, want %q", got, npmReactDistTagsLatest)
	}
}

func TestStdio_NPM_Latest_MajorFilter(t *testing.T) {
	sess := spawn(t)
	resp := callTool(t, sess, 2, "get_latest_version", map[string]any{
		"manager": "npm",
		"pkg":     "react",
		"major":   17,
	})
	sc := expectSuccess(t, resp)
	if got := sc["source"]; got != "computed-highest" {
		t.Errorf("source = %v, want %q", got, "computed-highest")
	}
	v, _ := sc["version"].(string)
	if !strings.HasPrefix(v, "17.") {
		t.Errorf("version = %q, want 17.* prefix", v)
	}
}

func TestStdio_NPM_Latest_FilterMiss(t *testing.T) {
	sess := spawn(t)
	resp := callTool(t, sess, 2, "get_latest_version", map[string]any{
		"manager": "npm",
		"pkg":     "react",
		"major":   99,
	})
	errType, _ := expectError(t, resp)
	if errType != "not_found" {
		t.Errorf("error type = %q, want %q", errType, "not_found")
	}
}
