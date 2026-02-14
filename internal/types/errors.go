package types

import (
	"errors"
	"fmt"
	"strings"
)

// ======================================== General ========================================

var (
	errNoError = errors.New("no error (internal)")
)

type FieldParseError struct {
	Field string
	Value string
	Err   error
}

func NewFieldParseError(field string, value string, err error) FieldParseError {
	if err == nil {
		err = errNoError
	}
	return FieldParseError{field, value, err}
}

func (e FieldParseError) Error() string {
	return fmt.Sprintf("Field '%s' parse failed: %v", e.Field, e.Err)
}

func (e FieldParseError) Unwrap() error {
	return e.Err
}

type FieldParseErrors struct {
	Errors []FieldParseError
}

func NewFieldParseErrors(fieldParseErrors []FieldParseError) FieldParseErrors {
	return FieldParseErrors{fieldParseErrors}
}

func (e FieldParseErrors) Error() string {
	if len(e.Errors) == 0 {
		return "No field parse errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var builder strings.Builder
	for i, err := range e.Errors {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(err.Error())
	}

	return builder.String()
}

type FieldValidationError struct {
	Field string
	Value string
	Err   error
}

func NewFieldValidationError(field string, value string, err error) FieldValidationError {
	if err == nil {
		err = errNoError
	}
	return FieldValidationError{field, value, err}
}

func (e FieldValidationError) Error() string {
	return fmt.Sprintf("Field '%s' validation failed: %v", e.Field, e.Err)
}

func (e FieldValidationError) Unwrap() error {
	return e.Err
}

type FieldValidationErrors struct {
	Errors []FieldValidationError
}

func NewFieldValidationErrors(fieldValidationErrors []FieldValidationError) FieldValidationErrors {
	return FieldValidationErrors{fieldValidationErrors}
}

func (e FieldValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "No field validation errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var builder strings.Builder
	for i, err := range e.Errors {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(err.Error())
	}

	return builder.String()
}

type UnmarshalError struct {
	error error
}

func NewUnmarshalError(err error) UnmarshalError {
	if err == nil {
		err = errNoError
	}
	return UnmarshalError{err}
}

func (e UnmarshalError) Error() string {
	return "Unmarshal error: " + e.error.Error()
}

func (e UnmarshalError) Unwrap() error {
	return e.error
}

// ======================================== General I/O ========================================

type FileReadError struct {
	Path string
	Err  error
}

func NewFileReadError(path string, err error) FileReadError {
	if err == nil {
		err = errNoError
	}
	return FileReadError{path, err}
}

func (e FileReadError) Error() string {
	return fmt.Sprintf("failed to read file %s: %v", e.Path, e.Err)
}

func (e FileReadError) Unwrap() error {
	return e.Err
}

type HTTPFetchError struct {
	URL string
	Err error
}

func NewHTTPFetchError(url string, err error) HTTPFetchError {
	if err == nil {
		err = errNoError
	}
	return HTTPFetchError{url, err}
}

func (e HTTPFetchError) Error() string {
	return fmt.Sprintf("failed to fetch %s: %v", e.URL, e.Err)
}

func (e HTTPFetchError) Unwrap() error {
	return e.Err
}

type HTTPStatusError struct {
	URL        string
	StatusCode int
	Status     string
}

func NewHTTPStatusError(url string, statusCode int, status string) HTTPStatusError {
	return HTTPStatusError{url, statusCode, status}
}

func (e HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d %s (url: %s)", e.StatusCode, e.Status, e.URL)
}

type URLParseError struct {
	URL string
	Err error
}

func NewURLParseError(url string, err error) URLParseError {
	if err == nil {
		err = errNoError
	}
	return URLParseError{url, err}
}

func (e URLParseError) Error() string {
	return fmt.Sprintf("invalid URL %q: %v", e.URL, e.Err)
}

func (e URLParseError) Unwrap() error {
	return e.Err
}

// ======================================== Template ========================================

var (
	ErrFileCacheNotInitialized = errors.New("file cache is not initialized")
	ErrFormDataOddArgs         = errors.New("body_FormData requires an even number of arguments (key-value pairs)")
)

type TemplateParseError struct {
	Err error
}

func NewTemplateParseError(err error) TemplateParseError {
	if err == nil {
		err = errNoError
	}
	return TemplateParseError{err}
}

func (e TemplateParseError) Error() string {
	return "template parse error: " + e.Err.Error()
}

func (e TemplateParseError) Unwrap() error {
	return e.Err
}

type TemplateRenderError struct {
	Err error
}

func NewTemplateRenderError(err error) TemplateRenderError {
	if err == nil {
		err = errNoError
	}
	return TemplateRenderError{err}
}

func (e TemplateRenderError) Error() string {
	return "template rendering: " + e.Err.Error()
}

func (e TemplateRenderError) Unwrap() error {
	return e.Err
}

// ======================================== CLI ========================================

var (
	ErrCLINoArgs = errors.New("CLI expects arguments but received none")
)

type CLIUnexpectedArgsError struct {
	Args []string
}

func NewCLIUnexpectedArgsError(args []string) CLIUnexpectedArgsError {
	return CLIUnexpectedArgsError{args}
}

func (e CLIUnexpectedArgsError) Error() string {
	return fmt.Sprintf("CLI received unexpected arguments: %v", strings.Join(e.Args, ","))
}

// ======================================== Config File ========================================

type ConfigFileReadError struct {
	error error
}

func NewConfigFileReadError(err error) ConfigFileReadError {
	if err == nil {
		err = errNoError
	}
	return ConfigFileReadError{err}
}

func (e ConfigFileReadError) Error() string {
	return "Config file read error: " + e.error.Error()
}

func (e ConfigFileReadError) Unwrap() error {
	return e.error
}

// ======================================== Proxy ========================================

type ProxyUnsupportedSchemeError struct {
	Scheme string
}

func NewProxyUnsupportedSchemeError(scheme string) ProxyUnsupportedSchemeError {
	return ProxyUnsupportedSchemeError{scheme}
}

func (e ProxyUnsupportedSchemeError) Error() string {
	return "unsupported proxy scheme: " + e.Scheme
}

type ProxyParseError struct {
	Err error
}

func NewProxyParseError(err error) ProxyParseError {
	if err == nil {
		err = errNoError
	}
	return ProxyParseError{err}
}

func (e ProxyParseError) Error() string {
	return "failed to parse proxy URL: " + e.Err.Error()
}

func (e ProxyParseError) Unwrap() error {
	return e.Err
}

type ProxyConnectError struct {
	Status string
}

func NewProxyConnectError(status string) ProxyConnectError {
	return ProxyConnectError{status}
}

func (e ProxyConnectError) Error() string {
	return "proxy CONNECT failed: " + e.Status
}

type ProxyResolveError struct {
	Host string
}

func NewProxyResolveError(host string) ProxyResolveError {
	return ProxyResolveError{host}
}

func (e ProxyResolveError) Error() string {
	return "no IP addresses found for host: " + e.Host
}

type ProxyDialError struct {
	Proxy string
	Err   error
}

func NewProxyDialError(proxy string, err error) ProxyDialError {
	if err == nil {
		err = errNoError
	}
	return ProxyDialError{proxy, err}
}

func (e ProxyDialError) Error() string {
	return "proxy \"" + e.Proxy + "\": " + e.Err.Error()
}

func (e ProxyDialError) Unwrap() error {
	return e.Err
}

// ======================================== Script ========================================

var (
	ErrScriptEmpty                 = errors.New("script cannot be empty")
	ErrScriptSourceEmpty           = errors.New("script source cannot be empty after @")
	ErrScriptTransformMissing      = errors.New("script must define a global 'transform' function")
	ErrScriptTransformReturnObject = errors.New("transform function must return an object")
	ErrScriptURLNoHost             = errors.New("script URL must have a host")
)

type ScriptLoadError struct {
	Source string
	Err    error
}

func NewScriptLoadError(source string, err error) ScriptLoadError {
	if err == nil {
		err = errNoError
	}
	return ScriptLoadError{source, err}
}

func (e ScriptLoadError) Error() string {
	return fmt.Sprintf("failed to load script from %q: %v", e.Source, e.Err)
}

func (e ScriptLoadError) Unwrap() error {
	return e.Err
}

type ScriptExecutionError struct {
	EngineType string
	Err        error
}

func NewScriptExecutionError(engineType string, err error) ScriptExecutionError {
	if err == nil {
		err = errNoError
	}
	return ScriptExecutionError{engineType, err}
}

func (e ScriptExecutionError) Error() string {
	return fmt.Sprintf("%s script error: %v", e.EngineType, e.Err)
}

func (e ScriptExecutionError) Unwrap() error {
	return e.Err
}

type ScriptChainError struct {
	EngineType string
	Index      int
	Err        error
}

func NewScriptChainError(engineType string, index int, err error) ScriptChainError {
	if err == nil {
		err = errNoError
	}
	return ScriptChainError{engineType, index, err}
}

func (e ScriptChainError) Error() string {
	return fmt.Sprintf("%s script[%d]: %v", e.EngineType, e.Index, e.Err)
}

func (e ScriptChainError) Unwrap() error {
	return e.Err
}

type ScriptUnknownEngineError struct {
	EngineType string
}

func NewScriptUnknownEngineError(engineType string) ScriptUnknownEngineError {
	return ScriptUnknownEngineError{engineType}
}

func (e ScriptUnknownEngineError) Error() string {
	return "unknown engine type: " + e.EngineType
}
