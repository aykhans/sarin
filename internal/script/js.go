package script

import (
	"errors"
	"strconv"

	"github.com/dop251/goja"
	"go.aykhans.me/sarin/internal/types"
)

// JsEngine implements the Engine interface using goja (JavaScript).
type JsEngine struct {
	runtime   *goja.Runtime
	transform goja.Callable
	jsonParse goja.Callable
	// errorConstructor is the built-in Error, captured before the script can replace it.
	errorConstructor goja.Value
	http             httpBridge
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
	engine := &JsEngine{runtime: vm}

	// Register the globals before running the script so its functions can use them
	if err := engine.registerHTTP(); err != nil {
		return nil, types.NewScriptExecutionError("JavaScript", err)
	}

	// Execute the script to define the transform function
	_, err := vm.RunString(scriptContent)
	if err != nil {
		return nil, types.NewScriptExecutionError("JavaScript", engine.exceptionError(err))
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

	engine.transform = transform

	return engine, nil
}

// SetHTTPDoer sets the doer that the script's http.* calls are sent through.
func (e *JsEngine) SetHTTPDoer(doer HTTPDoer) {
	e.http.doer = doer
}

// Transform executes the JavaScript transform function with the given request data.
// It can return the following errors:
//   - types.ScriptExecutionError
func (e *JsEngine) Transform(req *RequestData) error {
	e.http.active = true
	defer func() { e.http.active = false }()

	// Convert RequestData to JavaScript object
	reqObj := e.requestDataToObject(req)

	// Call transform(req)
	result, err := e.transform(goja.Undefined(), reqObj)
	if err != nil {
		return types.NewScriptExecutionError("JavaScript", e.exceptionError(err))
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
	e.jsonParse = nil
	e.errorConstructor = nil
}

// requestDataToObject converts RequestData to a goja Value (JavaScript object).
func (e *JsEngine) requestDataToObject(req *RequestData) goja.Value {
	obj := e.runtime.NewObject()

	_ = obj.Set("method", req.Method)
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

		// Check if it's an array
		if arr, ok := v.(*goja.Object); ok && arr.ClassName() == "Array" {
			length := int(arr.Get("length").ToInteger())
			values := make([]string, 0, length)
			for i := range length {
				if text, ok := jsStringValue(arr.Get(strconv.Itoa(i))); ok {
					values = append(values, text)
				}
			}
			result[key] = values
			continue
		}

		// Single value, wrap it in a slice
		if text, ok := jsStringValue(v); ok {
			result[key] = []string{text}
		}
	}
	return result
}

// exceptionError keeps the thrown value's message without goja's stack frame.
// Stringifying runs through Runtime.Try because a thrown value with no usable
// toString makes goja panic outside its own execution context.
func (e *JsEngine) exceptionError(err error) error {
	var exception *goja.Exception
	if !errors.As(err, &exception) {
		return err
	}

	var message string
	if thrown := e.runtime.Try(func() { message = exception.Value().String() }); thrown != nil {
		message = "unprintable thrown value"
	}
	return errors.New(message)
}

// jsStringValue renders a JavaScript primitive the way the script would see it.
// Objects, arrays and functions have no useful text form, so they are skipped.
func jsStringValue(v goja.Value) (string, bool) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "", false
	}
	if _, isObject := v.(*goja.Object); isObject {
		return "", false
	}
	return v.String(), true
}
