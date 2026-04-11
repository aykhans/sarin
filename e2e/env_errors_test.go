package e2e

import (
	"testing"
)

func TestEnvInvalidConcurrency(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":         "http://example.com",
		"SARIN_REQUESTS":    "1",
		"SARIN_CONCURRENCY": "not-a-number",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value for unsigned integer")
}

func TestEnvInvalidRequests(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "abc",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value for unsigned integer")
}

func TestEnvInvalidDuration(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_DURATION": "not-a-duration",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value duration")
}

func TestEnvInvalidTimeout(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_TIMEOUT":  "xyz",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value duration")
}

func TestEnvInvalidInsecure(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_INSECURE": "maybe",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value for boolean")
}

func TestEnvInvalidDryRun(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_DRY_RUN":  "yes",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value for boolean")
}

func TestEnvInvalidShowConfig(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":         "http://example.com",
		"SARIN_REQUESTS":    "1",
		"SARIN_SHOW_CONFIG": "nope",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "invalid value for boolean")
}
