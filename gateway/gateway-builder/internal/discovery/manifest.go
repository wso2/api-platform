/*
 * Copyright (c) 2025, WSO2 LLC. (https://wso2.com).
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
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

const (
	SupportedBuildFileVersion = "v1"
)

// LoadBuildFile loads and validates the build file
func LoadBuildFile(buildFilePath string) (*types.BuildFile, error) {
	slog.Debug("Reading build file", "path", buildFilePath, "phase", "discovery")

	data, err := os.ReadFile(buildFilePath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("failed to read build file: %s", buildFilePath),
			err,
		)
	}

	// Parse YAML
	var bf types.BuildFile
	if err := yaml.Unmarshal(data, &bf); err != nil {
		return nil, errors.NewDiscoveryError(
			"failed to parse build file YAML",
			err,
		)
	}

	slog.Debug("Parsed build file",
		"version", bf.Version,
		"policyCount", len(bf.Policies),
		"phase", "discovery")

	if err := validateBuildFile(&bf); err != nil {
		return nil, err
	}

	return &bf, nil
}

// validateBuildFile validates the build file structure and contents
func validateBuildFile(bf *types.BuildFile) error {
	if bf.Version == "" {
		return errors.NewDiscoveryError("build file version is required", nil)
	}

	if bf.Version != SupportedBuildFileVersion {
		return errors.NewDiscoveryError(
			fmt.Sprintf("unsupported build file version: %s (supported: %s)",
				bf.Version, SupportedBuildFileVersion),
			nil,
		)
	}

	if len(bf.Policies) == 0 {
		return errors.NewDiscoveryError("build file must declare at least one policy", nil)
	}

	// Validate each policy entry
	seen := make(map[string]bool)
	for i, entry := range bf.Policies {
		slog.Debug("Validating build file entry",
			"index", i,
			"name", entry.Name,
			"filePath", entry.FilePath,
			"gomodule", entry.Gomodule,
			"pipPackage", entry.PipPackage,
			"phase", "discovery")

		// Check required fields
		if entry.Name == "" {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d: name is required", i),
				nil,
			)
		}

		// Count sources provided
		sourceCount := 0
		if entry.FilePath != "" {
			sourceCount++
		}
		if entry.Gomodule != "" {
			sourceCount++
		}
		if entry.PipPackage != "" {
			sourceCount++
		}

		if sourceCount == 0 {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d (%s): either filePath, gomodule, or pipPackage must be provided", i, entry.Name),
				nil,
			)
		}

		if sourceCount > 1 {
			return errors.NewDiscoveryError(
				fmt.Sprintf("policy entry %d (%s): only one of filePath, gomodule, or pipPackage may be provided", i, entry.Name),
				nil,
			)
		}

		// Check for duplicates based on name + source to avoid ambiguity
		key := fmt.Sprintf("%s:%s|%s|%s", entry.Name, entry.FilePath, entry.Gomodule, entry.PipPackage)
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

// DiscoverPoliciesFromBuildFile discovers policies declared in a build file
func DiscoverPoliciesFromBuildFile(buildFilePath string, baseDir string) ([]*types.DiscoveredPolicy, error) {
	absBuildFilePath, err := filepath.Abs(buildFilePath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			"failed to resolve absolute path for build file",
			err,
		)
	}

	slog.Debug("Resolved build file path",
		"original", buildFilePath,
		"absolute", absBuildFilePath,
		"phase", "discovery")

	bf, err := LoadBuildFile(absBuildFilePath)
	if err != nil {
		return nil, err
	}

	if baseDir == "" {
		baseDir = filepath.Dir(absBuildFilePath)
		slog.Debug("Using build file directory as baseDir",
			"baseDir", baseDir,
			"phase", "discovery")
	}

	var discovered []*types.DiscoveredPolicy

	for _, entry := range bf.Policies {
		// Handle pip package (explicit Python remote)
		if entry.PipPackage != "" {
			policy, err := discoverPipPolicy(entry)
			if err != nil {
				return nil, err
			}
			discovered = append(discovered, policy)
			continue
		}

		// Handle Go module (explicit Go remote)
		if entry.Gomodule != "" {
			policy, err := discoverGoPolicy(entry, baseDir)
			if err != nil {
				return nil, err
			}
			discovered = append(discovered, policy)
			continue
		}

		// Handle filePath — auto-detect runtime by directory fingerprint
		if entry.FilePath != "" {
			policyPath := filepath.Join(baseDir, entry.FilePath)

			if err := fsutil.ValidatePathExists(policyPath, "policy path"); err != nil {
				return nil, errors.NewDiscoveryError(
					fmt.Sprintf("from build file entry %s: %v", entry.Name, err),
					err,
				)
			}

			runtime := DetectRuntime(policyPath)

			if runtime == "python" {
				policy, err := discoverLocalPythonPolicy(entry, baseDir)
				if err != nil {
					return nil, err
				}
				discovered = append(discovered, policy)
			} else {
				policy, err := discoverGoPolicy(entry, baseDir)
				if err != nil {
					return nil, err
				}
				discovered = append(discovered, policy)
			}
			continue
		}
	}

	return discovered, nil
}

// DetectRuntime auto-detects the policy runtime by examining the directory contents.
// Presence of go.mod → "go"; presence of .py files (and no go.mod) → "python".
func DetectRuntime(policyDir string) string {
	goMod := filepath.Join(policyDir, "go.mod")
	if _, err := os.Stat(goMod); err == nil {
		return "go"
	}

	entries, err := os.ReadDir(policyDir)
	if err != nil {
		return "go"
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".py" {
			return "python"
		}
	}

	return "go"
}

// discoverPipPolicy discovers a Python policy from a pip package reference.
// It downloads the wheel (without deps), extracts metadata and source for
// build-time validation, and records the pip spec so the python-deps Docker
// stage can do the real install on the correct platform.
func discoverPipPolicy(entry types.BuildEntry) (*types.DiscoveredPolicy, error) {
	pkgInfo, err := FetchPipPackage(entry.PipPackage)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("failed to fetch pip package for %s: %v", entry.Name, err),
			err,
		)
	}

	slog.Info("Resolved Python policy entry via pip",
		"name", entry.Name,
		"pipPackage", entry.PipPackage,
		"topLevelModule", pkgInfo.TopLevelModule,
		"resolvedPath", pkgInfo.Dir)

	// Validate extracted source and parse definition
	discovered, err := buildPythonDiscoveredPolicy(entry, pkgInfo.Dir, "pipPackage")
	if err != nil {
		return nil, err
	}

	// Set pip-specific fields
	discovered.IsPipPackage = true
	discovered.PipSpec = pkgInfo.PipSpec
	discovered.PipIndexURL = pkgInfo.IndexURL
	discovered.PythonTopLevelModule = pkgInfo.TopLevelModule

	return discovered, nil
}

// discoverLocalPythonPolicy discovers a Python policy from a local filePath entry
func discoverLocalPythonPolicy(entry types.BuildEntry, baseDir string) (*types.DiscoveredPolicy, error) {
	policyPath := filepath.Join(baseDir, entry.FilePath)

	slog.Info("Resolved Python policy entry via filePath",
		"name", entry.Name,
		"filePath", entry.FilePath,
		"resolvedPath", policyPath)

	return buildPythonDiscoveredPolicy(entry, policyPath, "filePath")
}

// buildPythonDiscoveredPolicy is the shared logic for building a discovered Python policy
func buildPythonDiscoveredPolicy(entry types.BuildEntry, policyPath string, source string) (*types.DiscoveredPolicy, error) {
	if err := fsutil.ValidatePathExists(policyPath, "policy path"); err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("from build file entry %s: %v", entry.Name, err),
			err,
		)
	}

	if err := ValidatePythonDirectoryStructure(policyPath); err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("invalid Python policy structure for %s at %s", entry.Name, policyPath),
			err,
		)
	}

	policyYAMLPath := filepath.Join(policyPath, types.PolicyDefinitionFile)
	definition, err := ParsePolicyYAML(policyYAMLPath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("failed to parse %s for %s at %s", types.PolicyDefinitionFile, entry.Name, policyPath),
			err,
		)
	}

	if entry.Name != definition.Name {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("policy name mismatch: build file declares '%s' but %s has '%s' at %s",
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

	sourceFiles, err := CollectPythonSourceFiles(policyPath)
	if err != nil {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("failed to collect Python source files for %s:%s at %s", entry.Name, definition.Version, policyPath),
			err,
		)
	}

	processingMode := parseProcessingMode(definition.ProcessingModeConfig)

	discovered := &types.DiscoveredPolicy{
		Name:             definition.Name,
		Version:          definition.Version,
		Path:             policyPath,
		YAMLPath:         policyYAMLPath,
		SourceFiles:      sourceFiles,
		SystemParameters: ExtractDefaultValues(definition.SystemParameters),
		Definition:       definition,
		Runtime:          "python",
		PythonSourceDir:  policyPath,
		ProcessingMode:   processingMode,
	}

	slog.Info("Discovered Python policy",
		"name", discovered.Name,
		"version", discovered.Version,
		"source", source,
		"path", policyPath,
		"phase", "discovery")

	return discovered, nil
}

// discoverGoPolicy discovers a Go policy from build file entry
func discoverGoPolicy(entry types.BuildEntry, baseDir string) (*types.DiscoveredPolicy, error) {
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
			fmt.Sprintf("from build file entry %s: %v", entry.Name, err),
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

	// Validate build file entry matches policy definition name
	if entry.Name != definition.Name {
		return nil, errors.NewDiscoveryError(
			fmt.Sprintf("policy name mismatch: build file declares '%s' but %s has '%s' at %s",
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
	discovered := &types.DiscoveredPolicy{
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
		Runtime:          "go", // Default runtime for Go policies
	}

	return discovered, nil
}

// parseProcessingMode converts ProcessingModeConfig to ProcessingMode
func parseProcessingMode(config *policy.ProcessingModeConfig) *policy.ProcessingMode {
	if config == nil {
		// Default: process request headers only
		return &policy.ProcessingMode{
			RequestHeaderMode:  policy.HeaderModeProcess,
			RequestBodyMode:    policy.BodyModeSkip,
			ResponseHeaderMode: policy.HeaderModeSkip,
			ResponseBodyMode:   policy.BodyModeSkip,
		}
	}

	mode := &policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}

	if config.RequestHeaderMode == "SKIP" {
		mode.RequestHeaderMode = policy.HeaderModeSkip
	}
	if config.RequestBodyMode == "BUFFER" {
		mode.RequestBodyMode = policy.BodyModeBuffer
	}
	if config.ResponseHeaderMode == "PROCESS" {
		mode.ResponseHeaderMode = policy.HeaderModeProcess
	}
	if config.ResponseBodyMode == "BUFFER" {
		mode.ResponseBodyMode = policy.BodyModeBuffer
	}

	return mode
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

	if modFile.Module == nil || modFile.Module.Mod.Path == "" {
		return "", fmt.Errorf("module directive missing in go.mod: %s", goModPath)
	}

	return modFile.Module.Mod.Path, nil
}
