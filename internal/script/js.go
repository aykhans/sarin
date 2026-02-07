package script

import (
	"errors"

	"github.com/dop251/goja"
	"go.aykhans.me/sarin/internal/types"
)

// JsEngine implements the Engine interface using goja (JavaScript).
type JsEngine struct {
	runtime   *goja.Runtime
	transform goja.Callable
}

// NewJsEngine creates a new JavaScript script engine with the given script content.
// The script must define a global `transform` function that takes a request object
// and returns the modified request object.
//
// Example JavaScript script:
//
//	function transform(req) {
//	    req.headers["X-Custom"] = ["value"];
//	    return req;
//	}
//
// It can return the following errors:
//   - types.ErrScriptTransformMissing
//   - types.ScriptExecutionError
func NewJsEngine(scriptContent string) (*JsEngine, error) {
	vm := goja.New()

	// Execute the script to define the transform function
	_, err := vm.RunString(scriptContent)
	if err != nil {
		return nil, types.NewScriptExecutionError("JavaScript", err)
	}

	// Get the transform function
	transformVal := vm.Get("transform")
	if transformVal == nil || goja.IsUndefined(transformVal) || goja.IsNull(transformVal) {
		return nil, types.ErrScriptTransformMissing
	}

	transform, ok := goja.AssertFunction(transformVal)
	if !ok {
		return nil, types.NewScriptExecutionError("JavaScript", errors.New("'transform' must be a function"))
	}

	return &JsEngine{
		runtime:   vm,
		transform: transform,
	}, nil
}

// Transform executes the JavaScript transform function with the given request data.
// It can return the following errors:
//   - types.ScriptExecutionError
func (e *JsEngine) Transform(req *RequestData) error {
	// Convert RequestData to JavaScript object
	reqObj := e.requestDataToObject(req)

	// Call transform(req)
	result, err := e.transform(goja.Undefined(), reqObj)
	if err != nil {
		return types.NewScriptExecutionError("JavaScript", err)
	}

	// Update RequestData from the returned object
	if err := e.objectToRequestData(result, req); err != nil {
		return types.NewScriptExecutionError("JavaScript", err)
	}

	return nil
}

// Close releases the JavaScript runtime resources.
func (e *JsEngine) Close() {
	// goja doesn't have an explicit close method, but we can help GC
	e.runtime = nil
	e.transform = nil
}

// requestDataToObject converts RequestData to a goja Value (JavaScript object).
func (e *JsEngine) requestDataToObject(req *RequestData) goja.Value {
	obj := e.runtime.NewObject()

	_ = obj.Set("method", req.Method)
	_ = obj.Set("url", req.URL)
	_ = obj.Set("path", req.Path)
	_ = obj.Set("body", req.Body)

	// Headers (map[string][]string -> object of arrays)
	headers := e.runtime.NewObject()
	for k, values := range req.Headers {
		_ = headers.Set(k, e.stringSliceToArray(values))
	}
	_ = obj.Set("headers", headers)

	// Params (map[string][]string -> object of arrays)
	params := e.runtime.NewObject()
	for k, values := range req.Params {
		_ = params.Set(k, e.stringSliceToArray(values))
	}
	_ = obj.Set("params", params)

	// Cookies (map[string][]string -> object of arrays)
	cookies := e.runtime.NewObject()
	for k, values := range req.Cookies {
		_ = cookies.Set(k, e.stringSliceToArray(values))
	}
	_ = obj.Set("cookies", cookies)

	return obj
}

// objectToRequestData updates RequestData from a JavaScript object.
func (e *JsEngine) objectToRequestData(val goja.Value, req *RequestData) error {
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return types.ErrScriptTransformReturnObject
	}

	obj := val.ToObject(e.runtime)
	if obj == nil {
		return types.ErrScriptTransformReturnObject
	}

	// Method
	if v := obj.Get("method"); v != nil && !goja.IsUndefined(v) {
		req.Method = v.String()
	}

	// URL
	if v := obj.Get("url"); v != nil && !goja.IsUndefined(v) {
		req.URL = v.String()
	}

	// Path
	if v := obj.Get("path"); v != nil && !goja.IsUndefined(v) {
		req.Path = v.String()
	}

	// Body
	if v := obj.Get("body"); v != nil && !goja.IsUndefined(v) {
		req.Body = v.String()
	}

	// Headers
	if v := obj.Get("headers"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		req.Headers = e.objectToStringSliceMap(v.ToObject(e.runtime))
	}

	// Params
	if v := obj.Get("params"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		req.Params = e.objectToStringSliceMap(v.ToObject(e.runtime))
	}

	// Cookies
	if v := obj.Get("cookies"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		req.Cookies = e.objectToStringSliceMap(v.ToObject(e.runtime))
	}

	return nil
}

// stringSliceToArray converts a Go []string to a JavaScript array.
func (e *JsEngine) stringSliceToArray(values []string) *goja.Object {
	ifaces := make([]any, len(values))
	for i, v := range values {
		ifaces[i] = v
	}
	return e.runtime.NewArray(ifaces...)
}

// objectToStringSliceMap converts a JavaScript object to a Go map[string][]string.
// Supports both single string values and array values.
func (e *JsEngine) objectToStringSliceMap(obj *goja.Object) map[string][]string {
	if obj == nil {
		return make(map[string][]string)
	}

	result := make(map[string][]string)
	for _, key := range obj.Keys() {
		v := obj.Get(key)
		if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
			continue
		}

		// Check if it's an array
		if arr, ok := v.Export().([]any); ok {
			var values []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					values = append(values, s)
				}
			}
			result[key] = values
		} else {
			// Single value - wrap in slice
			result[key] = []string{v.String()}
		}
	}
	return result
}
