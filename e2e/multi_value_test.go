package e2e

import (
	"net/http"
	"testing"
)

// --- CLI: multiple same-key values are all sent in every request ---

func TestMultipleHeadersSameKeyCLI(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-H", "X-Multi: value1", "-H", "X-Multi: value2")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals := req.Headers["X-Multi"]
	if len(vals) < 2 {
		t.Fatalf("expected 2 values for X-Multi, got %v", vals)
	}
	found := map[string]bool{}
	for _, v := range vals {
		found[v] = true
	}
	if !found["value1"] || !found["value2"] {
		t.Errorf("expected both value1 and value2, got %v", vals)
	}
}

func TestMultipleParamsSameKeyCLI(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-P", "color=red", "-P", "color=blue")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals := req.Query["color"]
	if len(vals) < 2 {
		t.Fatalf("expected 2 values for color param, got %v", vals)
	}
	found := map[string]bool{}
	for _, v := range vals {
		found[v] = true
	}
	if !found["red"] || !found["blue"] {
		t.Errorf("expected both red and blue, got %v", vals)
	}
}

func TestMultipleCookiesSameKeyCLI(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-C", "token=abc", "-C", "token=def")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	cookieHeader := ""
	if v := req.Headers["Cookie"]; len(v) > 0 {
		cookieHeader = v[0]
	}
	assertContains(t, cookieHeader, "token=abc")
	assertContains(t, cookieHeader, "token=def")
}

// --- Config file: multiple values for same key cycle across requests ---

func TestMultipleHeadersSameKeyYAMLCycle(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 20
concurrency: 1
quiet: true
output: json
headers:
  - X-Multi: [val-a, val-b]
`
	configPath := writeTemp(t, "multi_header.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	seen := map[string]bool{}
	for _, req := range cs.allRequests() {
		if vals := req.Headers["X-Multi"]; len(vals) > 0 {
			seen[vals[0]] = true
		}
	}
	if !seen["val-a"] {
		t.Error("expected val-a to appear in some requests")
	}
	if !seen["val-b"] {
		t.Error("expected val-b to appear in some requests")
	}
}

func TestMultipleParamsSameKeyYAMLCycle(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 20
concurrency: 1
quiet: true
output: json
params:
  - tag: [go, rust]
`
	configPath := writeTemp(t, "multi_param.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	seen := map[string]bool{}
	for _, req := range cs.allRequests() {
		if vals := req.Query["tag"]; len(vals) > 0 {
			seen[vals[0]] = true
		}
	}
	if !seen["go"] {
		t.Error("expected 'go' to appear in some requests")
	}
	if !seen["rust"] {
		t.Error("expected 'rust' to appear in some requests")
	}
}

// --- Multiple bodies cycle ---

func TestMultipleBodiesCycle(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "10", "-c", "1", "-M", "POST", "-q", "-o", "json",
		"-B", "body-alpha", "-B", "body-beta")
	assertExitCode(t, res, 0)

	bodies := map[string]bool{}
	for _, req := range cs.allRequests() {
		bodies[req.Body] = true
	}
	if !bodies["body-alpha"] {
		t.Error("expected body-alpha to appear in requests")
	}
	if !bodies["body-beta"] {
		t.Error("expected body-beta to appear in requests")
	}
}

// --- Multiple methods cycling ---

func TestMultipleMethodsCycleDistribution(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "20", "-c", "1", "-q", "-o", "json",
		"-M", "GET", "-M", "POST", "-M", "PUT")
	assertExitCode(t, res, 0)

	methods := map[string]int{}
	for _, req := range cs.allRequests() {
		methods[req.Method]++
	}
	if methods["GET"] == 0 {
		t.Error("expected GET to appear")
	}
	if methods["POST"] == 0 {
		t.Error("expected POST to appear")
	}
	if methods["PUT"] == 0 {
		t.Error("expected PUT to appear")
	}
}

// --- Template in method ---

func TestTemplateInMethod(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-M", `{{ strings_ToUpper "post" }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodPost {
		t.Errorf("expected method POST from template, got %s", req.Method)
	}
}

// --- Template in cookie value ---

func TestTemplateInCookie(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-C", `session={{ fakeit_UUID }}`)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Cookies["session"] == "" {
		t.Error("expected session cookie with UUID value, got empty")
	}
	if len(req.Cookies["session"]) < 10 {
		t.Errorf("expected UUID-like session cookie, got %q", req.Cookies["session"])
	}
}
