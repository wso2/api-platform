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

package testutils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// CreatePolicyDir creates a policy directory structure at baseDir/policies/name/version.
// Returns the full path to the policy directory.
func CreatePolicyDir(t *testing.T, baseDir, name, version string) string {
	t.Helper()
	policyPath := filepath.Join(baseDir, "policies", name, version)
	err := os.MkdirAll(policyPath, 0755)
	require.NoError(t, err, "failed to create policy directory %s", policyPath)
	return policyPath
}

// CreatePolicyWithDefinition creates a complete policy directory with:
// - policy-definition.yaml
// - go.mod
// - minimal Go source file
// Returns the full path to the policy directory.
func CreatePolicyWithDefinition(t *testing.T, baseDir, name, version string) string {
	t.Helper()
	policyPath := CreatePolicyDir(t, baseDir, name, version)

	// Create policy-definition.yaml
	defContent := fmt.Sprintf(`name: %s
version: %s
displayName: %s Policy
description: Test policy for %s
`, name, version, name, name)
	WriteFile(t, filepath.Join(policyPath, "policy-definition.yaml"), defContent)

	// Create go.mod
	modulePath := fmt.Sprintf("github.com/example/policies/%s", name)
	WriteGoMod(t, policyPath, modulePath)

	// Create minimal source file
	CreateMinimalGoSource(t, policyPath, name)

	return policyPath
}

// CreatePolicyGoMod creates a go.mod file in the policy directory with the specified module path.
func CreatePolicyGoMod(t *testing.T, policyDir, modulePath string) {
	t.Helper()
	WriteGoMod(t, policyDir, modulePath)
}

// CreatePolicyDefinitionYAML creates a policy-definition.yaml file in the specified directory.
func CreatePolicyDefinitionYAML(t *testing.T, dir, name, version string) {
	t.Helper()
	content := fmt.Sprintf(`name: %s
version: %s
displayName: %s Policy
description: Test policy for %s
`, name, version, name, name)
	WriteFile(t, filepath.Join(dir, "policy-definition.yaml"), content)
}

// CreatePolicyDefinitionYAMLWithContent creates a policy-definition.yaml with custom content.
func CreatePolicyDefinitionYAMLWithContent(t *testing.T, dir, content string) {
	t.Helper()
	WriteFile(t, filepath.Join(dir, "policy-definition.yaml"), content)
}

// NewDiscoveredPolicy creates a new DiscoveredPolicy with the given parameters.
// This is a builder helper that doesn't require *testing.T since it doesn't do I/O.
func NewDiscoveredPolicy(name, version, path, modulePath string, isLocal bool) *types.DiscoveredPolicy {
	return &types.DiscoveredPolicy{
		Name:            name,
		Version:         version,
		Path:            path,
		GoModulePath:    modulePath,
		IsFilePathEntry: isLocal,
	}
}

// NewLocalDiscoveredPolicy creates a DiscoveredPolicy for a local (filePath) policy.
func NewLocalDiscoveredPolicy(name, version, path, modulePath string) *types.DiscoveredPolicy {
	return NewDiscoveredPolicy(name, version, path, modulePath, true)
}

// NewRemoteDiscoveredPolicy creates a DiscoveredPolicy for a remote (gomodule) policy.
func NewRemoteDiscoveredPolicy(name, version, modulePath, moduleVersion string) *types.DiscoveredPolicy {
	return &types.DiscoveredPolicy{
		Name:            name,
		Version:         version,
		Path:            "",
		GoModulePath:    modulePath,
		GoModuleVersion: moduleVersion,
		IsFilePathEntry: false,
	}
}

// CreatePolicySourceFile creates a Go source file in the policy directory.
// The packageName is sanitized to be a valid Go identifier.
func CreatePolicySourceFile(t *testing.T, policyDir, packageName string) {
	t.Helper()
	safeName := SanitizePackageName(packageName)
	content := fmt.Sprintf(`package %s

import (
	"context"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

type TestPolicy struct{}

func (p *TestPolicy) OnRequest(ctx context.Context, reqCtx *policy.RequestContext, params policy.PolicyParameters) policy.RequestAction {
	return &policy.UpstreamRequestModifications{}
}

func (p *TestPolicy) OnResponse(ctx context.Context, respCtx *policy.ResponseContext, params policy.PolicyParameters) policy.ResponseAction {
	return &policy.DownstreamResponseModifications{}
}

func (p *TestPolicy) Mode() policy.PolicyMode {
	return policy.PolicyMode{}
}

func Factory(metadata policy.PolicyMetadata, initParams map[string]interface{}) (policy.Policy, map[string]interface{}, error) {
	return &TestPolicy{}, initParams, nil
}
`, safeName)
	CreateSourceFile(t, policyDir, safeName+".go", content)
}

// CreateValidPolicyDir creates a complete valid policy directory at baseDir/name with:
// - policy.yaml (with name and version)
// - go.mod
// - policy.go (with minimal valid methods)
// Returns the full path to the policy directory.
// This matches the pattern used in validation tests.
// The policy name is sanitized to create valid Go package names.
func CreateValidPolicyDir(t *testing.T, baseDir, name, version string) string {
	t.Helper()
	policyDir := filepath.Join(baseDir, name)
	err := os.MkdirAll(policyDir, 0755)
	require.NoError(t, err, "failed to create policy directory %s", policyDir)

	// Create policy.yaml
	yamlContent := fmt.Sprintf("name: %s\nversion: %s", name, version)
	WriteFile(t, filepath.Join(policyDir, "policy.yaml"), yamlContent)

	// Create go.mod
	goModContent := fmt.Sprintf("module github.com/test/%s", name)
	WriteFile(t, filepath.Join(policyDir, "go.mod"), goModContent)

	// Sanitize name for valid Go package name
	pkgName := SanitizePackageName(name)

	// Create valid policy.go with all required methods
	goContent := fmt.Sprintf(`package %s

type Policy struct{}

func GetPolicy() *Policy { return &Policy{} }
func (p *Policy) Mode() int { return 0 }
func (p *Policy) OnRequest() {}
func (p *Policy) OnResponse() {}
`, pkgName)
	WriteFile(t, filepath.Join(policyDir, "policy.go"), goContent)

	return policyDir
}

// CreatePolicyYAML creates a policy.yaml file in the specified directory.
// This is simpler than policy-definition.yaml and used in docker tests.
func CreatePolicyYAML(t *testing.T, dir, name, version string) {
	t.Helper()
	content := fmt.Sprintf("name: %s\nversion: %s", name, version)
	WriteFile(t, filepath.Join(dir, "policy.yaml"), content)
}
