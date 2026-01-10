package types

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// General
	ErrNoError = errors.New("no error (internal)")

	// CLI
	ErrCLINoArgs = errors.New("CLI expects arguments but received none")
)

// ======================================== General ========================================

type FieldParseError struct {
	Field string
	Value string
	Err   error
}

func NewFieldParseError(field string, value string, err error) FieldParseError {
	if err == nil {
		err = ErrNoError
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
		err = ErrNoError
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
		err = ErrNoError
	}
	return UnmarshalError{err}
}

func (e UnmarshalError) Error() string {
	return "Unmarshal error: " + e.error.Error()
}

func (e UnmarshalError) Unwrap() error {
	return e.error
}

// ======================================== CLI ========================================

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
		err = ErrNoError
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

type ProxyDialError struct {
	Proxy string
	Err   error
}

func NewProxyDialError(proxy string, err error) ProxyDialError {
	if err == nil {
		err = ErrNoError
	}
	return ProxyDialError{proxy, err}
}

func (e ProxyDialError) Error() string {
	return "proxy \"" + e.Proxy + "\": " + e.Err.Error()
}

func (e ProxyDialError) Unwrap() error {
	return e.Err
}
