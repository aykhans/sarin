package sarin

import (
	"bytes"
	"fmt"
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

type RequestGeneratorWithData func(*fasthttp.Request, any) error

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
	templateFuncMap := NewDefaultTemplateFuncMap(randSource, fileCache)

	pathGenerator, isPathGeneratorDynamic := createTemplateFunc(requestURL.Path, templateFuncMap)
	methodGenerator, isMethodGeneratorDynamic := NewMethodGeneratorFunc(localRand, methods, templateFuncMap)
	paramsGenerator, isParamsGeneratorDynamic := NewParamsGeneratorFunc(localRand, params, templateFuncMap)
	headersGenerator, isHeadersGeneratorDynamic := NewHeadersGeneratorFunc(localRand, headers, templateFuncMap)
	cookiesGenerator, isCookiesGeneratorDynamic := NewCookiesGeneratorFunc(localRand, cookies, templateFuncMap)

	bodyTemplateFuncMapData := &BodyTemplateFuncMapData{}
	bodyTemplateFuncMap := NewDefaultBodyTemplateFuncMap(randSource, bodyTemplateFuncMapData, fileCache)
	bodyGenerator, isBodyGeneratorDynamic := NewBodyGeneratorFunc(localRand, bodies, bodyTemplateFuncMap)

	valuesGenerator := NewValuesGeneratorFunc(values, templateFuncMap)

	hasScripts := scriptTransformer != nil && !scriptTransformer.IsEmpty()

	var (
		data valuesData
		path string
		err  error
	)
	return func(req *fasthttp.Request) error {
			req.Header.SetHost(requestURL.Host)

			data, err = valuesGenerator()
			if err != nil {
				return err
			}

			path, err = pathGenerator(data)
			if err != nil {
				return err
			}
			req.SetRequestURI(path)

			if err = methodGenerator(req, data); err != nil {
				return err
			}

			bodyTemplateFuncMapData.ClearFormDataContenType()
			if err = bodyGenerator(req, data); err != nil {
				return err
			}

			if err = headersGenerator(req, data); err != nil {
				return err
			}
			if bodyTemplateFuncMapData.GetFormDataContenType() != "" {
				req.Header.Add("Content-Type", bodyTemplateFuncMapData.GetFormDataContenType())
			}

			if err = paramsGenerator(req, data); err != nil {
				return err
			}
			if err = cookiesGenerator(req, data); err != nil {
				return err
			}

			if requestURL.Scheme == "https" {
				req.URI().SetScheme("https")
			}

			// Apply script transformations if any
			if hasScripts {
				reqData := script.RequestDataFromFastHTTP(req)
				if err = scriptTransformer.Transform(reqData); err != nil {
					return err
				}
				script.ApplyToFastHTTP(reqData, req)
			}

			return nil
		}, isPathGeneratorDynamic ||
			isMethodGeneratorDynamic ||
			isParamsGeneratorDynamic ||
			isHeadersGeneratorDynamic ||
			isCookiesGeneratorDynamic ||
			isBodyGeneratorDynamic ||
			hasScripts
}

func NewMethodGeneratorFunc(localRand *rand.Rand, methods []string, templateFunctions template.FuncMap) (RequestGeneratorWithData, bool) {
	methodGenerator, isDynamic := buildStringSliceGenerator(localRand, methods, templateFunctions)

	var (
		method string
		err    error
	)
	return func(req *fasthttp.Request, data any) error {
		method, err = methodGenerator()(data)
		if err != nil {
			return err
		}

		req.Header.SetMethod(method)
		return nil
	}, isDynamic
}

func NewBodyGeneratorFunc(localRand *rand.Rand, bodies []string, templateFunctions template.FuncMap) (RequestGeneratorWithData, bool) {
	bodyGenerator, isDynamic := buildStringSliceGenerator(localRand, bodies, templateFunctions)

	var (
		body string
		err  error
	)
	return func(req *fasthttp.Request, data any) error {
		body, err = bodyGenerator()(data)
		if err != nil {
			return err
		}

		req.SetBody([]byte(body))
		return nil
	}, isDynamic
}

func NewParamsGeneratorFunc(localRand *rand.Rand, params types.Params, templateFunctions template.FuncMap) (RequestGeneratorWithData, bool) {
	generators, isDynamic := buildKeyValueGenerators(localRand, params, templateFunctions)

	var (
		key, value string
		err        error
	)
	return func(req *fasthttp.Request, data any) error {
		for _, gen := range generators {
			key, err = gen.Key(data)
			if err != nil {
				return err
			}

			value, err = gen.Value()(data)
			if err != nil {
				return err
			}

			req.URI().QueryArgs().Add(key, value)
		}
		return nil
	}, isDynamic
}

func NewHeadersGeneratorFunc(localRand *rand.Rand, headers types.Headers, templateFunctions template.FuncMap) (RequestGeneratorWithData, bool) {
	generators, isDynamic := buildKeyValueGenerators(localRand, headers, templateFunctions)

	var (
		key, value string
		err        error
	)
	return func(req *fasthttp.Request, data any) error {
		for _, gen := range generators {
			key, err = gen.Key(data)
			if err != nil {
				return err
			}

			value, err = gen.Value()(data)
			if err != nil {
				return err
			}

			req.Header.Add(key, value)
		}
		return nil
	}, isDynamic
}

func NewCookiesGeneratorFunc(localRand *rand.Rand, cookies types.Cookies, templateFunctions template.FuncMap) (RequestGeneratorWithData, bool) {
	generators, isDynamic := buildKeyValueGenerators(localRand, cookies, templateFunctions)

	var (
		key, value string
		err        error
	)
	if len(generators) > 0 {
		return func(req *fasthttp.Request, data any) error {
			cookieStrings := make([]string, 0, len(generators))
			for _, gen := range generators {
				key, err = gen.Key(data)
				if err != nil {
					return err
				}

				value, err = gen.Value()(data)
				if err != nil {
					return err
				}

				cookieStrings = append(cookieStrings, key+"="+value)
			}
			req.Header.Add("Cookie", strings.Join(cookieStrings, "; "))
			return nil
		}, isDynamic
	}

	return func(req *fasthttp.Request, data any) error {
		return nil
	}, isDynamic
}

func NewValuesGeneratorFunc(values []string, templateFunctions template.FuncMap) func() (valuesData, error) {
	generators := make([]func(_ any) (string, error), len(values))

	for i, v := range values {
		generators[i], _ = createTemplateFunc(v, templateFunctions)
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
				return valuesData{}, fmt.Errorf("values rendering: %w", err)
			}

			data, err = godotenv.Unmarshal(rendered)
			if err != nil {
				return valuesData{}, fmt.Errorf("values rendering: %w", err)
			}

			maps.Copy(result, data)
		}

		return valuesData{Values: result}, nil
	}
}

func createTemplateFunc(value string, templateFunctions template.FuncMap) (func(data any) (string, error), bool) {
	tmpl, err := template.New("").Funcs(templateFunctions).Parse(value)
	if err == nil && hasTemplateActions(tmpl) {
		var err error
		return func(data any) (string, error) {
			var buf bytes.Buffer
			if err = tmpl.Execute(&buf, data); err != nil {
				return "", fmt.Errorf("template rendering: %w", err)
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
	templateFunctions template.FuncMap,
) ([]keyValueGenerator, bool) {
	isDynamic := false
	generators := make([]keyValueGenerator, len(items))

	for generatorIndex, item := range items {
		// Convert to KeyValue to access fields
		keyValue := types.KeyValue[string, []string](item)

		// Generate key function
		keyFunc, keyIsDynamic := createTemplateFunc(keyValue.Key, templateFunctions)
		if keyIsDynamic {
			isDynamic = true
		}

		// Generate value functions
		valueFuncs := make([]func(data any) (string, error), len(keyValue.Value))
		for j, v := range keyValue.Value {
			valueFunc, valueIsDynamic := createTemplateFunc(v, templateFunctions)
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
	templateFunctions template.FuncMap,
) (func() func(data any) (string, error), bool) {
	// Return a function that returns an empty string generator if values is empty
	if len(values) == 0 {
		emptyFunc := func(_ any) (string, error) { return "", nil }
		return func() func(_ any) (string, error) { return emptyFunc }, false
	}

	isDynamic := len(values) > 1
	valueFuncs := make([]func(data any) (string, error), len(values))

	for i, value := range values {
		valueFunc, valueIsDynamic := createTemplateFunc(value, templateFunctions)
		if valueIsDynamic {
			isDynamic = true
		}
		valueFuncs[i] = valueFunc
	}

	return utilsSlice.RandomCycle(localRand, valueFuncs...), isDynamic
}
