package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/yourorg/policy-engine/worker/policies"
)

// LoadPolicyDefinitionFromYAML loads a policy definition from a YAML file
func LoadPolicyDefinitionFromYAML(path string) (*policies.PolicyDefinition, error) {
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy.yaml: %w", err)
	}

	// Parse YAML
	var def policies.PolicyDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse policy.yaml: %w", err)
	}

	// Validate schema
	if err := validatePolicyDefinition(&def); err != nil {
		return nil, fmt.Errorf("invalid policy definition: %w", err)
	}

	return &def, nil
}

// RegisterFromDirectory discovers and registers policies from a directory
func (r *PolicyRegistry) RegisterFromDirectory(path string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for policy.yaml files
		if !info.IsDir() && info.Name() == "policy.yaml" {
			def, err := LoadPolicyDefinitionFromYAML(filePath)
			if err != nil {
				return fmt.Errorf("failed to load %s: %w", filePath, err)
			}

			// Note: Implementation registration happens via generated plugin_registry.go
			// Here we only load definitions
			policyDir := filepath.Dir(filePath)
			ctx := context.Background()
			slog.InfoContext(ctx, "Discovered policy",
				"name", def.Name,
				"version", def.Version,
				"path", policyDir)

			// Store definition (implementation will be registered by plugin code)
			key := compositeKey(def.Name, def.Version)
			r.mu.Lock()
			r.Definitions[key] = def
			r.mu.Unlock()
		}

		return nil
	})
}

// validatePolicyDefinition validates the policy.yaml structure
func validatePolicyDefinition(def *policies.PolicyDefinition) error {
	// Name must be non-empty
	if def.Name == "" {
		return fmt.Errorf("policy name cannot be empty")
	}

	// Version must be non-empty
	if def.Version == "" {
		return fmt.Errorf("policy version cannot be empty")
	}

	// Must support at least one phase
	if !def.SupportsRequestPhase && !def.SupportsResponsePhase {
		return fmt.Errorf("policy must support at least one phase (request or response)")
	}

	// Validate parameter schemas
	paramNames := make(map[string]bool)
	for _, param := range def.ParameterSchemas {
		// Check for duplicate parameter names
		if paramNames[param.Name] {
			return fmt.Errorf("duplicate parameter name: %s", param.Name)
		}
		paramNames[param.Name] = true

		// Validate parameter schema
		if param.Name == "" {
			return fmt.Errorf("parameter name cannot be empty")
		}

		if param.Type == "" {
			return fmt.Errorf("parameter type cannot be empty for parameter: %s", param.Name)
		}

		// Required parameters should not have defaults
		if param.Required && param.Default != nil {
			return fmt.Errorf("required parameter cannot have default value: %s", param.Name)
		}
	}

	return nil
}
