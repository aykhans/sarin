package e2e

import (
	"net/http"
	"testing"
)

func TestConfigFileBasic(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
	assertResponseCount(t, out, 1)
}

func TestConfigFileWithMethod(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
method: POST
requests: 1
quiet: true
output: json
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodPost {
		t.Errorf("expected method POST from config, got %s", req.Method)
	}
}

func TestConfigFileWithHeaders(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
headers:
  - X-Config: config-value
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Config"]; len(v) == 0 || v[0] != "config-value" {
		t.Errorf("expected X-Config: config-value, got %v", v)
	}
}

func TestConfigFileWithParams(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
params:
  - key1: value1
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["key1"]; len(v) == 0 || v[0] != "value1" {
		t.Errorf("expected key1=value1, got %v", v)
	}
}

func TestConfigFileWithCookies(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
cookies:
  - session: abc123
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["session"]; !ok || v != "abc123" {
		t.Errorf("expected cookie session=abc123, got %v", req.Cookies)
	}
}

func TestConfigFileWithBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
method: POST
requests: 1
quiet: true
output: json
body: "hello from config"
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "hello from config" {
		t.Errorf("expected body 'hello from config', got %q", req.Body)
	}
}

func TestConfigFileCLIOverridesScalars(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "http://should-be-overridden.invalid"
requests: 1
quiet: true
output: json
`
	configPath := writeTemp(t, "config.yaml", config)

	// CLI -U should override the config file URL (scalar override)
	res := run("-f", configPath, "-U", cs.URL)
	assertExitCode(t, res, 0)
	assertResponseCount(t, res.jsonOutput(t), 1)

	// Verify it actually hit our server
	if cs.requestCount() != 1 {
		t.Errorf("expected 1 request to capture server, got %d", cs.requestCount())
	}
}

func TestConfigFileCLIOverridesMethods(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
method: GET
requests: 4
quiet: true
output: json
`
	configPath := writeTemp(t, "config.yaml", config)

	// CLI -M POST overrides config file's method: GET
	res := run("-f", configPath, "-M", "POST")
	assertExitCode(t, res, 0)

	for _, r := range cs.allRequests() {
		if r.Method != http.MethodPost {
			t.Errorf("expected all requests to be POST (CLI overrides config), got %s", r.Method)
		}
	}
}

func TestConfigFileInvalidYAML(t *testing.T) {
	t.Parallel()
	configPath := writeTemp(t, "bad.yaml", `{{{not valid yaml`)

	res := run("-f", configPath)
	assertExitCode(t, res, 1)
}

func TestConfigFileNotFound(t *testing.T) {
	t.Parallel()
	res := run("-f", "/nonexistent/path/config.yaml")
	assertExitCode(t, res, 1)
}

func TestConfigFileWithDryRun(t *testing.T) {
	t.Parallel()

	config := `
url: "http://example.com"
requests: 3
quiet: true
output: json
dryRun: true
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "dry-run")
	assertResponseCount(t, out, 3)
}

func TestConfigFileWithConcurrency(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 6
concurrency: 3
quiet: true
output: json
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 6)
}

func TestConfigFileNestedIncludes(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Create inner config
	innerConfig := `
headers:
  - X-Inner: from-inner
`
	innerPath := writeTemp(t, "inner.yaml", innerConfig)

	// Create outer config that includes inner
	outerConfig := `
configFile: "` + innerPath + `"
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
`
	outerPath := writeTemp(t, "outer.yaml", outerConfig)

	res := run("-f", outerPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Inner"]; len(v) == 0 || v[0] != "from-inner" {
		t.Errorf("expected X-Inner: from-inner from nested config, got %v", v)
	}
}

func TestConfigFileFromHTTPURL(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
headers:
  - X-Remote-Config: yes
`
	// Serve config via HTTP
	configServer := statusServerWithBody(config)
	defer configServer.Close()

	res := run("-f", configServer.URL)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Remote-Config"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Remote-Config: yes from HTTP config, got %v", v)
	}
}

func TestConfigFileMultiValueHeaders(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
headers:
  - X-Multi:
    - val1
    - val2
`
	configPath := writeTemp(t, "config.yaml", config)

	// With multiple values, sarin cycles through them (random start).
	// With -r 1, we should see exactly one of them.
	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	v, ok := req.Headers["X-Multi"]
	if !ok || len(v) == 0 {
		t.Fatalf("expected X-Multi header, got headers: %v", req.Headers)
	}
	if v[0] != "val1" && v[0] != "val2" {
		t.Errorf("expected X-Multi to be val1 or val2, got %v", v)
	}
}

func TestConfigFileWithTimeout(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
timeout: 5s
quiet: true
output: json
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)
	assertResponseCount(t, res.jsonOutput(t), 1)
}

func TestConfigFileWithInsecure(t *testing.T) {
	t.Parallel()

	config := `
url: "http://example.com"
requests: 1
insecure: true
quiet: true
output: json
dryRun: true
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)
}

func TestConfigFileWithLuaScript(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	scriptContent := `function transform(req) req.headers["X-Config-Lua"] = {"yes"} return req end`
	scriptPath := writeTemp(t, "script.lua", scriptContent)

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
lua: "@` + scriptPath + `"
`
	configPath := writeTemp(t, "config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Config-Lua"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Config-Lua: yes, got %v", v)
	}
}
