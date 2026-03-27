package e2e

import (
	"net/http"
	"testing"
)

func TestLuaScriptInline(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.headers["X-Lua"] = {"from-lua"} return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Headers["X-Lua"]; !ok || len(v) == 0 || v[0] != "from-lua" {
		t.Errorf("expected X-Lua: from-lua, got headers: %v", req.Headers)
	}
}

func TestJsScriptInline(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.headers["X-Js"] = ["from-js"]; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-js", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Headers["X-Js"]; !ok || len(v) == 0 || v[0] != "from-js" {
		t.Errorf("expected X-Js: from-js, got headers: %v", req.Headers)
	}
}

func TestLuaScriptFromFile(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	scriptContent := `function transform(req)
    req.headers["X-From-File"] = {"yes"}
    return req
end`
	scriptPath := writeTemp(t, "test.lua", scriptContent)

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", "@"+scriptPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Headers["X-From-File"]; !ok || len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-From-File: yes, got headers: %v", req.Headers)
	}
}

func TestJsScriptFromFile(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	scriptContent := `function transform(req) {
    req.headers["X-From-File"] = ["yes"];
    return req;
}`
	scriptPath := writeTemp(t, "test.js", scriptContent)

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-js", "@"+scriptPath)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Headers["X-From-File"]; !ok || len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-From-File: yes, got headers: %v", req.Headers)
	}
}

func TestLuaScriptModifiesMethod(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.method = "PUT" return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodPut {
		t.Errorf("expected method PUT after Lua transform, got %s", req.Method)
	}
}

func TestJsScriptModifiesMethod(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.method = "DELETE"; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-js", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Method != http.MethodDelete {
		t.Errorf("expected method DELETE after JS transform, got %s", req.Method)
	}
}

func TestLuaScriptModifiesPath(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.path = "/modified" return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Path != "/modified" {
		t.Errorf("expected path /modified, got %s", req.Path)
	}
}

func TestLuaScriptModifiesBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.body = "lua-body" return req end`

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "lua-body" {
		t.Errorf("expected body 'lua-body', got %q", req.Body)
	}
}

func TestJsScriptModifiesBody(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.body = "js-body"; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-M", "POST", "-q", "-o", "json",
		"-js", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if req.Body != "js-body" {
		t.Errorf("expected body 'js-body', got %q", req.Body)
	}
}

func TestLuaScriptModifiesParams(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.params["lua_param"] = {"lua_value"} return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Query["lua_param"]; !ok || len(v) == 0 || v[0] != "lua_value" {
		t.Errorf("expected lua_param=lua_value, got query: %v", req.Query)
	}
}

func TestJsScriptModifiesParams(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.params["js_param"] = ["js_value"]; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-js", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Query["js_param"]; !ok || len(v) == 0 || v[0] != "js_value" {
		t.Errorf("expected js_param=js_value, got query: %v", req.Query)
	}
}

func TestLuaScriptModifiesCookies(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.cookies["lua_cookie"] = {"lua_val"} return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["lua_cookie"]; !ok || v != "lua_val" {
		t.Errorf("expected cookie lua_cookie=lua_val, got cookies: %v", req.Cookies)
	}
}

func TestJsScriptModifiesCookies(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) { req.cookies["js_cookie"] = ["js_val"]; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-js", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Cookies["js_cookie"]; !ok || v != "js_val" {
		t.Errorf("expected cookie js_cookie=js_val, got cookies: %v", req.Cookies)
	}
}

func TestScriptChainLuaThenJs(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	luaScript := `function transform(req) req.headers["X-Step"] = {"lua"} return req end`
	jsScript := `function transform(req) { req.headers["X-Js-Step"] = ["js"]; return req; }`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", luaScript,
		"-js", jsScript)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v, ok := req.Headers["X-Step"]; !ok || len(v) == 0 || v[0] != "lua" {
		t.Errorf("expected X-Step: lua from Lua script, got %v", req.Headers["X-Step"])
	}
	if v, ok := req.Headers["X-Js-Step"]; !ok || len(v) == 0 || v[0] != "js" {
		t.Errorf("expected X-Js-Step: js from JS script, got %v", req.Headers["X-Js-Step"])
	}
}

func TestMultipleLuaScriptsChained(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	lua1 := `function transform(req) req.headers["X-First"] = {"1"} return req end`
	lua2 := `function transform(req) req.headers["X-Second"] = {"2"} return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", lua1,
		"-lua", lua2)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-First"]; len(v) == 0 || v[0] != "1" {
		t.Errorf("expected X-First: 1, got %v", v)
	}
	if v := req.Headers["X-Second"]; len(v) == 0 || v[0] != "2" {
		t.Errorf("expected X-Second: 2, got %v", v)
	}
}

func TestScriptWithEscapedAt(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// @@ means the first @ is stripped, rest is treated as inline script
	script := `@@function transform(req) req.headers["X-At"] = {"escaped"} return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	// The @@ prefix strips one @, leaving "@function transform..." which is valid Lua?
	// Actually no — after stripping the first @, it becomes:
	// "@function transform(req) ..." which would be interpreted as a file reference.
	// Wait — the code says: strings starting with "@@" → content = source[1:] = "@function..."
	// Then it's returned as inline content "@function transform..."
	// Lua would fail because "@" is not valid Lua syntax.
	// So this test just validates that the @@ mechanism doesn't crash.
	// It should fail at the validation step since "@function..." is not valid Lua.
	assertExitCode(t, res, 1)
}

func TestLuaScriptMultipleHeaderValues(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	script := `function transform(req) req.headers["X-Multi"] = {"val1", "val2"} return req end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	vals, ok := req.Headers["X-Multi"]
	if !ok {
		t.Fatalf("expected X-Multi header, got headers: %v", req.Headers)
	}
	if len(vals) != 2 || vals[0] != "val1" || vals[1] != "val2" {
		t.Errorf("expected X-Multi: [val1, val2], got %v", vals)
	}
}

func TestJsScriptCanReadExistingHeaders(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Set a header via CLI, then read it in JS and set a new one based on it
	script := `function transform(req) {
		var original = req.headers["X-Original"];
		if (original && original.length > 0) {
			req.headers["X-Copy"] = [original[0]];
		}
		return req;
	}`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-H", "X-Original: hello",
		"-js", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Copy"]; len(v) == 0 || v[0] != "hello" {
		t.Errorf("expected X-Copy: hello (copied from X-Original), got %v", v)
	}
}

func TestLuaScriptCanReadExistingParams(t *testing.T) {
	t.Parallel()
	cs := newCaptureServer()
	defer cs.Close()

	// Set a param via CLI, then read it in Lua
	script := `function transform(req)
		local original = req.params["key1"]
		if original and #original > 0 then
			req.params["key1_copy"] = {original[1]}
		end
		return req
	end`

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-P", "key1=val1",
		"-lua", script)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Query["key1_copy"]; len(v) == 0 || v[0] != "val1" {
		t.Errorf("expected key1_copy=val1 (copied from key1), got %v", v)
	}
}

func TestScriptFromHTTPURL(t *testing.T) {
	t.Parallel()

	// Serve a Lua script via HTTP
	scriptContent := `function transform(req) req.headers["X-Remote"] = {"yes"} return req end`
	scriptServer := statusServerWithBody(scriptContent)
	defer scriptServer.Close()

	cs := newCaptureServer()
	defer cs.Close()

	res := run("-U", cs.URL, "-r", "1", "-q", "-o", "json",
		"-lua", "@"+scriptServer.URL)
	assertExitCode(t, res, 0)

	req := cs.lastRequest()
	if v := req.Headers["X-Remote"]; len(v) == 0 || v[0] != "yes" {
		t.Errorf("expected X-Remote: yes from remote script, got %v", req.Headers)
	}
}
