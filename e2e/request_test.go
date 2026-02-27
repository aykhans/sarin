package e2e

import (
	"net/http"
	"slices"
	"testing"
)

func TestMethodGET(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodGet {
		t.Errorf("expected default method GET, got %s", req.Method)
	}
}

func TestMethodPOST(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodPost {
		t.Errorf("expected method POST, got %s", req.Method)
	}
}

func TestMethodExplicit(t *testing.T) {
	t.Parallel()
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			cs := newCaptureServer()
			defer cs.Close()

			res := run("-U", cs.URL, "-r", "1", "-M", method, "-q", "-o", "json")
			assertExitCode(t, res, 0)

			req := cs.lastRequest()
			if req.Method != method {
				t.Errorf("expected method %s, got %s", method, req.Method)
			}
		})
	}
}

func TestMultipleMethods(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// With multiple methods, sarin cycles through them
	res := run("-U", cs.URL, "-r", "4", "-M", "GET", "-M", "POST", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	reqs := cs.allRequests()
	if len(reqs) != 4 {
		t.Fatalf("expected 4 requests, got %d", len(reqs))
	}

	// Should see both GET and POST used
	methods := make(map[string]bool)
	for _, r := range reqs {
		methods[r.Method] = true
	}
	if !methods["GET"] || !methods["POST"] {
		t.Errorf("expected both GET and POST to be used, got methods: %v", methods)
	}
}

func TestSingleHeader(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-H", "X-Custom: hello", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals, ok := req.Headers["X-Custom"]
	if !ok {
		t.Fatalf("expected X-Custom header, got headers: %v", req.Headers)
	}
	if len(vals) != 1 || vals[0] != "hello" {
		t.Errorf("expected X-Custom: [hello], got %v", vals)
	}
}

func TestMultipleHeaders(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1",
		"-H", "X-First: one",
		"-H", "X-Second: two",
		"-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-First"]; len(v) == 0 || v[0] != "one" {
		t.Errorf("expected X-First: one, got %v", v)
	}
	if v := req.Headers["X-Second"]; len(v) == 0 || v[0] != "two" {
		t.Errorf("expected X-Second: two, got %v", v)
	}
}

func TestHeaderWithEmptyValue(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Header without ": " separator should have empty value
	res := run("-U", cs.URL, "-r", "1", "-H", "X-Empty", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if _, ok := req.Headers["X-Empty"]; !ok {
		t.Errorf("expected X-Empty header to be present, got headers: %v", req.Headers)
	}
}

func TestDefaultUserAgentHeader(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	ua, ok := req.Headers["User-Agent"]
	if !ok || len(ua) == 0 {
		t.Fatalf("expected User-Agent header, got headers: %v", req.Headers)
	}
	assertContains(t, ua[0], "Sarin/")
}

func TestCustomUserAgentOverridesDefault(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-H", "User-Agent: MyAgent/1.0", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	ua := req.Headers["User-Agent"]
	if len(ua) == 0 {
		t.Fatal("expected User-Agent header")
	}
	// When user sets User-Agent, the default should not be added
	if slices.Contains(ua, "MyAgent/1.0") {
		return // found the custom one
	}
	t.Errorf("expected custom User-Agent 'MyAgent/1.0', got %v", ua)
}

func TestSingleParam(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-P", "key1=value1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals, ok := req.Query["key1"]
	if !ok {
		t.Fatalf("expected key1 param, got query: %v", req.Query)
	}
	if len(vals) != 1 || vals[0] != "value1" {
		t.Errorf("expected key1=[value1], got %v", vals)
	}
}

func TestMultipleParams(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1",
		"-P", "a=1",
		"-P", "b=2",
		"-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["a"]; len(v) == 0 || v[0] != "1" {
		t.Errorf("expected a=1, got %v", v)
	}
	if v := req.Query["b"]; len(v) == 0 || v[0] != "2" {
		t.Errorf("expected b=2, got %v", v)
	}
}

func TestParamsFromURL(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Params in the URL itself should be extracted and sent
	res := run("-U", cs.URL+"?fromurl=yes", "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["fromurl"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected fromurl=yes from URL query, got %v", v)
	}
}

func TestParamsFromURLAndFlag(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Both URL params and -P params should be sent
	res := run("-U", cs.URL+"?fromurl=yes", "-r", "1", "-P", "fromflag=also", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["fromurl"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected fromurl=yes, got %v", v)
	}
	if v := req.Query["fromflag"]; len(v) == 0 || v[0] != "also" {
		t.Errorf("expected fromflag=also, got %v", v)
	}
}

func TestSingleCookie(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-C", "session=abc123", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["session"]; !ok || v != "abc123" {
		t.Errorf("expected cookie session=abc123, got cookies: %v", req.Cookies)
	}
}

func TestMultipleCookies(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1",
		"-C", "session=abc",
		"-C", "token=xyz",
		"-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["session"]; !ok || v != "abc" {
		t.Errorf("expected cookie session=abc, got %v", req.Cookies)
	}
	if v, ok := req.Cookies["token"]; !ok || v != "xyz" {
		t.Errorf("expected cookie token=xyz, got %v", req.Cookies)
	}
}

func TestBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-B", "hello world", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "hello world" {
		t.Errorf("expected body 'hello world', got %q", req.Body)
	}
}

func TestBodyJSON(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	jsonBody := `{"name":"test","value":42}`
	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-B", jsonBody, "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != jsonBody {
		t.Errorf("expected body %q, got %q", jsonBody, req.Body)
	}
}

func TestURLPath(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL+"/api/v1/users", "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Path != "/api/v1/users" {
		t.Errorf("expected path /api/v1/users, got %s", req.Path)
	}
}

func TestParamWithEmptyValue(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Param without = value
	res := run("-U", cs.URL, "-r", "1", "-P", "empty", "-q", "-o", "json")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if _, ok := req.Query["empty"]; !ok {
		t.Errorf("expected 'empty' param to be present, got query: %v", req.Query)
	}
}
