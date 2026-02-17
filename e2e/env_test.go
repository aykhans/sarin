package e2e

import (
	"net/http"
	"testing"
)

func TestEnvURL(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
	assertResponseCount(t, out, 1)
}

func TestEnvMethod(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_METHOD":   "POST",
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodPost {
		t.Errorf("expected method POST from env, got %s", req.Method)
	}
}

func TestEnvConcurrency(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":         cs.URL,
		"SARIN_REQUESTS":    "6",
		"SARIN_CONCURRENCY": "3",
		"SARIN_QUIET":       "true",
		"SARIN_OUTPUT":      "json",
	})
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 6)
}

func TestEnvDuration(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_DURATION": "1s",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	count, _ := out.Total.Count.Int64()
	if count < 1 {
		t.Errorf("expected at least 1 request during 1s, got %d", count)
	}
}

func TestEnvTimeout(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_TIMEOUT":  "5s",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)
	assertResponseCount(t, res.jsonOutput(t), 1)
}

func TestEnvHeader(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_HEADER":   "X-From-Env: env-value",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-From-Env"]; len(v) == 0 || v[0] != "env-value" {
		t.Errorf("expected X-From-Env: env-value, got %v", v)
	}
}

func TestEnvParam(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_PARAM":    "env_key=env_val",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["env_key"]; len(v) == 0 || v[0] != "env_val" {
		t.Errorf("expected env_key=env_val, got %v", v)
	}
}

func TestEnvCookie(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_COOKIE":   "env_session=env_abc",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["env_session"]; !ok || v != "env_abc" {
		t.Errorf("expected cookie env_session=env_abc, got %v", req.Cookies)
	}
}

func TestEnvBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_METHOD":   "POST",
		"SARIN_REQUESTS": "1",
		"SARIN_BODY":     "env-body-content",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "env-body-content" {
		t.Errorf("expected body 'env-body-content', got %q", req.Body)
	}
}

func TestEnvDryRun(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "3",
		"SARIN_DRY_RUN":  "true",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "dry-run")
	assertResponseCount(t, out, 3)
}

func TestEnvInsecure(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_INSECURE": "true",
		"SARIN_DRY_RUN":  "true",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)
}

func TestEnvOutputNone(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "none",
	})
	assertExitCode(t, res, 0)

	if res.Stdout != "" {
		t.Errorf("expected empty stdout with output=none, got: %s", res.Stdout)
	}
}

func TestEnvConfigFile(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	config := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
headers:
  - X-From-Env-Config: yes
`
	configPath := writeTemp(t, "env_config.yaml", config)

	res := runWithEnv(map[string]string{
		"SARIN_CONFIG_FILE": configPath,
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-From-Env-Config"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-From-Env-Config: yes, got %v", v)
	}
}

func TestEnvCLIOverridesEnv(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// CLI should take priority over env vars
	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://should-be-overridden.invalid",
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	}, "-U", cs.URL)
	assertExitCode(t, res, 0)

	if cs.requestCount() != 1 {
		t.Errorf("expected CLI URL to override env, but server got %d requests", cs.requestCount())
	}
}

func TestEnvInvalidBool(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "not-a-bool",
	})
	assertExitCode(t, res, 1)
}

func TestEnvLuaScript(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.headers["X-Env-Lua"] = {"yes"} return req end`

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
		"SARIN_LUA":      script,
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Env-Lua"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Env-Lua: yes, got %v", v)
	}
}

func TestEnvJsScript(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.headers["X-Env-Js"] = ["yes"]; return req; }`

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
		"SARIN_JS":       script,
	})
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Env-Js"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Env-Js: yes, got %v", v)
	}
}

func TestEnvValues(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      cs.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
		"SARIN_VALUES":   "MY_KEY=my_val",
	}, "-H", "X-Val: {{ .Values.MY_KEY }}")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Val"]; len(v) == 0 || v[0] != "my_val" {
		t.Errorf("expected X-Val: my_val, got %v", v)
	}
}
