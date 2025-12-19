package discovery

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/policy-engine/gateway-builder/pkg/errors"
	"github.com/policy-engine/gateway-builder/pkg/fsutil"
	"github.com/policy-engine/gateway-builder/pkg/types"
	"gopkg.in/yaml.v3"
)

const (
	SupportedManifestVersion = "v1"
)

// LoadManifest loads and validates the policy manifest lock file
func LoadManifest(manifestLockPath string) (*types.PolicyManifestLock, error) {
	slog.Debug("Reading manifest lock file", "path", manifestLockPath, "phase", "discovery")

	// Read manifest lock file
	data, err := os.ReadFile(manifestLockPath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("failed to read manifest lock file: %s", manifestLockPath),
			err,
		)
	}

	// Parse YAML
	var manifest types.PolicyManifestLock
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, errors.NewDiscoveryError(
			"failed to parse manifest YAML",
			err,
		)
	}

	slog.Debug("Parsed manifest",
		"version", manifest.Version,
		"policyCount", len(manifest.Policies),
		"phase", "discovery")

	// Validate manifest
	if err := validateManifest(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// validateManifest validates the manifest lock structure and contents
func validateManifest(manifest *types.PolicyManifestLock) error {
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
		slog.Debug("Validating manifest entry",
			"index", i,
			"name", entry.Name,
			"version", entry.Version,
			"derivedPath", entry.DeriveFilePath(),
			"phase", "discovery")

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

		// Check for duplicates (name:version combination must be unique)
		key := fmt.Sprintf("%s:%s", entry.Name, entry.Version)
		if seen[key] {
			return errors.NewDiscoveryError(
				fmt.Sprintf("duplicate policy entry: %s", key),
				nil,
			)
		}
		slog.Debug("Policy entry is unique", "key", key, "phase", "discovery")
		seen[key] = true
	}

	return nil
}

// DiscoverPoliciesFromManifest discovers policies declared in a manifest lock file
func DiscoverPoliciesFromManifest(manifestLockPath string, baseDir string) ([]*types.DiscoveredPolicy, error) {
	// Convert manifestLockPath to absolute at the start for consistent path handling
	absManifestLockPath, err := filepath.Abs(manifestLockPath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			"failed to resolve absolute path for manifest lock",
			err,
		)
	}

	slog.Debug("Resolved manifest lock path",
		"original", manifestLockPath,
		"absolute", absManifestLockPath,
		"phase", "discovery")

	// Load manifest lock
	manifest, err := LoadManifest(absManifestLockPath)
	if err != nil {
		return nil, err
	}

	// Set baseDir to manifest lock's directory if not provided.
	if baseDir == "" {
		baseDir = filepath.Dir(absManifestLockPath)
		slog.Debug("Using manifest lock directory as baseDir",
			"baseDir", baseDir,
			"phase", "discovery")
	}

	var discovered []*types.DiscoveredPolicy

	// Process each manifest entry
	for _, entry := range manifest.Policies {
		// Derive file path from policy name and version (always relative to baseDir)
		derivedPath := entry.DeriveFilePath()
		policyPath := filepath.Join(baseDir, derivedPath)

		slog.Debug("Resolving policy path",
			"policy", entry.Name,
			"version", entry.Version,
			"derivedPath", derivedPath,
			"resolvedPath", policyPath,
			"phase", "discovery")

		// Check path exists and is accessible
		if err := fsutil.ValidatePathExists(policyPath, "policy path"); err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("from manifest entry %s:%s: %v", entry.Name, entry.Version, err),
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

		// Parse policy definition
		policyYAMLPath := filepath.Join(policyPath, types.PolicyDefinitionFile)
		definition, err := ParsePolicyYAML(policyYAMLPath)
		if err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("failed to parse %s for %s:%s at %s", types.PolicyDefinitionFile, entry.Name, entry.Version, policyPath),
				err,
			)
		}

		slog.Debug("Parsed policy definition",
			"name", definition.Name,
			"version", definition.Version,
			"path", policyYAMLPath,
			"phase", "discovery")

		// Validate manifest entry matches policy definition
		if entry.Name != definition.Name {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy name mismatch: manifest declares '%s' but %s has '%s' at %s",
					entry.Name, types.PolicyDefinitionFile, definition.Name, policyPath),
				nil,
			)
		}

		if entry.Version != definition.Version {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy version mismatch: manifest declares '%s' but %s has '%s' for %s at %s",
					entry.Version, types.PolicyDefinitionFile, definition.Version, entry.Name, policyPath),
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

		slog.Debug("Collected source files",
			"policy", entry.Name,
			"count", len(sourceFiles),
			"files", sourceFiles,
			"phase", "discovery")

		// Create discovered policy
		policy := &types.DiscoveredPolicy{
			Name:             definition.Name,
			Version:          definition.Version,
			Path:             policyPath,
			YAMLPath:         policyYAMLPath,
			GoModPath:        filepath.Join(policyPath, "go.mod"),
			SourceFiles:      sourceFiles,
			SystemParameters: ExtractDefaultValues(definition.SystemParameters),
			Definition:       definition,
		}

		discovered = append(discovered, policy)
	}

	return discovered, nil
}
