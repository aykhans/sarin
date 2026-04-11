package e2e

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSWithInsecureFlag(t *testing.T) {
	t.Parallel()

	// Create a TLS server with a self-signed cert
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Without --insecure, it should fail (cert not trusted)
	// With --insecure, it should succeed
	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "json", "-I")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
}

func TestHTTPSWithoutInsecureFails(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Without --insecure, should get a TLS error (not a clean 200)
	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "json")
	assertExitCode(t, res, 0) // Process still exits 0, but response key is an error

	out := res.jsonOutput(t)
	// Should NOT have a "200" key — should have a TLS error
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected TLS error without --insecure, but got 200")
	}
}

func TestHTTPSInsecureViaCLILongFlag(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use the long form flag
	res := run("-U", srv.URL, "-r", "1", "-q", "-o", "json", "-insecure")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
}

func TestHTTPSInsecureViaConfigFile(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	config := `
url: "` + srv.URL + `"
requests: 1
insecure: true
quiet: true
output: json
`
	configPath := writeTemp(t, "tls_config.yaml", config)

	res := run("-f", configPath)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
}

func TestHTTPSInsecureViaEnv(t *testing.T) {
	t.Parallel()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := runWithEnv(map[string]string{
		"SARIN_URL":      srv.URL,
		"SARIN_REQUESTS": "1",
		"SARIN_INSECURE": "true",
		"SARIN_QUIET":    "true",
		"SARIN_OUTPUT":   "json",
	})
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
}

func TestHTTPSEchoServer(t *testing.T) {
	t.Parallel()

	// TLS echo server that returns request details
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"method": r.Method,
			"path":   r.URL.Path,
			"tls":    r.TLS != nil,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Verify request was received over TLS
	res := run("-U", srv.URL+"/secure-path", "-r", "1", "-q", "-o", "json", "-I")
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	assertHasResponseKey(t, out, "200")
}

// tlsCaptureServer is like captureServer but with TLS
func tlsCaptureServer() *captureServer {
	cs := &captureServer{}
	cs.Server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.mu.Lock()
		cs.requests = append(cs.requests, echoResponse{
			Method: r.Method,
			Path:   r.URL.Path,
		})
		cs.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	cs.TLS = &tls.Config{}
	cs.StartTLS()
	return cs
}

func TestHTTPSHeadersSentCorrectly(t *testing.T) {
	t.Parallel()
	cs := tlsCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL+"/api/test", "-r", "1", "-M", "POST", "-q", "-o", "json", "-I")
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodPost {
		t.Errorf("expected POST over HTTPS, got %s", req.Method)
	}
	if req.Path != "/api/test" {
		t.Errorf("expected path /api/test over HTTPS, got %s", req.Path)
	}
}
