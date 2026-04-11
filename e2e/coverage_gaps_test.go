package e2e

import "testing"

func TestValidation_InvalidTemplateInMethod(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-M", "{{ invalid_func }}")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Method[0]")
}

func TestValidation_InvalidTemplateInParamKey(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-P", "{{ invalid_func }}=value")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Param[0].Key")
}

func TestValidation_InvalidTemplateInCookieValue(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-C", "session={{ invalid_func }}")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Cookie[0].Value[0]")
}

func TestValidation_InvalidTemplateInURLPath(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com/{{ invalid_func }}", "-r", "1")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "URL.Path")
}

func TestValidation_InvalidTemplateInValues(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-V", "A={{ invalid_func }}")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Values[0]")
}

func TestValidation_ScriptURLWithoutHost(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-lua", "@http://")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "host")
}

func TestEnvInvalidURL(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "://bad-url",
		"SARIN_REQUESTS": "1",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "SARIN_URL")
}

func TestEnvInvalidProxy(t *testing.T) {
	t.Parallel()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      "http://example.com",
		"SARIN_REQUESTS": "1",
		"SARIN_PROXY":    "://bad-proxy",
	}, "-q")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "SARIN_PROXY")
}

func TestConfigFileInvalidURLParse(t *testing.T) {
	t.Parallel()

	configPath := writeTemp(t, "invalid_url.yaml", `
url: "://bad-url"
requests: 1
`)

	res := run("-f", configPath)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Field 'url'")
}

func TestConfigFileInvalidProxyParse(t *testing.T) {
	t.Parallel()

	configPath := writeTemp(t, "invalid_proxy.yaml", `
url: "http://example.com"
requests: 1
proxy: "://bad-proxy"
`)

	res := run("-f", configPath)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "proxy[0]")
}

func TestConfigFileInvalidHeadersType(t *testing.T) {
	t.Parallel()

	configPath := writeTemp(t, "invalid_headers_type.yaml", `
url: "http://example.com"
requests: 1
headers:
  - X-Test: value
  - 42
`)

	res := run("-f", configPath)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Failed to parse config file")
}
