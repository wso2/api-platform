package validation

import (
	"fmt"
	"math"

	"github.com/envoy-policy-engine/sdk/policies"
)

// validateInt validates integer parameters with constraints
func validateInt(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
	var intVal int64

	// Handle different numeric types
	switch v := value.(type) {
	case int:
		intVal = int64(v)
	case int32:
		intVal = int64(v)
	case int64:
		intVal = v
	case float64:
		// JSON unmarshaling often produces float64 for numbers
		intVal = int64(v)
		if float64(intVal) != v {
			return nil, fmt.Errorf("parameter '%s' must be an integer, got float %f", schema.Name, v)
		}
	default:
		return nil, fmt.Errorf("parameter '%s' must be an integer, got %T", schema.Name, value)
	}

	floatVal := float64(intVal)

	// Min validation
	if schema.Validation.Min != nil && floatVal < *schema.Validation.Min {
		return nil, fmt.Errorf("parameter '%s' must be at least %v, got %d",
			schema.Name, *schema.Validation.Min, intVal)
	}

	// Max validation
	if schema.Validation.Max != nil && floatVal > *schema.Validation.Max {
		return nil, fmt.Errorf("parameter '%s' must be at most %v, got %d",
			schema.Name, *schema.Validation.Max, intVal)
	}

	// MultipleOf validation
	if schema.Validation.MultipleOf != nil && *schema.Validation.MultipleOf != 0 {
		if math.Mod(floatVal, *schema.Validation.MultipleOf) != 0 {
			return nil, fmt.Errorf("parameter '%s' must be a multiple of %v, got %d",
				schema.Name, *schema.Validation.MultipleOf, intVal)
		}
	}

	return &policies.TypedValue{
		Type:  policies.ParameterTypeInt,
		Value: intVal,
	}, nil
}

// validateFloat validates float parameters with constraints
func validateFloat(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
	var floatVal float64

	// Handle different numeric types
	switch v := value.(type) {
	case float32:
		floatVal = float64(v)
	case float64:
		floatVal = v
	case int:
		floatVal = float64(v)
	case int32:
		floatVal = float64(v)
	case int64:
		floatVal = float64(v)
	default:
		return nil, fmt.Errorf("parameter '%s' must be a float, got %T", schema.Name, value)
	}

	// Min validation
	if schema.Validation.Min != nil && floatVal < *schema.Validation.Min {
		return nil, fmt.Errorf("parameter '%s' must be at least %v, got %f",
			schema.Name, *schema.Validation.Min, floatVal)
	}

	// Max validation
	if schema.Validation.Max != nil && floatVal > *schema.Validation.Max {
		return nil, fmt.Errorf("parameter '%s' must be at most %v, got %f",
			schema.Name, *schema.Validation.Max, floatVal)
	}

	// MultipleOf validation
	if schema.Validation.MultipleOf != nil && *schema.Validation.MultipleOf != 0 {
		if math.Mod(floatVal, *schema.Validation.MultipleOf) != 0 {
			return nil, fmt.Errorf("parameter '%s' must be a multiple of %v, got %f",
				schema.Name, *schema.Validation.MultipleOf, floatVal)
		}
	}

	return &policies.TypedValue{
		Type:  policies.ParameterTypeFloat,
		Value: floatVal,
	}, nil
}

// validateIntArray validates integer array parameters
func validateIntArray(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
	arrayVal, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be an array, got %T", schema.Name, value)
	}

	// Convert to []int64
	intArray := make([]int64, len(arrayVal))
	for i, item := range arrayVal {
		switch v := item.(type) {
		case int:
			intArray[i] = int64(v)
		case int32:
			intArray[i] = int64(v)
		case int64:
			intArray[i] = v
		case float64:
			intArray[i] = int64(v)
			if float64(intArray[i]) != v {
				return nil, fmt.Errorf("parameter '%s'[%d] must be an integer, got float %f", schema.Name, i, v)
			}
		default:
			return nil, fmt.Errorf("parameter '%s'[%d] must be an integer, got %T", schema.Name, i, item)
		}
	}

	// MinItems validation
	if schema.Validation.MinItems != nil && len(intArray) < *schema.Validation.MinItems {
		return nil, fmt.Errorf("parameter '%s' must have at least %d items, got %d",
			schema.Name, *schema.Validation.MinItems, len(intArray))
	}

	// MaxItems validation
	if schema.Validation.MaxItems != nil && len(intArray) > *schema.Validation.MaxItems {
		return nil, fmt.Errorf("parameter '%s' must have at most %d items, got %d",
			schema.Name, *schema.Validation.MaxItems, len(intArray))
	}

	// UniqueItems validation
	if schema.Validation.UniqueItems {
		seen := make(map[int64]bool)
		for _, item := range intArray {
			if seen[item] {
				return nil, fmt.Errorf("parameter '%s' must have unique items, found duplicate: %d",
					schema.Name, item)
			}
			seen[item] = true
		}
	}

	return &policies.TypedValue{
		Type:  policies.ParameterTypeIntArray,
		Value: intArray,
	}, nil
}
