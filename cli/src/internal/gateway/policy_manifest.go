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

package gateway

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// PolicyManifest represents the policy-manifest file structure
type PolicyManifest struct {
	Version           string   `yaml:"version"`
	VersionResolution string   `yaml:"versionResolution,omitempty"`
	Policies          []Policy `yaml:"policies"`
}

// Policy represents a single policy in the manifest
type Policy struct {
	Name              string `yaml:"name"`
	Version           string `yaml:"version"`
	VersionResolution string `yaml:"versionResolution,omitempty"`
	FilePath          string `yaml:"filePath,omitempty"`
}

// ValidatePolicyManifest validates the policy-manifest structure
func ValidatePolicyManifest(manifest *PolicyManifest) error {

	// Validate policies array
	if len(manifest.Policies) == 0 {
		return fmt.Errorf("'policies' array is required and must not be empty")
	}

	// Validate each policy
	for i, policy := range manifest.Policies {
		if err := validatePolicy(&policy, i); err != nil {
			return err
		}
	}

	return nil
}

// validatePolicy validates a single policy entry
func validatePolicy(policy *Policy, index int) error {
	// Validate name (required)
	if policy.Name == "" {
		return fmt.Errorf("policy[%d]: 'name' field is required", index)
	}

	// Validate filePath if provided (check if file exists)
	if policy.FilePath != "" {
		if _, err := os.Stat(policy.FilePath); os.IsNotExist(err) {
			return fmt.Errorf("policy[%d] (%s): file path does not exist: %s", index, policy.Name, policy.FilePath)
		}
	}

	return nil
}

// LoadPolicyManifest loads and validates a policy-manifest file
func LoadPolicyManifest(filePath string) (*PolicyManifest, error) {
	// Read the file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy-manifest file: %w", err)
	}

	// Parse YAML
	var manifest PolicyManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse policy-manifest YAML: %w", err)
	}

	// Validate the manifest
	if err := ValidatePolicyManifest(&manifest); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &manifest, nil
}
