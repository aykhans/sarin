package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"go.aykhans.me/sarin/internal/script"
	"go.aykhans.me/sarin/internal/types"
	"go.aykhans.me/sarin/internal/version"
	"go.aykhans.me/utils/common"
	utilsErr "go.aykhans.me/utils/errors"
	"go.yaml.in/yaml/v4"
)

var Defaults = struct {
	UserAgent      string
	Method         string
	RequestTimeout time.Duration
	Concurrency    uint
	ShowConfig     bool
	Quiet          bool
	Insecure       bool
	Output         ConfigOutputType
	DryRun         bool
}{
	UserAgent:      "Sarin/" + version.Version,
	Method:         "GET",
	RequestTimeout: time.Second * 10,
	Concurrency:    1,
	ShowConfig:     false,
	Quiet:          false,
	Insecure:       false,
	Output:         ConfigOutputTypeTable,
	DryRun:         false,
}

var (
	ValidProxySchemes      = []string{"http", "https", "socks5", "socks5h"}
	ValidRequestURLSchemes = []string{"http", "https"}
)

var (
	StyleYellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	StyleRed    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type IParser interface {
	Parse() (*Config, error)
}

type ConfigOutputType string

var (
	ConfigOutputTypeTable ConfigOutputType = "table"
	ConfigOutputTypeJSON  ConfigOutputType = "json"
	ConfigOutputTypeYAML  ConfigOutputType = "yaml"
	ConfigOutputTypeNone  ConfigOutputType = "none"
)

type Config struct {
	ShowConfig  *bool              `yaml:"showConfig,omitempty"`
	Files       []types.ConfigFile `yaml:"files,omitempty"`
	Methods     []string           `yaml:"methods,omitempty"`
	URL         *url.URL           `yaml:"url,omitempty"`
	Timeout     *time.Duration     `yaml:"timeout,omitempty"`
	Concurrency *uint              `yaml:"concurrency,omitempty"`
	Requests    *uint64            `yaml:"requests,omitempty"`
	Duration    *time.Duration     `yaml:"duration,omitempty"`
	Quiet       *bool              `yaml:"quiet,omitempty"`
	Output      *ConfigOutputType  `yaml:"output,omitempty"`
	Insecure    *bool              `yaml:"insecure,omitempty"`
	DryRun      *bool              `yaml:"dryRun,omitempty"`
	Params      types.Params       `yaml:"params,omitempty"`
	Headers     types.Headers      `yaml:"headers,omitempty"`
	Cookies     types.Cookies      `yaml:"cookies,omitempty"`
	Bodies      []string           `yaml:"bodies,omitempty"`
	Proxies     types.Proxies      `yaml:"proxies,omitempty"`
	Values      []string           `yaml:"values,omitempty"`
	Lua         []string           `yaml:"lua,omitempty"`
	Js          []string           `yaml:"js,omitempty"`
}

func NewConfig() *Config {
	return &Config{}
}

func (config Config) MarshalYAML() (any, error) {
	const randomValueComment = "Cycles through all values, with a new random start each round"

	toNode := func(v any) *yaml.Node {
		node := &yaml.Node{}
		_ = node.Encode(v)
		return node
	}

	addField := func(content *[]*yaml.Node, key string, value *yaml.Node, comment string) {
		if value.Kind == 0 || (value.Kind == yaml.ScalarNode && value.Value == "") ||
			(value.Kind == yaml.SequenceNode && len(value.Content) == 0) {
			return
		}
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, LineComment: comment}
		*content = append(*content, keyNode, value)
	}

	addStringSlice := func(content *[]*yaml.Node, key string, items []string, withComment bool) {
		comment := ""
		if withComment && len(items) > 1 {
			comment = randomValueComment
		}
		switch len(items) {
		case 1:
			addField(content, key, toNode(items[0]), "")
		default:
			addField(content, key, toNode(items), comment)
		}
	}

	marshalKeyValues := func(items []types.KeyValue[string, []string]) *yaml.Node {
		seqNode := &yaml.Node{Kind: yaml.SequenceNode}
		for _, item := range items {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: item.Key}
			var valueNode *yaml.Node

			switch len(item.Value) {
			case 1:
				valueNode = &yaml.Node{Kind: yaml.ScalarNode, Value: item.Value[0]}
			default:
				valueNode = &yaml.Node{Kind: yaml.SequenceNode}
				for _, v := range item.Value {
					valueNode.Content = append(valueNode.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
				}
				if len(item.Value) > 1 {
					keyNode.LineComment = randomValueComment
				}
			}

			mapNode := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{keyNode, valueNode}}
			seqNode.Content = append(seqNode.Content, mapNode)
		}
		return seqNode
	}

	root := &yaml.Node{Kind: yaml.MappingNode}
	content := &root.Content

	if config.ShowConfig != nil {
		addField(content, "showConfig", toNode(*config.ShowConfig), "")
	}

	addStringSlice(content, "method", config.Methods, true)

	if config.URL != nil {
		addField(content, "url", toNode(config.URL.String()), "")
	}
	if config.Timeout != nil {
		addField(content, "timeout", toNode(*config.Timeout), "")
	}
	if config.Concurrency != nil {
		addField(content, "concurrency", toNode(*config.Concurrency), "")
	}
	if config.Requests != nil {
		addField(content, "requests", toNode(*config.Requests), "")
	}
	if config.Duration != nil {
		addField(content, "duration", toNode(*config.Duration), "")
	}
	if config.Quiet != nil {
		addField(content, "quiet", toNode(*config.Quiet), "")
	}
	if config.Output != nil {
		addField(content, "output", toNode(string(*config.Output)), "")
	}
	if config.Insecure != nil {
		addField(content, "insecure", toNode(*config.Insecure), "")
	}
	if config.DryRun != nil {
		addField(content, "dryRun", toNode(*config.DryRun), "")
	}

	if len(config.Params) > 0 {
		items := make([]types.KeyValue[string, []string], len(config.Params))
		for i, p := range config.Params {
			items[i] = types.KeyValue[string, []string](p)
		}
		addField(content, "params", marshalKeyValues(items), "")
	}
	if len(config.Headers) > 0 {
		items := make([]types.KeyValue[string, []string], len(config.Headers))
		for i, h := range config.Headers {
			items[i] = types.KeyValue[string, []string](h)
		}
		addField(content, "headers", marshalKeyValues(items), "")
	}
	if len(config.Cookies) > 0 {
		items := make([]types.KeyValue[string, []string], len(config.Cookies))
		for i, c := range config.Cookies {
			items[i] = types.KeyValue[string, []string](c)
		}
		addField(content, "cookies", marshalKeyValues(items), "")
	}

	addStringSlice(content, "body", config.Bodies, true)

	if len(config.Proxies) > 0 {
		proxyStrings := make([]string, len(config.Proxies))
		for i, p := range config.Proxies {
			proxyStrings[i] = p.String()
		}
		addStringSlice(content, "proxy", proxyStrings, true)
	}

	addStringSlice(content, "values", config.Values, false)
	addStringSlice(content, "lua", config.Lua, false)
	addStringSlice(content, "js", config.Js, false)

	return root, nil
}

func (config Config) Print() bool {
	configYAML, err := yaml.Marshal(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, StyleRed.Render("Error marshaling config to yaml: "+err.Error()))
		os.Exit(1)
	}

	// Pipe mode: output raw content directly
	if !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Println(string(configYAML))
		os.Exit(0)
	}

	style := styles.TokyoNightStyleConfig
	style.Document.Margin = common.ToPtr[uint](0)
	style.CodeBlock.Margin = common.ToPtr[uint](0)

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, StyleRed.Render(err.Error()))
		os.Exit(1)
	}

	content, err := renderer.Render("```yaml\n" + string(configYAML) + "```")
	if err != nil {
		fmt.Fprintln(os.Stderr, StyleRed.Render(err.Error()))
		os.Exit(1)
	}

	p := tea.NewProgram(
		printConfigModel{content: strings.Trim(content, "\n"), rawContent: configYAML},
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	m, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, StyleRed.Render(err.Error()))
		os.Exit(1)
	}

	return m.(printConfigModel).start //nolint:forcetypeassert // m is guaranteed to be of type printConfigModel as it was the only model passed to tea.NewProgram
}

func (config *Config) Merge(newConfig *Config) {
	config.Files = append(config.Files, newConfig.Files...)
	if len(newConfig.Methods) > 0 {
		config.Methods = append(config.Methods, newConfig.Methods...)
	}
	if newConfig.URL != nil {
		config.URL = newConfig.URL
	}
	if newConfig.Timeout != nil {
		config.Timeout = newConfig.Timeout
	}
	if newConfig.Concurrency != nil {
		config.Concurrency = newConfig.Concurrency
	}
	if newConfig.Requests != nil {
		config.Requests = newConfig.Requests
	}
	if newConfig.Duration != nil {
		config.Duration = newConfig.Duration
	}
	if newConfig.ShowConfig != nil {
		config.ShowConfig = newConfig.ShowConfig
	}
	if newConfig.Quiet != nil {
		config.Quiet = newConfig.Quiet
	}
	if newConfig.Output != nil {
		config.Output = newConfig.Output
	}
	if newConfig.Insecure != nil {
		config.Insecure = newConfig.Insecure
	}
	if newConfig.DryRun != nil {
		config.DryRun = newConfig.DryRun
	}
	if len(newConfig.Params) != 0 {
		config.Params = append(config.Params, newConfig.Params...)
	}
	if len(newConfig.Headers) != 0 {
		config.Headers = append(config.Headers, newConfig.Headers...)
	}
	if len(newConfig.Cookies) != 0 {
		config.Cookies = append(config.Cookies, newConfig.Cookies...)
	}
	if len(newConfig.Bodies) != 0 {
		config.Bodies = append(config.Bodies, newConfig.Bodies...)
	}
	if len(newConfig.Proxies) != 0 {
		config.Proxies.Append(newConfig.Proxies...)
	}
	if len(newConfig.Values) != 0 {
		config.Values = append(config.Values, newConfig.Values...)
	}
	if len(newConfig.Lua) != 0 {
		config.Lua = append(config.Lua, newConfig.Lua...)
	}
	if len(newConfig.Js) != 0 {
		config.Js = append(config.Js, newConfig.Js...)
	}
}

func (config *Config) SetDefaults() {
	if config.URL != nil && len(config.URL.Query()) > 0 {
		urlParams := types.Params{}
		for key, values := range config.URL.Query() {
			for _, value := range values {
				urlParams = append(urlParams, types.Param{
					Key:   key,
					Value: []string{value},
				})
			}
		}

		config.Params = append(urlParams, config.Params...)
		config.URL.RawQuery = ""
	}

	if len(config.Methods) == 0 {
		config.Methods = []string{Defaults.Method}
	}
	if config.Timeout == nil {
		config.Timeout = &Defaults.RequestTimeout
	}
	if config.Concurrency == nil {
		config.Concurrency = common.ToPtr(Defaults.Concurrency)
	}
	if config.ShowConfig == nil {
		config.ShowConfig = common.ToPtr(Defaults.ShowConfig)
	}
	if config.Quiet == nil {
		config.Quiet = common.ToPtr(Defaults.Quiet)
	}
	if config.Insecure == nil {
		config.Insecure = common.ToPtr(Defaults.Insecure)
	}
	if config.DryRun == nil {
		config.DryRun = common.ToPtr(Defaults.DryRun)
	}
	if !config.Headers.Has("User-Agent") {
		config.Headers = append(config.Headers, types.Header{Key: "User-Agent", Value: []string{Defaults.UserAgent}})
	}

	if config.Output == nil {
		config.Output = common.ToPtr(Defaults.Output)
	}
}

// Validate validates the config fields.
// It can return the following errors:
// - types.FieldValidationErrors
func (config Config) Validate() error {
	validationErrors := make([]types.FieldValidationError, 0)

	if len(config.Methods) == 0 {
		validationErrors = append(validationErrors, types.NewFieldValidationError("Method", "", errors.New("method is required")))
	}

	switch {
	case config.URL == nil:
		validationErrors = append(validationErrors, types.NewFieldValidationError("URL", "", errors.New("URL is required")))
	case !slices.Contains(ValidRequestURLSchemes, config.URL.Scheme):
		validationErrors = append(validationErrors, types.NewFieldValidationError("URL", config.URL.String(), fmt.Errorf("URL scheme must be one of: %s", strings.Join(ValidRequestURLSchemes, ", "))))
	case config.URL.Host == "":
		validationErrors = append(validationErrors, types.NewFieldValidationError("URL", config.URL.String(), errors.New("URL must have a host")))
	}

	switch {
	case config.Concurrency == nil:
		validationErrors = append(validationErrors, types.NewFieldValidationError("Concurrency", "", errors.New("concurrency count is required")))
	case *config.Concurrency == 0:
		validationErrors = append(validationErrors, types.NewFieldValidationError("Concurrency", "0", errors.New("concurrency must be greater than 0")))
	case *config.Concurrency > 100_000_000:
		validationErrors = append(validationErrors, types.NewFieldValidationError("Concurrency", strconv.FormatUint(uint64(*config.Concurrency), 10), errors.New("concurrency must not exceed 100,000,000")))
	}

	switch {
	case config.Requests == nil && config.Duration == nil:
		validationErrors = append(validationErrors, types.NewFieldValidationError("Requests / Duration", "", errors.New("either request count or duration must be specified")))
	case (config.Requests != nil && config.Duration != nil) && (*config.Requests == 0 && *config.Duration == 0):
		validationErrors = append(validationErrors, types.NewFieldValidationError("Requests / Duration", "0", errors.New("both request count and duration cannot be zero")))
	case config.Requests != nil && config.Duration == nil && *config.Requests == 0:
		validationErrors = append(validationErrors, types.NewFieldValidationError("Requests", "0", errors.New("request count must be greater than 0")))
	case config.Requests == nil && config.Duration != nil && *config.Duration == 0:
		validationErrors = append(validationErrors, types.NewFieldValidationError("Duration", "0", errors.New("duration must be greater than 0")))
	}

	if *config.Timeout < 1 {
		validationErrors = append(validationErrors, types.NewFieldValidationError("Timeout", "0", errors.New("timeout must be greater than 0")))
	}

	if config.ShowConfig == nil {
		validationErrors = append(validationErrors, types.NewFieldValidationError("ShowConfig", "", errors.New("showConfig field is required")))
	}

	if config.Quiet == nil {
		validationErrors = append(validationErrors, types.NewFieldValidationError("Quiet", "", errors.New("quiet field is required")))
	}

	if config.Output == nil {
		validationErrors = append(validationErrors, types.NewFieldValidationError("Output", "", errors.New("output field is required")))
	} else {
		switch *config.Output {
		case "":
			validationErrors = append(validationErrors, types.NewFieldValidationError("Output", "", errors.New("output field is required")))
		case ConfigOutputTypeTable, ConfigOutputTypeJSON, ConfigOutputTypeYAML, ConfigOutputTypeNone:
		default:
			validOutputs := []string{string(ConfigOutputTypeTable), string(ConfigOutputTypeJSON), string(ConfigOutputTypeYAML), string(ConfigOutputTypeNone)}
			validationErrors = append(validationErrors,
				types.NewFieldValidationError(
					"Output",
					string(*config.Output),
					fmt.Errorf(
						"output type must be one of: %s",
						strings.Join(validOutputs, ", "),
					),
				),
			)
		}
	}

	if config.Insecure == nil {
		validationErrors = append(validationErrors, types.NewFieldValidationError("Insecure", "", errors.New("insecure field is required")))
	}

	if config.DryRun == nil {
		validationErrors = append(validationErrors, types.NewFieldValidationError("DryRun", "", errors.New("dryRun field is required")))
	}

	for i, proxy := range config.Proxies {
		if !slices.Contains(ValidProxySchemes, proxy.Scheme) {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Proxy[%d]", i),
					proxy.String(),
					fmt.Errorf("proxy scheme must be one of: %v", ValidProxySchemes),
				),
			)
		}
	}

	// Create a context with timeout for script validation (loading from URLs)
	scriptCtx, scriptCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer scriptCancel()

	for i, scriptSrc := range config.Lua {
		if err := validateScriptSource(scriptSrc); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(fmt.Sprintf("Lua[%d]", i), scriptSrc, err),
			)
			continue
		}
		// Validate script syntax
		if err := script.ValidateScript(scriptCtx, scriptSrc, script.EngineTypeLua); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(fmt.Sprintf("Lua[%d]", i), scriptSrc, err),
			)
		}
	}

	for i, scriptSrc := range config.Js {
		if err := validateScriptSource(scriptSrc); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(fmt.Sprintf("Js[%d]", i), scriptSrc, err),
			)
			continue
		}
		// Validate script syntax
		if err := script.ValidateScript(scriptCtx, scriptSrc, script.EngineTypeJavaScript); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(fmt.Sprintf("Js[%d]", i), scriptSrc, err),
			)
		}
	}

	templateErrors := ValidateTemplates(&config)
	validationErrors = append(validationErrors, templateErrors...)

	if len(validationErrors) > 0 {
		return types.NewFieldValidationErrors(validationErrors)
	}

	return nil
}

func ReadAllConfigs() *Config {
	envParser := NewConfigENVParser("SARIN")
	envConfig, err := envParser.Parse()
	_ = utilsErr.MustHandle(err,
		utilsErr.OnType(func(err types.FieldParseErrors) error {
			printParseErrors("ENV", err.Errors...)
			fmt.Println()
			os.Exit(1)
			return nil
		}),
	)

	cliParser := NewConfigCLIParser(os.Args)
	cliConf, err := cliParser.Parse()
	_ = utilsErr.MustHandle(err,
		utilsErr.OnSentinel(types.ErrCLINoArgs, func(err error) error {
			cliParser.PrintHelp()
			fmt.Fprintln(os.Stderr, StyleYellow.Render("\nNo arguments provided."))
			os.Exit(1)
			return nil
		}),
		utilsErr.OnType(func(err types.CLIUnexpectedArgsError) error {
			cliParser.PrintHelp()
			fmt.Fprintln(os.Stderr,
				StyleYellow.Render(
					"\nUnexpected CLI arguments provided: ",
				)+strings.Join(err.Args, ", "),
			)
			os.Exit(1)
			return nil
		}),
		utilsErr.OnType(func(err types.FieldParseErrors) error {
			cliParser.PrintHelp()
			fmt.Println()
			printParseErrors("CLI", err.Errors...)
			os.Exit(1)
			return nil
		}),
	)

	for _, configFile := range append(envConfig.Files, cliConf.Files...) {
		fileConfig, err := parseConfigFile(configFile, 10)
		_ = utilsErr.MustHandle(err,
			utilsErr.OnType(func(err types.ConfigFileReadError) error {
				cliParser.PrintHelp()
				fmt.Fprintln(os.Stderr,
					StyleYellow.Render(
						fmt.Sprintf("\nFailed to read config file (%s): ", configFile.Path())+err.Error(),
					),
				)
				os.Exit(1)
				return nil
			}),
			utilsErr.OnType(func(err types.UnmarshalError) error {
				fmt.Fprintln(os.Stderr,
					StyleYellow.Render(
						fmt.Sprintf("\nFailed to parse config file (%s): ", configFile.Path())+err.Error(),
					),
				)
				os.Exit(1)
				return nil
			}),
			utilsErr.OnType(func(err types.FieldParseErrors) error {
				printParseErrors(fmt.Sprintf("CONFIG FILE '%s'", configFile.Path()), err.Errors...)
				os.Exit(1)
				return nil
			}),
		)

		envConfig.Merge(fileConfig)
	}

	envConfig.Merge(cliConf)

	return envConfig
}

// parseConfigFile recursively parses a config file and its nested files up to maxDepth levels.
// Returns the merged configuration or an error if parsing fails.
// It can return the following errors:
// - types.ConfigFileReadError
// - types.UnmarshalError
// - types.FieldParseErrors
func parseConfigFile(configFile types.ConfigFile, maxDepth int) (*Config, error) {
	configFileParser := NewConfigFileParser(configFile)
	fileConfig, err := configFileParser.Parse()
	if err != nil {
		return nil, err
	}

	if maxDepth <= 0 {
		return fileConfig, nil
	}

	for _, c := range fileConfig.Files {
		innerFileConfig, err := parseConfigFile(c, maxDepth-1)
		if err != nil {
			return nil, err
		}

		innerFileConfig.Merge(fileConfig)
		fileConfig = innerFileConfig
	}

	return fileConfig, nil
}

// validateScriptSource validates a script source string.
// Scripts can be:
//   - Inline script: any string not starting with "@"
//   - Escaped "@": strings starting with "@@" (literal "@" at start)
//   - File reference: "@/path/to/file" or "@./relative/path"
//   - URL reference: "@http://..." or "@https://..."
func validateScriptSource(script string) error {
	// Empty script is invalid
	if script == "" {
		return errors.New("script cannot be empty")
	}

	// Not a file/URL reference - it's an inline script
	if !strings.HasPrefix(script, "@") {
		return nil
	}

	// Escaped @ - it's an inline script starting with literal @
	if strings.HasPrefix(script, "@@") {
		return nil
	}

	// It's a file or URL reference - validate the source
	source := script[1:] // Remove the @ prefix

	if source == "" {
		return errors.New("script source cannot be empty after @")
	}

	// Check if it's a URL
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		parsedURL, err := url.Parse(source)
		if err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		if parsedURL.Host == "" {
			return errors.New("URL must have a host")
		}
		return nil
	}

	// It's a file path - basic validation (not empty, checked above)
	return nil
}

func printParseErrors(parserName string, errors ...types.FieldParseError) {
	for _, fieldErr := range errors {
		if fieldErr.Value == "" {
			fmt.Fprintln(os.Stderr,
				StyleYellow.Render(fmt.Sprintf("[%s] Field '%s': ", parserName, fieldErr.Field))+fieldErr.Err.Error(),
			)
		} else {
			fmt.Fprintln(os.Stderr,
				StyleYellow.Render(fmt.Sprintf("[%s] Field '%s' (%s): ", parserName, fieldErr.Field, fieldErr.Value))+fieldErr.Err.Error(),
			)
		}
	}
}

const (
	scrollbarWidth       = 1
	scrollbarBottomSpace = 1
	statusDisplayTime    = 3 * time.Second
)

var (
	printConfigBorderStyle = func() lipgloss.Border {
		b := lipgloss.RoundedBorder()
		return b
	}()

	printConfigHelpStyle          = lipgloss.NewStyle().BorderStyle(printConfigBorderStyle).Padding(0, 1)
	printConfigSuccessStatusStyle = lipgloss.NewStyle().BorderStyle(printConfigBorderStyle).Padding(0, 1).Foreground(lipgloss.Color("10"))
	printConfigErrorStatusStyle   = lipgloss.NewStyle().BorderStyle(printConfigBorderStyle).Padding(0, 1).Foreground(lipgloss.Color("9"))
	printConfigKeyStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)
	printConfigDescStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

type printConfigClearStatusMsg struct{}

type printConfigModel struct {
	viewport   viewport.Model
	content    string
	rawContent []byte
	statusMsg  string
	ready      bool
	start      bool
}

func (m printConfigModel) Init() tea.Cmd { return nil }

func (m printConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "ctrl+s":
			return m.saveContent()
		case "enter":
			m.start = true
			return m, tea.Quit
		}

	case printConfigClearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.handleResize(msg)
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m printConfigModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, m.viewport.View(), m.scrollbar())
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), content, m.footerView())
}

func (m *printConfigModel) saveContent() (printConfigModel, tea.Cmd) {
	filename := fmt.Sprintf("sarin_config_%s.yaml", time.Now().Format("2006-01-02_15-04-05"))
	if err := os.WriteFile(filename, m.rawContent, 0600); err != nil {
		m.statusMsg = printConfigErrorStatusStyle.Render("✗ Error saving file: " + err.Error())
	} else {
		m.statusMsg = printConfigSuccessStatusStyle.Render("✓ Saved to " + filename)
	}
	return *m, tea.Tick(statusDisplayTime, func(time.Time) tea.Msg { return printConfigClearStatusMsg{} })
}

func (m *printConfigModel) handleResize(msg tea.WindowSizeMsg) {
	headerHeight := lipgloss.Height(m.headerView())
	footerHeight := lipgloss.Height(m.footerView())
	height := msg.Height - headerHeight - footerHeight
	width := msg.Width - scrollbarWidth

	if !m.ready {
		m.viewport = viewport.New(width, height)
		m.viewport.SetContent(m.contentWithLineNumbers())
		m.ready = true
	} else {
		m.viewport.Width = width
		m.viewport.Height = height
	}
}

func (m printConfigModel) headerView() string {
	var title string
	if m.statusMsg != "" {
		title = ("" + m.statusMsg)
	} else {
		sep := printConfigDescStyle.Render(" / ")
		help := printConfigKeyStyle.Render("ENTER") + printConfigDescStyle.Render(" start") + sep +
			printConfigKeyStyle.Render("CTRL+S") + printConfigDescStyle.Render(" save") + sep +
			printConfigKeyStyle.Render("ESC") + printConfigDescStyle.Render(" exit")
		title = printConfigHelpStyle.Render(help)
	}
	line := strings.Repeat("─", max(0, m.viewport.Width+scrollbarWidth-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m printConfigModel) footerView() string {
	return strings.Repeat("─", m.viewport.Width+scrollbarWidth)
}

func (m printConfigModel) contentWithLineNumbers() string {
	lines := strings.Split(m.content, "\n")
	width := len(strconv.Itoa(len(lines)))
	lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("246"))

	var sb strings.Builder
	for i, line := range lines {
		lineNum := lineNumStyle.Render(fmt.Sprintf("%*d", width, i+1))
		sb.WriteString(lineNum)
		sb.WriteString("  ")
		sb.WriteString(line)
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func (m printConfigModel) scrollbar() string {
	height := m.viewport.Height
	trackHeight := height - scrollbarBottomSpace
	totalLines := m.viewport.TotalLineCount()

	if totalLines <= height {
		return strings.Repeat(" \n", trackHeight) + " "
	}

	thumbSize := max(1, (height*trackHeight)/totalLines)
	thumbPos := int(m.viewport.ScrollPercent() * float64(trackHeight-thumbSize))

	var sb strings.Builder
	for i := range trackHeight {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteByte('\xe2') // █ (U+2588)
			sb.WriteByte('\x96')
			sb.WriteByte('\x88')
		} else {
			sb.WriteByte('\xe2') // ░ (U+2591)
			sb.WriteByte('\x96')
			sb.WriteByte('\x91')
		}
		sb.WriteByte('\n')
	}
	sb.WriteByte(' ')
	return sb.String()
}
