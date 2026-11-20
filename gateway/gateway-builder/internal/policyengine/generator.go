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

package policyengine

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/discovery"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/errors"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

const BuilderVersion = "v1.0.0"

// GenerateCode orchestrates all code generation tasks
func GenerateCode(srcDir string, policies []*types.DiscoveredPolicy, outputDir string) error {
	slog.Debug("Starting code generation",
		"srcDir", srcDir,
		"outputDir", outputDir,
		"policyCount", len(policies),
		"phase", "generation")

	// Separate Go and Python policies
	var goPolicies []*types.DiscoveredPolicy
	var pythonPolicies []*types.DiscoveredPolicy

	for _, p := range policies {
		if p.Runtime == "python" {
			pythonPolicies = append(pythonPolicies, p)
		} else {
			goPolicies = append(goPolicies, p)
		}
	}

	slog.Info("Generating code for policies",
		"goPolicies", len(goPolicies),
		"pythonPolicies", len(pythonPolicies),
		"phase", "generation")

	// Always create Python executor base (even if no Python policies)
	// This ensures the Docker build doesn't fail
	if err := generatePythonExecutorBase(srcDir, outputDir); err != nil {
		return errors.NewGenerationError("failed to generate Python executor base", err)
	}

	// Generated files go in cmd/policy-engine (main package)
	mainPkgDir := filepath.Join(srcDir, "cmd", "policy-engine")
	slog.Debug("Code generation target", "mainPkgDir", mainPkgDir, "phase", "generation")

	// Generate plugin_registry.go (includes both Go and Python)
	registryCode, err := GeneratePluginRegistry(policies, srcDir)
	if err != nil {
		return errors.NewGenerationError("failed to generate plugin registry", err)
	}

	registryPath := filepath.Join(mainPkgDir, "plugin_registry.go")
	slog.Info("Writing plugin_registry.go",
		"path", registryPath,
		"phase", "generation")
	if err := os.WriteFile(registryPath, []byte(registryCode), 0600); err != nil {
		return errors.NewGenerationError("failed to write plugin_registry.go", err)
	}

	slog.Info("Generated plugin_registry.go",
		"policies", len(policies),
		"path", registryPath,
		"phase", "generation")

	// Generate build_info.go
	buildInfoCode, err := GenerateBuildInfo(policies, BuilderVersion)
	if err != nil {
		return errors.NewGenerationError("failed to generate build info", err)
	}

	buildInfoPath := filepath.Join(mainPkgDir, "build_info.go")
	if err := os.WriteFile(buildInfoPath, []byte(buildInfoCode), 0600); err != nil {
		return errors.NewGenerationError("failed to write build_info.go", err)
	}

	slog.Info("Generated build_info.go",
		"path", buildInfoPath,
		"phase", "generation")

	// Update go.mod: 'go get' for remote policies, replace directives for local ones
	if len(goPolicies) > 0 {
		if err := UpdateGoMod(srcDir, goPolicies); err != nil {
			return errors.NewGenerationError("failed to update go.mod", err)
		}

		slog.Info("Updated go.mod for Go policies",
			"count", len(goPolicies),
			"phase", "generation")
	}

	// Generate Python artifacts if there are Python policies
	if len(pythonPolicies) > 0 {
		if err := GeneratePythonArtifacts(srcDir, pythonPolicies, outputDir); err != nil {
			return errors.NewGenerationError("failed to generate Python artifacts", err)
		}
	}

	return nil
}

// generatePythonExecutorBase generates the base Python executor files
// This is always called to ensure Docker builds don't fail
func generatePythonExecutorBase(srcDir string, outputDir string) error {
	pythonOutputDir := filepath.Join(outputDir, "python-executor")
	if err := os.MkdirAll(pythonOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create python output directory: %w", err)
	}

	// Copy Python executor source to output
	// srcDir is the policy-engine directory, so go up one level to get gateway-runtime
	executorSrcDir := filepath.Join(srcDir, "..", "python-executor")
	if err := copyDir(executorSrcDir, pythonOutputDir); err != nil {
		return fmt.Errorf("failed to copy Python executor source: %w", err)
	}

	// Always emit the requirement files the Docker build expects, even when the
	// build has no Python policies.
	return writePythonRequirementFiles(pythonOutputDir, nil)
}

// GeneratePythonArtifacts generates Python-specific build artifacts.
// This includes:
//  1. Copy each local policy's Python source into output/python-executor/policies/<name>_<major>/
//     (the module name carries the major version; pip policies are skipped — they are installed by pip in the python-deps Docker stage)
//  2. Generate python_policy_registry.py
//  3. Write the policy requirement files (see writePythonRequirementFiles)
func GeneratePythonArtifacts(srcDir string, pythonPolicies []*types.DiscoveredPolicy, outputDir string) error {
	slog.Info("Generating Python artifacts",
		"policyCount", len(pythonPolicies),
		"outputDir", outputDir,
		"phase", "generation")

	pythonOutputDir := filepath.Join(outputDir, "python-executor")

	// 1. Copy local (filePath) policies only — pip policies are installed on the correct
	//    platform by pip in the python-deps Docker stage.
	hasLocalPolicies := false
	for _, p := range pythonPolicies {
		if !p.IsPipPackage {
			hasLocalPolicies = true
			break
		}
	}

	if hasLocalPolicies {
		policiesDir := filepath.Join(pythonOutputDir, "policies")
		if err := os.MkdirAll(policiesDir, 0755); err != nil {
			return fmt.Errorf("failed to create policies directory: %w", err)
		}

		for _, p := range pythonPolicies {
			if p.IsPipPackage {
				continue
			}
			policyDestDir := filepath.Join(policiesDir, localPythonModuleName(p))
			if err := copyDir(p.PythonSourceDir, policyDestDir); err != nil {
				return fmt.Errorf("failed to copy Python policy %s: %w", p.Name, err)
			}
			slog.Debug("Copied local Python policy", "name", p.Name, "dest", policyDestDir)
		}
	}

	// 2. Generate python_policy_registry.py
	registryContent := generatePythonRegistry(pythonPolicies)
	registryPath := filepath.Join(pythonOutputDir, "python_policy_registry.py")
	if err := os.WriteFile(registryPath, []byte(registryContent), 0644); err != nil {
		return fmt.Errorf("failed to write python_policy_registry.py: %w", err)
	}

	slog.Debug("Generated python_policy_registry.py", "path", registryPath)

	// 3. Write the requirement files used by the python-deps Docker stage
	if err := writePythonRequirementFiles(pythonOutputDir, pythonPolicies); err != nil {
		return err
	}

	slog.Info("Generated Python artifacts successfully",
		"policies", len(pythonPolicies),
		"phase", "generation")

	return nil
}

// generatePythonRegistry generates the python_policy_registry.py file.
// For local (filePath) policies, the module path is "policies.<sanitized_name>_<major>.policy".
// For pip policies, the module path uses the real top-level module name: "<top_level>.policy".
func generatePythonRegistry(policies []*types.DiscoveredPolicy) string {
	var sb strings.Builder

	sb.WriteString("# Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.\n")
	sb.WriteString("#\n")
	sb.WriteString("# Licensed under the Apache License, Version 2.0 (the \"License\");\n")
	sb.WriteString("# you may not use this file except in compliance with the License.\n")
	sb.WriteString("# You may obtain a copy of the License at\n")
	sb.WriteString("#\n")
	sb.WriteString("# http://www.apache.org/licenses/LICENSE-2.0\n")
	sb.WriteString("#\n")
	sb.WriteString("# Unless required by applicable law or agreed to in writing,\n")
	sb.WriteString("# software distributed under the License is distributed on an \"AS IS\" BASIS,\n")
	sb.WriteString("# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.\n")
	sb.WriteString("# See the License for the specific language governing permissions and\n")
	sb.WriteString("# limitations under the License.\n")
	sb.WriteString("\n")
	sb.WriteString("# Auto-generated by Gateway Builder. DO NOT EDIT.\n")
	sb.WriteString("\n")
	sb.WriteString("PYTHON_POLICIES = {\n")

	for _, p := range policies {
		majorVersion := strings.Split(p.Version, ".")[0]
		key := fmt.Sprintf("%s:%s", p.Name, majorVersion)

		var modulePath string
		if p.IsPipPackage {
			modulePath = fmt.Sprintf("%s.policy", p.PythonTopLevelModule)
		} else {
			modulePath = fmt.Sprintf("policies.%s.policy", localPythonModuleName(p))
		}

		sb.WriteString(fmt.Sprintf("    \"%s\": \"%s\",\n", key, modulePath))
	}

	sb.WriteString("}\n")

	return sb.String()
}

// Requirement files written to output/python-executor. The python-deps Docker
// stage installs them in this order:
//
//  1. BaseRequirementsFile: the executor's own dependencies, a fully pinned lock
//     with a sha256 for every artifact, installed with --require-hashes.
//  2. HashedPolicyRequirementsFile: indexed policy packages pinned with a
//     checksum in the build file, installed with --require-hashes --no-deps.
//  3. PolicyRequirementsFile: everything else the policies need (git policy
//     packages pinned to a commit SHA, the indexed packages again so their
//     dependencies are resolved, and local policies' requirements.txt lines),
//     installed with BaseConstraintsFile as constraints so the verified base
//     packages cannot be replaced.
//
// The files are kept apart because pip's hash-checking mode applies to a whole
// install and cannot verify git checkouts.
const (
	BaseRequirementsFile         = "requirements.txt"
	BaseConstraintsFile          = "base-constraints.txt"
	PolicyRequirementsFile       = "policy-requirements.txt"
	HashedPolicyRequirementsFile = "policy-requirements-hashed.txt"
)

// writePythonRequirementFiles writes the constraints and policy requirement
// files next to the base requirements lock in pythonOutputDir.
func writePythonRequirementFiles(pythonOutputDir string, policies []*types.DiscoveredPolicy) error {
	baseRequirements := ""
	data, err := os.ReadFile(filepath.Join(pythonOutputDir, BaseRequirementsFile))
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read base %s: %w", BaseRequirementsFile, err)
		}
	} else {
		baseRequirements = string(data)
	}

	policyRequirements, err := mergeRequirements(policies)
	if err != nil {
		return fmt.Errorf("failed to merge policy requirements: %w", err)
	}

	files := map[string]string{
		BaseConstraintsFile:          baseConstraints(baseRequirements),
		PolicyRequirementsFile:       policyRequirements,
		HashedPolicyRequirementsFile: hashedPolicyRequirements(policies),
	}
	for name, content := range files {
		path := filepath.Join(pythonOutputDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", name, err)
		}
		slog.Debug("Generated Python requirement file", "path", path, "content", content)
	}
	return nil
}

// baseConstraints turns a hash-pinned requirements lock into a pip constraints
// file holding just the "name==version[; marker]" lines, so a later install can
// add packages without changing any locked version.
func baseConstraints(lock string) string {
	var out []string
	for _, line := range strings.Split(lock, "\n") {
		// Requirement lines start in column 0; hash continuations and pip-compile
		// "# via" comments are indented.
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '#' || line[0] == '-' {
			continue
		}
		line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "\\"))
		if strings.Contains(line, "==") {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// hashedPolicyRequirements renders the indexed policy packages that carry a
// checksum in the build file in pip's hash-checking form.
func hashedPolicyRequirements(policies []*types.DiscoveredPolicy) string {
	var indexURLs, lines []string
	seenIndex := map[string]bool{}
	seen := map[string]bool{}
	for _, p := range policies {
		if !p.IsPipPackage || p.PipChecksum == "" {
			continue
		}
		spec, indexURL := splitIndexedPipSpec(p)
		line := discovery.HashedPipRequirement(spec, p.PipChecksum)
		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
		if indexURL != "" && !seenIndex[indexURL] {
			seenIndex[indexURL] = true
			indexURLs = append(indexURLs, "--extra-index-url "+indexURL)
		}
	}
	all := append(indexURLs, lines...)
	if len(all) == 0 {
		return ""
	}
	return strings.Join(all, "\n") + "\n"
}

// splitIndexedPipSpec returns the "name==version" part of an indexed pip
// policy spec and its index URL, if any.
func splitIndexedPipSpec(p *types.DiscoveredPolicy) (spec, indexURL string) {
	spec = p.PipSpec
	if p.PipIndexURL != "" {
		spec = strings.TrimSuffix(spec, "@"+p.PipIndexURL)
	}
	return spec, p.PipIndexURL
}

// mergeRequirements merges the requirements of all Python policies.
// For local (filePath) policies, it reads requirements.txt from the source directory.
// For pip policies, it adds the pip spec directly (pip handles transitive deps).
// The executor's base requirements are installed separately (see BaseRequirementsFile).
func mergeRequirements(policies []*types.DiscoveredPolicy) (string, error) {
	var allRequirements []string
	seen := make(map[string]bool)
	var extraIndexURLs []string
	seenIndexURLs := make(map[string]bool)

	for _, p := range policies {
		if p.IsPipPackage {
			// For pip packages, add the pip spec directly.
			// pip will resolve all transitive dependencies from the package metadata.
			spec := p.PipSpec
			if p.PipIndexURL != "" {
				// The index is passed with --extra-index-url below; keep the
				// requirement itself in plain "name==version" form.
				spec, _ = splitIndexedPipSpec(p)
			}
			if !seen[spec] {
				seen[spec] = true
				allRequirements = append(allRequirements, spec)
			}
			if p.PipIndexURL != "" && !seenIndexURLs[p.PipIndexURL] {
				seenIndexURLs[p.PipIndexURL] = true
				extraIndexURLs = append(extraIndexURLs, p.PipIndexURL)
			}
			continue
		}

		// For local policies, read requirements.txt from the source directory.
		// For src-layout policies, PythonSourceDir points inside src/<pkg>/ so
		// we also check the policy root (p.Path) where requirements.txt lives.
		reqPath := filepath.Join(p.PythonSourceDir, "requirements.txt")
		data, err := os.ReadFile(reqPath)
		if err != nil && os.IsNotExist(err) && p.Path != p.PythonSourceDir {
			reqPath = filepath.Join(p.Path, "requirements.txt")
			data, err = os.ReadFile(reqPath)
		}
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("failed to read requirements.txt for %s: %w", p.Name, err)
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if !seen[line] {
				seen[line] = true
				allRequirements = append(allRequirements, line)
			}
		}
	}

	// Prepend any extra index URLs so pip can find packages from private registries
	var result []string
	for _, url := range extraIndexURLs {
		result = append(result, fmt.Sprintf("--extra-index-url %s", url))
	}
	result = append(result, allRequirements...)
	if len(result) == 0 {
		return "", nil
	}

	return strings.Join(result, "\n") + "\n", nil
}

// localPythonModuleName is the module a local (filePath) Python policy is copied to and registered
// under. It carries the major version so two versions of one policy get separate modules instead of
// sharing one directory and one implementation.
func localPythonModuleName(p *types.DiscoveredPolicy) string {
	return fmt.Sprintf("%s_%s", sanitizeModuleName(p.Name), strings.Split(p.Version, ".")[0])
}

// sanitizeModuleName converts a policy name to a valid Python module name
func sanitizeModuleName(name string) string {
	// Replace hyphens with underscores
	return strings.ReplaceAll(name, "-", "_")
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Skip common virtual environment and cache directories
		if info.IsDir() && path != src {
			name := info.Name()
			if name == ".venv" || name == "venv" || name == "env" || name == "__pycache__" || name == ".git" {
				return filepath.SkipDir
			}
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			if info.Name() == ".venv" || info.Name() == "__pycache__" || info.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(dstPath, info.Mode())
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}
