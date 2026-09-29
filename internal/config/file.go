package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"go.aykhans.me/sarin/internal/types"
	"go.yaml.in/yaml/v4"
)

var _ IParser = ConfigFileParser{}

type ConfigFileParser struct {
	configFile types.ConfigFile
}

func NewConfigFileParser(configFile types.ConfigFile) *ConfigFileParser {
	return &ConfigFileParser{configFile}
}

// Parse parses config file arguments into a Config object.
// It can return the following errors:
// - types.ConfigFileReadError
// - types.UnmarshalError
// - types.FieldParseErrors
func (parser ConfigFileParser) Parse() (*Config, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	configFileData, err := fetchFile(ctx, parser.configFile.Path())
	if err != nil {
		return nil, types.NewConfigFileReadError(err)
	}

	switch parser.configFile.Type() {
	case types.ConfigFileTypeYAML, types.ConfigFileTypeUnknown:
		return parser.ParseYAML(configFileData)
	default:
		panic("unhandled config file type")
	}
}

// fetchFile retrieves file contents from a local path or HTTP/HTTPS URL.
// It can return the following errors:
//   - types.FileReadError
//   - types.HTTPFetchError
//   - types.HTTPStatusError
func fetchFile(ctx context.Context, src string) ([]byte, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return fetchHTTP(ctx, src)
	}
	return fetchLocal(src)
}

// fetchHTTP downloads file contents from an HTTP/HTTPS URL.
// It can return the following errors:
//   - types.HTTPFetchError
//   - types.HTTPStatusError
func fetchHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, types.NewHTTPFetchError(url, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, types.NewHTTPFetchError(url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewHTTPStatusError(url, resp.StatusCode, resp.Status)
	}

	// A declared size above the limit is refused before anything is read.
	if resp.ContentLength > types.RemoteFetchLimit {
		return nil, types.NewHTTPFetchError(url, types.ErrRemoteFileTooLarge)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, types.RemoteFetchLimit+1))
	if err != nil {
		return nil, types.NewHTTPFetchError(url, err)
	}
	if len(data) > types.RemoteFetchLimit {
		return nil, types.NewHTTPFetchError(url, types.ErrRemoteFileTooLarge)
	}

	return data, nil
}

// fetchLocal reads file contents from the local filesystem.
// It resolves relative paths from the current working directory.
// It can return the following errors:
//   - types.FileReadError
func fetchLocal(src string) ([]byte, error) {
	path := src
	if !filepath.IsAbs(src) {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, types.NewFileReadError(src, err)
		}
		path = filepath.Join(pwd, src)
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, types.NewFileReadError(path, err)
	}

	return data, nil
}

type stringOrSliceField []string

func (ss *stringOrSliceField) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		// Handle single string value
		*ss = []string{node.Value}
		return nil
	case yaml.SequenceNode:
		// Handle array of strings
		var slice []string
		if err := node.Decode(&slice); err != nil {
			return err //nolint:wrapcheck
		}
		*ss = slice
		return nil
	default:
		return fmt.Errorf("expected a string or a sequence of strings, but got %v", node.Kind)
	}
}

// keyValuesField handles flexible YAML formats for key-value pairs.
// Supported formats:
//   - Sequence of maps: [{key1: value1}, {key2: [value2, value3]}]
//   - Single map: {key1: value1, key2: [value2, value3]}
//
// Values can be either a single string or an array of strings.
type keyValuesField []types.KeyValue[string, []string]

func (kv *keyValuesField) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		// Handle single map: {key1: value1, key2: [value2]}
		return kv.unmarshalMapping(node)
	case yaml.SequenceNode:
		// Handle sequence of maps: [{key1: value1}, {key2: value2}]
		for _, item := range node.Content {
			if item.Kind != yaml.MappingNode {
				return fmt.Errorf("expected a mapping in sequence, but got %v", item.Kind)
			}
			if err := kv.unmarshalMapping(item); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("expected a mapping or sequence of mappings, but got %v", node.Kind)
	}
}

func (kv *keyValuesField) unmarshalMapping(node *yaml.Node) error {
	// MappingNode content is [key1, value1, key2, value2, ...]
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valueNode := node.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode {
			return fmt.Errorf("expected a string key, but got %v", keyNode.Kind)
		}

		key := keyNode.Value
		var values []string

		switch valueNode.Kind {
		case yaml.ScalarNode:
			values = []string{valueNode.Value}
		case yaml.SequenceNode:
			for _, v := range valueNode.Content {
				if v.Kind != yaml.ScalarNode {
					return fmt.Errorf("expected string values in array for key %q, but got %v", key, v.Kind)
				}
				values = append(values, v.Value)
			}
		default:
			return fmt.Errorf("expected a string or array of strings for key %q, but got %v", key, valueNode.Kind)
		}

		*kv = append(*kv, types.KeyValue[string, []string]{Key: key, Value: values})
	}
	return nil
}

type configYAML struct {
	ShowConfig      *bool              `yaml:"showConfig"`
	ConfigFiles     stringOrSliceField `yaml:"configFile"`
	Concurrency     *uint              `yaml:"concurrency"`
	RequestCount    *uint64            `yaml:"requests"`
	Duration        *time.Duration     `yaml:"duration"`
	LogLevel        *string            `yaml:"logLevel"`
	LogFile         *string            `yaml:"logFile"`
	Progress        *string            `yaml:"progress"`
	Output          *string            `yaml:"output"`
	DryRun          *bool              `yaml:"dryRun"`
	URL             *string            `yaml:"url"`
	Method          stringOrSliceField `yaml:"method"`
	Bodies          stringOrSliceField `yaml:"body"`
	Params          keyValuesField     `yaml:"params"`
	Headers         keyValuesField     `yaml:"headers"`
	Cookies         keyValuesField     `yaml:"cookies"`
	Proxies         stringOrSliceField `yaml:"proxy"`
	Values          stringOrSliceField `yaml:"values"`
	Timeout         *time.Duration     `yaml:"timeout"`
	MaxResponseBody *byteSize          `yaml:"maxResponseBody"`
	Insecure        *bool              `yaml:"insecure"`
	Lua             stringOrSliceField `yaml:"lua"`
	Js              stringOrSliceField `yaml:"js"`
}

// ParseYAML parses YAML config file arguments into a Config object.
// It can return the following errors:
// - types.UnmarshalError
// - types.FieldParseErrors
func (parser ConfigFileParser) ParseYAML(data []byte) (*Config, error) {
	var (
		config     = &Config{}
		parsedData = &configYAML{}
	)

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	// The decoder returns a bare io.EOF for an empty file, errors.Is would also match a wrapped one.
	if err := decoder.Decode(parsedData); err != nil && err != io.EOF { //nolint:errorlint
		return nil, types.NewUnmarshalError(yamlError(err))
	}

	// Decode reads one document, so anything after the first "---" would be dropped silently.
	switch err := decoder.Decode(&yaml.Node{}); {
	case err == nil:
		return nil, types.NewUnmarshalError(types.ErrYAMLMultipleDocuments)
	case err != io.EOF: //nolint:errorlint
		return nil, types.NewUnmarshalError(yamlError(err))
	}

	var fieldParseErrors []types.FieldParseError

	config.ShowConfig = parsedData.ShowConfig

	if len(parsedData.ConfigFiles) > 0 {
		for _, configFile := range parsedData.ConfigFiles {
			config.Files = append(config.Files, *types.ParseConfigFile(configFile))
		}
	}

	config.Concurrency = parsedData.Concurrency
	config.Requests = parsedData.RequestCount
	config.Duration = parsedData.Duration
	config.LogLevel = parsedData.LogLevel
	config.LogFile = parsedData.LogFile

	if parsedData.Progress != nil {
		config.Progress = new(ConfigProgressType(*parsedData.Progress))
	}

	if parsedData.Output != nil {
		config.Output = new(ConfigOutputType(*parsedData.Output))
	}

	config.DryRun = parsedData.DryRun

	if parsedData.URL != nil {
		urlParsed, err := url.Parse(*parsedData.URL)
		if err != nil {
			fieldParseErrors = append(fieldParseErrors, types.NewFieldParseError("url", *parsedData.URL, err))
		} else {
			config.URL = urlParsed
		}
	}

	config.Methods = append(config.Methods, parsedData.Method...)
	config.Bodies = append(config.Bodies, parsedData.Bodies...)
	for _, kv := range parsedData.Params {
		config.Params = append(config.Params, types.Param(kv))
	}
	for _, kv := range parsedData.Headers {
		config.Headers = append(config.Headers, types.Header(kv))
	}
	for _, kv := range parsedData.Cookies {
		config.Cookies = append(config.Cookies, types.Cookie(kv))
	}

	for i, proxy := range parsedData.Proxies {
		err := config.Proxies.Parse(proxy)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(fmt.Sprintf("proxy[%d]", i), proxy, err),
			)
		}
	}

	config.Values = append(config.Values, parsedData.Values...)
	config.Timeout = parsedData.Timeout
	if parsedData.MaxResponseBody != nil {
		config.MaxResponseBody = new(uint64(*parsedData.MaxResponseBody))
	}
	config.Insecure = parsedData.Insecure
	config.Lua = append(config.Lua, parsedData.Lua...)
	config.Js = append(config.Js, parsedData.Js...)

	if len(fieldParseErrors) > 0 {
		return nil, types.NewFieldParseErrors(fieldParseErrors)
	}

	return config, nil
}

// configYAMLTypeName is the Go type name the yaml package puts in its messages.
var configYAMLTypeName = reflect.TypeFor[configYAML]().String()

// yamlError rewrites yaml decode errors so they name config keys instead of Go types.
func yamlError(err error) error {
	var loadErrors *yaml.LoadErrors
	if !errors.As(err, &loadErrors) || len(loadErrors.Errors) == 0 {
		return err
	}

	var builder strings.Builder
	for i, loadError := range loadErrors.Errors {
		if i > 0 {
			builder.WriteString("\n")
		}
		if loadError.Mark.Line > 0 {
			fmt.Fprintf(&builder, "line %d: ", loadError.Mark.Line)
		}
		builder.WriteString(yamlErrorMessage(loadError.Message))
	}

	return errors.New(builder.String())
}

// yamlErrorMessage turns "field x not found in type config.configYAML" into "unknown key "x"".
func yamlErrorMessage(message string) string {
	if field, ok := strings.CutPrefix(message, "field "); ok {
		if name, _, ok := strings.Cut(field, " not found in type "); ok {
			return fmt.Sprintf("unknown key %q", name)
		}
	}
	return strings.ReplaceAll(message, configYAMLTypeName, "the config file")
}
