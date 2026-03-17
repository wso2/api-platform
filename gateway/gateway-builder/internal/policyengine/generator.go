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

package policyengine

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
	if err := os.WriteFile(registryPath, []byte(registryCode), 0644); err != nil {
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
	if err := os.WriteFile(buildInfoPath, []byte(buildInfoCode), 0644); err != nil {
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

	return nil
}

// GeneratePythonArtifacts generates Python-specific build artifacts.
// This includes:
//  1. Copy each local policy's Python source into output/python-executor/policies/<name>/
//     (pip policies are skipped — they are installed by pip in the python-deps Docker stage)
//  2. Generate python_policy_registry.py
//  3. Merge requirements.txt files
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
			policyDestDir := filepath.Join(policiesDir, sanitizeModuleName(p.Name))
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

	// 3. Merge requirements.txt files (including base requirements)
	baseReqPath := filepath.Join(pythonOutputDir, "requirements.txt")
	baseRequirements := ""
	data, err := os.ReadFile(baseReqPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read base requirements.txt: %w", err)
		}
	} else {
		baseRequirements = string(data)
	}

	requirements, err := mergeRequirements(pythonPolicies, baseRequirements)
	if err != nil {
		return fmt.Errorf("failed to merge requirements.txt: %w", err)
	}

	reqPath := filepath.Join(pythonOutputDir, "requirements.txt")
	if err := os.WriteFile(reqPath, []byte(requirements), 0644); err != nil {
		return fmt.Errorf("failed to write requirements.txt: %w", err)
	}

	slog.Debug("Generated requirements.txt", "path", reqPath, "content", requirements)

	slog.Info("Generated Python artifacts successfully",
		"policies", len(pythonPolicies),
		"phase", "generation")

	return nil
}

// generatePythonRegistry generates the python_policy_registry.py file.
// For local (filePath) policies, the module path is "policies.<sanitized_name>.policy".
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
			moduleName := sanitizeModuleName(p.Name)
			modulePath = fmt.Sprintf("policies.%s.policy", moduleName)
		}

		sb.WriteString(fmt.Sprintf("    \"%s\": \"%s\",\n", key, modulePath))
	}

	sb.WriteString("}\n")

	return sb.String()
}

// mergeRequirements merges all policy requirements with base requirements.
// For local (filePath) policies, it reads requirements.txt from the source directory.
// For pip policies, it adds the pip spec directly (pip handles transitive deps).
func mergeRequirements(policies []*types.DiscoveredPolicy, baseRequirements string) (string, error) {
	var allRequirements []string
	seen := make(map[string]bool)
	var extraIndexURLs []string
	seenIndexURLs := make(map[string]bool)

	// First add base requirements
	if baseRequirements != "" {
		lines := strings.Split(baseRequirements, "\n")
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

	for _, p := range policies {
		if p.IsPipPackage {
			// For pip packages, add the pip spec directly.
			// pip will resolve all transitive dependencies from the package metadata.
			if !seen[p.PipSpec] {
				seen[p.PipSpec] = true
				allRequirements = append(allRequirements, p.PipSpec)
			}
			if p.PipIndexURL != "" && !seenIndexURLs[p.PipIndexURL] {
				seenIndexURLs[p.PipIndexURL] = true
				extraIndexURLs = append(extraIndexURLs, p.PipIndexURL)
			}
			continue
		}

		// For local policies, read requirements.txt from the source directory
		reqPath := filepath.Join(p.PythonSourceDir, "requirements.txt")
		data, err := os.ReadFile(reqPath)
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

	return strings.Join(result, "\n"), nil
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

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}
