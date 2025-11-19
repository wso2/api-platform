package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/envoy-policy-engine/builder/pkg/errors"
	"github.com/envoy-policy-engine/builder/pkg/types"
)

// DiscoverPolicies walks the /policies directory and discovers all policy implementations
func DiscoverPolicies(policiesDir string) ([]*types.DiscoveredPolicy, error) {
	if _, err := os.Stat(policiesDir); os.IsNotExist(err) {
		return nil, errors.NewDiscoveryError("policies directory does not exist", err)
	}

	var discovered []*types.DiscoveredPolicy

	// Walk the policies directory
	// Expected structure: policies/policy-name/version/
	policyEntries, err := os.ReadDir(policiesDir)
	if err != nil {
		return nil, errors.NewDiscoveryError("failed to read policies directory", err)
	}

	for _, policyEntry := range policyEntries {
		if !policyEntry.IsDir() {
			continue
		}

		policyName := policyEntry.Name()
		policyPath := filepath.Join(policiesDir, policyName)

		// Read version directories
		versionEntries, err := os.ReadDir(policyPath)
		if err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("failed to read policy directory %s", policyName),
				err,
			)
		}

		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}

			version := versionEntry.Name()
			versionPath := filepath.Join(policyPath, version)

			// Validate directory structure
			if err := ValidateDirectoryStructure(versionPath); err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("invalid structure for %s/%s", policyName, version),
					err,
				)
			}

			// Parse policy.yaml
			policyYAMLPath := filepath.Join(versionPath, "policy.yaml")
			definition, err := ParsePolicyYAML(policyYAMLPath)
			if err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("failed to parse policy.yaml for %s/%s", policyName, version),
					err,
				)
			}

			// Validate version consistency
			if err := ValidateVersionConsistency(version, definition.Version); err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("version mismatch for %s", policyName),
					err,
				)
			}

			// Note: Directory name validation removed - users can organize directories however they want
			// Policy name comes from policy.yaml definition, not directory structure

			// Collect source files
			sourceFiles, err := CollectSourceFiles(versionPath)
			if err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("failed to collect source files for %s/%s", policyName, version),
					err,
				)
			}

			// Create discovered policy
			policy := &types.DiscoveredPolicy{
				Name:        definition.Name,
				Version:     definition.Version,
				Path:        versionPath,
				YAMLPath:    policyYAMLPath,
				GoModPath:   filepath.Join(versionPath, "go.mod"),
				SourceFiles: sourceFiles,
				Definition:  definition,
			}

			discovered = append(discovered, policy)
		}
	}

	if len(discovered) == 0 {
		return nil, errors.NewDiscoveryError("no policies found in directory", nil)
	}

	return discovered, nil
}
