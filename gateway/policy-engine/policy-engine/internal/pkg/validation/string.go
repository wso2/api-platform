package validation

import (
	"fmt"
	"regexp"

	"github.com/policy-engine/sdk/policy"
)

// validateString validates string parameters with constraints
func validateString(value interface{}, schema policy.ParameterSchema) (*policy.TypedValue, error) {
	strVal, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("parameter '%s' must be a string, got %T", schema.Name, value)
	}

	// MinLength validation
	if schema.Validation.MinLength != nil && len(strVal) < *schema.Validation.MinLength {
		return nil, fmt.Errorf("parameter '%s' must be at least %d characters, got %d",
			schema.Name, *schema.Validation.MinLength, len(strVal))
	}

	// MaxLength validation
	if schema.Validation.MaxLength != nil && len(strVal) > *schema.Validation.MaxLength {
		return nil, fmt.Errorf("parameter '%s' must be at most %d characters, got %d",
			schema.Name, *schema.Validation.MaxLength, len(strVal))
	}

	// Pattern validation (regex)
	if schema.Validation.Pattern != "" {
		matched, err := regexp.MatchString(schema.Validation.Pattern, strVal)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern for parameter '%s': %w", schema.Name, err)
		}
		if !matched {
			return nil, fmt.Errorf("parameter '%s' does not match pattern %s",
				schema.Name, schema.Validation.Pattern)
		}
	}

	// Enum validation
	if len(schema.Validation.Enum) > 0 {
		found := false
		for _, allowed := range schema.Validation.Enum {
			if strVal == allowed {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("parameter '%s' must be one of %v, got '%s'",
				schema.Name, schema.Validation.Enum, strVal)
		}
	}

	return &policy.TypedValue{
		Type:  policy.ParameterTypeString,
		Value: strVal,
	}, nil
}
