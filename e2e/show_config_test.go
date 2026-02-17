package e2e

import (
	"testing"
)

func TestShowConfigNonTTY(t *testing.T) {
	t.Parallel()

	// In non-TTY mode (like tests), -s should output raw YAML and exit
	res := run("-U", "http://example.com", "-r", "1", "-s")
	assertExitCode(t, res, 0)

	// Should contain YAML-formatted config
	assertContains(t, res.Stdout, "url:")
	assertContains(t, res.Stdout, "example.com")
	assertContains(t, res.Stdout, "requests:")
}

func TestShowConfigContainsMethod(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-M", "POST", "-s")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "method:")
	assertContains(t, res.Stdout, "POST")
}

func TestShowConfigContainsHeaders(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-s",
		"-H", "X-Custom: test-value")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "X-Custom")
	assertContains(t, res.Stdout, "test-value")
}

func TestShowConfigContainsTimeout(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-T", "5s", "-s")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "timeout:")
}

func TestShowConfigWithEnvVars(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "5",
	}, "-s")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "example.com")
	assertContains(t, res.Stdout, "requests:")
}
