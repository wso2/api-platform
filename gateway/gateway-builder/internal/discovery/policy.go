package discovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/policy-engine/gateway-builder/pkg/fsutil"
	"github.com/policy-engine/gateway-builder/pkg/types"
	policy "github.com/policy-engine/sdk/policy/v1alpha"
	"gopkg.in/yaml.v3"
)

// ParsePolicyYAML reads and parses a policy definition file
func ParsePolicyYAML(path string) (*policy.PolicyDefinition, error) {
	slog.Debug("Reading policy definition", "path", path, "phase", "discovery")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", types.PolicyDefinitionFile, err)
	}

	var def policy.PolicyDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	slog.Debug("Parsed policy definition",
		"name", def.Name,
		"version", def.Version,
		"phase", "discovery")

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
	slog.Debug("Validating directory structure", "dir", policyDir, "phase", "discovery")

	// Check for policy definition file
	policyYAML := filepath.Join(policyDir, types.PolicyDefinitionFile)
	if err := fsutil.ValidatePathExists(policyYAML, types.PolicyDefinitionFile); err != nil {
		return fmt.Errorf("in %s: %w", policyDir, err)
	}

	// Check for go.mod
	goMod := filepath.Join(policyDir, "go.mod")
	if err := fsutil.ValidatePathExists(goMod, "go.mod"); err != nil {
		return fmt.Errorf("in %s: %w", policyDir, err)
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

	slog.Debug("Go file check", "dir", policyDir, "hasGoFiles", hasGoFiles, "phase", "discovery")

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
			fullPath := filepath.Join(policyDir, file.Name())
			goFiles = append(goFiles, fullPath)
			slog.Debug("Discovered Go source file",
				"file", file.Name(),
				"path", fullPath,
				"phase", "discovery")
		}
	}

	return goFiles, nil
}
