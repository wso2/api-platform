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
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"text/template"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
	"github.com/wso2/api-platform/gateway/gateway-builder/templates"
)

// PolicyImport represents a policy import for code generation
type PolicyImport struct {
	Name             string
	Version          string
	ImportPath       string
	ImportAlias      string
	SystemParameters map[string]interface{} // from policy-definition.yaml
}

// GeneratePluginRegistry generates the plugin_registry.go file
func GeneratePluginRegistry(policies []*types.DiscoveredPolicy, srcDir string) (string, error) {
	slog.Debug("Generating plugin registry",
		"policyCount", len(policies),
		"phase", "generation")

	// Create import list
	imports := make([]PolicyImport, 0, len(policies))
	for _, policy := range policies {
		importPath := generateImportPath(policy)
		importAlias := generateImportAlias(policy.Name, policy.Version)

		slog.Debug("Creating policy import",
			"name", policy.Name,
			"version", policy.Version,
			"importPath", importPath,
			"alias", importAlias,
			"phase", "generation")

		imports = append(imports, PolicyImport{
			Name:             policy.Name,
			Version:          policy.Version,
			ImportPath:       importPath,
			ImportAlias:      importAlias,
			SystemParameters: policy.SystemParameters,
		})
	}

	// Parse embedded template
	tmpl, err := template.New("plugin_registry").Parse(templates.PluginRegistryTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	slog.Debug("Executing plugin registry template",
		"importCount", len(imports),
		"phase", "generation")

	// Execute template
	var buf bytes.Buffer
	data := struct {
		Policies []PolicyImport
	}{
		Policies: imports,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// generateImportPath returns the Go import path for a policy.
// For gomodule entries this is the real published module path;
// for filePath entries it is the module path declared in the policy's go.mod.
func generateImportPath(policy *types.DiscoveredPolicy) string {
	return policy.GoModulePath
}

// generateImportAlias creates a valid Go identifier for import alias
func generateImportAlias(name, version string) string {
	// Sanitize name and version to create a valid Go identifier
	alias := sanitizeIdentifier(name)
	versionSuffix := sanitizeIdentifier(version)

	// Combine name and version to ensure uniqueness
	return fmt.Sprintf("%s_%s", alias, versionSuffix)
}

// sanitizeIdentifier converts a string to a valid Go identifier
func sanitizeIdentifier(s string) string {
	// Remove 'v' prefix from versions
	s = strings.TrimPrefix(s, "v")

	// Replace invalid characters
	var result strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r == '_') {
			result.WriteRune(r)
		} else if i > 0 && r >= '0' && r <= '9' {
			result.WriteRune(r)
		} else if r == '.' || r == '-' || r == ' ' {
			result.WriteRune('_')
		}
	}

	return result.String()
}
