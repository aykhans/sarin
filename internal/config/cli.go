package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"go.aykhans.me/sarin/internal/types"
	versionpkg "go.aykhans.me/sarin/internal/version"
	"go.aykhans.me/utils/common"
)

const cliUsageText = `Usage:
  sarin [flags]

Simple usage:
  sarin -U https://example.com -r 1

Flags:
  General Config:
    -h, -help                     Help for sarin
    -v, -version                  Version for sarin
    -s, -show-config   bool       Show the final config after parsing all sources (default %v)
    -f, -config-file   string     Path to the config file (local file / http URL)
    -c, -concurrency   uint       Number of concurrent requests (default %d)
    -r, -requests      uint       Number of total requests
    -d, -duration      time       Maximum duration for the test (e.g. 30s, 1m, 5h)
    -q, -quiet         bool       Hide the progress bar and runtime logs (default %v)
    -o, -output        string     Output format (possible values: table, json, yaml, none) (default '%v')
    -z, -dry-run       bool       Run without sending requests (default %v)

  Request Config:
    -U, -url           string     Target URL for the request
    -M, -method        []string   HTTP method for the request (default %s)
    -B, -body          []string   Body for the request (e.g. "body text")
    -P, -param         []string   URL parameter for the request (e.g. "key1=value1")
    -H, -header        []string   Header for the request (e.g. "key1: value1")
    -C, -cookie        []string   Cookie for the request (e.g. "key1=value1")
    -X, -proxy         []string   Proxy for the request (e.g. "http://proxy.example.com:8080")
    -V, -values        []string   List of values for templating (e.g. "key1=value1")
    -T, -timeout       time       Timeout for the request (e.g. 400ms, 3s, 1m10s) (default %v)
    -I, -insecure      bool       Skip SSL/TLS certificate verification (default %v)
    -lua               []string   Lua script for request transformation (inline or @file/@url)
    -js                []string   JavaScript script for request transformation (inline or @file/@url)`

var _ IParser = ConfigCLIParser{}

type ConfigCLIParser struct {
	args []string
}

func NewConfigCLIParser(args []string) *ConfigCLIParser {
	if args == nil {
		args = []string{}
	}
	return &ConfigCLIParser{args: args}
}

type stringSliceArg []string

func (arg *stringSliceArg) String() string {
	return strings.Join(*arg, ",")
}

func (arg *stringSliceArg) Set(value string) error {
	*arg = append(*arg, value)
	return nil
}

// Parse parses command-line arguments into a Config object.
// It can return the following errors:
// - types.ErrCLINoArgs
// - types.CLIUnexpectedArgsError
// - types.FieldParseErrors
func (parser ConfigCLIParser) Parse() (*Config, error) {
	flagSet := flag.NewFlagSet("sarin", flag.ExitOnError)

	flagSet.Usage = func() { parser.PrintHelp() }

	var (
		config = &Config{}

		// General config
		version      bool
		showConfig   bool
		configFiles  = stringSliceArg{}
		concurrency  uint
		requestCount uint64
		duration     time.Duration
		quiet        bool
		output       string
		dryRun       bool

		// Request config
		urlInput   string
		methods    = stringSliceArg{}
		bodies     = stringSliceArg{}
		params     = stringSliceArg{}
		headers    = stringSliceArg{}
		cookies    = stringSliceArg{}
		proxies    = stringSliceArg{}
		values     = stringSliceArg{}
		timeout    time.Duration
		insecure   bool
		luaScripts = stringSliceArg{}
		jsScripts  = stringSliceArg{}
	)

	{
		// General config
		flagSet.BoolVar(&version, "version", false, "Version for sarin")
		flagSet.BoolVar(&version, "v", false, "Version for sarin")

		flagSet.BoolVar(&showConfig, "show-config", false, "Show the final config after parsing all sources")
		flagSet.BoolVar(&showConfig, "s", false, "Show the final config after parsing all sources")

		flagSet.Var(&configFiles, "config-file", "Path to the config file")
		flagSet.Var(&configFiles, "f", "Path to the config file")

		flagSet.UintVar(&concurrency, "concurrency", 0, "Number of concurrent requests")
		flagSet.UintVar(&concurrency, "c", 0, "Number of concurrent requests")

		flagSet.Uint64Var(&requestCount, "requests", 0, "Number of total requests")
		flagSet.Uint64Var(&requestCount, "r", 0, "Number of total requests")

		flagSet.DurationVar(&duration, "duration", 0, "Maximum duration for the test")
		flagSet.DurationVar(&duration, "d", 0, "Maximum duration for the test")

		flagSet.BoolVar(&quiet, "quiet", false, "Hide the progress bar and runtime logs")
		flagSet.BoolVar(&quiet, "q", false, "Hide the progress bar and runtime logs")

		flagSet.StringVar(&output, "output", "", "Output format (possible values: table, json, yaml, none)")
		flagSet.StringVar(&output, "o", "", "Output format (possible values: table, json, yaml, none)")

		flagSet.BoolVar(&dryRun, "dry-run", false, "Run without sending requests")
		flagSet.BoolVar(&dryRun, "z", false, "Run without sending requests")

		// Request config
		flagSet.StringVar(&urlInput, "url", "", "Target URL for the request")
		flagSet.StringVar(&urlInput, "U", "", "Target URL for the request")

		flagSet.Var(&methods, "method", "HTTP method for the request")
		flagSet.Var(&methods, "M", "HTTP method for the request")

		flagSet.Var(&bodies, "body", "Body for the request")
		flagSet.Var(&bodies, "B", "Body for the request")

		flagSet.Var(&params, "param", "URL parameter for the request")
		flagSet.Var(&params, "P", "URL parameter for the request")

		flagSet.Var(&headers, "header", "Header for the request")
		flagSet.Var(&headers, "H", "Header for the request")

		flagSet.Var(&cookies, "cookie", "Cookie for the request")
		flagSet.Var(&cookies, "C", "Cookie for the request")

		flagSet.Var(&proxies, "proxy", "Proxy for the request")
		flagSet.Var(&proxies, "X", "Proxy for the request")

		flagSet.Var(&values, "values", "List of values for templating")
		flagSet.Var(&values, "V", "List of values for templating")

		flagSet.DurationVar(&timeout, "timeout", 0, "Timeout for the request (e.g. 400ms, 15s, 1m10s)")
		flagSet.DurationVar(&timeout, "T", 0, "Timeout for the request (e.g. 400ms, 15s, 1m10s)")

		flagSet.BoolVar(&insecure, "insecure", false, "Skip SSL/TLS certificate verification")
		flagSet.BoolVar(&insecure, "I", false, "Skip SSL/TLS certificate verification")

		flagSet.Var(&luaScripts, "lua", "Lua script for request transformation (inline or @file/@url)")

		flagSet.Var(&jsScripts, "js", "JavaScript script for request transformation (inline or @file/@url)")
	}

	// Parse the specific arguments provided to the parser, skipping the program name.
	if err := flagSet.Parse(parser.args[1:]); err != nil {
		panic(err)
	}

	// Check if no flags were set and no non-flag arguments were provided.
	// This covers cases where `sarin` is run without any meaningful arguments.
	if flagSet.NFlag() == 0 && len(flagSet.Args()) == 0 {
		return nil, types.ErrCLINoArgs
	}

	// Check for any unexpected non-flag arguments remaining after parsing.
	if args := flagSet.Args(); len(args) > 0 {
		return nil, types.NewCLIUnexpectedArgsError(args)
	}

	if version {
		fmt.Printf("Version: %s\nGit Commit: %s\nBuild Date: %s\nGo Version: %s\n",
			versionpkg.Version, versionpkg.GitCommit, versionpkg.BuildDate, versionpkg.GoVersion)
		os.Exit(0)
	}

	var fieldParseErrors []types.FieldParseError
	// Iterate over flags that were explicitly set on the command line.
	flagSet.Visit(func(flagVar *flag.Flag) {
		switch flagVar.Name {
		// General config
		case "show-config", "s":
			config.ShowConfig = common.ToPtr(showConfig)
		case "config-file", "f":
			for _, configFile := range configFiles {
				config.Files = append(config.Files, *types.ParseConfigFile(configFile))
			}
		case "concurrency", "c":
			config.Concurrency = common.ToPtr(concurrency)
		case "requests", "r":
			config.Requests = common.ToPtr(requestCount)
		case "duration", "d":
			config.Duration = common.ToPtr(duration)
		case "quiet", "q":
			config.Quiet = common.ToPtr(quiet)
		case "output", "o":
			config.Output = common.ToPtr(ConfigOutputType(output))
		case "dry-run", "z":
			config.DryRun = common.ToPtr(dryRun)

		// Request config
		case "url", "U":
			urlParsed, err := url.Parse(urlInput)
			if err != nil {
				fieldParseErrors = append(fieldParseErrors, types.NewFieldParseError("url", urlInput, err))
			} else {
				config.URL = urlParsed
			}
		case "method", "M":
			config.Methods = append(config.Methods, methods...)
		case "body", "B":
			config.Bodies = append(config.Bodies, bodies...)
		case "param", "P":
			config.Params.Parse(params...)
		case "header", "H":
			config.Headers.Parse(headers...)
		case "cookie", "C":
			config.Cookies.Parse(cookies...)
		case "proxy", "X":
			for i, proxy := range proxies {
				err := config.Proxies.Parse(proxy)
				if err != nil {
					fieldParseErrors = append(
						fieldParseErrors,
						types.NewFieldParseError(fmt.Sprintf("proxy[%d]", i), proxy, err),
					)
				}
			}
		case "values", "V":
			config.Values = append(config.Values, values...)
		case "timeout", "T":
			config.Timeout = common.ToPtr(timeout)
		case "insecure", "I":
			config.Insecure = common.ToPtr(insecure)
		case "lua":
			config.Lua = append(config.Lua, luaScripts...)
		case "js":
			config.Js = append(config.Js, jsScripts...)
		}
	})

	if len(fieldParseErrors) > 0 {
		return nil, types.NewFieldParseErrors(fieldParseErrors)
	}

	return config, nil
}

func (parser ConfigCLIParser) PrintHelp() {
	fmt.Printf(
		cliUsageText+"\n",
		Defaults.ShowConfig,
		Defaults.Concurrency,
		Defaults.Quiet,
		Defaults.Output,
		Defaults.DryRun,

		Defaults.Method,
		Defaults.RequestTimeout,
		Defaults.Insecure,
	)
}
