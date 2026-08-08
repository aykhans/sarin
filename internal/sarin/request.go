package sarin

import (
	"bytes"
	"maps"
	"math/rand/v2"
	"net/url"
	"strings"
	"text/template"

	"github.com/joho/godotenv"
	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
	utilsSlice "go.aykhans.me/utils/slice"
)

type RequestGenerator func(*fasthttp.Request) error

// stringRenderer renders a single-valued request component (method, path, body).
type stringRenderer func(data any) (string, error)

// keyValueApplier receives every rendered key/value pair of a request component.
type keyValueApplier func(key, value string)

// keyValueRenderer renders a multi-valued request component (params, headers,
// cookies), handing each rendered pair to apply. It is nil when the component
// has nothing configured.
type keyValueRenderer func(apply keyValueApplier, data any) error

type valuesData struct {
	Values map[string]string
}

// NewRequestGenerator creates a new RequestGenerator function that generates HTTP requests
// with the specified configuration. The returned RequestGenerator is NOT safe for concurrent
// use by multiple goroutines.
//
// Note: Scripts must be validated before calling this function (e.g., in NewSarin).
// The caller is responsible for managing the scriptTransformer lifecycle.
func NewRequestGenerator(
	methods []string,
	requestURL *url.URL,
	params types.Params,
	headers types.Headers,
	cookies types.Cookies,
	bodies []string,
	values []string,
	fileCache *FileCache,
	scriptTransformer *script.Transformer,
) (RequestGenerator, bool) {
	randSource := NewDefaultRandSource()
	//nolint:gosec // G404: Using non-cryptographic rand for load testing, not security
	localRand := rand.New(randSource)

	// Funcs() is only called if a value actually contains template syntax.
	// The root template is shared across all createTemplateFunc calls so Funcs() is called at most once.
	var templateRoot *templateSet
	lazyTemplateRoot := func() *templateSet {
		if templateRoot == nil {
			funcs := NewDefaultTemplateFuncMap(randSource, fileCache)
			templateRoot = &templateSet{root: template.New("").Funcs(funcs), funcs: funcs}
		}
		return templateRoot
	}

	pathRenderer, isPathDynamic := createTemplateFunc(requestURL.Path, lazyTemplateRoot)
	methodRenderer, isMethodDynamic := newStringRenderer(localRand, methods, lazyTemplateRoot)
	paramsRenderer, isParamsDynamic := newKeyValueRenderer(localRand, params, lazyTemplateRoot)
	headersRenderer, isHeadersDynamic := newKeyValueRenderer(localRand, headers, lazyTemplateRoot)
	cookiesRenderer, isCookiesDynamic := newKeyValueRenderer(localRand, cookies, lazyTemplateRoot)

	bodyTemplateFuncMapData := &BodyTemplateFuncMapData{}
	var bodyTemplateRoot *templateSet
	lazyBodyTemplateRoot := func() *templateSet {
		if bodyTemplateRoot == nil {
			funcs := NewDefaultBodyTemplateFuncMap(randSource, bodyTemplateFuncMapData, fileCache)
			bodyTemplateRoot = &templateSet{root: template.New("").Funcs(funcs), funcs: funcs}
		}
		return bodyTemplateRoot
	}
	bodyRenderer, isBodyDynamic := newStringRenderer(localRand, bodies, lazyBodyTemplateRoot)

	valuesGenerator := NewValuesGeneratorFunc(values, lazyTemplateRoot)

	hasScripts := scriptTransformer != nil && !scriptTransformer.IsEmpty()

	isDynamic := isPathDynamic ||
		isMethodDynamic ||
		isParamsDynamic ||
		isHeadersDynamic ||
		isCookiesDynamic ||
		isBodyDynamic ||
		hasScripts

	components := requestComponents{
		host:             requestURL.Host,
		isTLS:            requestURL.Scheme == "https",
		values:           valuesGenerator,
		path:             pathRenderer,
		method:           methodRenderer,
		body:             bodyRenderer,
		params:           paramsRenderer,
		headers:          headersRenderer,
		cookies:          cookiesRenderer,
		bodyTemplateData: bodyTemplateFuncMapData,
	}

	if !hasScripts {
		return newDirectRequestGenerator(components), isDynamic
	}

	return newScriptedRequestGenerator(components, scriptTransformer), isDynamic
}

// requestComponents bundles everything a request builder needs to render one
// request.
type requestComponents struct {
	host             string
	isTLS            bool
	values           func() (valuesData, error)
	path             stringRenderer
	method           stringRenderer
	body             stringRenderer
	params           keyValueRenderer
	headers          keyValueRenderer
	cookies          keyValueRenderer
	bodyTemplateData *BodyTemplateFuncMapData
}

// newDirectRequestGenerator builds requests straight into the fasthttp request.
//
// This is the path taken whenever no script is configured. The scripted path has
// to stage everything in maps because a script may rewrite or extend them, but
// staging is pure overhead otherwise: for a request with a handful of headers
// the map clears, inserts and iterations cost about as much as the rest of the
// request build put together.
func newDirectRequestGenerator(c requestComponents) RequestGenerator {
	// req and args are rebound on every call before any applier runs. The
	// generator belongs to a single worker goroutine, so keeping them here lets
	// the appliers be built once instead of per request.
	var (
		req       *fasthttp.Request
		args      *fasthttp.Args
		cookieBuf []byte
	)

	addHeader := func(key, value string) { req.Header.Add(key, value) }
	addParam := func(key, value string) { args.Add(key, value) }
	addCookie := func(key, value string) {
		if len(cookieBuf) > 0 {
			cookieBuf = append(cookieBuf, "; "...)
		}
		cookieBuf = append(cookieBuf, key...)
		cookieBuf = append(cookieBuf, '=')
		cookieBuf = append(cookieBuf, value...)
	}

	return func(request *fasthttp.Request) error {
		req = request

		data, err := c.values()
		if err != nil {
			return err
		}

		path, err := c.path(data)
		if err != nil {
			return err
		}

		method, err := c.method(data)
		if err != nil {
			return err
		}

		c.bodyTemplateData.ClearFormDataContentType()
		body, err := c.body(data)
		if err != nil {
			return err
		}

		request.Header.SetHost(c.host)
		request.SetRequestURI(path)
		request.Header.SetMethod(method)
		request.SetBodyString(body)

		if c.headers != nil {
			if err = c.headers(addHeader, data); err != nil {
				return err
			}
		}
		if contentType := c.bodyTemplateData.GetFormDataContentType(); contentType != "" {
			request.Header.Add("Content-Type", contentType)
		}

		if c.params != nil {
			args = request.URI().QueryArgs()
			if err = c.params(addParam, data); err != nil {
				return err
			}
		}

		if c.cookies != nil {
			cookieBuf = cookieBuf[:0]
			if err = c.cookies(addCookie, data); err != nil {
				return err
			}
			if len(cookieBuf) > 0 {
				request.Header.AddBytesV("Cookie", cookieBuf)
			}
		}

		if c.isTLS {
			request.URI().SetScheme("https")
		}

		return nil
	}
}

// newScriptedRequestGenerator stages the request in the maps a script expects,
// runs the script chain over them and only then writes the result out.
func newScriptedRequestGenerator(c requestComponents, scriptTransformer *script.Transformer) RequestGenerator {
	reqData := &script.RequestData{
		Headers: make(map[string][]string),
		Params:  make(map[string][]string),
		Cookies: make(map[string][]string),
	}

	addHeader := func(key, value string) { reqData.Headers[key] = append(reqData.Headers[key], value) }
	addParam := func(key, value string) { reqData.Params[key] = append(reqData.Params[key], value) }
	addCookie := func(key, value string) { reqData.Cookies[key] = append(reqData.Cookies[key], value) }

	var cookieBuf []byte

	// The maps are cleared rather than truncated in place: a script may add keys
	// of its own, and reusing the value slices would leave it observing an empty
	// slice where a key used to be absent.
	return func(req *fasthttp.Request) error {
		clear(reqData.Headers)
		clear(reqData.Params)
		clear(reqData.Cookies)
		reqData.Method = ""
		reqData.Path = ""
		reqData.Body = ""

		data, err := c.values()
		if err != nil {
			return err
		}

		reqData.Path, err = c.path(data)
		if err != nil {
			return err
		}

		reqData.Method, err = c.method(data)
		if err != nil {
			return err
		}

		c.bodyTemplateData.ClearFormDataContentType()
		reqData.Body, err = c.body(data)
		if err != nil {
			return err
		}

		if c.headers != nil {
			if err = c.headers(addHeader, data); err != nil {
				return err
			}
		}
		if contentType := c.bodyTemplateData.GetFormDataContentType(); contentType != "" {
			reqData.Headers["Content-Type"] = append(reqData.Headers["Content-Type"], contentType)
		}

		if c.params != nil {
			if err = c.params(addParam, data); err != nil {
				return err
			}
		}
		if c.cookies != nil {
			if err = c.cookies(addCookie, data); err != nil {
				return err
			}
		}

		if err = scriptTransformer.Transform(reqData); err != nil {
			return err
		}

		cookieBuf = applyRequestDataToFastHTTP(reqData, req, c.host, c.isTLS, cookieBuf)

		return nil
	}
}

// applyRequestDataToFastHTTP writes staged request data onto req. cookieBuf is
// passed in and returned so its backing array survives across requests.
func applyRequestDataToFastHTTP(reqData *script.RequestData, req *fasthttp.Request, host string, isTLS bool, cookieBuf []byte) []byte {
	req.Header.SetHost(host)
	req.SetRequestURI(reqData.Path)
	req.Header.SetMethod(reqData.Method)
	req.SetBodyString(reqData.Body)

	for k, values := range reqData.Headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	if len(reqData.Params) > 0 {
		args := req.URI().QueryArgs()
		for k, values := range reqData.Params {
			for _, v := range values {
				args.Add(k, v)
			}
		}
	}

	if len(reqData.Cookies) > 0 {
		cookieBuf = cookieBuf[:0]
		for k, values := range reqData.Cookies {
			for _, v := range values {
				if len(cookieBuf) > 0 {
					cookieBuf = append(cookieBuf, "; "...)
				}
				cookieBuf = append(cookieBuf, k...)
				cookieBuf = append(cookieBuf, '=')
				cookieBuf = append(cookieBuf, v...)
			}
		}
		req.Header.AddBytesV("Cookie", cookieBuf)
	}

	if isTLS {
		req.URI().SetScheme("https")
	}

	return cookieBuf
}

// newStringRenderer builds the renderer for a single-valued component. When more
// than one value is configured, one is picked per request.
func newStringRenderer(localRand *rand.Rand, values []string, lazyRoot func() *templateSet) (stringRenderer, bool) {
	generator, isDynamic := buildStringSliceGenerator(localRand, values, lazyRoot)

	return func(data any) (string, error) {
		return generator()(data)
	}, isDynamic
}

// newKeyValueRenderer builds the renderer for a multi-valued component. It
// returns a nil renderer when nothing is configured, so callers can skip the
// component entirely. The second result reports whether the component is
// dynamic.
func newKeyValueRenderer[T keyValueItem](
	localRand *rand.Rand,
	items []T,
	lazyRoot func() *templateSet,
) (keyValueRenderer, bool) {
	if len(items) == 0 {
		return nil, false
	}

	generators, isDynamic := buildKeyValueGenerators(localRand, items, lazyRoot)

	return func(apply keyValueApplier, data any) error {
		for _, gen := range generators {
			key, err := gen.Key(data)
			if err != nil {
				return err
			}

			value, err := gen.Value()(data)
			if err != nil {
				return err
			}

			apply(key, value)
		}
		return nil
	}, isDynamic
}

func NewValuesGeneratorFunc(values []string, lazyRoot func() *templateSet) func() (valuesData, error) {
	// No values configured: hand back one shared empty map instead of allocating a
	// fresh one for every request. Nothing ever writes to it.
	if len(values) == 0 {
		empty := valuesData{Values: map[string]string{}}
		return func() (valuesData, error) { return empty, nil }
	}

	generators := make([]func(_ any) (string, error), len(values))

	isDynamic := false
	for i, v := range values {
		var valueIsDynamic bool
		generators[i], valueIsDynamic = createTemplateFunc(v, lazyRoot)
		if valueIsDynamic {
			isDynamic = true
		}
	}

	var (
		rendered string
		data     map[string]string
		err      error
	)
	generate := func() (valuesData, error) {
		result := make(map[string]string)
		for _, generator := range generators {
			rendered, err = generator(nil)
			if err != nil {
				return valuesData{}, types.NewTemplateRenderError(err)
			}

			data, err = godotenv.Unmarshal(rendered)
			if err != nil {
				return valuesData{}, types.NewTemplateRenderError(err)
			}

			maps.Copy(result, data)
		}

		return valuesData{Values: result}, nil
	}

	// Every value is a literal, so the parsed result is identical on every request:
	// render and parse once instead of re-running godotenv per request.
	if !isDynamic {
		staticResult, staticErr := generate()
		return func() (valuesData, error) { return staticResult, staticErr }
	}

	return generate
}

func createTemplateFunc(value string, lazyRoot func() *templateSet) (func(data any) (string, error), bool) {
	if !strings.Contains(value, "{{") {
		return func(_ any) (string, error) { return value, nil }, false
	}

	set := lazyRoot()
	tmpl, err := set.root.New("").Parse(value)
	if err == nil && hasTemplateActions(tmpl) {
		// Simple values render without going through text/template at all.
		if render := compileSimpleTemplate(tmpl, set.funcs); render != nil {
			return func(_ any) (string, error) { return render(), nil }, true
		}

		var (
			buf bytes.Buffer
			err error
		)
		return func(data any) (string, error) {
			buf.Reset()
			if err = tmpl.Execute(&buf, data); err != nil {
				return "", types.NewTemplateRenderError(err)
			}
			return buf.String(), nil
		}, true
	}

	return func(_ any) (string, error) { return value, nil }, false
}

type keyValueGenerator struct {
	Key   func(data any) (string, error)
	Value func() func(data any) (string, error)
}

type keyValueItem interface {
	types.Param | types.Header | types.Cookie
}

func buildKeyValueGenerators[T keyValueItem](
	localRand *rand.Rand,
	items []T,
	lazyRoot func() *templateSet,
) ([]keyValueGenerator, bool) {
	isDynamic := false
	generators := make([]keyValueGenerator, len(items))

	for generatorIndex, item := range items {
		// Convert to KeyValue to access fields
		keyValue := types.KeyValue[string, []string](item)

		// Generate key function
		keyFunc, keyIsDynamic := createTemplateFunc(keyValue.Key, lazyRoot)
		if keyIsDynamic {
			isDynamic = true
		}

		// Generate value functions
		valueFuncs := make([]func(data any) (string, error), len(keyValue.Value))
		for j, v := range keyValue.Value {
			valueFunc, valueIsDynamic := createTemplateFunc(v, lazyRoot)
			if valueIsDynamic {
				isDynamic = true
			}
			valueFuncs[j] = valueFunc
		}

		generators[generatorIndex] = keyValueGenerator{
			Key:   keyFunc,
			Value: utilsSlice.RandomCycle(localRand, valueFuncs...),
		}

		if len(keyValue.Value) > 1 {
			isDynamic = true
		}
	}

	return generators, isDynamic
}

func buildStringSliceGenerator(
	localRand *rand.Rand,
	values []string,
	lazyRoot func() *templateSet,
) (func() func(data any) (string, error), bool) {
	// Return a function that returns an empty string generator if values is empty
	if len(values) == 0 {
		emptyFunc := func(_ any) (string, error) { return "", nil }
		return func() func(_ any) (string, error) { return emptyFunc }, false
	}

	isDynamic := len(values) > 1
	valueFuncs := make([]func(data any) (string, error), len(values))

	for i, value := range values {
		valueFunc, valueIsDynamic := createTemplateFunc(value, lazyRoot)
		if valueIsDynamic {
			isDynamic = true
		}
		valueFuncs[i] = valueFunc
	}

	return utilsSlice.RandomCycle(localRand, valueFuncs...), isDynamic
}
