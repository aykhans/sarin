package script

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RequestData represents the request data passed to scripts for transformation.
// Scripts can modify any field and the changes will be applied to the actual request.
// Headers, Params, and Cookies use []string values to support multiple values per key.
type RequestData struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers"`
	Params  map[string][]string `json:"params"`
	Cookies map[string][]string `json:"cookies"`
	Body    string              `json:"body"`
}

// Engine defines the interface for script engines (Lua, JavaScript).
// Each engine must be able to transform request data using a user-provided script.
type Engine interface {
	// Transform executes the script's transform function with the given request data.
	// The script should modify the RequestData and return it.
	Transform(req *RequestData) error

	// Close releases any resources held by the engine.
	Close()
}

// EngineType represents the type of script engine.
type EngineType string

const (
	EngineTypeLua        EngineType = "lua"
	EngineTypeJavaScript EngineType = "js"
)

// Source represents a loaded script source.
type Source struct {
	Content    string
	EngineType EngineType
}

// LoadSource loads a script from the given source string.
// The source can be:
//   - Inline script: any string not starting with "@"
//   - Escaped "@": strings starting with "@@" (literal "@" at start, returns string without first @)
//   - File reference: "@/path/to/file" or "@./relative/path"
//   - URL reference: "@http://..." or "@https://..."
func LoadSource(ctx context.Context, source string, engineType EngineType) (*Source, error) {
	if source == "" {
		return nil, errors.New("script source cannot be empty")
	}

	var content string
	var err error

	switch {
	case strings.HasPrefix(source, "@@"):
		// Escaped @ - it's an inline script starting with literal @
		content = source[1:] // Remove first @, keep the rest
	case strings.HasPrefix(source, "@"):
		// File or URL reference
		ref := source[1:]
		if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			content, err = fetchURL(ctx, ref)
		} else {
			content, err = readFile(ref)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to load script from %q: %w", ref, err)
		}
	default:
		// Inline script
		content = source
	}

	return &Source{
		Content:    content,
		EngineType: engineType,
	}, nil
}

// LoadSources loads multiple script sources.
func LoadSources(ctx context.Context, sources []string, engineType EngineType) ([]*Source, error) {
	loaded := make([]*Source, 0, len(sources))
	for i, src := range sources {
		source, err := LoadSource(ctx, src, engineType)
		if err != nil {
			return nil, fmt.Errorf("script[%d]: %w", i, err)
		}
		loaded = append(loaded, source)
	}
	return loaded, nil
}

// ValidateScript validates a script source by loading it and checking syntax.
// It loads the script (from file/URL/inline), parses it, and verifies
// that a 'transform' function is defined.
func ValidateScript(ctx context.Context, source string, engineType EngineType) error {
	// Load the script source
	src, err := LoadSource(ctx, source, engineType)
	if err != nil {
		return err
	}

	// Try to create an engine - this validates syntax and transform function
	var engine Engine
	switch engineType {
	case EngineTypeLua:
		engine, err = NewLuaEngine(src.Content)
	case EngineTypeJavaScript:
		engine, err = NewJsEngine(src.Content)
	default:
		return fmt.Errorf("unknown engine type: %s", engineType)
	}

	if err != nil {
		return err
	}

	// Clean up the engine - we only needed it for validation
	engine.Close()
	return nil
}

// ValidateScripts validates multiple script sources.
func ValidateScripts(ctx context.Context, sources []string, engineType EngineType) error {
	for i, src := range sources {
		if err := ValidateScript(ctx, src, engineType); err != nil {
			return fmt.Errorf("script[%d]: %w", i, err)
		}
	}
	return nil
}

// fetchURL downloads content from an HTTP/HTTPS URL.
func fetchURL(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(data), nil
}

// readFile reads content from a local file.
func readFile(path string) (string, error) {
	if !filepath.IsAbs(path) {
		pwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		path = filepath.Join(pwd, path)
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}
