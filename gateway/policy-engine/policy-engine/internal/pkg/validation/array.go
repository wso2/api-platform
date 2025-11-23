package validation

import (
	"fmt"

	"github.com/policy-engine/sdk/policy"
)

// validateStringArray validates string array parameters
func validateStringArray(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	arrayVal, ok := value.([]interface{})
	if !ok {
		// Try direct string array (from Go config)
		strArray, ok := value.([]string)
		if ok {
			return validateStringArrayDirect(strArray, schema)
		}
		return nil, fmt.Errorf("parameter '%s' must be an array, got %T", schema.Name, value)
	}

	// Convert interface{} array to string array
	strArray := make([]string, len(arrayVal))
	for i, item := range arrayVal {
		strVal, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("parameter '%s'[%d] must be a string, got %T", schema.Name, i, item)
		}
		strArray[i] = strVal
	}

	return validateStringArrayDirect(strArray, schema)
}

// validateStringArrayDirect validates a []string directly
func validateStringArrayDirect(strArray []string, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	// MinItems validation
	if schema.Validation.MinItems != nil && len(strArray) < *schema.Validation.MinItems {
		return nil, fmt.Errorf("parameter '%s' must have at least %d items, got %d",
			schema.Name, *schema.Validation.MinItems, len(strArray))
	}

	// MaxItems validation
	if schema.Validation.MaxItems != nil && len(strArray) > *schema.Validation.MaxItems {
		return nil, fmt.Errorf("parameter '%s' must have at most %d items, got %d",
			schema.Name, *schema.Validation.MaxItems, len(strArray))
	}

	// UniqueItems validation
	if schema.Validation.UniqueItems {
		seen := make(map[string]bool)
		for _, item := range strArray {
			if seen[item] {
				return nil, fmt.Errorf("parameter '%s' must have unique items, found duplicate: '%s'",
					schema.Name, item)
			}
			seen[item] = true
		}
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeStringArray,
		Value: strArray,
	}, nil
}
