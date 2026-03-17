package utils

import (
	"encoding/json"
	"fmt"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// ExtractNameVersion returns the name and version from an API configuration
// Supports both HTTP REST APIs and async/websub kinds.
func ExtractNameVersion(cfg any) (string, string, error) {
	switch c := cfg.(type) {
	case api.RestAPI:
		return c.Spec.DisplayName, c.Spec.Version, nil
	case api.WebSubAPI:
		return c.Spec.DisplayName, c.Spec.Version, nil
	default:
		return "", "", fmt.Errorf("unsupported api config type: %T", cfg)
	}
}

// GetValueFromSourceConfig extracts a value from sourceConfig using a key path.
// The key can be a simple key (e.g., "kind") or a nested path (e.g., "spec.template").
// Returns the value if found, nil otherwise.
func GetValueFromSourceConfig(sourceConfig any, key string) (any, error) {
	if sourceConfig == nil {
		return nil, fmt.Errorf("sourceConfig is nil")
	}

	// Convert sourceConfig to a map for easy traversal
	var configMap map[string]interface{}
	j, err := json.Marshal(sourceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sourceConfig: %w", err)
	}
	if err := json.Unmarshal(j, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sourceConfig: %w", err)
	}

	// Split the key by dots to handle nested paths
	keys := strings.Split(key, ".")
	current := configMap

	// Traverse the nested structure
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, return the value
			if val, ok := current[k]; ok {
				return val, nil
			}
			return nil, fmt.Errorf("key '%s' not found in sourceConfig", key)
		}

		// Navigate further down the nested structure
		if next, ok := current[k].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, fmt.Errorf("key path '%s' is invalid: '%s' is not a map", key, strings.Join(keys[:i+1], "."))
		}
	}

	return nil, fmt.Errorf("key '%s' not found in sourceConfig", key)
}
