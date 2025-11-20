package validation

import (
	"fmt"

	"github.com/envoy-policy-engine/builder/pkg/types"
)

// ValidateYAMLSchema validates the policy.yaml structure and required fields
func ValidateYAMLSchema(policy *types.DiscoveredPolicy) []types.ValidationError {
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
