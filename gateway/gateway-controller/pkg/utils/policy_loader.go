/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	policy "github.com/policy-engine/sdk/policy/v1alpha"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// PolicyLoader loads policy definitions from files
type PolicyLoader struct {
	logger *zap.Logger
}

// NewPolicyLoader creates a new policy loader
func NewPolicyLoader(logger *zap.Logger) *PolicyLoader {
	return &PolicyLoader{
		logger: logger,
	}
}

// LoadPoliciesFromDirectory loads all policy definition files from a directory
// Supports both JSON and YAML files
func (pl *PolicyLoader) LoadPoliciesFromDirectory(dirPath string) (map[string]api.PolicyDefinition, error) {
	policies := make(map[string]api.PolicyDefinition)

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		pl.logger.Warn("Policy directory does not exist", zap.String("path", dirPath))
		return policies, nil
	}

	// Walk through the directory
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process JSON and YAML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			pl.logger.Debug("Skipping non-policy file", zap.String("file", path))
			return nil
		}

		// Load the policy definition
		policy, err := pl.loadPolicyFile(path)
		if err != nil {
			pl.logger.Error("Failed to load policy file",
				zap.String("file", path),
				zap.Error(err))
			return err
		}

		// Validate required fields
		if err := pl.validatePolicy(policy); err != nil {
			pl.logger.Error("Invalid policy definition",
				zap.String("file", path),
				zap.Error(err))
			return err
		}

		// Check for duplicates
		key := policy.Name + "|" + policy.Version
		if _, exists := policies[key]; exists {
			return fmt.Errorf("duplicate policy definition: %s (version %s)", policy.Name, policy.Version)
		}

		policies[key] = *policy
		pl.logger.Info("Loaded policy definition",
			zap.String("name", policy.Name),
			zap.String("version", policy.Version),
			zap.String("file", path))

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load policies from directory: %w", err)
	}

	pl.logger.Info("Successfully loaded policy definitions",
		zap.Int("count", len(policies)),
		zap.String("directory", dirPath))

	return policies, nil
}

// convertParameterSchemasToJSONSchema converts SDK ParameterSchema slice to JSON Schema format
func convertParameterSchemasToJSONSchema(params []policy.ParameterSchema) *map[string]interface{} {
	if len(params) == 0 {
		return nil
	}

	properties := make(map[string]interface{})
	required := []string{}

	for _, param := range params {
		propSchema := map[string]interface{}{
			"type":        string(param.Type),
			"description": param.Description,
		}

		if param.Default != nil {
			propSchema["default"] = param.Default
		}

		// Add validation rules if present
		if param.Validation.Min != nil {
			propSchema["minimum"] = *param.Validation.Min
		}
		if param.Validation.Max != nil {
			propSchema["maximum"] = *param.Validation.Max
		}
		if param.Validation.Pattern != "" {
			propSchema["pattern"] = param.Validation.Pattern
		}
		if param.Validation.Format != "" {
			propSchema["format"] = param.Validation.Format
		}
		if len(param.Validation.Enum) > 0 {
			propSchema["enum"] = param.Validation.Enum
		}
		if param.Validation.MinLength != nil {
			propSchema["minLength"] = *param.Validation.MinLength
		}
		if param.Validation.MaxLength != nil {
			propSchema["maxLength"] = *param.Validation.MaxLength
		}
		if param.Validation.MultipleOf != nil {
			propSchema["multipleOf"] = *param.Validation.MultipleOf
		}
		if param.Validation.MinItems != nil {
			propSchema["minItems"] = *param.Validation.MinItems
		}
		if param.Validation.MaxItems != nil {
			propSchema["maxItems"] = *param.Validation.MaxItems
		}
		if param.Validation.UniqueItems {
			propSchema["uniqueItems"] = true
		}

		properties[param.Name] = propSchema

		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return &schema
}

// loadPolicyFile loads a single policy definition file
func (pl *PolicyLoader) loadPolicyFile(filePath string) (*api.PolicyDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))

	if ext == ".json" {
		var policy api.PolicyDefinition
		if err := json.Unmarshal(data, &policy); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return &policy, nil
	}

	var policyDef policy.PolicyDefinition

	if err := yaml.Unmarshal(data, &policyDef); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert to api.PolicyDefinition
	policy := api.PolicyDefinition{
		Name:             policyDef.Name,
		Version:          policyDef.Version,
		Description:      &policyDef.Description,
		ParametersSchema: convertParameterSchemasToJSONSchema(policyDef.ParameterSchemas),
	}

	return &policy, nil
}

// validatePolicy validates a policy definition
func (pl *PolicyLoader) validatePolicy(policy *api.PolicyDefinition) error {
	if strings.TrimSpace(policy.Name) == "" {
		return fmt.Errorf("policy name is required")
	}

	if strings.TrimSpace(policy.Version) == "" {
		return fmt.Errorf("policy version is required")
	}

	// Validate version format (should match pattern ^v\d+\.\d+\.\d+$)
	versionPattern := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	if !versionPattern.MatchString(policy.Version) {
		return fmt.Errorf("policy version must match semantic version format (e.g., v1.0.0, v2.1.3)")
	}

	return nil
}
