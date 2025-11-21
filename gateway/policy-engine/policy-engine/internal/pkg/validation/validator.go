package validation

import (
	"fmt"

	"github.com/policy-engine/sdk/policies"
)

// ValidateParameter validates a parameter value against its schema
func ValidateParameter(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
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
	case policies.ParameterTypeString:
		return validateString(value, schema)
	case policies.ParameterTypeInt:
		return validateInt(value, schema)
	case policies.ParameterTypeFloat:
		return validateFloat(value, schema)
	case policies.ParameterTypeBool:
		return validateBool(value, schema)
	case policies.ParameterTypeDuration:
		return validateDuration(value, schema)
	case policies.ParameterTypeStringArray:
		return validateStringArray(value, schema)
	case policies.ParameterTypeIntArray:
		return validateIntArray(value, schema)
	case policies.ParameterTypeMap:
		return validateMap(value, schema)
	case policies.ParameterTypeURI:
		return validateURI(value, schema)
	case policies.ParameterTypeEmail:
		return validateEmail(value, schema)
	case policies.ParameterTypeHostname:
		return validateHostname(value, schema)
	case policies.ParameterTypeIPv4:
		return validateIPv4(value, schema)
	case policies.ParameterTypeIPv6:
		return validateIPv6(value, schema)
	case policies.ParameterTypeUUID:
		return validateUUID(value, schema)
	default:
		return nil, fmt.Errorf("unsupported parameter type: %s", schema.Type)
	}
}

// validateBool validates boolean parameters
func validateBool(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
	boolVal, ok := value.(bool)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a boolean, got %T", schema.Name, value)
	}

	return &policies.TypedValue{
		Type:  policies.ParameterTypeBool,
		Value: boolVal,
	}, nil
}

// validateMap validates map parameters
func validateMap(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
	mapVal, ok := value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a map, got %T", schema.Name, value)
	}

	return &policies.TypedValue{
		Type:  policies.ParameterTypeMap,
		Value: mapVal,
	}, nil
}
