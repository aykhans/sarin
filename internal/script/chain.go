package script

import (
	"fmt"

	"github.com/valyala/fasthttp"
)

// Chain holds the loaded script sources and can create engine instances.
// The sources are loaded once, but engines are created per-worker since they're not thread-safe.
type Chain struct {
	luaSources []*Source
	jsSources  []*Source
}

// NewChain creates a new script chain from loaded sources.
// Lua scripts run first, then JavaScript scripts, in the order provided.
func NewChain(luaSources, jsSources []*Source) *Chain {
	return &Chain{
		luaSources: luaSources,
		jsSources:  jsSources,
	}
}

// IsEmpty returns true if there are no scripts to execute.
func (c *Chain) IsEmpty() bool {
	return len(c.luaSources) == 0 && len(c.jsSources) == 0
}

// Transformer holds instantiated script engines for a single worker.
// It is NOT safe for concurrent use.
type Transformer struct {
	luaEngines []*LuaEngine
	jsEngines  []*JsEngine
}

// NewTransformer creates engine instances from the chain's sources.
// Call this once per worker goroutine.
func (c *Chain) NewTransformer() (*Transformer, error) {
	if c.IsEmpty() {
		return &Transformer{}, nil
	}

	t := &Transformer{
		luaEngines: make([]*LuaEngine, 0, len(c.luaSources)),
		jsEngines:  make([]*JsEngine, 0, len(c.jsSources)),
	}

	// Create Lua engines
	for i, src := range c.luaSources {
		engine, err := NewLuaEngine(src.Content)
		if err != nil {
			t.Close() // Clean up already created engines
			return nil, fmt.Errorf("lua script[%d]: %w", i, err)
		}
		t.luaEngines = append(t.luaEngines, engine)
	}

	// Create JS engines
	for i, src := range c.jsSources {
		engine, err := NewJsEngine(src.Content)
		if err != nil {
			t.Close() // Clean up already created engines
			return nil, fmt.Errorf("js script[%d]: %w", i, err)
		}
		t.jsEngines = append(t.jsEngines, engine)
	}

	return t, nil
}

// Transform applies all scripts to the request data.
// Lua scripts run first, then JavaScript scripts.
func (t *Transformer) Transform(req *RequestData) error {
	// Run Lua scripts
	for i, engine := range t.luaEngines {
		if err := engine.Transform(req); err != nil {
			return fmt.Errorf("lua script[%d]: %w", i, err)
		}
	}

	// Run JS scripts
	for i, engine := range t.jsEngines {
		if err := engine.Transform(req); err != nil {
			return fmt.Errorf("js script[%d]: %w", i, err)
		}
	}

	return nil
}

// Close releases all engine resources.
func (t *Transformer) Close() {
	for _, engine := range t.luaEngines {
		engine.Close()
	}
	for _, engine := range t.jsEngines {
		engine.Close()
	}
}

// IsEmpty returns true if there are no engines.
func (t *Transformer) IsEmpty() bool {
	return len(t.luaEngines) == 0 && len(t.jsEngines) == 0
}

// RequestDataFromFastHTTP extracts RequestData from a fasthttp.Request.
func RequestDataFromFastHTTP(req *fasthttp.Request) *RequestData {
	data := &RequestData{
		Method:  string(req.Header.Method()),
		URL:     string(req.URI().FullURI()),
		Path:    string(req.URI().Path()),
		Body:    string(req.Body()),
		Headers: make(map[string][]string),
		Params:  make(map[string][]string),
		Cookies: make(map[string][]string),
	}

	// Extract headers (supports multiple values per key)
	req.Header.All()(func(key, value []byte) bool {
		k := string(key)
		data.Headers[k] = append(data.Headers[k], string(value))
		return true
	})

	// Extract query params (supports multiple values per key)
	req.URI().QueryArgs().All()(func(key, value []byte) bool {
		k := string(key)
		data.Params[k] = append(data.Params[k], string(value))
		return true
	})

	// Extract cookies (supports multiple values per key)
	req.Header.Cookies()(func(key, value []byte) bool {
		k := string(key)
		data.Cookies[k] = append(data.Cookies[k], string(value))
		return true
	})

	return data
}

// ApplyToFastHTTP applies the modified RequestData back to a fasthttp.Request.
func ApplyToFastHTTP(data *RequestData, req *fasthttp.Request) {
	// Method
	req.Header.SetMethod(data.Method)

	// Path (preserve scheme and host)
	req.URI().SetPath(data.Path)

	// Body
	req.SetBody([]byte(data.Body))

	// Clear and set headers (supports multiple values per key)
	req.Header.All()(func(key, _ []byte) bool {
		keyStr := string(key)
		if keyStr != "Host" {
			req.Header.Del(keyStr)
		}
		return true
	})
	for k, values := range data.Headers {
		if k != "Host" { // Don't overwrite Host
			for _, v := range values {
				req.Header.Add(k, v)
			}
		}
	}

	// Clear and set query params (supports multiple values per key)
	req.URI().QueryArgs().Reset()
	for k, values := range data.Params {
		for _, v := range values {
			req.URI().QueryArgs().Add(k, v)
		}
	}

	// Clear and set cookies (supports multiple values per key)
	req.Header.DelAllCookies()
	for k, values := range data.Cookies {
		for _, v := range values {
			req.Header.SetCookie(k, v)
		}
	}
}
