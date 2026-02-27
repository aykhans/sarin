package e2e

import (
	"testing"
)

// Note: We can't easily test actual proxy connections in E2E tests without
// setting up real proxy servers. These tests verify the validation and
// error handling around proxy configuration.

func TestProxyValidSchemes(t *testing.T) {
	t.Parallel()

	// Valid proxy scheme should not cause a validation error
	// (will fail at connection time since no proxy is running, but should pass validation)
	for _, scheme := range []string{"http", "https", "socks5", "socks5h"} {
		t.Run(scheme, func(t *testing.T) {
			t.Parallel()

			res := run("-U", "http://example.com", "-r", "1", "-z", "-q", "-o", "json",
				"-X", scheme+"://127.0.0.1:9999")
			assertExitCode(t, res, 0)

			out := res.jsonOutput(t)
			assertHasResponseKey(t, out, "dry-run")
		})
	}
}

func TestProxyInvalidScheme(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-q", "-o", "json",
		"-X", "ftp://proxy.example.com:8080")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestMultipleProxiesDryRun(t *testing.T) {
	t.Parallel()

	// Multiple proxies with dry-run to verify they're accepted
	res := run("-U", "http://example.com", "-r", "3", "-z", "-q", "-o", "json",
		"-X", "http://127.0.0.1:8080",
		"-X", "http://127.0.0.1:8081")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertResponseCount(t, out, 3)
}

func TestProxyConnectionFailure(t *testing.T) {
	t.Parallel()

	// Use a proxy that doesn't exist — should get a connection error
	res := run("-U", "http://example.com", "-r", "1", "-q", "-o", "json",
		"-X", "http://127.0.0.1:1")
	// The process should still exit (may exit 0 with error in output or exit 1)
	if res.ExitCode == 0 {
		out := res.jsonOutput(t)
		// Should NOT get a 200 — should have a proxy error
		if _, ok := out.Responses["200"]; ok {
			t.Error("expected proxy connection error, but got 200")
		}
	}
}

func TestProxyFromConfigFile(t *testing.T) {
	t.Parallel()

	config := `
url: "http://example.com"
requests: 1
quiet: true
output: json
dryRun: true
proxy:
  - http://127.0.0.1:8080
`
	configPath := writeTemp(t, "proxy_config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "dry-run")
}

func TestProxyFromEnv(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_DRY_RUN":  "true",
		"SARIN_OUTPUT":   "json",
		"SARIN_PROXY":    "http://127.0.0.1:8080",
	}, "-q")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "dry-run")
}
