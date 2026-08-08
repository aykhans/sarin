package sarin

import (
	"bytes"
	"math/rand/v2"
	"net/url"
	"sort"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/valyala/fasthttp"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
)

// fixedRandSource returns a rand.Source seeded identically on every call so two
// generators draw the same sequence of fake values.
func fixedRandSource() rand.Source { return rand.NewPCG(1, 2) }

func newFixedTemplateSet() *templateSet {
	funcs := NewDefaultTemplateFuncMap(fixedRandSource(), nil)
	return &templateSet{root: template.New("").Funcs(funcs), funcs: funcs}
}

// TestCompileSimpleTemplateMatchesTextTemplate renders the same value through
// the compiled fast path and through text/template, each with its own but
// identically seeded faker, and requires the results to be equal.
func TestCompileSimpleTemplateMatchesTextTemplate(t *testing.T) {
	values := []string{
		"{{ fakeit_UUID }}",
		"{{fakeit_Name}}",
		`{"id":"{{ fakeit_UUID }}","name":"{{ fakeit_Name }}"}`,
		"prefix-{{ fakeit_Word }}-suffix",
		"{{ fakeit_Email }}{{ fakeit_Email }}",
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			fastSet := newFixedTemplateSet()
			fastTmpl, err := fastSet.root.New("").Parse(value)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			render := compileSimpleTemplate(fastTmpl, fastSet.funcs)
			if render == nil {
				t.Fatalf("value did not take the compiled path")
			}

			slowSet := newFixedTemplateSet()
			slowTmpl, err := slowSet.root.New("").Parse(value)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			var buf bytes.Buffer
			for i := range 20 {
				buf.Reset()
				if err := slowTmpl.Execute(&buf, nil); err != nil {
					t.Fatalf("execute: %v", err)
				}
				if got, want := render(), buf.String(); got != want {
					t.Fatalf("render %d: compiled = %q, text/template = %q", i, got, want)
				}
			}
		})
	}
}

// TestCompileSimpleTemplateFallback pins the values that must keep going through
// text/template, so the fast path can never silently change their output.
func TestCompileSimpleTemplateFallback(t *testing.T) {
	values := []string{
		"{{ .Values.token }}",                 // data reference
		"{{ fakeit_LetterN 5 }}",              // takes an argument
		"{{ strings_ToUpper fakeit_Word }}",   // nested call
		"{{ fakeit_Word | strings_ToUpper }}", // pipeline
		"{{ fakeit_Age }}",                    // returns int, not string
		"{{ if true }}a{{ end }}",             // control flow
		"{{ $x := fakeit_Word }}{{ $x }}",     // variable declaration
	}

	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			set := newFixedTemplateSet()
			tmpl, err := set.root.New("").Parse(value)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if render := compileSimpleTemplate(tmpl, set.funcs); render != nil {
				t.Fatalf("value unexpectedly took the compiled path, got %q", render())
			}
		})
	}
}

// requestSnapshot is the observable content of a generated request. Everything
// HTTP treats as an unordered set - headers, cookie pairs, query args - is
// sorted, because the two builders legitimately emit them in different orders:
// the map-staging one inherits Go's randomized map iteration, the map-free one
// follows configuration order.
type requestSnapshot struct {
	method  string
	path    string
	query   []string
	body    string
	headers []string
}

func snapshot(t *testing.T, req *fasthttp.Request) requestSnapshot {
	t.Helper()

	var headers []string
	for key, value := range req.Header.All() {
		if string(key) == "Cookie" {
			pairs := strings.Split(string(value), "; ")
			sort.Strings(pairs)
			headers = append(headers, "Cookie: "+strings.Join(pairs, "; "))
			continue
		}
		headers = append(headers, string(key)+": "+string(value))
	}
	sort.Strings(headers)

	var query []string
	for key, value := range req.URI().QueryArgs().All() {
		query = append(query, string(key)+"="+string(value))
	}
	sort.Strings(query)

	return requestSnapshot{
		method:  string(req.Header.Method()),
		path:    string(req.URI().Path()),
		query:   query,
		body:    string(req.Body()),
		headers: headers,
	}
}

func (s requestSnapshot) String() string {
	return s.method + " " + s.path + "?" + strings.Join(s.query, "&") + "\n" +
		strings.Join(s.headers, "\n") + "\n\n" + s.body
}

const noopLuaScript = `function transform(req) return req end`

func newNoopTransformer(t *testing.T) *script.Transformer {
	t.Helper()

	chain := script.NewChain([]*script.Source{{Content: noopLuaScript, EngineType: script.EngineTypeLua}}, nil)
	transformer, err := chain.NewTransformer()
	if err != nil {
		t.Fatalf("transformer: %v", err)
	}
	t.Cleanup(transformer.Close)
	return transformer
}

// TestDirectAndScriptedGeneratorsAgree runs the same configuration through the
// map-free builder and through the script-staging builder (with a script that
// changes nothing) and requires both to produce the same request.
func TestDirectAndScriptedGeneratorsAgree(t *testing.T) {
	cases := []struct {
		name    string
		rawURL  string
		params  types.Params
		headers types.Headers
		cookies types.Cookies
		bodies  []string
	}{
		{
			name:   "bare",
			rawURL: "http://example.com/",
		},
		{
			name:    "headers only",
			rawURL:  "http://example.com/api",
			headers: types.Headers{{Key: "User-Agent", Value: []string{"sarin"}}},
		},
		{
			name:   "everything",
			rawURL: "https://example.com/api/v1/items",
			params: types.Params{
				{Key: "page", Value: []string{"2"}},
				{Key: "sort", Value: []string{"name"}},
			},
			headers: types.Headers{
				{Key: "User-Agent", Value: []string{"sarin"}},
				{Key: "Accept", Value: []string{"application/json"}},
				{Key: "X-Token", Value: []string{"abc123"}},
			},
			cookies: types.Cookies{
				{Key: "session", Value: []string{"xyz"}},
				{Key: "theme", Value: []string{"dark"}},
			},
			bodies: []string{`{"name":"item"}`},
		},
		{
			name:    "repeated header key",
			rawURL:  "http://example.com/",
			headers: types.Headers{{Key: "X-Tag", Value: []string{"a"}}, {Key: "X-Tag", Value: []string{"b"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requestURL, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatalf("parse url: %v", err)
			}

			build := func(transformer *script.Transformer) requestSnapshot {
				generator, _ := NewRequestGenerator(
					[]string{"POST"}, requestURL, tc.params, tc.headers, tc.cookies, tc.bodies, nil,
					NewFileCache(time.Second), transformer,
				)
				req := fasthttp.AcquireRequest()
				defer fasthttp.ReleaseRequest(req)
				// Generate twice: the second pass catches state left behind by the first.
				for range 2 {
					req.Reset()
					if err := generator(req); err != nil {
						t.Fatalf("generate: %v", err)
					}
				}
				return snapshot(t, req)
			}

			direct := build(nil)
			scripted := build(newNoopTransformer(t))

			if direct.String() != scripted.String() {
				t.Fatalf("builders disagree:\n--- direct ---\n%s\n--- scripted ---\n%s", direct, scripted)
			}
		})
	}
}

// TestDirectGeneratorContent checks the request the map-free builder produces
// against the expected wire content, not just against the other builder.
func TestDirectGeneratorContent(t *testing.T) {
	requestURL, err := url.Parse("https://example.com/api/v1/items")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	generator, isDynamic := NewRequestGenerator(
		[]string{"POST"},
		requestURL,
		types.Params{{Key: "page", Value: []string{"2"}}},
		types.Headers{{Key: "X-Token", Value: []string{"abc"}}},
		types.Cookies{{Key: "a", Value: []string{"1"}}, {Key: "b", Value: []string{"2"}}},
		[]string{`{"k":"v"}`},
		nil,
		NewFileCache(time.Second),
		nil,
	)
	if isDynamic {
		t.Fatalf("configuration has no templates, want static")
	}

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	if err := generator(req); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if got, want := string(req.Header.Method()), "POST"; got != want {
		t.Errorf("method = %q, want %q", got, want)
	}
	if got, want := string(req.Header.Host()), "example.com"; got != want {
		t.Errorf("host = %q, want %q", got, want)
	}
	if got, want := string(req.URI().Scheme()), "https"; got != want {
		t.Errorf("scheme = %q, want %q", got, want)
	}
	if got, want := string(req.URI().Path()), "/api/v1/items"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got, want := string(req.URI().QueryArgs().Peek("page")), "2"; got != want {
		t.Errorf("page = %q, want %q", got, want)
	}
	if got, want := string(req.Header.Peek("X-Token")), "abc"; got != want {
		t.Errorf("X-Token = %q, want %q", got, want)
	}
	if got, want := string(req.Header.Peek("Cookie")), "a=1; b=2"; got != want {
		t.Errorf("Cookie = %q, want %q", got, want)
	}
	if got, want := string(req.Body()), `{"k":"v"}`; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

// TestDirectGeneratorReuseIsClean guards the buffers the builder keeps across
// requests: a shorter cookie or body must not leave a tail from the previous one.
func TestDirectGeneratorReuseIsClean(t *testing.T) {
	requestURL, err := url.Parse("http://example.com/")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	generator, _ := NewRequestGenerator(
		[]string{"GET"},
		requestURL,
		nil,
		nil,
		types.Cookies{{Key: "c", Value: []string{"longlonglongvalue", "s"}}},
		nil,
		nil,
		NewFileCache(time.Second),
		nil,
	)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	for range 50 {
		req.Reset()
		if err := generator(req); err != nil {
			t.Fatalf("generate: %v", err)
		}
		cookie := string(req.Header.Peek("Cookie"))
		if cookie != "c=longlonglongvalue" && cookie != "c=s" {
			t.Fatalf("unexpected cookie header %q", cookie)
		}
	}
}
