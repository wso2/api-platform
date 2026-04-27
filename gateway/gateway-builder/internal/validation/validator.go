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

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// ValidatePolicies runs all validation checks on discovered policies
func ValidatePolicies(policies []*types.DiscoveredPolicy) (*types.ValidationResult, error) {
	slog.Debug("Starting policy validation",
		"count", len(policies),
		"phase", "validation")

	result := &types.ValidationResult{
		Valid:    true,
		Errors:   []types.ValidationError{},
		Warnings: []types.ValidationWarning{},
	}

	// Check for duplicate policy name/version combinations
	seen := make(map[string]bool)
	for _, policy := range policies {
		key := fmt.Sprintf("%s:%s", policy.Name, policy.Version)
		slog.Debug("Checking for duplicates", "policy", key, "phase", "validation")
		if seen[key] {
			result.Errors = append(result.Errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      policy.Path,
				Message:       fmt.Sprintf("duplicate policy detected: %s", key),
			})
			result.Valid = false
		}
		seen[key] = true
	}

	// Validate each policy
	for _, policy := range policies {
		slog.Debug("Validating policy",
			"name", policy.Name,
			"version", policy.Version,
			"runtime", policy.Runtime,
			"phase", "validation")

		// YAML schema validation (applies to both Go and Python)
		yamlErrors := ValidateYAMLSchema(policy)
		result.Errors = append(result.Errors, yamlErrors...)
		if len(yamlErrors) > 0 {
			result.Valid = false
		}

		if policy.Runtime == "python" {
			// Python-specific validations
			structErrors := ValidatePythonDirectoryStructure(policy)
			result.Errors = append(result.Errors, structErrors...)
			if len(structErrors) > 0 {
				result.Valid = false
			}
		} else {
			// Go-specific validations
			// Directory structure validation
			structErrors := ValidateDirectoryStructure(policy)
			result.Errors = append(result.Errors, structErrors...)
			if len(structErrors) > 0 {
				result.Valid = false
			}

			// Go interface validation
			goErrors := ValidateGoInterface(policy)
			result.Errors = append(result.Errors, goErrors...)
			if len(goErrors) > 0 {
				result.Valid = false
			}

			// Go module validation
			goModErrors := ValidateGoMod(policy)
			result.Errors = append(result.Errors, goModErrors...)
			if len(goModErrors) > 0 {
				result.Valid = false
			}
		}
	}

	if !result.Valid {
		return result, errors.NewValidationError(
			fmt.Sprintf("validation failed with %d error(s)", len(result.Errors)),
			nil,
		)
	}

	return result, nil
}

// FormatValidationErrors creates a human-readable error report
func FormatValidationErrors(result *types.ValidationResult) string {
	if result.Valid {
		return "All validations passed"
	}

	report := fmt.Sprintf("Validation failed with %d error(s):\n\n", len(result.Errors))

	for i, err := range result.Errors {
		report += fmt.Sprintf("%d. [%s v%s] %s\n", i+1, err.PolicyName, err.PolicyVersion, err.Message)
		if err.FilePath != "" {
			report += fmt.Sprintf("   File: %s", err.FilePath)
			if err.LineNumber > 0 {
				report += fmt.Sprintf(":%d", err.LineNumber)
			}
			report += "\n"
		}
		report += "\n"
	}

	if len(result.Warnings) > 0 {
		report += fmt.Sprintf("\nWarnings (%d):\n\n", len(result.Warnings))
		for i, warn := range result.Warnings {
			report += fmt.Sprintf("%d. [%s v%s] %s\n", i+1, warn.PolicyName, warn.PolicyVersion, warn.Message)
			if warn.FilePath != "" {
				report += fmt.Sprintf("   File: %s\n", warn.FilePath)
			}
			report += "\n"
		}
	}

	return report
}
