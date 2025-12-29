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

package validation

import (
	"fmt"
	"log/slog"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// ValidateYAMLSchema validates the policy.yaml structure and required fields
func ValidateYAMLSchema(policy *types.DiscoveredPolicy) []types.ValidationError {
	slog.Debug("Validating YAML schema",
		"policy", policy.Name,
		"version", policy.Version,
		"phase", "validation")

	var errors []types.ValidationError

	def := policy.Definition

	// Validate required fields
	if def.Name == "" {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       "policy name is required",
		})
	}

	if def.Version == "" {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       "policy version is required",
		})
	}

	// Validate version format (basic semver check)
	if !isValidVersion(def.Version) {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       fmt.Sprintf("invalid version format: %s (expected semver like v1.0.0)", def.Version),
		})
	}

	return errors
}

// isValidVersion checks if version follows basic semver format
func isValidVersion(version string) bool {
	// Simple check: starts with 'v' and has at least one dot
	if len(version) < 5 {
		return false
	}
	if version[0] != 'v' {
		return false
	}
	// Should have format like v1.0.0
	hasVersion := false
	for _, c := range version[1:] {
		if c == '.' {
			hasVersion = true
			break
		}
	}
	return hasVersion
}
