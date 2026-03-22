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

type requestDataGenerator func(*script.RequestData, any) error

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
	var templateRoot *template.Template
	lazyTemplateRoot := func() *template.Template {
		if templateRoot == nil {
			templateRoot = template.New("").Funcs(NewDefaultTemplateFuncMap(randSource, fileCache))
		}
		return templateRoot
	}

	pathGenerator, isPathGeneratorDynamic := createTemplateFunc(requestURL.Path, lazyTemplateRoot)
	methodGenerator, isMethodGeneratorDynamic := NewMethodGeneratorFunc(localRand, methods, lazyTemplateRoot)
	paramsGenerator, isParamsGeneratorDynamic := NewParamsGeneratorFunc(localRand, params, lazyTemplateRoot)
	headersGenerator, isHeadersGeneratorDynamic := NewHeadersGeneratorFunc(localRand, headers, lazyTemplateRoot)
	cookiesGenerator, isCookiesGeneratorDynamic := NewCookiesGeneratorFunc(localRand, cookies, lazyTemplateRoot)

	bodyTemplateFuncMapData := &BodyTemplateFuncMapData{}
	var bodyTemplateRoot *template.Template
	lazyBodyTemplateRoot := func() *template.Template {
		if bodyTemplateRoot == nil {
			bodyTemplateRoot = template.New("").Funcs(NewDefaultBodyTemplateFuncMap(randSource, bodyTemplateFuncMapData, fileCache))
		}
		return bodyTemplateRoot
	}
	bodyGenerator, isBodyGeneratorDynamic := NewBodyGeneratorFunc(localRand, bodies, lazyBodyTemplateRoot)

	valuesGenerator := NewValuesGeneratorFunc(values, lazyTemplateRoot)

	hasScripts := scriptTransformer != nil && !scriptTransformer.IsEmpty()

	host := requestURL.Host
	scheme := requestURL.Scheme

	reqData := &script.RequestData{
		Headers: make(map[string][]string),
		Params:  make(map[string][]string),
		Cookies: make(map[string][]string),
	}

	var (
		data valuesData
		path string
		err  error
	)
	return func(req *fasthttp.Request) error {
			resetRequestData(reqData)

			data, err = valuesGenerator()
			if err != nil {
				return err
			}

			path, err = pathGenerator(data)
			if err != nil {
				return err
			}
			reqData.Path = path

			if err = methodGenerator(reqData, data); err != nil {
				return err
			}

			bodyTemplateFuncMapData.ClearFormDataContentType()
			if err = bodyGenerator(reqData, data); err != nil {
				return err
			}

			if err = headersGenerator(reqData, data); err != nil {
				return err
			}
			if bodyTemplateFuncMapData.GetFormDataContentType() != "" {
				reqData.Headers["Content-Type"] = append(reqData.Headers["Content-Type"], bodyTemplateFuncMapData.GetFormDataContentType())
			}

			if err = paramsGenerator(reqData, data); err != nil {
				return err
			}
			if err = cookiesGenerator(reqData, data); err != nil {
				return err
			}

			if hasScripts {
				if err = scriptTransformer.Transform(reqData); err != nil {
					return err
				}
			}

			applyRequestDataToFastHTTP(reqData, req, host, scheme)

			return nil
		}, isPathGeneratorDynamic ||
			isMethodGeneratorDynamic ||
			isParamsGeneratorDynamic ||
			isHeadersGeneratorDynamic ||
			isCookiesGeneratorDynamic ||
			isBodyGeneratorDynamic ||
			hasScripts
}

func resetRequestData(reqData *script.RequestData) {
	reqData.Method = ""
	reqData.Path = ""
	reqData.Body = ""
	clear(reqData.Headers)
	clear(reqData.Params)
	clear(reqData.Cookies)
}

func applyRequestDataToFastHTTP(reqData *script.RequestData, req *fasthttp.Request, host, scheme string) {
	req.Header.SetHost(host)
	req.SetRequestURI(reqData.Path)
	req.Header.SetMethod(reqData.Method)
	req.SetBody([]byte(reqData.Body))

	for k, values := range reqData.Headers {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	for k, values := range reqData.Params {
		for _, v := range values {
			req.URI().QueryArgs().Add(k, v)
		}
	}

	if len(reqData.Cookies) > 0 {
		cookieStrings := make([]string, 0, len(reqData.Cookies))
		for k, values := range reqData.Cookies {
			for _, v := range values {
				cookieStrings = append(cookieStrings, k+"="+v)
			}
		}
		req.Header.Add("Cookie", strings.Join(cookieStrings, "; "))
	}

	if scheme == "https" {
		req.URI().SetScheme("https")
	}
}

func NewMethodGeneratorFunc(localRand *rand.Rand, methods []string, lazyRoot func() *template.Template) (requestDataGenerator, bool) {
	methodGenerator, isDynamic := buildStringSliceGenerator(localRand, methods, lazyRoot)

	var (
		method string
		err    error
	)
	return func(reqData *script.RequestData, data any) error {
		method, err = methodGenerator()(data)
		if err != nil {
			return err
		}

		reqData.Method = method
		return nil
	}, isDynamic
}

func NewBodyGeneratorFunc(localRand *rand.Rand, bodies []string, lazyRoot func() *template.Template) (requestDataGenerator, bool) {
	bodyGenerator, isDynamic := buildStringSliceGenerator(localRand, bodies, lazyRoot)

	var (
		body string
		err  error
	)
	return func(reqData *script.RequestData, data any) error {
		body, err = bodyGenerator()(data)
		if err != nil {
			return err
		}

		reqData.Body = body
		return nil
	}, isDynamic
}

func NewParamsGeneratorFunc(localRand *rand.Rand, params types.Params, lazyRoot func() *template.Template) (requestDataGenerator, bool) {
	generators, isDynamic := buildKeyValueGenerators(localRand, params, lazyRoot)

	var (
		key, value string
		err        error
	)
	return func(reqData *script.RequestData, data any) error {
		for _, gen := range generators {
			key, err = gen.Key(data)
			if err != nil {
				return err
			}

			value, err = gen.Value()(data)
			if err != nil {
				return err
			}

			reqData.Params[key] = append(reqData.Params[key], value)
		}
		return nil
	}, isDynamic
}

func NewHeadersGeneratorFunc(localRand *rand.Rand, headers types.Headers, lazyRoot func() *template.Template) (requestDataGenerator, bool) {
	generators, isDynamic := buildKeyValueGenerators(localRand, headers, lazyRoot)

	var (
		key, value string
		err        error
	)
	return func(reqData *script.RequestData, data any) error {
		for _, gen := range generators {
			key, err = gen.Key(data)
			if err != nil {
				return err
			}

			value, err = gen.Value()(data)
			if err != nil {
				return err
			}

			reqData.Headers[key] = append(reqData.Headers[key], value)
		}
		return nil
	}, isDynamic
}

func NewCookiesGeneratorFunc(localRand *rand.Rand, cookies types.Cookies, lazyRoot func() *template.Template) (requestDataGenerator, bool) {
	generators, isDynamic := buildKeyValueGenerators(localRand, cookies, lazyRoot)

	var (
		key, value string
		err        error
	)
	return func(reqData *script.RequestData, data any) error {
		for _, gen := range generators {
			key, err = gen.Key(data)
			if err != nil {
				return err
			}

			value, err = gen.Value()(data)
			if err != nil {
				return err
			}

			reqData.Cookies[key] = append(reqData.Cookies[key], value)
		}
		return nil
	}, isDynamic
}

func NewValuesGeneratorFunc(values []string, lazyRoot func() *template.Template) func() (valuesData, error) {
	generators := make([]func(_ any) (string, error), len(values))

	for i, v := range values {
		generators[i], _ = createTemplateFunc(v, lazyRoot)
	}

	var (
		rendered string
		data     map[string]string
		err      error
	)
	return func() (valuesData, error) {
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
}

func createTemplateFunc(value string, lazyRoot func() *template.Template) (func(data any) (string, error), bool) {
	if !strings.Contains(value, "{{") {
		return func(_ any) (string, error) { return value, nil }, false
	}

	tmpl, err := lazyRoot().New("").Parse(value)
	if err == nil && hasTemplateActions(tmpl) {
		var err error
		return func(data any) (string, error) {
			var buf bytes.Buffer
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
	lazyRoot func() *template.Template,
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
	lazyRoot func() *template.Template,
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
