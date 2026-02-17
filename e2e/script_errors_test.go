package e2e

import (
	"testing"
)

func TestJsScriptModifiesPath(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.path = "/js-modified"; return req; }`
	scriptPath := writeTemp(t, "modify_path.js", script)

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json", "-js", "@"+scriptPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Path != "/js-modified" {
		t.Errorf("expected path /js-modified from JS script, got %s", req.Path)
	}
}

func TestJsScriptRuntimeError(t *testing.T) {
	t.Parallel()

	// This script throws an error at runtime
	script := `function transform(req) { throw new Error("runtime boom"); }`

	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json", "-js", script)
	assertExitCode(t, res, 0)

	// The request should fail with a script error, not a 200
	out := res.jsonOutput(t)
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected script runtime error, but got 200")
	}
}

func TestLuaScriptRuntimeError(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Script that will error at runtime
	script := `function transform(req) error("lua runtime boom") end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json", "-lua", script)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected script runtime error, but got 200")
	}
}

func TestJsScriptReturnsNull(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// transform returns null instead of object
	script := `function transform(req) { return null; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json", "-js", script)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected error for null return, but got 200")
	}
}

func TestJsScriptReturnsUndefined(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// transform returns nothing (undefined)
	script := `function transform(req) { }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json", "-js", script)
	assertExitCode(t, res, 0)

	out := res.jsonOutput(t)
	if _, ok := out.Responses["200"]; ok {
		t.Error("expected error for undefined return, but got 200")
	}
}

func TestScriptFromNonexistentFile(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-q", "-o", "json",
		"-lua", "@/nonexistent/path/script.lua")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
	assertContains(t, res.Stderr, "failed to load script")
}

func TestScriptFromNonexistentURL(t *testing.T) {
	t.Parallel()

	res := run("-U", "http://example.com", "-r", "1", "-q", "-o", "json",
		"-js", "@http://127.0.0.1:1/nonexistent.js")
	assertExitCode(t, res, 1)
	assertContains(t, res.Stderr, "VALIDATION")
	assertContains(t, res.Stderr, "failed to load script")
}

func TestMultipleLuaAndJsScripts(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	lua1 := `function transform(req) req.headers["X-Lua-1"] = {"yes"} return req end`
	lua2 := `function transform(req) req.headers["X-Lua-2"] = {"yes"} return req end`
	js1 := `function transform(req) { req.headers["X-Js-1"] = ["yes"]; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", lua1, "-lua", lua2, "-js", js1)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Lua-1"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Lua-1: yes, got %v", v)
	}
	if v := req.Headers["X-Lua-2"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Lua-2: yes, got %v", v)
	}
	if v := req.Headers["X-Js-1"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Js-1: yes, got %v", v)
	}
}
