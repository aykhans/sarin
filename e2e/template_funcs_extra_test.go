package e2e

import (
	"strings"
	"testing"
)

func TestDictStr(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// dict_Str creates a map; use with index to retrieve a value
	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{{ $d := dict_Str "name" "alice" "role" "admin" }}{{ index $d "name" }}-{{ index $d "role" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "alice-admin" {
		t.Errorf("expected body alice-admin, got %q", req.Body)
	}
}

func TestStringsToDate(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// strings_ToDate parses a date string; verify it produces a non-empty result
	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-H", `X-Date: {{ strings_ToDate "2024-06-15" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Date"]; len(v) == 0 || v[0] == "" {
		t.Error("expected X-Date to have a non-empty value")
	} else {
		assertContains(t, v[0], "2024")
	}
}

func TestFileBase64NonexistentFile(t *testing.T) {
	t.Parallel()

	// file_Base64 errors at runtime, the error becomes the response key
	res := run("-U", "http://example.com", "-r", "1", "-z", "-q", "-o", "json",
		"-B", `{{ file_Base64 "/nonexistent/file.txt" }}`)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	// Should have a template rendering error as response key, not "dry-run"
	if _, ok := out.Responses["dry-run"]; ok {
		t.Error("expected template error, but got dry-run response")
	}
	assertResponseCount(t, out, 1)
}

func TestFileBase64FailedHTTP(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-z", "-q", "-o", "json",
		"-B", `{{ file_Base64 "http://127.0.0.1:1/nonexistent" }}`)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	if _, ok := out.Responses["dry-run"]; ok {
		t.Error("expected template error, but got dry-run response")
	}
	assertResponseCount(t, out, 1)
}

func TestMultipleValuesFlags(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-V", "KEY1=val1", "-V", "KEY2=val2",
		"-H", "X-K1: {{ .Values.KEY1 }}",
		"-H", "X-K2: {{ .Values.KEY2 }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-K1"]; len(v) == 0 || v[0] != "val1" {
		t.Errorf("expected X-K1: val1, got %v", v)
	}
	if v := req.Headers["X-K2"]; len(v) == 0 || v[0] != "val2" {
		t.Errorf("expected X-K2: val2, got %v", v)
	}
}

func TestValuesUsedInBodyAndHeader(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Same value used in both header and body within the same request
	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-V", "ID={{ fakeit_UUID }}",
		"-H", "X-Request-Id: {{ .Values.ID }}",
		"-B", `{"id":"{{ .Values.ID }}"}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	headerID := ""
	if v := req.Headers["X-Request-Id"]; len(v) > 0 {
		headerID = v[0]
	}
	if headerID == "" {
		t.Fatal("expected X-Request-Id to have a value")
	}
	// Body should contain the same UUID as the header
	if !strings.Contains(req.Body, headerID) {
		t.Errorf("expected body to contain same ID as header (%s), got body: %s", headerID, req.Body)
	}
}
