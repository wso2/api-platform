package validation

import (
	"fmt"
	"time"

	"github.com/envoy-policy-engine/sdk/policies"
)

// validateDuration validates duration parameters (e.g., "30s", "5m", "1h")
func validateDuration(value interface{}, schema policies.ParameterSchema) (*policies.TypedValue, error) {
	var duration time.Duration

	// Handle different input types
	switch v := value.(type) {
	case string:
		// Parse duration string (e.g., "30s", "5m", "1h")
		var err error
		duration, err = time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("parameter '%s' must be a valid duration (e.g., '30s', '5m', '1h'), got '%s': %w",
				schema.Name, v, err)
		}
	case time.Duration:
		duration = v
	case int:
		// Treat as seconds
		duration = time.Duration(v) * time.Second
	case int64:
		// Treat as seconds
		duration = time.Duration(v) * time.Second
	case float64:
		// Treat as seconds
		duration = time.Duration(v * float64(time.Second))
	default:
		return nil, fmt.Errorf("parameter '%s' must be a duration string or number, got %T", schema.Name, value)
	}

	// MinDuration validation
	if schema.Validation.MinDuration != nil && duration < *schema.Validation.MinDuration {
		return nil, fmt.Errorf("parameter '%s' must be at least %s, got %s",
			schema.Name, schema.Validation.MinDuration, duration)
	}

	// MaxDuration validation
	if schema.Validation.MaxDuration != nil && duration > *schema.Validation.MaxDuration {
		return nil, fmt.Errorf("parameter '%s' must be at most %s, got %s",
			schema.Name, schema.Validation.MaxDuration, duration)
	}

	return &policies.TypedValue{
		Type:  policies.ParameterTypeDuration,
		Value: duration,
	}, nil
}
