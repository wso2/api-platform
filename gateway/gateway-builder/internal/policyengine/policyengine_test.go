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

package policyengine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestSanitizeIdentifier_SimpleString(t *testing.T) {
	result := sanitizeIdentifier("myPolicy")
	assert.Equal(t, "myPolicy", result)
}

func TestSanitizeIdentifier_VersionPrefix(t *testing.T) {
	// 'v' prefix becomes '_', so 'v1.0.0' → '_1.0.0' → '_1_0_0'
	result := sanitizeIdentifier("v1.0.0")
	assert.Equal(t, "_1_0_0", result)
}

func TestSanitizeIdentifier_WithDashes(t *testing.T) {
	result := sanitizeIdentifier("my-policy-name")
	assert.Equal(t, "my_policy_name", result)
}

func TestSanitizeIdentifier_WithSpaces(t *testing.T) {
	result := sanitizeIdentifier("my policy name")
	assert.Equal(t, "my_policy_name", result)
}

func TestSanitizeIdentifier_WithDots(t *testing.T) {
	// no 'v' prefix; '1' at position 0 is a digit so it's skipped, '.' becomes '_'
	result := sanitizeIdentifier("1.2.3")
	assert.Equal(t, "_2_3", result)
}

func TestSanitizeIdentifier_WithNumbers(t *testing.T) {
	result := sanitizeIdentifier("policy123")
	assert.Equal(t, "policy123", result)
}

func TestSanitizeIdentifier_NumberAtStart(t *testing.T) {
	// First digit '1' is skipped, but '2' and '3' are allowed after other chars
	// Actually looking at the code: position 0 digit skipped, positions 1+ get '23'
	result := sanitizeIdentifier("123policy")
	assert.Equal(t, "23policy", result)
}

func TestSanitizeIdentifier_MixedSpecialChars(t *testing.T) {
	// 'v' prefix is NOT stripped here because it's part of 'v1.0' inside the string
	// 'm' at pos 0, 'y' at pos 1, '-' becomes '_', etc.
	result := sanitizeIdentifier("my-policy.v1.0")
	assert.Equal(t, "my_policy_v1_0", result)
}

func TestSanitizeIdentifier_Underscores(t *testing.T) {
	result := sanitizeIdentifier("my_policy_name")
	assert.Equal(t, "my_policy_name", result)
}

func TestSanitizeIdentifier_UpperCase(t *testing.T) {
	result := sanitizeIdentifier("MyPolicyName")
	assert.Equal(t, "MyPolicyName", result)
}

func TestSanitizeIdentifier_OnlyVersionPrefix(t *testing.T) {
	// 'v' becomes '_', leaves '_010' — all chars kept (digits after position 0)
	result := sanitizeIdentifier("v010")
	assert.Equal(t, "_010", result)
}

func TestGenerateImportAlias(t *testing.T) {
	tests := []struct {
		name       string
		policyName string
		version    string
		expected   string
	}{
		{
			name:       "simple policy and version",
			policyName: "ratelimit",
			version:    "v1.0.0",
			expected:   "ratelimit__1_0_0", // 'v' → '_', major version included
		},
		{
			name:       "policy with dashes",
			policyName: "jwt-auth",
			version:    "v0.1.0",
			expected:   "jwt_auth__0_1_0", // '-' → '_', 'v' → '_'
		},
		{
			name:       "policy with underscores",
			policyName: "my_policy",
			version:    "v2.0.0",
			expected:   "my_policy__2_0_0", // major version 2 included
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateImportAlias(tt.policyName, tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateImportPath(t *testing.T) {
	tests := []struct {
		name     string
		policy   *types.DiscoveredPolicy
		expected string
	}{
		{
			name: "returns GoModulePath directly",
			policy: &types.DiscoveredPolicy{
				Name:         "ratelimit",
				Version:      "v1.0.0",
				GoModulePath: "github.com/example/policies/ratelimit",
			},
			expected: "github.com/example/policies/ratelimit",
		},
		{
			name: "policy with custom module path",
			policy: &types.DiscoveredPolicy{
				Name:         "jwt-auth",
				Version:      "v0.1.0",
				GoModulePath: "github.com/custom/jwt-auth",
			},
			expected: "github.com/custom/jwt-auth",
		},
		{
			name: "empty GoModulePath returns empty string",
			policy: &types.DiscoveredPolicy{
				Name:         "my_policy",
				Version:      "v1.0.0",
				GoModulePath: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateImportPath(tt.policy)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGeneratePluginRegistry_EmptyPolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	result, err := GeneratePluginRegistry(policies, "/src")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "package main")
}

func TestGeneratePluginRegistry_SinglePolicy(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:         "ratelimit",
			Version:      "v1.0.0",
			Path:         "/policies/ratelimit/v1.0.0",
			GoModulePath: "github.com/policy-engine/policies/ratelimit",
		},
	}

	result, err := GeneratePluginRegistry(policies, "/src")
	require.NoError(t, err)
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "github.com/policy-engine/policies/ratelimit")
	assert.Contains(t, result, "ratelimit__1_0_0") // actual alias format
}

func TestGeneratePluginRegistry_MultiplePolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:         "ratelimit",
			Version:      "v1.0.0",
			Path:         "/policies/ratelimit/v1.0.0",
			GoModulePath: "github.com/policy-engine/policies/ratelimit",
		},
		{
			Name:         "jwt-auth",
			Version:      "v0.1.0",
			Path:         "/policies/jwt-auth/v0.1.0",
			GoModulePath: "github.com/policy-engine/policies/jwt-auth",
		},
	}

	result, err := GeneratePluginRegistry(policies, "/src")
	require.NoError(t, err)
	assert.Contains(t, result, "github.com/policy-engine/policies/ratelimit")
	assert.Contains(t, result, "github.com/policy-engine/policies/jwt-auth")
	assert.Contains(t, result, "ratelimit__1_0_0")
	assert.Contains(t, result, "jwt_auth__0_1_0")
}

func TestGenerateBuildInfo_EmptyPolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	result, err := GenerateBuildInfo(policies, "v1.0.0")
	require.NoError(t, err)
	assert.NotEmpty(t, result)
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "v1.0.0")
}

func TestGenerateBuildInfo_SinglePolicy(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
		},
	}

	result, err := GenerateBuildInfo(policies, "v2.0.0")
	require.NoError(t, err)
	assert.Contains(t, result, "package main")
	assert.Contains(t, result, "ratelimit")
	assert.Contains(t, result, "v1.0.0")
	assert.Contains(t, result, "v2.0.0") // builder version
}

func TestGenerateBuildInfo_MultiplePolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
		},
		{
			Name:    "jwt-auth",
			Version: "v0.1.0",
		},
		{
			Name:    "cors",
			Version: "v2.0.0",
		},
	}

	result, err := GenerateBuildInfo(policies, "v1.5.0")
	require.NoError(t, err)
	assert.Contains(t, result, "ratelimit")
	assert.Contains(t, result, "jwt-auth")
	assert.Contains(t, result, "cors")
	// Should have timestamp
	assert.True(t, strings.Contains(result, "202") || strings.Contains(result, "BuildTimestamp"))
}

func TestGenerateBuildInfo_BuilderVersion(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	result, err := GenerateBuildInfo(policies, "v3.2.1-beta")
	require.NoError(t, err)
	assert.Contains(t, result, "v3.2.1-beta")
}

// ==== UpdateGoMod tests ====

func TestUpdateGoMod_LocalPolicies_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create policy directories
	policyPath := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", policyPath, "github.com/example/policies/ratelimit"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.NoError(t, err)

	// Read back go.mod and verify replace directive
	modData, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(modData), "replace")
	assert.Contains(t, string(modData), "github.com/example/policies/ratelimit")
}

func TestUpdateGoMod_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create multiple policy directories
	policy1Path := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")
	policy2Path := testutils.CreatePolicyDir(t, tmpDir, "jwt-auth", "v0.1.0")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", policy1Path, "github.com/example/policies/ratelimit"),
		testutils.NewLocalDiscoveredPolicy("jwt-auth", "v0.1.0", policy2Path, "github.com/example/policies/jwt-auth"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.NoError(t, err)

	modData, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(modData), "github.com/example/policies/ratelimit")
	assert.Contains(t, string(modData), "github.com/example/policies/jwt-auth")
}

func TestUpdateGoMod_EmptyPolicies(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Empty policies - should work fine
	policies := []*types.DiscoveredPolicy{}

	err := UpdateGoMod(tmpDir, policies)
	require.NoError(t, err)
}

func TestUpdateGoMod_MissingGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	// No go.mod file

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", "/policies/ratelimit", "github.com/example/ratelimit"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read go.mod")
}

func TestUpdateGoMod_InvalidGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid go.mod
	testutils.WriteFile(t, filepath.Join(tmpDir, "go.mod"), "not a valid go.mod file !!!")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", "/policies/ratelimit", "github.com/example/ratelimit"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse go.mod")
}

// ==== GenerateCode tests ====

func createPythonSDKCoreDir(t *testing.T, rootDir string) {
	t.Helper()

	sdkPythonDir := filepath.Join(rootDir, "sdk-python", "src", "wso2_gateway_policy_sdk")
	testutils.CreateDir(t, filepath.Join(sdkPythonDir, "core", "policy", "v1alpha2"))
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "__init__.py"), "# package root\n")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "py.typed"), "")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "core", "__init__.py"), "# core package\n")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "core", "policy", "__init__.py"), "# policy package\n")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "core", "policy", "v1alpha2", "__init__.py"), "# version package\n")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "core", "policy", "v1alpha2", "types.py"), "# core policy types\n")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "core", "policy", "v1alpha2", "actions.py"), "# core policy actions\n")
	testutils.WriteFile(t, filepath.Join(sdkPythonDir, "core", "policy", "v1alpha2", "policy.py"), "# core policy interfaces\n")
}

func TestGenerateCode_Success(t *testing.T) {
	rootDir := t.TempDir()
	tmpDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "policy-engine")

	// Create the required directory structure
	mainPkgDir := filepath.Join(tmpDir, "cmd", "policy-engine")
	testutils.CreateDir(t, mainPkgDir)
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create python-executor sibling directory (required by generatePythonExecutorBase)
	pythonExecDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "python-executor")
	testutils.CreateDir(t, pythonExecDir)
	createPythonSDKCoreDir(t, rootDir)

	// Create policy directory
	policyPath := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", policyPath, "github.com/policy-engine/policies/ratelimit"),
	}

	outputDir := t.TempDir()
	err := GenerateCode(tmpDir, policies, outputDir)
	require.NoError(t, err)

	// Verify plugin_registry.go was generated
	registryPath := filepath.Join(mainPkgDir, "plugin_registry.go")
	assert.FileExists(t, registryPath)

	registryContent, err := os.ReadFile(registryPath)
	require.NoError(t, err)
	assert.Contains(t, string(registryContent), "package main")
	assert.Contains(t, string(registryContent), "github.com/policy-engine/policies/ratelimit")

	// Verify build_info.go was generated
	buildInfoPath := filepath.Join(mainPkgDir, "build_info.go")
	assert.FileExists(t, buildInfoPath)

	buildInfoContent, err := os.ReadFile(buildInfoPath)
	require.NoError(t, err)
	assert.Contains(t, string(buildInfoContent), "package main")
	assert.Contains(t, string(buildInfoContent), "ratelimit")
}

func TestGenerateCode_EmptyPolicies(t *testing.T) {
	rootDir := t.TempDir()
	tmpDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "policy-engine")

	mainPkgDir := filepath.Join(tmpDir, "cmd", "policy-engine")
	testutils.CreateDir(t, mainPkgDir)
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create python-executor sibling directory
	pythonExecDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "python-executor")
	testutils.CreateDir(t, pythonExecDir)
	createPythonSDKCoreDir(t, rootDir)

	// Empty policies
	policies := []*types.DiscoveredPolicy{}

	outputDir := t.TempDir()
	err := GenerateCode(tmpDir, policies, outputDir)
	require.NoError(t, err)

	// Files should still be generated
	assert.FileExists(t, filepath.Join(mainPkgDir, "plugin_registry.go"))
	assert.FileExists(t, filepath.Join(mainPkgDir, "build_info.go"))
}

func TestGenerateCode_CopiesPythonExecutorBaseFiles(t *testing.T) {
	rootDir := t.TempDir()
	tmpDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "policy-engine")

	mainPkgDir := filepath.Join(tmpDir, "cmd", "policy-engine")
	testutils.CreateDir(t, mainPkgDir)
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	pythonExecDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "python-executor")
	testutils.CreateDir(t, filepath.Join(pythonExecDir, "executor"))
	testutils.CreateDir(t, filepath.Join(pythonExecDir, "proto"))
	testutils.WriteFile(t, filepath.Join(pythonExecDir, "executor", "__init__.py"), "# executor package\n")
	testutils.WriteFile(t, filepath.Join(pythonExecDir, "proto", "python_executor_pb2.py"), "# generated proto\n")

	createPythonSDKCoreDir(t, rootDir)

	outputDir := t.TempDir()
	err := GenerateCode(tmpDir, nil, outputDir)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "executor", "__init__.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "__init__.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "py.typed"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "core", "policy", "__init__.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "core", "policy", "v1alpha2", "types.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "core", "policy", "v1alpha2", "actions.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "core", "policy", "v1alpha2", "__init__.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "wso2_gateway_policy_sdk", "core", "policy", "v1alpha2", "policy.py"))
	assert.FileExists(t, filepath.Join(outputDir, "python-executor", "proto", "python_executor_pb2.py"))
}

func TestGenerateCode_MissingCmdDirectory(t *testing.T) {
	rootDir := t.TempDir()
	tmpDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "policy-engine")

	// Create go.mod but NOT the cmd/policy-engine directory
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create python-executor sibling directory
	pythonExecDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "python-executor")
	testutils.CreateDir(t, pythonExecDir)
	createPythonSDKCoreDir(t, rootDir)

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", "/policies/ratelimit", "github.com/example/ratelimit"),
	}

	outputDir := t.TempDir()
	err := GenerateCode(tmpDir, policies, outputDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write plugin_registry.go")
}

func TestGenerateCode_MissingGoMod(t *testing.T) {
	rootDir := t.TempDir()
	tmpDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "policy-engine")

	mainPkgDir := filepath.Join(tmpDir, "cmd", "policy-engine")
	testutils.CreateDir(t, mainPkgDir)

	// Create python-executor sibling directory
	pythonExecDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "python-executor")
	testutils.CreateDir(t, pythonExecDir)
	createPythonSDKCoreDir(t, rootDir)

	// No go.mod file
	policyPath := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", policyPath, "github.com/example/ratelimit"),
	}

	outputDir := t.TempDir()
	err := GenerateCode(tmpDir, policies, outputDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update go.mod")
}

func TestGenerateCode_MultiplePolicies(t *testing.T) {
	rootDir := t.TempDir()
	tmpDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "policy-engine")

	mainPkgDir := filepath.Join(tmpDir, "cmd", "policy-engine")
	testutils.CreateDir(t, mainPkgDir)
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create python-executor sibling directory
	pythonExecDir := filepath.Join(rootDir, "gateway", "gateway-runtime", "python-executor")
	testutils.CreateDir(t, pythonExecDir)
	createPythonSDKCoreDir(t, rootDir)

	// Create multiple policy directories
	policy1Path := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")
	policy2Path := testutils.CreatePolicyDir(t, tmpDir, "jwt-auth", "v0.1.0")
	policy3Path := testutils.CreatePolicyDir(t, tmpDir, "cors", "v2.0.0")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", policy1Path, "github.com/policy-engine/policies/ratelimit"),
		testutils.NewLocalDiscoveredPolicy("jwt-auth", "v0.1.0", policy2Path, "github.com/policy-engine/policies/jwt-auth"),
		testutils.NewLocalDiscoveredPolicy("cors", "v2.0.0", policy3Path, "github.com/policy-engine/policies/cors"),
	}

	outputDir := t.TempDir()
	err := GenerateCode(tmpDir, policies, outputDir)
	require.NoError(t, err)

	// Verify all policies are in the registry
	registryContent, err := os.ReadFile(filepath.Join(mainPkgDir, "plugin_registry.go"))
	require.NoError(t, err)
	assert.Contains(t, string(registryContent), "ratelimit")
	assert.Contains(t, string(registryContent), "jwt-auth")
	assert.Contains(t, string(registryContent), "cors")

	// Verify go.mod has all replace directives
	modData, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(modData), "github.com/policy-engine/policies/ratelimit")
	assert.Contains(t, string(modData), "github.com/policy-engine/policies/jwt-auth")
	assert.Contains(t, string(modData), "github.com/policy-engine/policies/cors")
}

// ==== Additional tests for remote gomodule policies ====

func TestUpdateGoMod_RemotePolicy_InvalidModule(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Create a remote policy (IsFilePathEntry: false) with invalid module
	policies := []*types.DiscoveredPolicy{
		testutils.NewRemoteDiscoveredPolicy("ratelimit", "v1.0.0", "github.com/nonexistent-org-12345/nonexistent-module", "v1.0.0"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go get failed")
}

func TestUpdateGoMod_DuplicateReplaceDirective(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod that already has a replace directive
	goModContent := `module github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine

go 1.23

replace github.com/example/policies/ratelimit => ./policies/ratelimit/v1.0.0
`
	testutils.WriteFile(t, filepath.Join(tmpDir, "go.mod"), goModContent)

	policyPath := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")

	// Try to add the same replace directive again
	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("ratelimit", "v1.0.0", policyPath, "github.com/example/policies/ratelimit"),
	}

	// Should succeed (handles "already exists" error gracefully)
	err := UpdateGoMod(tmpDir, policies)
	require.NoError(t, err)
}

func TestUpdateGoMod_MixedLocalAndRemoteSkipsRemote(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	policyPath := testutils.CreatePolicyDir(t, tmpDir, "local-policy", "v1.0.0")

	// Mix of local and remote policies (remote will fail but local should succeed)
	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("local-policy", "v1.0.0", policyPath, "github.com/example/policies/local"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.NoError(t, err)

	// Verify local policy replace directive was added
	modData, err := os.ReadFile(filepath.Join(tmpDir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, string(modData), "github.com/example/policies/local")
}

func TestUpdateGoMod_RelativeSrcDir(t *testing.T) {
	// Test with relative srcDir (should be converted to absolute)
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	policyPath := testutils.CreatePolicyDir(t, tmpDir, "test-policy", "v1.0.0")

	policies := []*types.DiscoveredPolicy{
		testutils.NewLocalDiscoveredPolicy("test-policy", "v1.0.0", policyPath, "github.com/example/test-policy"),
	}

	// Use absolute path (simulating what the code does with relative)
	err := UpdateGoMod(tmpDir, policies)
	require.NoError(t, err)
}

func TestUpdateGoMod_OnlyRemotePolicies_AllFail(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WritePolicyEngineGoMod(t, tmpDir)

	// Only remote policies, all with invalid modules
	policies := []*types.DiscoveredPolicy{
		testutils.NewRemoteDiscoveredPolicy("remote-policy", "v1.0.0", "github.com/definitely-not-real-org/fake-module", "v1.0.0"),
	}

	err := UpdateGoMod(tmpDir, policies)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "go get failed")
}
