/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

// Package discovery discovers and resolves policies from a build.yaml file.
// Only Go policies (gomodule and filePath) are supported; Python policies are
// not supported for the event-gateway.
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
	"strings"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-builder/pkg/types"
	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

const (
	policyDefinitionFile       = "policy-definition.yaml"
	systemParamConfigRefKey    = "__wso2_internal_ref"
	systemParamDefaultValueKey = "__wso2_internal_default"
	systemParamRequiredKey     = "__wso2_internal_required"
)

// policyDefinitionYAML is a minimal struct for parsing policy-definition.yaml.
type policyDefinitionYAML struct {
	Name             string                 `yaml:"name"`
	Version          string                 `yaml:"version"`
	SystemParameters map[string]interface{} `yaml:"systemParameters"`
}

// parsePolicyDefinitionYAML reads and parses a policy-definition.yaml file.
func parsePolicyDefinitionYAML(path string) (*policyDefinitionYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", policyDefinitionFile, err)
	}
	var def policyDefinitionYAML
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", path, err)
	}
	return &def, nil
}

// extractDefaultValues extracts system parameter default values from the
// systemParameters JSON schema defined in a policy-definition.yaml file.
// It mirrors the logic in the gateway builder's ExtractDefaultValues.
func extractDefaultValues(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return map[string]interface{}{}
	}
	return extractDefaultsFromSchema(schema)
}

func extractDefaultsFromSchema(schema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return result
	}

	requiredProps := extractRequiredProperties(schema)

	for propName, propDef := range properties {
		propDefMap, ok := propDef.(map[string]interface{})
		if !ok {
			continue
		}
		if val, has := extractPropertyValue(propDefMap, requiredProps[propName]); has {
			result[propName] = val
			continue
		}
		// Recurse into nested object schemas.
		nested := extractDefaultsFromSchema(propDefMap)
		if len(nested) > 0 {
			result[propName] = nested
		}
	}
	return result
}

func extractRequiredProperties(schema map[string]interface{}) map[string]bool {
	required := map[string]bool{}
	raw, ok := schema["required"]
	if !ok {
		return required
	}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if name, ok := item.(string); ok && name != "" {
				required[name] = true
			}
		}
	case []string:
		for _, name := range v {
			if name != "" {
				required[name] = true
			}
		}
	}
	return required
}

func extractPropertyValue(propDef map[string]interface{}, required bool) (interface{}, bool) {
	wso2Default, hasWso2Default := propDef["wso2/defaultValue"]
	defaultVal, hasDefault := propDef["default"]
	switch {
	case hasWso2Default && hasDefault:
		return map[string]interface{}{
			systemParamConfigRefKey:    wso2Default,
			systemParamDefaultValueKey: defaultVal,
			systemParamRequiredKey:     required,
		}, true
	case hasWso2Default:
		return map[string]interface{}{
			systemParamConfigRefKey: wso2Default,
			systemParamRequiredKey:  required,
		}, true
	case hasDefault:
		return defaultVal, true
	default:
		return nil, false
	}
}

const SupportedBuildFileVersion = "v1"

// DiscoverPoliciesFromBuildFile loads the build.yaml file and resolves all
// listed Go policies into DiscoveredPolicy structs ready for code generation.
func DiscoverPoliciesFromBuildFile(buildFilePath string) ([]*types.DiscoveredPolicy, error) {
	data, err := os.ReadFile(buildFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read build file %s: %w", buildFilePath, err)
	}

	var bf types.BuildFile
	if err := yaml.Unmarshal(data, &bf); err != nil {
		return nil, fmt.Errorf("failed to parse build file: %w", err)
	}

	if bf.Version != SupportedBuildFileVersion {
		return nil, fmt.Errorf("unsupported build file version %q (expected %q)", bf.Version, SupportedBuildFileVersion)
	}

	var policies []*types.DiscoveredPolicy
	for _, entry := range bf.Policies {
		switch {
		case entry.FilePath != "":
			p, err := discoverFilePathPolicy(entry, buildFilePath)
			if err != nil {
				return nil, fmt.Errorf("policy %q: %w", entry.Name, err)
			}
			policies = append(policies, p)

		case entry.Gomodule != "":
			p, err := discoverGoModulePolicy(entry)
			if err != nil {
				return nil, fmt.Errorf("policy %q: %w", entry.Name, err)
			}
			policies = append(policies, p)

		case entry.PipPackage != "":
			slog.Warn("Python (pipPackage) policies are not supported by the event-gateway builder; skipping",
				"policy", entry.Name)

		default:
			return nil, fmt.Errorf("policy %q: one of filePath, gomodule, or pipPackage must be set", entry.Name)
		}
	}

	return policies, nil
}

// discoverFilePathPolicy resolves a local file-path policy entry.
func discoverFilePathPolicy(entry types.BuildEntry, buildFilePath string) (*types.DiscoveredPolicy, error) {
	buildDir := filepath.Dir(buildFilePath)
	policyDir := filepath.Join(buildDir, entry.FilePath)

	absDir, err := filepath.Abs(policyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	goModPath := filepath.Join(absDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod in %s: %w", absDir, err)
	}

	mf, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod in %s: %w", absDir, err)
	}

	version := ""
	if mf.Module.Mod.Version != "" {
		version = mf.Module.Mod.Version
	}

	// Parse policy-definition.yaml to extract system parameters.
	policyDefPath := filepath.Join(absDir, policyDefinitionFile)
	definition, err := parsePolicyDefinitionYAML(policyDefPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", policyDefinitionFile, err)
	}

	// Prefer the version from policy-definition.yaml when available.
	if definition.Version != "" {
		version = definition.Version
	}

	slog.Info("Discovered local policy",
		"name", entry.Name,
		"path", absDir,
		"modulePath", mf.Module.Mod.Path)

	return &types.DiscoveredPolicy{
		Name:             entry.Name,
		Version:          version,
		Path:             absDir,
		GoModulePath:     mf.Module.Mod.Path,
		IsFilePathEntry:  true,
		Runtime:          "go",
		SystemParameters: extractDefaultValues(definition.SystemParameters),
	}, nil
}

// discoverGoModulePolicy resolves a remote gomodule policy entry.
func discoverGoModulePolicy(entry types.BuildEntry) (*types.DiscoveredPolicy, error) {
	// Parse "module@version" format (e.g. "github.com/foo/bar@v1")
	spec := entry.Gomodule
	modulePath, versionQuery, ok := strings.Cut(spec, "@")
	if !ok {
		return nil, fmt.Errorf("gomodule must be in 'module@version' format, got: %q", spec)
	}

	// Use `go mod download -json` to resolve the canonical version and local directory.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	target := fmt.Sprintf("%s@%s", modulePath, versionQuery)
	cmd := exec.CommandContext(ctx, "go", "mod", "download", "-json", target)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go mod download failed for %s: %w; stderr: %s", target, err, stderr.String())
	}

	var info struct {
		Path    string `json:"Path"`
		Version string `json:"Version"`
		Dir     string `json:"Dir"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		return nil, fmt.Errorf("failed to parse go mod download output: %w", err)
	}

	if info.Dir == "" {
		return nil, fmt.Errorf("go mod download did not return a Dir for %s", target)
	}

	// Parse policy-definition.yaml from the downloaded module directory.
	policyDefPath := filepath.Join(info.Dir, policyDefinitionFile)
	definition, err := parsePolicyDefinitionYAML(policyDefPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s for %s: %w", policyDefinitionFile, entry.Name, err)
	}

	slog.Info("Discovered remote policy",
		"name", entry.Name,
		"modulePath", info.Path,
		"version", info.Version)

	return &types.DiscoveredPolicy{
		Name:             entry.Name,
		Version:          info.Version,
		GoModulePath:     info.Path,
		GoModuleVersion:  info.Version,
		IsFilePathEntry:  false,
		Runtime:          "go",
		SystemParameters: extractDefaultValues(definition.SystemParameters),
	}, nil
}
