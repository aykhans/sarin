package e2e

import (
	"testing"
)

func TestShowConfigFromYAML(t *testing.T) {
	t.Parallel()
	config := `
url: "http://example.com"
requests: 1
showConfig: true
`
	configPath := writeTemp(t, "show_config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	// Non-TTY: should output raw YAML config
	assertContains(t, res.Stdout, "url:")
	assertContains(t, res.Stdout, "example.com")
}

func TestShowConfigFromEnv(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":         "http://example.com",
		"SARIN_REQUESTS":    "1",
		"SARIN_SHOW_CONFIG": "true",
	}, "-q")
	assertExitCode(t, res, 0)

	assertContains(t, res.Stdout, "url:")
	assertContains(t, res.Stdout, "example.com")
}
