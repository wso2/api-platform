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

package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/fsutil"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

const (
	SupportedManifestVersion = "v1"
)

// LoadManifest loads and validates the policy manifest lock file
func LoadManifest(manifestLockPath string) (*types.PolicyManifest, error) {
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
	var manifest types.PolicyManifest
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
func validateManifest(manifest *types.PolicyManifest) error {
	// Check manifest version
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
			"filePath", entry.FilePath,
			"gomodule", entry.Gomodule,
			"phase", "discovery")

		// Check required fields
		if entry.Name == "" {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d: name is required", i),
				nil,
			)
		}

		if entry.FilePath == "" && entry.Gomodule == "" {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d (%s): either filePath or gomodule must be provided", i, entry.Name),
				nil,
			)
		}

		if entry.FilePath != "" && entry.Gomodule != "" {
			slog.Debug("Both filePath and gomodule provided; preferring filePath", "name", entry.Name)
		}

		// Check for duplicates based on name + filePath/gomodule to avoid ambiguity
		key := fmt.Sprintf("%s:%s|%s", entry.Name, entry.FilePath, entry.Gomodule)
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
		var policyPath string
		var source string
		var goModulePath string
		var goModuleVersion string
		var isFilePathEntry bool

		if entry.FilePath != "" {
			policyPath = filepath.Join(baseDir, entry.FilePath)
			source = "filePath"
			isFilePathEntry = true

			// Read the module path from the policy's own go.mod
			modulePath, err := extractModulePathFromGoMod(filepath.Join(policyPath, "go.mod"))
			if err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("failed to read module path from go.mod for %s: %v", entry.Name, err),
					err,
				)
			}
			goModulePath = modulePath

			slog.Info("Resolved policy entry via filePath",
				"name", entry.Name,
				"filePath", entry.FilePath,
				"resolvedPath", policyPath,
				"goModulePath", goModulePath)
		} else if entry.Gomodule != "" {
			modInfo, err := resolveModuleInfo(entry.Gomodule)
			if err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("failed to resolve gomodule for %s: %v", entry.Name, err),
					err,
				)
			}
			policyPath = modInfo.Dir
			goModulePath = modInfo.Path
			goModuleVersion = modInfo.Version
			source = "gomodule"

			slog.Info("Resolved policy entry via remote module",
				"name", entry.Name,
				"gomodule", entry.Gomodule,
				"resolvedPath", policyPath,
				"goModuleVersion", goModuleVersion)
		} else {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %s: either filePath or gomodule must be provided", entry.Name),
				nil,
			)
		}

		slog.Debug("Resolving policy",
			"policy", entry.Name,
			"source", source,
			"path", policyPath,
			"goModulePath", goModulePath,
			"goModuleVersion", goModuleVersion,
			"phase", "discovery")

		// Check path exists and is accessible
		if err := fsutil.ValidatePathExists(policyPath, "policy path"); err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("from manifest entry %s: %v", entry.Name, err),
				err,
			)
		}

		// Validate directory structure
		if err := ValidateDirectoryStructure(policyPath); err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("invalid structure for %s at %s", entry.Name, policyPath),
				err,
			)
		}

		// Parse policy definition
		policyYAMLPath := filepath.Join(policyPath, types.PolicyDefinitionFile)
		definition, err := ParsePolicyYAML(policyYAMLPath)
		if err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("failed to parse %s for %s at %s", types.PolicyDefinitionFile, entry.Name, policyPath),
				err,
			)
		}

		slog.Debug("Parsed policy definition",
			"name", definition.Name,
			"version", definition.Version,
			"path", policyYAMLPath,
			"phase", "discovery")

		// Validate manifest entry matches policy definition name
		if entry.Name != definition.Name {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy name mismatch: manifest declares '%s' but %s has '%s' at %s",
					entry.Name, types.PolicyDefinitionFile, definition.Name, policyPath),
				nil,
			)
		}

		if definition.Version == "" {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("policy version cannot be found in definition for %s", entry.Name),
				nil,
			)
		}

		// Collect source files
		sourceFiles, err := CollectSourceFiles(policyPath)
		if err != nil {
			return nil, errors.NewDiscoveryError(
				fmt.Sprintf("failed to collect source files for %s:%s at %s", entry.Name, definition.Version, policyPath),
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
			GoModulePath:     goModulePath,
			GoModuleVersion:  goModuleVersion,
			IsFilePathEntry:  isFilePathEntry,
		}

		discovered = append(discovered, policy)
	}

	return discovered, nil
}

// moduleInfo contains resolved module information from 'go mod download'
type moduleInfo struct {
	Path    string // e.g., "github.com/wso2/gateway-controllers/policies/add-headers"
	Version string // e.g., "v0.1.0"
	Dir     string // Local directory path in module cache
}

// resolveModuleInfo resolves a Go module and returns full module information
func resolveModuleInfo(gomodule string) (*moduleInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", gomodule)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("timed out while running 'go mod download -json %s'", gomodule)
		}
		return nil, fmt.Errorf("failed to run 'go mod download -json %s': %w; stderr: %s", gomodule, err, stderr.String())
	}

	var info struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
		Dir     string `json:"Dir"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("failed to parse 'go mod download' output: %w", err)
	}

	if info.Dir == "" {
		return nil, fmt.Errorf("module download did not return a Dir for %s", gomodule)
	}

	return &moduleInfo{
		Path:    info.Path,
		Version: info.Version,
		Dir:     info.Dir,
	}, nil
}

// extractModulePathFromGoMod reads the module path from a go.mod file
func extractModulePathFromGoMod(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return "", fmt.Errorf("failed to parse go.mod: %w", err)
	}

	return modFile.Module.Mod.Path, nil
}
