package validation

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// UUID regex pattern (RFC 4122)
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// Hostname regex pattern (simplified RFC 1123)
var hostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`)

// validateEmail validates email format parameters
func validateEmail(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// Use net/mail for email validation
	_, err := mail.ParseAddress(strVal)
	if err != nil {
		return nil, fmt.Errorf("parameter '%s' must be a valid email address, got '%s': %w",
			schema.Name, strVal, err)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeEmail,
		Value: strVal,
	}, nil
}

// validateURI validates URI format parameters
func validateURI(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// Parse and validate URI
	parsedURL, err := url.Parse(strVal)
	if err != nil {
		return nil, fmt.Errorf("parameter '%s' must be a valid URI, got '%s': %w",
			schema.Name, strVal, err)
	}

	// URI must have a scheme
	if parsedURL.Scheme == "" {
		return nil, fmt.Errorf("parameter '%s' must have a scheme (e.g., https://), got '%s'",
			schema.Name, strVal)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeURI,
		Value: strVal,
	}, nil
}

// validateHostname validates hostname format parameters
func validateHostname(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// Validate hostname format (RFC 1123)
	if !hostnameRegex.MatchString(strVal) {
		return nil, fmt.Errorf("parameter '%s' must be a valid hostname, got '%s'",
			schema.Name, strVal)
	}

	// Hostname length must not exceed 253 characters
	if len(strVal) > 253 {
		return nil, fmt.Errorf("parameter '%s' hostname too long (max 253 characters), got %d",
			schema.Name, len(strVal))
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeHostname,
		Value: strVal,
	}, nil
}

// validateIPv4 validates IPv4 address format parameters
func validateIPv4(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// Parse IP address
	ip := net.ParseIP(strVal)
	if ip == nil {
		return nil, fmt.Errorf("parameter '%s' must be a valid IP address, got '%s'",
			schema.Name, strVal)
	}

	// Check if it's IPv4
	if ip.To4() == nil {
		return nil, fmt.Errorf("parameter '%s' must be an IPv4 address, got '%s'",
			schema.Name, strVal)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeIPv4,
		Value: strVal,
	}, nil
}

// validateIPv6 validates IPv6 address format parameters
func validateIPv6(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// Parse IP address
	ip := net.ParseIP(strVal)
	if ip == nil {
		return nil, fmt.Errorf("parameter '%s' must be a valid IP address, got '%s'",
			schema.Name, strVal)
	}

	// Check if it's IPv6
	if ip.To4() != nil {
		return nil, fmt.Errorf("parameter '%s' must be an IPv6 address, got '%s'",
			schema.Name, strVal)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeIPv6,
		Value: strVal,
	}, nil
}

// validateUUID validates UUID format parameters
func validateUUID(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// Validate UUID format (lowercase)
	if !uuidRegex.MatchString(strVal) {
		return nil, fmt.Errorf("parameter '%s' must be a valid UUID, got '%s'",
			schema.Name, strVal)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeUUID,
		Value: strVal,
	}, nil
}
