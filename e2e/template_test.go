package e2e

import (
	"testing"
)

func TestTemplateInHeader(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Use a template function that generates a UUID
	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-H", "X-Request-Id: {{ fakeit_UUID }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals, ok := req.Headers["X-Request-Id"]
	if !ok || len(vals) == 0 {
		t.Fatalf("expected X-Request-Id header, got headers: %v", req.Headers)
	}
	// UUID format: 8-4-4-4-12
	if len(vals[0]) != 36 {
		t.Errorf("expected UUID (36 chars), got %q (%d chars)", vals[0], len(vals[0]))
	}
}

func TestTemplateInParam(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-P", "id={{ fakeit_UUID }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals, ok := req.Query["id"]
	if !ok || len(vals) == 0 {
		t.Fatalf("expected 'id' param, got query: %v", req.Query)
	}
	if len(vals[0]) != 36 {
		t.Errorf("expected UUID in param value, got %q", vals[0])
	}
}

func TestTemplateInBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-B", `{"id":"{{ fakeit_UUID }}"}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if len(req.Body) < 36 {
		t.Errorf("expected body to contain a UUID, got %q", req.Body)
	}
	assertContains(t, req.Body, `"id":"`)
}

func TestTemplateInURLPath(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL+"/api/{{ fakeit_UUID }}", "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if len(req.Path) < 5+36 { // "/api/" + UUID
		t.Errorf("expected path to contain a UUID, got %q", req.Path)
	}
}

func TestValuesBasic(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-V", "MY_VAR=hello",
		"-H", "X-Val: {{ .Values.MY_VAR }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Val"]; len(v) == 0 || v[0] != "hello" {
		t.Errorf("expected X-Val: hello from Values, got %v", v)
	}
}

func TestValuesMultiple(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-V", "A=first",
		"-V", "B=second",
		"-H", "X-A: {{ .Values.A }}",
		"-H", "X-B: {{ .Values.B }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-A"]; len(v) == 0 || v[0] != "first" {
		t.Errorf("expected X-A: first, got %v", v)
	}
	if v := req.Headers["X-B"]; len(v) == 0 || v[0] != "second" {
		t.Errorf("expected X-B: second, got %v", v)
	}
}

func TestValuesWithTemplate(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Values themselves can contain templates
	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-V", "REQ_ID={{ fakeit_UUID }}",
		"-H", "X-Request-Id: {{ .Values.REQ_ID }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals, ok := req.Headers["X-Request-Id"]
	if !ok || len(vals) == 0 {
		t.Fatalf("expected X-Request-Id header, got %v", req.Headers)
	}
	if len(vals[0]) != 36 {
		t.Errorf("expected UUID from value template, got %q", vals[0])
	}
}

func TestValuesInParam(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-V", "TOKEN=abc123",
		"-P", "token={{ .Values.TOKEN }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["token"]; len(v) == 0 || v[0] != "abc123" {
		t.Errorf("expected token=abc123, got %v", v)
	}
}

func TestValuesInBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-V", "NAME=test-user",
		"-B", `{"name":"{{ .Values.NAME }}"}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != `{"name":"test-user"}` {
		t.Errorf("expected body with interpolated value, got %q", req.Body)
	}
}

func TestValuesInURLPath(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL+"/users/{{ .Values.USER_ID }}", "-r", "1", "-q", "-o", "json",
		"-V", "USER_ID=42")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Path != "/users/42" {
		t.Errorf("expected path /users/42, got %s", req.Path)
	}
}

func TestTemplateGeneratesDifferentValues(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "5", "-c", "1", "-q", "-o", "json",
		"-H", "X-Unique: {{ fakeit_UUID }}")
	assertExitCode(t, res, 0)

	reqs := cs.allRequests()
	if len(reqs) < 5 {
		t.Fatalf("expected 5 requests, got %d", len(reqs))
	}

	// UUIDs should be unique across requests
	seen := make(map[string]bool)
	for _, r := range reqs {
		vals := r.Headers["X-Unique"]
		if len(vals) > 0 {
			seen[vals[0]] = true
		}
	}
	if len(seen) < 2 {
		t.Errorf("expected template to generate different UUIDs across requests, got %d unique values", len(seen))
	}
}

func TestTemplateFunctionFakeit(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	t.Cleanup(cs.Close)

	// Test various fakeit functions
	tests := []struct {
		name     string
		template string
	}{
		{"UUID", "{{ fakeit_UUID }}"},
		{"Name", "{{ fakeit_Name }}"},
		{"Email", "{{ fakeit_Email }}"},
		{"Number", "{{ fakeit_Number 1 100 }}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cs := newCaptureServer()
			defer cs.Close()

			res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
				"-H", "X-Test: "+tt.template)
			assertExitCode(t, res, 0)

			req := cs.lastRequest()
			if v := req.Headers["X-Test"]; len(v) == 0 || v[0] == "" {
				t.Errorf("expected non-empty value from %s, got %v", tt.template, v)
			}
		})
	}
}
