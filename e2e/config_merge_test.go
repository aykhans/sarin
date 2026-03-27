package e2e

import (
	"net/http"
	"testing"
)

// --- Multiple config files ---

func TestMultipleConfigFiles(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config1 := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
headers:
  - X-From-File1: yes
`
	config2 := `
headers:
  - X-From-File2: yes
`
	path1 := writeTemp(t, "merge1.yaml", config1)
	path2 := writeTemp(t, "merge2.yaml", config2)

	res := run("-f", path1, "-f", path2)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-From-File1"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-From-File1: yes, got %v", v)
	}
	if v := req.Headers["X-From-File2"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-From-File2: yes, got %v", v)
	}
}

func TestMultipleConfigFilesScalarOverride(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Second config file overrides URL from first
	config1 := `
url: "http://should-be-overridden.invalid"
requests: 1
quiet: true
output: json
`
	config2 := `
url: "` + cs.URL + `"
`
	path1 := writeTemp(t, "merge_scalar1.yaml", config1)
	path2 := writeTemp(t, "merge_scalar2.yaml", config2)

	res := run("-f", path1, "-f", path2)
	assertExitCode(t, res, 0)

	if cs.requestCount() != 1 {
		t.Errorf("expected request to go to second config's URL, got %d requests", cs.requestCount())
	}
}

// --- Three-way merge: env + config file + CLI ---

func TestThreeWayMergePriority(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
method: PUT
headers:
  - X-From-Config: config-value
`
	configPath := writeTemp(t, "three_way.yaml", config)

	// ENV sets URL and header, config file sets method and header, CLI overrides URL
	res := runWithEnv(map[string]string{
		"SARIN_HEADER": "X-From-Env: env-value",
	}, "-U", cs.URL, "-r", "1", "-q", "-o", "json", "-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	// Method should be PUT from config (not default GET)
	if req.Method != http.MethodPut {
		t.Errorf("expected method PUT from config, got %s", req.Method)
	}
	// Header from config file should be present
	if v := req.Headers["X-From-Config"]; len(v) == 0 || v[0] != "config-value" {
		t.Errorf("expected X-From-Config from config file, got %v", v)
	}
	// Header from env should be present
	if v := req.Headers["X-From-Env"]; len(v) == 0 || v[0] != "env-value" {
		t.Errorf("expected X-From-Env from env, got %v", v)
	}
}

// --- Config file nesting depth ---

func TestConfigFileNestedMaxDepth(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Create a chain of 12 config files (exceeds max depth of 10)
	// The innermost file has the actual URL config
	// When depth is exceeded, inner files are silently ignored

	files := make([]string, 12)

	// Innermost file (index 11) - has the real config
	files[11] = writeTemp(t, "depth11.yaml", `
url: "`+cs.URL+`"
requests: 1
quiet: true
output: json
headers:
  - X-Depth: deep
`)

	// Chain each file to include the next one
	for i := 10; i >= 0; i-- {
		content := `configFile: "` + files[i+1] + `"`
		files[i] = writeTemp(t, "depth"+string(rune('0'+i))+".yaml", content)
	}

	// The outermost file: this will recurse but max depth will prevent
	// reaching the innermost file with the URL
	res := run("-f", files[0], "-q")
	// This should fail because URL is never reached (too deep)
	assertExitCode(t, res, 1)
}

// --- YAML format flexibility ---

func TestConfigFileParamsMapFormat(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
params:
  key1: value1
  key2: value2
`
	configPath := writeTemp(t, "params_map.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["key1"]; len(v) == 0 || v[0] != "value1" {
		t.Errorf("expected key1=value1, got %v", v)
	}
	if v := req.Query["key2"]; len(v) == 0 || v[0] != "value2" {
		t.Errorf("expected key2=value2, got %v", v)
	}
}

func TestConfigFileHeadersMapFormat(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
headers:
  X-Map-A: map-val-a
  X-Map-B: map-val-b
`
	configPath := writeTemp(t, "headers_map.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Map-A"]; len(v) == 0 || v[0] != "map-val-a" {
		t.Errorf("expected X-Map-A: map-val-a, got %v", v)
	}
	if v := req.Headers["X-Map-B"]; len(v) == 0 || v[0] != "map-val-b" {
		t.Errorf("expected X-Map-B: map-val-b, got %v", v)
	}
}

func TestConfigFileCookiesMapFormat(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
cookies:
  sess: abc
  token: xyz
`
	configPath := writeTemp(t, "cookies_map.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["sess"]; !ok || v != "abc" {
		t.Errorf("expected cookie sess=abc, got %v", req.Cookies)
	}
	if v, ok := req.Cookies["token"]; !ok || v != "xyz" {
		t.Errorf("expected cookie token=xyz, got %v", req.Cookies)
	}
}

func TestConfigFileMultipleBodies(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 10
concurrency: 1
method: POST
quiet: true
output: json
body:
  - "body-one"
  - "body-two"
`
	configPath := writeTemp(t, "multi_body.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	bodies := map[string]bool{}
	for _, req := range cs.allRequests() {
		bodies[req.Body] = true
	}
	if !bodies["body-one"] || !bodies["body-two"] {
		t.Errorf("expected both body-one and body-two to appear, got %v", bodies)
	}
}

func TestConfigFileMultipleMethods(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 10
concurrency: 1
quiet: true
output: json
method:
  - GET
  - POST
`
	configPath := writeTemp(t, "multi_method.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	methods := map[string]bool{}
	for _, req := range cs.allRequests() {
		methods[req.Method] = true
	}
	if !methods["GET"] || !methods["POST"] {
		t.Errorf("expected both GET and POST, got %v", methods)
	}
}
