package script

import (
	"errors"

	"github.com/dop251/goja"
	"go.aykhans.me/sarin/internal/types"
)

// registerHTTP defines the global http function.
func (e *JsEngine) registerHTTP() error {
	rt := e.runtime

	jsonParse, ok := goja.AssertFunction(rt.Get("JSON").ToObject(rt).Get("parse"))
	if !ok {
		return errors.New("JSON.parse is not available")
	}
	e.jsonParse = jsonParse
	e.errorConstructor = rt.Get("Error")

	return rt.Set("http", func(call goja.FunctionCall) goja.Value {
		return e.httpCall(call.Argument(0), call.Argument(1))
	})
}

func (e *JsEngine) httpCall(urlValue, optsValue goja.Value) goja.Value {
	rt := e.runtime

	url, ok := urlValue.Export().(string)
	if !ok {
		panic(rt.NewTypeError("http: url must be a string, got %s", jsTypeName(urlValue)))
	}
	req := &HTTPRequest{Method: httpDefaultMethod, URL: url}

	if !goja.IsUndefined(optsValue) && !goja.IsNull(optsValue) {
		opts, ok := optsValue.(*goja.Object)
		if !ok {
			panic(rt.NewTypeError("http: options must be an object, got %s", jsTypeName(optsValue)))
		}
		if err := e.parseHTTPOptions(opts, req); err != nil {
			e.throw(err)
		}
	}

	resp, err := e.http.do(req)
	if err != nil {
		e.throw(err)
	}

	return e.httpResponseToObject(resp)
}

// parseHTTPOptions fills req from a JavaScript options object.
// It can return the following errors:
//   - types.ScriptHTTPOptionError
func (e *JsEngine) parseHTTPOptions(opts *goja.Object, req *HTTPRequest) error {
	for _, name := range opts.Keys() {
		value := opts.Get(name)
		if goja.IsUndefined(value) {
			continue
		}

		switch name {
		case httpOptionMethod:
			method, ok := value.Export().(string)
			if !ok {
				return types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("string", jsTypeName(value)))
			}

			req.Method = method
		case httpOptionHeaders, httpOptionParams, httpOptionCookies:
			object, ok := value.(*goja.Object)
			if !ok {
				return types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("object", jsTypeName(value)))
			}

			values := e.objectToStringSliceMap(object)
			switch name {
			case httpOptionHeaders:
				req.Headers = values
			case httpOptionParams:
				req.Params = values
			default:
				req.Cookies = values
			}
		case httpOptionBody:
			body, ok := value.Export().(string)
			if !ok {
				return types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("string", jsTypeName(value)))
			}

			req.Body = body
		case httpOptionTimeout:
			timeout, ok := value.Export().(string)
			if !ok {
				return types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("duration string", jsTypeName(value)))
			}

			var err error
			if req.Timeout, err = parseHTTPTimeout(timeout); err != nil {
				return err
			}
		case httpOptionInsecure:
			flag, ok := value.Export().(bool)
			if !ok {
				return types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("boolean", jsTypeName(value)))
			}

			req.Insecure = flag
		case httpOptionMaxRedirects:
			count, ok := numberValue(value)
			if !ok {
				return types.NewScriptHTTPOptionError(name, types.NewScriptTypeError("number", jsTypeName(value)))
			}

			var err error
			if req.MaxRedirects, err = parseHTTPMaxRedirects(count); err != nil {
				return err
			}
		default:
			return types.NewScriptHTTPOptionError(name, types.ErrScriptHTTPUnknownOption)
		}
	}
	return nil
}

// httpResponseToObject converts a response to a JavaScript object.
func (e *JsEngine) httpResponseToObject(resp *HTTPResponse) *goja.Object {
	rt := e.runtime
	obj := rt.NewObject()

	headers := rt.NewObject()
	for key, values := range resp.Headers {
		_ = headers.Set(key, e.stringSliceToArray(values))
	}

	lookup := func(find func(string) (string, bool)) func(goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			name, ok := call.Argument(0).Export().(string)
			if !ok {
				panic(rt.NewTypeError("name must be a string, got %s", jsTypeName(call.Argument(0))))
			}
			if value, ok := find(name); ok {
				return rt.ToValue(value)
			}
			return goja.Null()
		}
	}

	_ = obj.Set("status", resp.Status)
	_ = obj.Set("headers", headers)
	_ = obj.Set("body", resp.Body)
	_ = obj.Set("header", lookup(resp.Header))
	_ = obj.Set("cookie", lookup(resp.Cookie))
	_ = obj.Set("json", func(goja.FunctionCall) goja.Value {
		value, err := e.jsonParse(goja.Undefined(), rt.ToValue(resp.Body))
		if err != nil {
			panic(err)
		}
		return value
	})

	return obj
}

// jsTypeName returns a short type name for error messages.
func jsTypeName(value goja.Value) string {
	switch {
	case value == nil || goja.IsUndefined(value):
		return "undefined"
	case goja.IsNull(value):
		return "null"
	}
	switch value.Export().(type) {
	case string:
		return "string"
	case bool:
		return "boolean"
	case int64, float64:
		return "number"
	case []any:
		return "array"
	default:
		if _, ok := goja.AssertFunction(value); ok {
			return "function"
		}
		return "object"
	}
}

// throw raises err as a plain JavaScript Error instead of a GoError.
func (e *JsEngine) throw(err error) {
	errorValue, newErr := e.runtime.New(e.errorConstructor, e.runtime.ToValue(err.Error()))
	if newErr != nil {
		panic(e.runtime.NewGoError(err))
	}
	panic(errorValue)
}

// numberValue reports whether value is a JavaScript number, and returns it.
func numberValue(value goja.Value) (float64, bool) {
	switch number := value.Export().(type) {
	case int64:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}
