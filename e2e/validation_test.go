package e2e

import (
	"testing"
)

func TestValidation_MissingURL(t *testing.T) {
	t.Parallel()
	res := run("-r", "1")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "URL")
	assertContains(t, res.Stderr, "required")
}

func TestValidation_InvalidURLScheme(t *testing.T) {
	t.Parallel()
	res := run("-U", "ftp://example.com", "-r", "1")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "URL")
	assertContains(t, res.Stderr, "scheme")
}

func TestValidation_URLWithoutHost(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://", "-r", "1")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "URL")
}

func TestValidation_NoRequestsOrDuration(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "request count or duration")
}

func TestValidation_ZeroRequests(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "0")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Requests")
}

func TestValidation_ZeroDuration(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-d", "0s")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Duration")
}

func TestValidation_ZeroRequestsAndZeroDuration(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "0", "-d", "0s")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_ConcurrencyZero(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1", "-c", "0")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "concurrency")
}

func TestValidation_TimeoutZero(t *testing.T) {
	t.Parallel()
	// Timeout of 0 is invalid (must be > 0)
	res := run("-U", "http://example.com", "-r", "1", "-T", "0s")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "timeout")
}

func TestValidation_InvalidOutputFormat(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1", "-o", "xml")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "Output")
}

func TestValidation_InvalidProxyScheme(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1", "-X", "ftp://proxy.example.com:8080")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "proxy")
}

func TestValidation_EmptyLuaScript(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1", "-lua", "")
	assertExitCode(t, res, 1)
}

func TestValidation_EmptyJsScript(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1", "-js", "")
	assertExitCode(t, res, 1)
}

func TestValidation_LuaScriptMissingTransform(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1",
		"-lua", `print("hello")`)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_JsScriptMissingTransform(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1",
		"-js", `console.log("hello")`)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_LuaScriptSyntaxError(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1",
		"-lua", `function transform(req invalid syntax`)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_JsScriptSyntaxError(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1",
		"-js", `function transform(req { invalid`)
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_ScriptEmptyFileRef(t *testing.T) {
	t.Parallel()
	// "@" with nothing after it
	res := run("-U", "http://example.com", "-r", "1", "-lua", "@")
	assertExitCode(t, res, 1)
}

func TestValidation_ScriptNonexistentFile(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1",
		"-lua", "@/nonexistent/path/script.lua")
	assertExitCode(t, res, 1)
}

func TestValidation_InvalidTemplateInHeader(t *testing.T) {
	t.Parallel()
	res := run("-U", "http://example.com", "-r", "1",
		"-H", "X-Test: {{ invalid_func }}")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_InvalidTemplateInBody(t *testing.T) {
	t.Parallel()
	// Use a template with invalid syntax (unclosed action)
	res := run("-U", "http://example.com", "-r", "1",
		"-B", "{{ invalid_func_xyz }}")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
}

func TestValidation_MultipleErrors(t *testing.T) {
	t.Parallel()
	// No URL, no requests/duration — should report multiple validation errors
	res := run("-c", "1")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "URL")
}
