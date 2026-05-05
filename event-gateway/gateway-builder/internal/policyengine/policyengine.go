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

// Package policyengine provides code-generation and go.mod management for
// embedding policies into the event-gateway binary.
package policyengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/wso2/api-platform/event-gateway/gateway-builder/pkg/types"
	"github.com/wso2/api-platform/event-gateway/gateway-builder/templates"
	"golang.org/x/mod/modfile"
)

// PolicyImport holds the import information for a single policy.
type PolicyImport struct {
	Name             string
	Version          string
	ImportPath       string
	ImportAlias      string
	SystemParameters map[string]interface{}
}

// GeneratePluginRegistry generates the plugin_registry.go source for the event-gateway.
// The generated file defines registerPolicies(eng *engine.Engine) which registers all
// discovered policies directly into the embedded engine instance.
func GeneratePluginRegistry(policies []*types.DiscoveredPolicy) (string, error) {
	var goPolicies []PolicyImport
	for _, p := range policies {
		if p.Runtime != "go" {
			continue
		}
		goPolicies = append(goPolicies, PolicyImport{
			Name:             p.Name,
			Version:          p.Version,
			ImportPath:       p.GoModulePath,
			ImportAlias:      importAlias(p.Name, p.Version),
			SystemParameters: p.SystemParameters,
		})
	}

	tmpl, err := template.New("plugin_registry").Parse(templates.PluginRegistryTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse plugin_registry template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, struct {
		GoPolicies    []PolicyImport
		HasGoPolicies bool
	}{
		GoPolicies:    goPolicies,
		HasGoPolicies: len(goPolicies) > 0,
	}); err != nil {
		return "", fmt.Errorf("failed to execute plugin_registry template: %w", err)
	}

	return buf.String(), nil
}

// GenerateCode generates the plugin_registry.go and updates go.mod in the event-gateway
// source directory (srcDir), writing the generated file to cmd/event-gateway/.
func GenerateCode(srcDir string, policies []*types.DiscoveredPolicy) error {
	mainPkgDir := filepath.Join(srcDir, "cmd", "event-gateway")
	if _, err := os.Stat(mainPkgDir); err != nil {
		return fmt.Errorf("cmd/event-gateway directory not found in %s: %w", srcDir, err)
	}

	if _, err := os.Stat(filepath.Join(srcDir, "go.mod")); err != nil {
		return fmt.Errorf("go.mod not found in %s: %w", srcDir, err)
	}

	// Generate plugin_registry.go
	code, err := GeneratePluginRegistry(policies)
	if err != nil {
		return fmt.Errorf("failed to generate plugin_registry.go: %w", err)
	}

	registryPath := filepath.Join(mainPkgDir, "plugin_registry.go")
	if err := os.WriteFile(registryPath, []byte(code), 0600); err != nil {
		return fmt.Errorf("failed to write plugin_registry.go: %w", err)
	}
	slog.Info("Generated plugin_registry.go", "path", registryPath)

	// Update go.mod with policy dependencies
	var goPolicies []*types.DiscoveredPolicy
	for _, p := range policies {
		if p.Runtime == "go" {
			goPolicies = append(goPolicies, p)
		}
	}
	if len(goPolicies) > 0 {
		if err := UpdateGoMod(srcDir, goPolicies); err != nil {
			return fmt.Errorf("failed to update go.mod: %w", err)
		}
	}

	return nil
}

// UpdateGoMod updates the go.mod in srcDir:
//   - gomodule entries: runs 'go get' to pin the module at its resolved version
//   - filePath entries: adds a replace directive pointing to the local path
func UpdateGoMod(srcDir string, policies []*types.DiscoveredPolicy) error {
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		return fmt.Errorf("failed to resolve srcDir: %w", err)
	}

	goModPath := filepath.Join(absSrcDir, "go.mod")

	// 'go get' for remote policies
	for _, p := range policies {
		if p.IsFilePathEntry {
			continue
		}
		target := fmt.Sprintf("%s@%s", p.GoModulePath, p.GoModuleVersion)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, "go", "get", target)
		cmd.Dir = absSrcDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			cancel()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("go get timed out for %s: %w", target, err)
			}
			return fmt.Errorf("go get failed for %s: %w; stderr: %s", target, err, stderr.String())
		}
		cancel()
	}

	// Add replace directives for local policies
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("failed to read go.mod: %w", err)
	}

	mf, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}

	for _, p := range policies {
		if !p.IsFilePathEntry {
			continue
		}
		rel, err := filepath.Rel(absSrcDir, p.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", p.Name, err)
		}
		if err := mf.AddReplace(p.GoModulePath, "", rel, ""); err != nil {
			if !strings.Contains(err.Error(), "already") {
				return fmt.Errorf("failed to add replace directive for %s: %w", p.Name, err)
			}
		}
	}

	formatted, err := mf.Format()
	if err != nil {
		return fmt.Errorf("failed to format go.mod: %w", err)
	}
	if err := os.WriteFile(goModPath, formatted, 0600); err != nil {
		return fmt.Errorf("failed to write go.mod: %w", err)
	}

	return nil
}

// importAlias creates a valid Go identifier for the given policy name and version.
func importAlias(name, version string) string {
	return fmt.Sprintf("%s_%s", sanitize(name), sanitize(version))
}

func sanitize(s string) string {
	if strings.HasPrefix(s, "v") {
		s = "_" + s[1:]
	}
	var b strings.Builder
	for i, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_':
			b.WriteRune(r)
		case i > 0 && r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == ' ':
			b.WriteRune('_')
		}
	}
	return b.String()
}
