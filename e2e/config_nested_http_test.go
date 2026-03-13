package e2e

import (
	"testing"
)

func TestConfigFileNestedHTTPInclude(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Inner config served via HTTP
	innerConfig := `
headers:
  - X-From-HTTP-Nested: yes
`
	innerServer := statusServerWithBody(innerConfig)
	defer innerServer.Close()

	// Outer config references the inner config via HTTP URL
	outerConfig := `
url: "` + cs.URL + `"
requests: 1
quiet: true
output: json
configFile: "` + innerServer.URL + `"
`
	outerPath := writeTemp(t, "outer_http.yaml", outerConfig)

	res := run("-f", outerPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-From-Http-Nested"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-From-Http-Nested: yes from nested HTTP config, got %v", v)
	}
}
