package validation

import (
	"fmt"
	"path/filepath"

	"github.com/policy-engine/gateway-builder/pkg/fsutil"
	"github.com/policy-engine/gateway-builder/pkg/types"
)

// ValidateDirectoryStructure validates the policy directory structure
func ValidateDirectoryStructure(policy *types.DiscoveredPolicy) []types.ValidationError {
	var errors []types.ValidationError

	// Check policy definition file exists
	if err := fsutil.ValidatePathExists(policy.YAMLPath, types.PolicyDefinitionFile); err != nil {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.YAMLPath,
			Message:       err.Error(),
		})
	}

	// Check go.mod exists
	if err := fsutil.ValidatePathExists(policy.GoModPath, "go.mod"); err != nil {
		errors = append(errors, types.ValidationError{
			PolicyName:    policy.Name,
			PolicyVersion: policy.Version,
			FilePath:      policy.GoModPath,
			Message:       err.Error(),
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
		if err := fsutil.ValidatePathExists(sourceFile, "source file"); err != nil {
			errors = append(errors, types.ValidationError{
				PolicyName:    policy.Name,
				PolicyVersion: policy.Version,
				FilePath:      sourceFile,
				Message:       fmt.Sprintf("%s: %s", filepath.Base(sourceFile), err.Error()),
			})
		}
	}

	return errors
}
