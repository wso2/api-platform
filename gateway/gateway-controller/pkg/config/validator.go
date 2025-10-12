package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
)

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Validator validates API configurations
type Validator struct {
	// pathParamRegex matches {param} placeholders
	pathParamRegex *regexp.Regexp
	// versionRegex matches semantic version patterns
	versionRegex *regexp.Regexp
}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{
		pathParamRegex: regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`),
		versionRegex:   regexp.MustCompile(`^v?\d+(\.\d+)?(\.\d+)?$`),
	}
}

// Validate performs comprehensive validation on an API configuration
func (v *Validator) Validate(config *api.APIConfiguration) []ValidationError {
	var errors []ValidationError

	// Validate version
	if config.Version != "api-platform.wso2.com/v1" {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "Unsupported API version (must be 'api-platform.wso2.com/v1')",
		})
	}

	// Validate kind
	if config.Kind != "http/rest" {
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: "Unsupported API kind (only 'http/rest' is supported)",
		})
	}

	// Validate data section
	errors = append(errors, v.validateData(&config.Data)...)

	return errors
}

// validateData validates the data section of the configuration
func (v *Validator) validateData(data *api.APIConfigData) []ValidationError {
	var errors []ValidationError

	// Validate name
	if data.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "data.name",
			Message: "API name is required",
		})
	} else if len(data.Name) > 100 {
		errors = append(errors, ValidationError{
			Field:   "data.name",
			Message: "API name must be 1-100 characters",
		})
	}

	// Validate version
	if data.Version == "" {
		errors = append(errors, ValidationError{
			Field:   "data.version",
			Message: "API version is required",
		})
	} else if !v.versionRegex.MatchString(data.Version) {
		errors = append(errors, ValidationError{
			Field:   "data.version",
			Message: "API version must follow semantic versioning pattern (e.g., v1.0, v2.1.3)",
		})
	}

	// Validate context
	errors = append(errors, v.validateContext(data.Context)...)

	// Validate upstream
	errors = append(errors, v.validateUpstream(data.Upstream)...)

	// Validate operations
	errors = append(errors, v.validateOperations(data.Operations)...)

	return errors
}

// validateContext validates the context path
func (v *Validator) validateContext(context string) []ValidationError {
	var errors []ValidationError

	if context == "" {
		errors = append(errors, ValidationError{
			Field:   "data.context",
			Message: "Context is required",
		})
		return errors
	}

	if !strings.HasPrefix(context, "/") {
		errors = append(errors, ValidationError{
			Field:   "data.context",
			Message: "Context must start with /",
		})
	}

	if strings.HasSuffix(context, "/") && context != "/" {
		errors = append(errors, ValidationError{
			Field:   "data.context",
			Message: "Context cannot end with / (except for root context)",
		})
	}

	if len(context) > 200 {
		errors = append(errors, ValidationError{
			Field:   "data.context",
			Message: "Context must be 1-200 characters",
		})
	}

	return errors
}

// validateUpstream validates the upstream configuration
func (v *Validator) validateUpstream(upstream []api.Upstream) []ValidationError {
	var errors []ValidationError

	if len(upstream) == 0 {
		errors = append(errors, ValidationError{
			Field:   "data.upstream",
			Message: "At least one upstream URL is required",
		})
		return errors
	}

	for i, up := range upstream {
		if up.Url == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.upstream[%d].url", i),
				Message: "Upstream URL is required",
			})
			continue
		}

		// Validate URL format
		parsedURL, err := url.Parse(up.Url)
		if err != nil {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.upstream[%d].url", i),
				Message: fmt.Sprintf("Invalid URL format: %v", err),
			})
			continue
		}

		// Ensure scheme is http or https
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.upstream[%d].url", i),
				Message: "Upstream URL must use http or https scheme",
			})
		}

		// Ensure host is present
		if parsedURL.Host == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.upstream[%d].url", i),
				Message: "Upstream URL must include a host",
			})
		}
	}

	return errors
}

// validateOperations validates the operations configuration
func (v *Validator) validateOperations(operations []api.Operation) []ValidationError {
	var errors []ValidationError

	if len(operations) == 0 {
		errors = append(errors, ValidationError{
			Field:   "data.operations",
			Message: "At least one operation is required",
		})
		return errors
	}

	validMethods := map[string]bool{
		"GET": true, "POST": true, "PUT": true, "DELETE": true,
		"PATCH": true, "HEAD": true, "OPTIONS": true,
	}

	for i, op := range operations {
		// Validate method
		if op.Method == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.operations[%d].method", i),
				Message: "HTTP method is required",
			})
		} else if !validMethods[strings.ToUpper(string(op.Method))] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.operations[%d].method", i),
				Message: fmt.Sprintf("Invalid HTTP method '%s' (must be GET, POST, PUT, DELETE, PATCH, HEAD, or OPTIONS)", op.Method),
			})
		}

		// Validate path
		if op.Path == "" {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.operations[%d].path", i),
				Message: "Operation path is required",
			})
			continue
		}

		if !strings.HasPrefix(op.Path, "/") {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.operations[%d].path", i),
				Message: "Operation path must start with /",
			})
		}

		// Validate path parameters have balanced braces
		if !v.validatePathParameters(op.Path) {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("data.operations[%d].path", i),
				Message: "Operation path has unbalanced braces in parameters",
			})
		}
	}

	return errors
}

// validatePathParameters checks if path parameters have balanced braces
func (v *Validator) validatePathParameters(path string) bool {
	openCount := strings.Count(path, "{")
	closeCount := strings.Count(path, "}")
	return openCount == closeCount
}
