package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once before all tests.
	tmpDir, err := os.MkdirTemp("", "sarin-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	binaryPath = filepath.Join(tmpDir, "sarin")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binaryPath, "../cmd/cli/main.go")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to build binary: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// --- Result type ---

// runResult holds the output of a sarin binary execution.
type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// jsonOutput parses the stdout as JSON output from sarin.
// Fails the test if parsing fails.
func (r runResult) jsonOutput(t *testing.T) outputData {
	t.Helper()
	var out outputData
	if err := json.Unmarshal([]byte(r.Stdout), &out); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %s", err, r.Stdout)
	}
	return out
}

// --- JSON output structures ---

type responseStat struct {
	Count   json.Number `json:"count"`
	Min     string      `json:"min"`
	Max     string      `json:"max"`
	Average string      `json:"average"`
	P90     string      `json:"p90"`
	P95     string      `json:"p95"`
	P99     string      `json:"p99"`
}

type outputData struct {
	Responses map[string]responseStat `json:"responses"`
	Total     responseStat            `json:"total"`
}

// --- echoResponse is the JSON structure returned by echoServer ---

type echoResponse struct {
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   map[string][]string `json:"query"`
	Headers map[string][]string `json:"headers"`
	Cookies map[string]string   `json:"cookies"`
	Body    string              `json:"body"`
}

// --- Helpers ---

// run executes the sarin binary with the given args and returns the result.
func run(args ...string) runResult {
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return runResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// runWithEnv executes the sarin binary with the given args and environment variables.
func runWithEnv(env map[string]string, args ...string) runResult {
	cmd := exec.Command(binaryPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start with a clean env, then add the requested vars
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return runResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

// startProcess starts the sarin binary and returns the exec.Cmd without waiting.
// The caller is responsible for managing the process lifecycle.
func startProcess(args ...string) (*exec.Cmd, *strings.Builder) {
	cmd := exec.Command(binaryPath, args...)
	var stdout strings.Builder
	cmd.Stdout = &stdout
	return cmd, &stdout
}

// slowServer returns a server that delays each response by the given duration.
func slowServer(delay time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
}

// echoServer starts an HTTP test server that echoes request details back as JSON.
// The response includes method, path, headers, query params, cookies, and body.
func echoServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		cookies := make(map[string]string)
		for _, c := range r.Cookies() {
			cookies[c.Name] = c.Value
		}

		resp := echoResponse{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.Query(),
			Headers: r.Header,
			Cookies: cookies,
			Body:    string(body),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

// captureServer records every request it receives and responds with 200.
// Use lastRequest() to inspect the most recent request.
type captureServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []echoResponse
}

func newCaptureServer() *captureServer {
	cs := &captureServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		cookies := make(map[string]string)
		for _, c := range r.Cookies() {
			cookies[c.Name] = c.Value
		}

		cs.mu.Lock()
		cs.requests = append(cs.requests, echoResponse{
			Method:  r.Method,
			Path:    r.URL.Path,
			Query:   r.URL.Query(),
			Headers: r.Header,
			Cookies: cookies,
			Body:    string(body),
		})
		cs.mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	return cs
}

func (cs *captureServer) lastRequest() echoResponse {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if len(cs.requests) == 0 {
		return echoResponse{}
	}
	return cs.requests[len(cs.requests)-1]
}

func (cs *captureServer) allRequests() []echoResponse {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	copied := make([]echoResponse, len(cs.requests))
	copy(copied, cs.requests)
	return copied
}

func (cs *captureServer) requestCount() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.requests)
}

// statusServer returns a server that always responds with the given status code.
func statusServer(code int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
}

// statusServerWithBody returns a server that responds with 200 and the given body.
func statusServerWithBody(body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
}

// writeTemp creates a temporary file with the given content and returns its path.
// The file is automatically cleaned up when the test finishes.
func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// --- Assertion helpers ---

func assertExitCode(t *testing.T, res runResult, want int) {
	t.Helper()
	if res.ExitCode != want {
		t.Errorf("expected exit code %d, got %d\nstdout: %s\nstderr: %s", want, res.ExitCode, res.Stdout, res.Stderr)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}

func assertResponseCount(t *testing.T, out outputData, wantTotal int) {
	t.Helper()
	got, err := out.Total.Count.Int64()
	if err != nil {
		t.Fatalf("failed to parse total count: %v", err)
	}
	if got != int64(wantTotal) {
		t.Errorf("expected total count %d, got %d", wantTotal, got)
	}
}

func assertHasResponseKey(t *testing.T, out outputData, key string) {
	t.Helper()
	if _, ok := out.Responses[key]; !ok {
		t.Errorf("expected %q in responses, got keys: %v", key, responseKeys(out))
	}
}

func responseKeys(out outputData) []string {
	keys := make([]string, 0, len(out.Responses))
	for k := range out.Responses {
		keys = append(keys, k)
	}
	return keys
}
