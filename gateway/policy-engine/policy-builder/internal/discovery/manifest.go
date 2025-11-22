package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
	"gopkg.in/yaml.v3"
)

const (
	SupportedManifestVersion = "v1"
)

// LoadManifest loads and validates the policy manifest file
func LoadManifest(manifestPath string) (*types.PolicyManifest, error) {
	// Read manifest file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("failed to read manifest file: %s", manifestPath),
			err,
		)
	}

	// Parse YAML
	var manifest types.PolicyManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, errors.NewDiscoveryError(
			"failed to parse manifest YAML",
			err,
		)
	}

	// Validate manifest
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// validateManifest validates the manifest structure and contents
func validateManifest(manifest *types.PolicyManifest) error {
	// Check version
	if manifest.Version == "" {
		return errors.NewDiscoveryError("manifest version is required", nil)
	}

	if manifest.Version != SupportedManifestVersion {
		return errors.NewDiscoveryError(
			fmt.Sprintf("unsupported manifest version: %s (supported: %s)",
				manifest.Version, SupportedManifestVersion),
			nil,
		)
	}

	// Check policies
	if len(manifest.Policies) == 0 {
		return errors.NewDiscoveryError("manifest must declare at least one policy", nil)
	}

	// Validate each policy entry
	seen := make(map[string]bool)
	for i, entry := range manifest.Policies {
		// Check required fields
		if entry.Name == "" {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d: name is required", i),
				nil,
			)
		}

		if entry.Version == "" {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d (%s): version is required", i, entry.Name),
				nil,
			)
		}

		if entry.URI == "" {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d (%s): uri is required", i, entry.Name),
				nil,
			)
		}

		// Check for duplicates (name:version combination must be unique)
		key := fmt.Sprintf("%s:%s", entry.Name, entry.Version)
		if seen[key] {
			return errors.NewDiscoveryError(
				fmt.Sprintf("duplicate policy entry: %s", key),
				nil,
			)
		}
		seen[key] = true
	}

	return nil
}

// DiscoverPoliciesFromManifest discovers policies declared in a manifest file
func DiscoverPoliciesFromManifest(manifestPath string, baseDir string) ([]*types.DiscoveredPolicy, error) {
	// Convert manifestPath to absolute at the start for consistent path handling
	absManifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			"failed to resolve absolute path for manifest",
			err,
		)
	}

	// Load manifest
	manifest, err := LoadManifest(absManifestPath)
	if err != nil {
		return nil, err
	}

	// Set baseDir to manifest's directory if not provided.
	if baseDir == "" {
		baseDir = filepath.Dir(absManifestPath)
	}

	var discovered []*types.DiscoveredPolicy

	// Process each manifest entry
	for _, entry := range manifest.Policies {
		// Resolve URI (support relative and absolute paths)
		policyPath := entry.URI // TODO: (renuka) This URI is not the exact path of the policy. It is the path to discover policies.
		if !filepath.IsAbs(policyPath) {
			// Relative to base directory (now guaranteed absolute)
			policyPath = filepath.Join(baseDir, entry.URI)
		}

		// Check path exists
		if _, err := os.Stat(policyPath); os.IsNotExist(err) { // TODO: (renuka) check other errors as well.
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy path does not exist: %s (from manifest entry %s:%s)",
					policyPath, entry.Name, entry.Version),
				err,
			)
		}

		// Validate directory structure
		if err := ValidateDirectoryStructure(policyPath); err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("invalid structure for %s:%s at %s", entry.Name, entry.Version, policyPath),
				err,
			)
		}

		// Parse policy.yaml
		policyYAMLPath := filepath.Join(policyPath, "policy.yaml")
		definition, err := ParsePolicyYAML(policyYAMLPath)
		if err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("failed to parse policy.yaml for %s:%s at %s", entry.Name, entry.Version, policyPath),
				err,
			)
		}

		// Validate manifest entry matches policy.yaml
		if entry.Name != definition.Name {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy name mismatch: manifest declares '%s' but policy.yaml has '%s' at %s",
					entry.Name, definition.Name, policyPath),
				nil,
			)
		}

		if entry.Version != definition.Version {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy version mismatch: manifest declares '%s' but policy.yaml has '%s' for %s at %s",
					entry.Version, definition.Version, entry.Name, policyPath),
				nil,
			)
		}

		// Collect source files
		sourceFiles, err := CollectSourceFiles(policyPath)
		if err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("failed to collect source files for %s:%s at %s", entry.Name, entry.Version, policyPath),
				err,
			)
		}

		// Create discovered policy
		policy := &types.DiscoveredPolicy{
			Name:        definition.Name,
			Version:     definition.Version,
			Path:        policyPath,
			YAMLPath:    policyYAMLPath,
			GoModPath:   filepath.Join(policyPath, "go.mod"),
			SourceFiles: sourceFiles,
			Definition:  definition,
		}

		discovered = append(discovered, policy)
	}

	return discovered, nil
}
