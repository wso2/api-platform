package validation

import (
	"fmt"

	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
)

// ValidatePolicies runs all validation checks on discovered policies
func ValidatePolicies(policies []*types.DiscoveredPolicy) (*types.ValidationResult, error) {
	result := &types.ValidationResult{
		Valid:    true,
		Errors:   []types.ValidationError{},
		Warnings: []types.ValidationWarning{},
	}

	// Check for duplicate policy name/version combinations
	seen := make(map[string]bool)
	for _, policy := range policies {
		key := fmt.Sprintf("%s:%s", policy.Name, policy.Version)
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
		// Directory structure validation
		structErrors := ValidateDirectoryStructure(policy)
		result.Errors = append(result.Errors, structErrors...)
		if len(structErrors) > 0 {
			result.Valid = false
		}

		// YAML schema validation
		yamlErrors := ValidateYAMLSchema(policy)
		result.Errors = append(result.Errors, yamlErrors...)
		if len(yamlErrors) > 0 {
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
