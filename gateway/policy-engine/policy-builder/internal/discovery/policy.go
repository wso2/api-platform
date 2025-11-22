package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/policy-engine/sdk/policies"
	"gopkg.in/yaml.v3"
)

// ParsePolicyYAML reads and parses a policy.yaml file
func ParsePolicyYAML(path string) (*policies.PolicyDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy.yaml: %w", err)
	}

	var def policies.PolicyDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Basic validation
	if def.Name == "" {
		return nil, fmt.Errorf("policy name is required")
	}
	if def.Version == "" {
		return nil, fmt.Errorf("policy version is required")
	}

	return &def, nil
}

// ValidateDirectoryStructure checks if a policy directory has required files
func ValidateDirectoryStructure(policyDir string) error {
	// Check for policy.yaml
	policyYAML := filepath.Join(policyDir, "policy.yaml")
	if _, err := os.Stat(policyYAML); os.IsNotExist(err) { // TODO: (renuka) check here as well.
		return fmt.Errorf("missing policy.yaml in %s", policyDir)
	}

	// Check for go.mod
	goMod := filepath.Join(policyDir, "go.mod")
	if _, err := os.Stat(goMod); os.IsNotExist(err) { // TODO: (renuka) check here as well.
		return fmt.Errorf("missing go.mod in %s", policyDir)
	}

	// Check for at least one .go file
	files, err := os.ReadDir(policyDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", policyDir, err)
	}

	hasGoFiles := false
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".go" {
			hasGoFiles = true
			break
		}
	}

	if !hasGoFiles {
		return fmt.Errorf("no .go files found in %s", policyDir)
	}

	return nil
}

// CollectSourceFiles finds all .go files in a policy directory
func CollectSourceFiles(policyDir string) ([]string, error) {
	var goFiles []string

	files, err := os.ReadDir(policyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".go" {
			goFiles = append(goFiles, filepath.Join(policyDir, file.Name()))
		}
	}

	return goFiles, nil
}
