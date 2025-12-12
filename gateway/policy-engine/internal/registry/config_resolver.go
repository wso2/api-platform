package registry

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// ConfigResolver resolves $config(...) CEL expressions from a configuration map
type ConfigResolver struct {
	config map[string]interface{}
}

// NewConfigResolver creates a new config resolver
func NewConfigResolver(config map[string]interface{}) *ConfigResolver {
	return &ConfigResolver{config: config}
}

// Regular expression to match $config(...) pattern
var configRefPattern = regexp.MustCompile(`^\$config\(([^)]+)\)$`)

// ResolveValue resolves a value that may contain a $config(...) reference
// If the value is not a config reference, it returns the value unchanged
func (r *ConfigResolver) ResolveValue(value interface{}) interface{} {
	// If no config loaded, return value as-is
	if r == nil || r.config == nil {
		return value
	}

	// Only process string values that match the pattern
	strValue, ok := value.(string)
	if !ok {
		return value
	}

	// Check if it matches $config(...) pattern
	matches := configRefPattern.FindStringSubmatch(strValue)
	if matches == nil {
		return value // Not a config reference
	}

	// Extract the path (e.g., "JWTAuth.AllowedAlgorithms")
	path := matches[1]

	slog.Debug("Resolving config reference",
		"reference", strValue,
		"path", path,
		"phase", "runtime")

	// Resolve the path in the config
	resolved, err := r.resolvePath(path)
	if err != nil {
		slog.Warn("Failed to resolve config reference, keeping as-is",
			"reference", strValue,
			"path", path,
			"error", err,
			"phase", "runtime")
		return value // Keep original reference on error
	}

	slog.Debug("Config reference resolved",
		"reference", strValue,
		"path", path,
		"resolvedValue", resolved,
		"phase", "runtime")

	return resolved
}

// resolvePath resolves a dot-notation path in the config map
// Example: "JWTAuth.AllowedAlgorithms" -> config["JWTAuth"]["AllowedAlgorithms"]
// Also handles case-insensitive keys which Viper uses
func (r *ConfigResolver) resolvePath(path string) (interface{}, error) {
	parts := strings.Split(path, ".")
	var current interface{} = r.config

	for i, part := range parts {
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path segment %d (%s) is not an object in config", i, part)
		}

		// Try exact match first
		value, exists := currentMap[part]
		if !exists {
			// Try lowercase (Viper lowercases all keys)
			value, exists = currentMap[strings.ToLower(part)]
			if !exists {
				return nil, fmt.Errorf("path segment %s not found in config", part)
			}
		}

		current = value
	}

	return current, nil
}

// ResolveMap resolves all $config(...) references in a map recursively
func (r *ConfigResolver) ResolveMap(m map[string]interface{}) map[string]interface{} {
	if r == nil || r.config == nil {
		return m // No config, return as-is
	}

	result := make(map[string]interface{})
	for key, value := range m {
		result[key] = r.resolveValueRecursive(value)
	}
	return result
}

// resolveValueRecursive resolves $config references recursively in nested structures
func (r *ConfigResolver) resolveValueRecursive(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return r.ResolveValue(v)
	case map[string]interface{}:
		return r.ResolveMap(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = r.resolveValueRecursive(item)
		}
		return result
	default:
		return value
	}
}
