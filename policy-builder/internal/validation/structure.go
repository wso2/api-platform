package validation

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/envoy-policy-engine/policy-builder/pkg/types"
)

// ValidateDirectoryStructure validates the policy directory structure
func ValidateDirectoryStructure(policy *types.DiscoveredPolicy) []types.ValidationError {
	var errors []types.ValidationError

	// Check policy.yaml exists
	if _, err := os.Stat(policy.YAMLPath); os.IsNotExist(err) {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       "policy.yaml file not found",
		})
	}

	// Check go.mod exists
	if _, err := os.Stat(policy.GoModPath); os.IsNotExist(err) {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.GoModPath,
			Message:       "go.mod file not found",
		})
	}

	// Check at least one .go file exists
	if len(policy.SourceFiles) == 0 {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.Path,
			Message:       "no .go source files found",
		})
	}

	// Verify all source files exist
	for _, sourceFile := range policy.SourceFiles {
		if _, err := os.Stat(sourceFile); os.IsNotExist(err) {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      sourceFile,
				Message:       fmt.Sprintf("source file not found: %s", filepath.Base(sourceFile)),
			})
		}
	}

	return errors
}
