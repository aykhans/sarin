package script

import (
	"go.aykhans.me/sarin/internal/types"
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
// It can return the following errors:
//   - types.ScriptChainError
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
			return nil, types.NewScriptChainError("lua", i, err)
		}
		t.luaEngines = append(t.luaEngines, engine)
	}

	// Create JS engines
	for i, src := range c.jsSources {
		engine, err := NewJsEngine(src.Content)
		if err != nil {
			t.Close() // Clean up already created engines
			return nil, types.NewScriptChainError("js", i, err)
		}
		t.jsEngines = append(t.jsEngines, engine)
	}

	return t, nil
}

// Transform applies all scripts to the request data.
// Lua scripts run first, then JavaScript scripts.
// It can return the following errors:
//   - types.ScriptChainError
func (t *Transformer) Transform(req *RequestData) error {
	// Run Lua scripts
	for i, engine := range t.luaEngines {
		if err := engine.Transform(req); err != nil {
			return types.NewScriptChainError("lua", i, err)
		}
	}

	// Run JS scripts
	for i, engine := range t.jsEngines {
		if err := engine.Transform(req); err != nil {
			return types.NewScriptChainError("js", i, err)
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
