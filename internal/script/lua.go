package script

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// LuaEngine implements the Engine interface using gopher-lua.
type LuaEngine struct {
	state     *lua.LState
	transform *lua.LFunction
}

// NewLuaEngine creates a new Lua script engine with the given script content.
// The script must define a global `transform` function that takes a request table
// and returns the modified request table.
//
// Example Lua script:
//
//	function transform(req)
//	    req.headers["X-Custom"] = "value"
//	    return req
//	end
func NewLuaEngine(scriptContent string) (*LuaEngine, error) {
	L := lua.NewState()

	// Execute the script to define the transform function
	if err := L.DoString(scriptContent); err != nil {
		L.Close()
		return nil, fmt.Errorf("failed to execute Lua script: %w", err)
	}

	// Get the transform function
	transform := L.GetGlobal("transform")
	if transform.Type() != lua.LTFunction {
		L.Close()
		return nil, errors.New("script must define a global 'transform' function")
	}

	return &LuaEngine{
		state:     L,
		transform: transform.(*lua.LFunction),
	}, nil
}

// Transform executes the Lua transform function with the given request data.
func (e *LuaEngine) Transform(req *RequestData) error {
	// Convert RequestData to Lua table
	reqTable := e.requestDataToTable(req)

	// Call transform(req)
	e.state.Push(e.transform)
	e.state.Push(reqTable)
	if err := e.state.PCall(1, 1, nil); err != nil {
		return fmt.Errorf("lua transform error: %w", err)
	}

	// Get the result
	result := e.state.Get(-1)
	e.state.Pop(1)

	if result.Type() != lua.LTTable {
		return fmt.Errorf("transform function must return a table, got %s", result.Type())
	}

	// Update RequestData from the returned table
	e.tableToRequestData(result.(*lua.LTable), req)

	return nil
}

// Close releases the Lua state resources.
func (e *LuaEngine) Close() {
	if e.state != nil {
		e.state.Close()
	}
}

// requestDataToTable converts RequestData to a Lua table.
func (e *LuaEngine) requestDataToTable(req *RequestData) *lua.LTable {
	L := e.state
	t := L.NewTable()

	t.RawSetString("method", lua.LString(req.Method))
	t.RawSetString("url", lua.LString(req.URL))
	t.RawSetString("path", lua.LString(req.Path))
	t.RawSetString("body", lua.LString(req.Body))

	// Headers (map[string][]string -> table of arrays)
	headers := L.NewTable()
	for k, values := range req.Headers {
		arr := L.NewTable()
		for _, v := range values {
			arr.Append(lua.LString(v))
		}
		headers.RawSetString(k, arr)
	}
	t.RawSetString("headers", headers)

	// Params (map[string][]string -> table of arrays)
	params := L.NewTable()
	for k, values := range req.Params {
		arr := L.NewTable()
		for _, v := range values {
			arr.Append(lua.LString(v))
		}
		params.RawSetString(k, arr)
	}
	t.RawSetString("params", params)

	// Cookies (map[string][]string -> table of arrays)
	cookies := L.NewTable()
	for k, values := range req.Cookies {
		arr := L.NewTable()
		for _, v := range values {
			arr.Append(lua.LString(v))
		}
		cookies.RawSetString(k, arr)
	}
	t.RawSetString("cookies", cookies)

	return t
}

// tableToRequestData updates RequestData from a Lua table.
func (e *LuaEngine) tableToRequestData(t *lua.LTable, req *RequestData) {
	// Method
	if v := t.RawGetString("method"); v.Type() == lua.LTString {
		req.Method = string(v.(lua.LString))
	}

	// URL
	if v := t.RawGetString("url"); v.Type() == lua.LTString {
		req.URL = string(v.(lua.LString))
	}

	// Path
	if v := t.RawGetString("path"); v.Type() == lua.LTString {
		req.Path = string(v.(lua.LString))
	}

	// Body
	if v := t.RawGetString("body"); v.Type() == lua.LTString {
		req.Body = string(v.(lua.LString))
	}

	// Headers
	if v := t.RawGetString("headers"); v.Type() == lua.LTTable {
		req.Headers = e.tableToStringSliceMap(v.(*lua.LTable))
	}

	// Params
	if v := t.RawGetString("params"); v.Type() == lua.LTTable {
		req.Params = e.tableToStringSliceMap(v.(*lua.LTable))
	}

	// Cookies
	if v := t.RawGetString("cookies"); v.Type() == lua.LTTable {
		req.Cookies = e.tableToStringSliceMap(v.(*lua.LTable))
	}
}

// tableToStringSliceMap converts a Lua table to a Go map[string][]string.
// Supports both single string values and array values.
func (e *LuaEngine) tableToStringSliceMap(t *lua.LTable) map[string][]string {
	result := make(map[string][]string)
	t.ForEach(func(k, v lua.LValue) {
		if k.Type() != lua.LTString {
			return
		}
		key := string(k.(lua.LString))

		switch v.Type() {
		case lua.LTString:
			// Single string value
			result[key] = []string{string(v.(lua.LString))}
		case lua.LTTable:
			// Array of strings
			var values []string
			v.(*lua.LTable).ForEach(func(_, item lua.LValue) {
				if item.Type() == lua.LTString {
					values = append(values, string(item.(lua.LString)))
				}
			})
			result[key] = values
		}
	})
	return result
}
