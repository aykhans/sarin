package script

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"

	lua "github.com/yuin/gopher-lua"
	"go.aykhans.me/sarin/internal/types"
)

// registerLuaJSON defines the global json table with encode and decode.
func registerLuaJSON(state *lua.LState) {
	module := state.NewTable()
	module.RawSetString("encode", state.NewFunction(luaJSONEncode))
	module.RawSetString("decode", state.NewFunction(luaJSONDecode))
	state.SetGlobal("json", module)
}

func luaJSONEncode(state *lua.LState) int {
	value, err := luaToGo(state.CheckAny(1), make(map[*lua.LTable]bool))
	if err != nil {
		state.RaiseError("%s", types.NewScriptJSONError("encode", err).Error())
	}

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		state.RaiseError("%s", types.NewScriptJSONError("encode", err).Error())
	}

	state.Push(lua.LString(strings.TrimSuffix(buf.String(), "\n")))
	return 1
}

func luaJSONDecode(state *lua.LState) int {
	var value any
	if err := json.Unmarshal([]byte(state.CheckString(1)), &value); err != nil {
		state.RaiseError("%s", types.NewScriptJSONError("decode", err).Error())
	}

	state.Push(goToLua(state, value))
	return 1
}

// luaToGo converts a Lua value to a JSON-encodable Go value.
// It can return the following errors:
//   - types.ErrScriptJSONCycle
//   - types.ScriptTypeError
func luaToGo(value lua.LValue, visiting map[*lua.LTable]bool) (any, error) {
	switch v := value.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LBool:
		return bool(v), nil
	case lua.LNumber:
		return float64(v), nil
	case lua.LString:
		return string(v), nil
	case *lua.LTable:
		if visiting[v] {
			return nil, types.ErrScriptJSONCycle
		}
		visiting[v] = true
		defer delete(visiting, v)

		if length, ok := luaArrayLength(v); ok {
			array := make([]any, length)
			for i := range length {
				item, err := luaToGo(v.RawGetInt(i+1), visiting)
				if err != nil {
					return nil, err
				}
				array[i] = item
			}
			return array, nil
		}

		object := make(map[string]any)
		var err error
		v.ForEach(func(key, item lua.LValue) {
			if err != nil {
				return
			}
			switch key.(type) {
			case lua.LString, lua.LNumber:
			default:
				err = types.NewScriptTypeError("string or number object key", key.Type().String())
				return
			}
			object[key.String()], err = luaToGo(item, visiting)
		})
		if err != nil {
			return nil, err
		}
		return object, nil
	default:
		return nil, types.NewScriptTypeError("nil, boolean, number, string or table", value.Type().String())
	}
}

// luaArrayLength reports whether t is a non-empty sequence with keys exactly 1..n.
func luaArrayLength(t *lua.LTable) (int, bool) {
	count := 0
	isArray := true
	maxKey := 0.0
	t.ForEach(func(key, _ lua.LValue) {
		count++
		number, ok := key.(lua.LNumber)
		if !ok || float64(number) < 1 || float64(number) != math.Trunc(float64(number)) {
			isArray = false
			return
		}
		maxKey = max(maxKey, float64(number))
	})
	if !isArray || count == 0 || maxKey != float64(count) {
		return 0, false
	}
	return count, true
}

// goToLua converts a value produced by encoding/json to a Lua value.
func goToLua(state *lua.LState, value any) lua.LValue {
	switch v := value.(type) {
	case bool:
		return lua.LBool(v)
	case float64:
		return lua.LNumber(v)
	case string:
		return lua.LString(v)
	case []any:
		t := state.CreateTable(len(v), 0)
		for i, item := range v {
			t.RawSetInt(i+1, goToLua(state, item))
		}
		return t
	case map[string]any:
		t := state.CreateTable(0, len(v))
		for key, item := range v {
			t.RawSetString(key, goToLua(state, item))
		}
		return t
	default:
		return lua.LNil
	}
}
