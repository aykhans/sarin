package config

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.aykhans.me/sarin/internal/types"
	"go.aykhans.me/utils/common"
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewHTTPFetchError(url, err)
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
	ConfigFiles  stringOrSliceField `yaml:"configFile"`
	Method       stringOrSliceField `yaml:"method"`
	URL          *string            `yaml:"url"`
	Timeout      *time.Duration     `yaml:"timeout"`
	Concurrency  *uint              `yaml:"concurrency"`
	RequestCount *uint64            `yaml:"requests"`
	Duration     *time.Duration     `yaml:"duration"`
	Quiet        *bool              `yaml:"quiet"`
	Output       *string            `yaml:"output"`
	Insecure     *bool              `yaml:"insecure"`
	ShowConfig   *bool              `yaml:"showConfig"`
	DryRun       *bool              `yaml:"dryRun"`
	Params       keyValuesField     `yaml:"params"`
	Headers      keyValuesField     `yaml:"headers"`
	Cookies      keyValuesField     `yaml:"cookies"`
	Bodies       stringOrSliceField `yaml:"body"`
	Proxies      stringOrSliceField `yaml:"proxy"`
	Values       stringOrSliceField `yaml:"values"`
	Lua          stringOrSliceField `yaml:"lua"`
	Js           stringOrSliceField `yaml:"js"`
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

	err := yaml.Unmarshal(data, &parsedData)
	if err != nil {
		return nil, types.NewUnmarshalError(err)
	}

	var fieldParseErrors []types.FieldParseError

	config.Methods = append(config.Methods, parsedData.Method...)
	config.Timeout = parsedData.Timeout
	config.Concurrency = parsedData.Concurrency
	config.Requests = parsedData.RequestCount
	config.Duration = parsedData.Duration
	config.ShowConfig = parsedData.ShowConfig
	config.Quiet = parsedData.Quiet

	if parsedData.Output != nil {
		config.Output = common.ToPtr(ConfigOutputType(*parsedData.Output))
	}

	config.Insecure = parsedData.Insecure
	config.DryRun = parsedData.DryRun
	for _, kv := range parsedData.Params {
		config.Params = append(config.Params, types.Param(kv))
	}
	for _, kv := range parsedData.Headers {
		config.Headers = append(config.Headers, types.Header(kv))
	}
	for _, kv := range parsedData.Cookies {
		config.Cookies = append(config.Cookies, types.Cookie(kv))
	}
	config.Bodies = append(config.Bodies, parsedData.Bodies...)
	config.Values = append(config.Values, parsedData.Values...)
	config.Lua = append(config.Lua, parsedData.Lua...)
	config.Js = append(config.Js, parsedData.Js...)

	if len(parsedData.ConfigFiles) > 0 {
		for _, configFile := range parsedData.ConfigFiles {
			config.Files = append(config.Files, *types.ParseConfigFile(configFile))
		}
	}

	if parsedData.URL != nil {
		urlParsed, err := url.Parse(*parsedData.URL)
		if err != nil {
			fieldParseErrors = append(fieldParseErrors, types.NewFieldParseError("url", *parsedData.URL, err))
		} else {
			config.URL = urlParsed
		}
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

	if len(fieldParseErrors) > 0 {
		return nil, types.NewFieldParseErrors(fieldParseErrors)
	}

	return config, nil
}
