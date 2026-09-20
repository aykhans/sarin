package script

import (
	lua "github.com/yuin/gopher-lua"
	"go.aykhans.me/sarin/internal/types"
)

// registerHTTP defines the global http function.
func (e *LuaEngine) registerHTTP() {
	L := e.state
	L.SetGlobal("http", L.NewFunction(func(L *lua.LState) int {
		return e.httpCall(L, 1)
	}))
}

// httpCall sends the request whose URL is at stack position urlIndex and pushes the response.
func (e *LuaEngine) httpCall(state *lua.LState, urlIndex int) int {
	req := &HTTPRequest{
		Method: httpDefaultMethod,
		URL:    state.CheckString(urlIndex),
	}

	switch opts := state.Get(urlIndex + 1).(type) {
	case *lua.LNilType:
	case *lua.LTable:
		if err := e.parseHTTPOptions(opts, req); err != nil {
			state.RaiseError("%s", err.Error())
		}
	default:
		state.ArgError(urlIndex+1, "options must be a table")
	}

	resp, err := e.http.do(req)
	if err != nil {
		state.RaiseError("%s", err.Error())
	}

	state.Push(httpResponseToTable(state, resp))
	return 1
}

// parseHTTPOptions fills req from a Lua options table.
// It can return the following errors:
//   - types.ScriptHTTPOptionError
func (e *LuaEngine) parseHTTPOptions(opts *lua.LTable, req *HTTPRequest) error {
	var err error
	opts.ForEach(func(k, v lua.LValue) {
		if err != nil {
			return
		}

		key, ok := k.(lua.LString)
		if !ok {
			err = types.NewScriptHTTPOptionError(k.String(), types.ErrScriptHTTPUnknownOption)
			return
		}

		switch name := string(key); name {
		case httpOptionMethod:
			method, ok := v.(lua.LString)
			if !ok {
				err = types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("string", v.Type().String()))
				return
			}
			req.Method = string(method)
		case httpOptionHeaders, httpOptionParams, httpOptionCookies:
			table, ok := v.(*lua.LTable)
			if !ok {
				err = types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("table", v.Type().String()))
				return
			}
			values := e.tableToStringSliceMap(table)
			switch name {
			case httpOptionHeaders:
				req.Headers = values
			case httpOptionParams:
				req.Params = values
			default:
				req.Cookies = values
			}
		case httpOptionBody:
			body, ok := v.(lua.LString)
			if !ok {
				err = types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("string", v.Type().String()))
				return
			}
			req.Body = string(body)
		case httpOptionTimeout:
			timeout, ok := v.(lua.LString)
			if !ok {
				err = types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("duration string", v.Type().String()))
				return
			}
			req.Timeout, err = parseHTTPTimeout(string(timeout))
		case httpOptionInsecure:
			flag, ok := v.(lua.LBool)
			if !ok {
				err = types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("boolean", v.Type().String()))
				return
			}
			req.Insecure = bool(flag)
		case httpOptionMaxRedirects:
			count, ok := v.(lua.LNumber)
			if !ok {
				err = types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("number", v.Type().String()))
				return
			}
			req.MaxRedirects, err = parseHTTPMaxRedirects(float64(count))
		default:
			err = types.NewScriptHTTPOptionError(name, types.ErrScriptHTTPUnknownOption)
		}
	})
	return err
}

// httpResponseToTable converts a response to a Lua table.
func httpResponseToTable(state *lua.LState, resp *HTTPResponse) *lua.LTable {
	t := state.NewTable()
	t.RawSetString("status", lua.LNumber(resp.Status))
	t.RawSetString("headers", stringSliceMapToTable(state, resp.Headers))
	t.RawSetString("body", lua.LString(resp.Body))

	lookup := func(find func(string) (string, bool)) *lua.LFunction {
		return state.NewFunction(func(L *lua.LState) int {
			// Last argument, so both res:header() and res.header() work.
			top := L.GetTop()
			if top == 0 {
				L.ArgError(1, "name expected")
			}
			if value, ok := find(L.CheckString(top)); ok {
				L.Push(lua.LString(value))
			} else {
				L.Push(lua.LNil)
			}
			return 1
		})
	}
	t.RawSetString("header", lookup(resp.Header))
	t.RawSetString("cookie", lookup(resp.Cookie))

	return t
}
