package config

import (
	"fmt"
	"text/template"

	"go.aykhans.me/sarin/internal/sarin"
	"go.aykhans.me/sarin/internal/types"
)

func validateTemplateString(value string, funcMap template.FuncMap) error {
	if value == "" {
		return nil
	}

	_, err := template.New("").Funcs(funcMap).Parse(value)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	return nil
}

func validateTemplateMethods(methods []string, funcMap template.FuncMap) []types.FieldValidationError {
	var validationErrors []types.FieldValidationError

	for i, method := range methods {
		if err := validateTemplateString(method, funcMap); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Method[%d]", i),
					method,
					err,
				),
			)
		}
	}

	return validationErrors
}

func validateTemplateParams(params types.Params, funcMap template.FuncMap) []types.FieldValidationError {
	var validationErrors []types.FieldValidationError

	for paramIndex, param := range params {
		// Validate param key
		if err := validateTemplateString(param.Key, funcMap); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Param[%d].Key", paramIndex),
					param.Key,
					err,
				),
			)
		}

		// Validate param values
		for valueIndex, value := range param.Value {
			if err := validateTemplateString(value, funcMap); err != nil {
				validationErrors = append(
					validationErrors,
					types.NewFieldValidationError(
						fmt.Sprintf("Param[%d].Value[%d]", paramIndex, valueIndex),
						value,
						err,
					),
				)
			}
		}
	}

	return validationErrors
}

func validateTemplateHeaders(headers types.Headers, funcMap template.FuncMap) []types.FieldValidationError {
	var validationErrors []types.FieldValidationError

	for headerIndex, header := range headers {
		// Validate header key
		if err := validateTemplateString(header.Key, funcMap); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Header[%d].Key", headerIndex),
					header.Key,
					err,
				),
			)
		}

		// Validate header values
		for valueIndex, value := range header.Value {
			if err := validateTemplateString(value, funcMap); err != nil {
				validationErrors = append(
					validationErrors,
					types.NewFieldValidationError(
						fmt.Sprintf("Header[%d].Value[%d]", headerIndex, valueIndex),
						value,
						err,
					),
				)
			}
		}
	}

	return validationErrors
}

func validateTemplateCookies(cookies types.Cookies, funcMap template.FuncMap) []types.FieldValidationError {
	var validationErrors []types.FieldValidationError

	for cookieIndex, cookie := range cookies {
		// Validate cookie key
		if err := validateTemplateString(cookie.Key, funcMap); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Cookie[%d].Key", cookieIndex),
					cookie.Key,
					err,
				),
			)
		}

		// Validate cookie values
		for valueIndex, value := range cookie.Value {
			if err := validateTemplateString(value, funcMap); err != nil {
				validationErrors = append(
					validationErrors,
					types.NewFieldValidationError(
						fmt.Sprintf("Cookie[%d].Value[%d]", cookieIndex, valueIndex),
						value,
						err,
					),
				)
			}
		}
	}

	return validationErrors
}

func validateTemplateBodies(bodies []string, funcMap template.FuncMap) []types.FieldValidationError {
	var validationErrors []types.FieldValidationError

	for i, body := range bodies {
		if err := validateTemplateString(body, funcMap); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Body[%d]", i),
					body,
					err,
				),
			)
		}
	}

	return validationErrors
}

func validateTemplateValues(values []string, funcMap template.FuncMap) []types.FieldValidationError {
	var validationErrors []types.FieldValidationError

	for i, value := range values {
		if err := validateTemplateString(value, funcMap); err != nil {
			validationErrors = append(
				validationErrors,
				types.NewFieldValidationError(
					fmt.Sprintf("Values[%d]", i),
					value,
					err,
				),
			)
		}
	}

	return validationErrors
}

func validateTemplateURLPath(urlPath string, funcMap template.FuncMap) []types.FieldValidationError {
	if err := validateTemplateString(urlPath, funcMap); err != nil {
		return []types.FieldValidationError{
			types.NewFieldValidationError("URL.Path", urlPath, err),
		}
	}
	return nil
}

func ValidateTemplates(config *Config) []types.FieldValidationError {
	// Create template function map using the same functions as sarin package
	// Use nil for fileCache during validation - templates are only parsed, not executed
	randSource := sarin.NewDefaultRandSource()
	funcMap := sarin.NewDefaultTemplateFuncMap(randSource, nil)

	bodyFuncMapData := &sarin.BodyTemplateFuncMapData{}
	bodyFuncMap := sarin.NewDefaultBodyTemplateFuncMap(randSource, bodyFuncMapData, nil)

	var allErrors []types.FieldValidationError

	// Validate URL path
	if config.URL != nil {
		allErrors = append(allErrors, validateTemplateURLPath(config.URL.Path, funcMap)...)
	}

	// Validate methods
	allErrors = append(allErrors, validateTemplateMethods(config.Methods, funcMap)...)

	// Validate params
	allErrors = append(allErrors, validateTemplateParams(config.Params, funcMap)...)

	// Validate headers
	allErrors = append(allErrors, validateTemplateHeaders(config.Headers, funcMap)...)

	// Validate cookies
	allErrors = append(allErrors, validateTemplateCookies(config.Cookies, funcMap)...)

	// Validate bodies
	allErrors = append(allErrors, validateTemplateBodies(config.Bodies, bodyFuncMap)...)

	// Validate values
	allErrors = append(allErrors, validateTemplateValues(config.Values, funcMap)...)

	return allErrors
}
