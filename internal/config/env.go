package config

import (
	"errors"
	"net/url"
	"os"
	"time"

	"go.aykhans.me/sarin/internal/types"
	"go.aykhans.me/utils/common"
	utilsParse "go.aykhans.me/utils/parser"
)

var _ IParser = ConfigENVParser{}

type ConfigENVParser struct {
	envPrefix string
}

func NewConfigENVParser(envPrefix string) *ConfigENVParser {
	return &ConfigENVParser{envPrefix}
}

// Parse parses env arguments into a Config object.
// It can return the following errors:
// - types.FieldParseErrors
func (parser ConfigENVParser) Parse() (*Config, error) {
	var (
		config           = &Config{}
		fieldParseErrors []types.FieldParseError
	)

	if showConfig := parser.getEnv("SHOW_CONFIG"); showConfig != "" {
		showConfigParsed, err := utilsParse.ParseString[bool](showConfig)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("SHOW_CONFIG"),
					showConfig,
					errors.New("invalid value for boolean, expected 'true' or 'false'"),
				),
			)
		} else {
			config.ShowConfig = &showConfigParsed
		}
	}

	if configFile := parser.getEnv("CONFIG_FILE"); configFile != "" {
		config.Files = append(config.Files, *types.ParseConfigFile(configFile))
	}

	if quiet := parser.getEnv("QUIET"); quiet != "" {
		quietParsed, err := utilsParse.ParseString[bool](quiet)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("QUIET"),
					quiet,
					errors.New("invalid value for boolean, expected 'true' or 'false'"),
				),
			)
		} else {
			config.Quiet = &quietParsed
		}
	}

	if output := parser.getEnv("OUTPUT"); output != "" {
		config.Output = common.ToPtr(ConfigOutputType(output))
	}

	if insecure := parser.getEnv("INSECURE"); insecure != "" {
		insecureParsed, err := utilsParse.ParseString[bool](insecure)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("INSECURE"),
					insecure,
					errors.New("invalid value for boolean, expected 'true' or 'false'"),
				),
			)
		} else {
			config.Insecure = &insecureParsed
		}
	}

	if dryRun := parser.getEnv("DRY_RUN"); dryRun != "" {
		dryRunParsed, err := utilsParse.ParseString[bool](dryRun)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("DRY_RUN"),
					dryRun,
					errors.New("invalid value for boolean, expected 'true' or 'false'"),
				),
			)
		} else {
			config.DryRun = &dryRunParsed
		}
	}

	if method := parser.getEnv("METHOD"); method != "" {
		config.Methods = []string{method}
	}

	if urlEnv := parser.getEnv("URL"); urlEnv != "" {
		urlEnvParsed, err := url.Parse(urlEnv)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(parser.getFullEnvName("URL"), urlEnv, err),
			)
		} else {
			config.URL = urlEnvParsed
		}
	}

	if concurrency := parser.getEnv("CONCURRENCY"); concurrency != "" {
		concurrencyParsed, err := utilsParse.ParseString[uint](concurrency)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("CONCURRENCY"),
					concurrency,
					errors.New("invalid value for unsigned integer"),
				),
			)
		} else {
			config.Concurrency = &concurrencyParsed
		}
	}

	if requests := parser.getEnv("REQUESTS"); requests != "" {
		requestsParsed, err := utilsParse.ParseString[uint64](requests)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("REQUESTS"),
					requests,
					errors.New("invalid value for unsigned integer"),
				),
			)
		} else {
			config.Requests = &requestsParsed
		}
	}

	if duration := parser.getEnv("DURATION"); duration != "" {
		durationParsed, err := utilsParse.ParseString[time.Duration](duration)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("DURATION"),
					duration,
					errors.New("invalid value duration, expected a duration string (e.g., '10s', '1h30m')"),
				),
			)
		} else {
			config.Duration = &durationParsed
		}
	}

	if timeout := parser.getEnv("TIMEOUT"); timeout != "" {
		timeoutParsed, err := utilsParse.ParseString[time.Duration](timeout)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("TIMEOUT"),
					timeout,
					errors.New("invalid value duration, expected a duration string (e.g., '10s', '1h30m')"),
				),
			)
		} else {
			config.Timeout = &timeoutParsed
		}
	}

	if param := parser.getEnv("PARAM"); param != "" {
		config.Params.Parse(param)
	}

	if header := parser.getEnv("HEADER"); header != "" {
		config.Headers.Parse(header)
	}

	if cookie := parser.getEnv("COOKIE"); cookie != "" {
		config.Cookies.Parse(cookie)
	}

	if body := parser.getEnv("BODY"); body != "" {
		config.Bodies = []string{body}
	}

	if proxy := parser.getEnv("PROXY"); proxy != "" {
		err := config.Proxies.Parse(proxy)
		if err != nil {
			fieldParseErrors = append(
				fieldParseErrors,
				types.NewFieldParseError(
					parser.getFullEnvName("PROXY"),
					proxy,
					err,
				),
			)
		}
	}

	if values := parser.getEnv("VALUES"); values != "" {
		config.Values = []string{values}
	}

	if lua := parser.getEnv("LUA"); lua != "" {
		config.Lua = []string{lua}
	}

	if js := parser.getEnv("JS"); js != "" {
		config.Js = []string{js}
	}

	if len(fieldParseErrors) > 0 {
		return nil, types.NewFieldParseErrors(fieldParseErrors)
	}

	return config, nil
}

func (parser ConfigENVParser) getFullEnvName(envName string) string {
	if parser.envPrefix == "" {
		return envName
	}
	return parser.envPrefix + "_" + envName
}

func (parser ConfigENVParser) getEnv(envName string) string {
	return os.Getenv(parser.getFullEnvName(envName))
}
