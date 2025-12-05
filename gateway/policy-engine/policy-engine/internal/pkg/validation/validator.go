package validation

import (
	"fmt"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// ValidateParameter validates a parameter value against its schema
func ValidateParameter(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	// Handle nil value
	if value == nil {
		if schema.Required {
			return nil, fmt.Errorf("required parameter '%s' is missing", schema.Name)
		}
		if schema.Default != nil {
			value = schema.Default
		} else {
			return nil, nil // Optional parameter not provided
		}
	}

	// Validate based on type
	switch schema.Type {
	case policy.ParameterTypeString:
		return validateString(value, schema)
	case policy.ParameterTypeInt:
		return validateInt(value, schema)
	case policy.ParameterTypeFloat:
		return validateFloat(value, schema)
	case policy.ParameterTypeBool:
		return validateBool(value, schema)
	case policy.ParameterTypeDuration:
		return validateDuration(value, schema)
	case policy.ParameterTypeStringArray:
		return validateStringArray(value, schema)
	case policy.ParameterTypeIntArray:
		return validateIntArray(value, schema)
	case policy.ParameterTypeMap:
		return validateMap(value, schema)
	case policy.ParameterTypeURI:
		return validateURI(value, schema)
	case policy.ParameterTypeEmail:
		return validateEmail(value, schema)
	case policy.ParameterTypeHostname:
		return validateHostname(value, schema)
	case policy.ParameterTypeIPv4:
		return validateIPv4(value, schema)
	case policy.ParameterTypeIPv6:
		return validateIPv6(value, schema)
	case policy.ParameterTypeUUID:
		return validateUUID(value, schema)
	default:
		return nil, fmt.Errorf("unsupported parameter type: %s", schema.Type)
	}
}

// validateBool validates boolean parameters
func validateBool(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	boolVal, ok := value.(bool)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a boolean, got %T", schema.Name, value)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeBool,
		Value: boolVal,
	}, nil
}

// validateMap validates map parameters
func validateMap(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	mapVal, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a map, got %T", schema.Name, value)
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeMap,
		Value: mapVal,
	}, nil
}
