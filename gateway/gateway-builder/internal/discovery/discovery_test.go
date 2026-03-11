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

package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
)

// ==== LoadBuildFile tests ====

func TestLoadBuildFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	manifest, err := LoadBuildFile(lockPath)

	require.NoError(t, err)
	assert.Equal(t, "v1", manifest.Version)
	assert.Len(t, manifest.Policies, 1)
	assert.Equal(t, "test-policy", manifest.Policies[0].Name)
}

func TestLoadBuildFile_FileNotFound(t *testing.T) {
	_, err := LoadBuildFile("/nonexistent/path.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read build file")
}

func TestLoadBuildFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, "invalid: yaml: content: -")

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse build file YAML")
}

func TestLoadBuildFile_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `policies:
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build file version is required")
}

func TestLoadBuildFile_UnsupportedVersion(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v2
policies:
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported build file version")
}

func TestLoadBuildFile_NoPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies: []
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "build file must declare at least one policy")
}

func TestLoadBuildFile_PolicyMissingName(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoadBuildFile_PolicyMissingPathAndModule(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: test-policy
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either filePath, gomodule, or pipPackage must be provided")
}

func TestLoadBuildFile_DuplicatePolicy(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: test-policy
    filePath: ./policies/test
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate policy entry")
}

func TestLoadBuildFile_BothFilePathAndGomodule(t *testing.T) {
	tmpDir := t.TempDir()

	// Multiple sources should be rejected
	manifestContent := `version: v1
policies:
  - name: test-policy
    filePath: ./policies/test
    gomodule: github.com/example/test
`
	lockPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, lockPath, manifestContent)

	_, err := LoadBuildFile(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only one of filePath, gomodule, or pipPackage may be provided")
}

// ==== ParsePolicyYAML tests ====

func TestParsePolicyYAML_Success(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `name: test-policy
version: v1.0.0
description: Test policy
`
	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	testutils.WriteFile(t, policyPath, policyContent)

	def, err := ParsePolicyYAML(policyPath)

	require.NoError(t, err)
	assert.Equal(t, "test-policy", def.Name)
	assert.Equal(t, "v1.0.0", def.Version)
}

func TestParsePolicyYAML_FileNotFound(t *testing.T) {
	_, err := ParsePolicyYAML("/nonexistent/policy.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read")
}

func TestParsePolicyYAML_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	testutils.WriteFile(t, policyPath, "invalid:: yaml:")

	_, err := ParsePolicyYAML(policyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestParsePolicyYAML_MissingName(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `version: v1.0.0`
	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	testutils.WriteFile(t, policyPath, policyContent)

	_, err := ParsePolicyYAML(policyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy name is required")
}

func TestParsePolicyYAML_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `name: test-policy`
	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	testutils.WriteFile(t, policyPath, policyContent)

	_, err := ParsePolicyYAML(policyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy version is required")
}

// ==== ValidateDirectoryStructure tests ====

func TestValidateDirectoryStructure_Success(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: test")
	testutils.WriteFile(t, filepath.Join(policyDir, "go.mod"), "module test")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")

	err := ValidateDirectoryStructure(policyDir)

	assert.NoError(t, err)
}

func TestValidateDirectoryStructure_MissingPolicyYAML(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "go.mod"), "module test")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")

	err := ValidateDirectoryStructure(policyDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy-definition.yaml")
}

func TestValidateDirectoryStructure_MissingGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: test")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")

	err := ValidateDirectoryStructure(policyDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go.mod")
}

func TestValidateDirectoryStructure_NoGoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: test")
	testutils.WriteFile(t, filepath.Join(policyDir, "go.mod"), "module test")

	err := ValidateDirectoryStructure(policyDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no .go files found")
}

// ==== CollectSourceFiles tests ====

func TestCollectSourceFiles_Success(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")
	testutils.WriteFile(t, filepath.Join(policyDir, "helper.go"), "package test")
	testutils.WriteFile(t, filepath.Join(policyDir, "config.yaml"), "key: value") // Non-.go file

	files, err := CollectSourceFiles(policyDir)

	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestCollectSourceFiles_DirectoryNotFound(t *testing.T) {
	_, err := CollectSourceFiles("/nonexistent/dir")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read directory")
}

func TestCollectSourceFiles_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "empty-policy")
	testutils.CreateDir(t, policyDir)

	files, err := CollectSourceFiles(policyDir)

	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestCollectSourceFiles_IgnoresDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package test")
	testutils.CreateDir(t, filepath.Join(policyDir, "subdir.go")) // Directory with .go suffix

	files, err := CollectSourceFiles(policyDir)

	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// ==== DiscoverPoliciesFromBuildFile tests ====

func TestDiscoverPoliciesFromBuildFile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory with all required files using testutils
	policyDir := testutils.CreatePolicyDir(t, tmpDir, "ratelimit", "v1.0.0")

	// Create policy-definition.yaml
	testutils.CreatePolicyDefinitionYAML(t, policyDir, "ratelimit", "v1.0.0")

	// Create go.mod
	testutils.WriteGoMod(t, policyDir, "github.com/example/policies/ratelimit")

	// Create a source file
	testutils.WriteFile(t, filepath.Join(policyDir, "ratelimit.go"), "package ratelimit\n")

	// Create manifest lock file
	manifestContent := `version: v1
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit/v1.0.0
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Discover policies
	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "ratelimit", policies[0].Name)
	assert.Equal(t, "v1.0.0", policies[0].Version)
	assert.Contains(t, policies[0].Path, "ratelimit")
	assert.Len(t, policies[0].SourceFiles, 1)
}

func TestDiscoverPoliciesFromBuildFile_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two policy directories
	policy1Dir := filepath.Join(tmpDir, "policies", "ratelimit")
	policy2Dir := filepath.Join(tmpDir, "policies", "jwt-auth")
	testutils.CreateDir(t, policy1Dir)
	testutils.CreateDir(t, policy2Dir)

	// Policy 1: ratelimit
	testutils.WriteFile(t, filepath.Join(policy1Dir, "policy-definition.yaml"), "name: ratelimit\nversion: v1.0.0\n")
	testutils.WriteGoMod(t, policy1Dir, "github.com/example/ratelimit")
	testutils.WriteFile(t, filepath.Join(policy1Dir, "policy.go"), "package ratelimit\n")

	// Policy 2: jwt-auth
	testutils.WriteFile(t, filepath.Join(policy2Dir, "policy-definition.yaml"), "name: jwt-auth\nversion: v0.1.0\n")
	testutils.WriteGoMod(t, policy2Dir, "github.com/example/jwt-auth")
	testutils.WriteFile(t, filepath.Join(policy2Dir, "policy.go"), "package jwtauth\n")

	// Create manifest lock
	manifestContent := `version: v1
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit
  - name: jwt-auth
    filePath: ./policies/jwt-auth
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 2)
	assert.Equal(t, "ratelimit", policies[0].Name)
	assert.Equal(t, "jwt-auth", policies[1].Name)
}

func TestDiscoverPoliciesFromBuildFile_ManifestNotFound(t *testing.T) {
	policies, err := DiscoverPoliciesFromBuildFile("/nonexistent/manifest.yaml", "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "failed to read build file")
}

func TestDiscoverPoliciesFromBuildFile_PolicyPathNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: missing-policy
    filePath: ./nonexistent-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "from build file entry missing-policy")
}

func TestDiscoverPoliciesFromBuildFile_NoFilePathOrGomodule(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: incomplete-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "either filePath, gomodule, or pipPackage must be provided")
}

func TestDiscoverPoliciesFromBuildFile_NameMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory with different name in definition
	policyDir := filepath.Join(tmpDir, "policies", "my-policy")
	testutils.CreateDir(t, policyDir)

	// Policy definition has a different name
	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: different-name\nversion: v1.0.0\n")
	testutils.WriteGoMod(t, policyDir, "github.com/example/policy")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package policy\n")

	manifestContent := `version: v1
policies:
  - name: my-policy
    filePath: ./policies/my-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "policy name mismatch")
}

func TestDiscoverPoliciesFromBuildFile_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()

	policyDir := filepath.Join(tmpDir, "policies", "noversion")
	testutils.CreateDir(t, policyDir)

	// Policy definition without version
	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: noversion\n")
	testutils.WriteGoMod(t, policyDir, "github.com/example/noversion")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package noversion\n")

	manifestContent := `version: v1
policies:
  - name: noversion
    filePath: ./policies/noversion
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "policy version is required")
}

func TestDiscoverPoliciesFromBuildFile_InvalidPolicyStructure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory without go.mod and without .py files — ambiguous
	policyDir := filepath.Join(tmpDir, "policies", "invalid")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: invalid\nversion: v1.0.0\n")
	// No go.mod, no .py files → defaults to Go, which will fail

	manifestContent := `version: v1
policies:
  - name: invalid
    filePath: ./policies/invalid
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "failed to read module path from go.mod for invalid")
}

func TestDiscoverPoliciesFromBuildFile_WithBaseDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy in a different location
	policiesBaseDir := filepath.Join(tmpDir, "custom-policies")
	policyDir := filepath.Join(policiesBaseDir, "ratelimit")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: ratelimit\nversion: v1.0.0\n")
	testutils.WriteGoMod(t, policyDir, "github.com/example/ratelimit")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package ratelimit\n")

	// Manifest uses relative path
	manifestContent := `version: v1
policies:
  - name: ratelimit
    filePath: ./ratelimit
`
	manifestPath := filepath.Join(tmpDir, "manifest", "build-lock.yaml")
	testutils.CreateDir(t, filepath.Dir(manifestPath))
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Provide custom baseDir
	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, policiesBaseDir)

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "ratelimit", policies[0].Name)
}

func TestDiscoverPoliciesFromBuildFile_EmptyManifest(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies: []
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	// Empty manifest is an error per the implementation
	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "must declare at least one policy")
}

func TestDiscoverPoliciesFromBuildFile_InvalidPolicyDefinition(t *testing.T) {
	tmpDir := t.TempDir()

	policyDir := filepath.Join(tmpDir, "policies", "baddef")
	testutils.CreateDir(t, policyDir)

	// Invalid YAML in policy definition
	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "invalid: yaml: content:::")
	testutils.WriteGoMod(t, policyDir, "github.com/example/baddef")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package baddef\n")

	manifestContent := `version: v1
policies:
  - name: baddef
    filePath: ./policies/baddef
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestDiscoverPoliciesFromBuildFile_WithSystemParameters(t *testing.T) {
	tmpDir := t.TempDir()

	policyDir := filepath.Join(tmpDir, "policies", "sysparam-policy")
	testutils.CreateDir(t, policyDir)

	// Policy with system parameters
	policyDefContent := `name: sysparam-policy
version: v1.0.0
systemParameters:
  type: object
  properties:
    keyManagers:
      type: array
      "wso2/defaultValue": "${config.keymanagers}"
`
	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), policyDefContent)
	testutils.WriteGoMod(t, policyDir, "github.com/example/sysparam")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package sysparam\n")

	manifestContent := `version: v1
policies:
  - name: sysparam-policy
    filePath: ./policies/sysparam-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "sysparam-policy", policies[0].Name)
	// SystemParameters should be extracted
	assert.NotNil(t, policies[0].SystemParameters)
}

// ==== Tests for extractModulePathFromGoMod ====

func TestExtractModulePathFromGoMod_Success(t *testing.T) {
	tmpDir := t.TempDir()

	goModContent := `module github.com/example/test-policy

go 1.23

require github.com/stretchr/testify v1.8.0
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	testutils.WriteFile(t, goModPath, goModContent)

	modulePath, err := extractModulePathFromGoMod(goModPath)

	require.NoError(t, err)
	assert.Equal(t, "github.com/example/test-policy", modulePath)
}

func TestExtractModulePathFromGoMod_FileNotFound(t *testing.T) {
	_, err := extractModulePathFromGoMod("/nonexistent/go.mod")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read go.mod")
}

func TestExtractModulePathFromGoMod_InvalidGoMod(t *testing.T) {
	tmpDir := t.TempDir()

	goModPath := filepath.Join(tmpDir, "go.mod")
	testutils.WriteFile(t, goModPath, "this is not valid go.mod syntax!!!")

	_, err := extractModulePathFromGoMod(goModPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse go.mod")
}

func TestExtractModulePathFromGoMod_MissingModuleDirective(t *testing.T) {
	tmpDir := t.TempDir()

	// go.mod with no module directive
	goModContent := `go 1.23

require github.com/stretchr/testify v1.8.0
`
	goModPath := filepath.Join(tmpDir, "go.mod")
	testutils.WriteFile(t, goModPath, goModContent)

	_, err := extractModulePathFromGoMod(goModPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "module directive missing")
}

// ==== Tests for resolveModuleInfo ====

func TestResolveModuleInfo_InvalidModule(t *testing.T) {
	// Test with a non-existent module
	_, err := resolveModuleInfo("github.com/nonexistent-org-12345/nonexistent-module@v1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to run 'go mod download")
}

func TestResolveModuleInfo_InvalidModuleFormat(t *testing.T) {
	// Test with an invalid module format
	_, err := resolveModuleInfo("not-a-valid-module-path")

	assert.Error(t, err)
}

// ==== Tests for ValidateDirectoryStructure error paths ====

func TestValidateDirectoryStructure_UnreadableDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a directory and make it unreadable
	testDir := filepath.Join(tmpDir, "unreadable")
	testutils.CreateDir(t, testDir)

	// Create required files first
	testutils.WriteFile(t, filepath.Join(testDir, "policy-definition.yaml"), "name: test\nversion: v1.0.0")
	testutils.WriteFile(t, filepath.Join(testDir, "go.mod"), "module test\n\ngo 1.23")

	// Now make directory unreadable (skip on Windows)
	err := os.Chmod(testDir, 0000)
	if err != nil {
		t.Skip("Cannot change directory permissions on this OS")
	}
	defer os.Chmod(testDir, 0755) // Restore for cleanup

	// If the directory is still readable (e.g., privileged CI), skip to avoid false failures.
	if _, err := os.ReadDir(testDir); err == nil {
		t.Skip("Directory still readable after chmod; skipping permission test")
	}

	err = ValidateDirectoryStructure(testDir)

	// Should fail due to permission error
	assert.Error(t, err)
}

// ==== Tests for DiscoverPoliciesFromBuildFile with gomodule entries ====

func TestDiscoverPoliciesFromBuildFile_GomoduleEntry_InvalidModule(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: remote-policy
    gomodule: github.com/nonexistent-org-12345/fake-policy@v1.0.0
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	_, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve gomodule")
}

func TestDiscoverPoliciesFromBuildFile_MixedEntries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a local policy
	policyDir := filepath.Join(tmpDir, "policies", "local-policy")
	testutils.CreateDir(t, policyDir)

	policyDefContent := `name: local-policy
version: v1.0.0
`
	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), policyDefContent)
	testutils.WriteGoMod(t, policyDir, "github.com/example/local")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package local\n")

	// Manifest with local entry only (remote would fail and we test that separately)
	manifestContent := `version: v1
policies:
  - name: local-policy
    filePath: ./policies/local-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "local-policy", policies[0].Name)
	assert.True(t, policies[0].IsFilePathEntry)
}

// ==== DetectRuntime tests ====

func TestDetectRuntime_GoPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteFile(t, filepath.Join(tmpDir, "go.mod"), "module test\n\ngo 1.23")
	testutils.WriteFile(t, filepath.Join(tmpDir, "policy.go"), "package test")

	runtime := DetectRuntime(tmpDir)

	assert.Equal(t, "go", runtime)
}

func TestDetectRuntime_PythonPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	testutils.WriteFile(t, filepath.Join(tmpDir, "policy.py"), "def on_request(): pass")
	testutils.WriteFile(t, filepath.Join(tmpDir, "requirements.txt"), "")

	runtime := DetectRuntime(tmpDir)

	assert.Equal(t, "python", runtime)
}

func TestDetectRuntime_GoTakesPrecedence(t *testing.T) {
	// If both go.mod and .py files exist, Go wins
	tmpDir := t.TempDir()
	testutils.WriteFile(t, filepath.Join(tmpDir, "go.mod"), "module test\n\ngo 1.23")
	testutils.WriteFile(t, filepath.Join(tmpDir, "policy.go"), "package test")
	testutils.WriteFile(t, filepath.Join(tmpDir, "helper.py"), "# python helper")

	runtime := DetectRuntime(tmpDir)

	assert.Equal(t, "go", runtime)
}

func TestDetectRuntime_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	runtime := DetectRuntime(tmpDir)

	assert.Equal(t, "go", runtime)
}

func TestDetectRuntime_NonexistentDirectory(t *testing.T) {
	runtime := DetectRuntime("/nonexistent/path")

	assert.Equal(t, "go", runtime)
}

// ==== ParsePipPackageRef tests ====

func TestParsePipPackageRef_SimplePackage(t *testing.T) {
	pkg, version, indexURL, err := ParsePipPackageRef("wso2-gateway-policy-prompt-compression==1.0.0")

	require.NoError(t, err)
	assert.Equal(t, "wso2-gateway-policy-prompt-compression", pkg)
	assert.Equal(t, "1.0.0", version)
	assert.Empty(t, indexURL)
}

func TestParsePipPackageRef_WithPrivateIndex(t *testing.T) {
	pkg, version, indexURL, err := ParsePipPackageRef("my-org-auth-policy==2.3.0@https://pypi.my-company.com/simple")

	require.NoError(t, err)
	assert.Equal(t, "my-org-auth-policy", pkg)
	assert.Equal(t, "2.3.0", version)
	assert.Equal(t, "https://pypi.my-company.com/simple", indexURL)
}

func TestParsePipPackageRef_MissingVersion(t *testing.T) {
	_, _, _, err := ParsePipPackageRef("my-package")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pipPackage format")
}

func TestParsePipPackageRef_EmptyVersion(t *testing.T) {
	_, _, _, err := ParsePipPackageRef("my-package==")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pipPackage format")
}

func TestParsePipPackageRef_EmptyPackageName(t *testing.T) {
	_, _, _, err := ParsePipPackageRef("==1.0.0")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pipPackage format")
}

// ==== Fingerprint-based discovery tests ====

func TestDiscoverPoliciesFromBuildFile_PythonAutoDetect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Python policy directory (no go.mod, has .py files)
	policyDir := filepath.Join(tmpDir, "policies", "my-python-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), `name: my-python-policy
version: v1.0.0
processingMode:
  requestHeaderMode: PROCESS
  requestBodyMode: SKIP
  responseHeaderMode: SKIP
  responseBodyMode: SKIP
`)
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.py"), "def on_request(): pass\n")
	testutils.WriteFile(t, filepath.Join(policyDir, "requirements.txt"), "")

	manifestContent := `version: v1
policies:
  - name: my-python-policy
    filePath: ./policies/my-python-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "my-python-policy", policies[0].Name)
	assert.Equal(t, "python", policies[0].Runtime)
	assert.NotNil(t, policies[0].ProcessingMode)
}

func TestDiscoverPoliciesFromBuildFile_GoAutoDetect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go policy directory
	policyDir := filepath.Join(tmpDir, "policies", "my-go-policy")
	testutils.CreateDir(t, policyDir)

	testutils.WriteFile(t, filepath.Join(policyDir, "policy-definition.yaml"), "name: my-go-policy\nversion: v1.0.0\n")
	testutils.WriteGoMod(t, policyDir, "github.com/example/my-go-policy")
	testutils.WriteFile(t, filepath.Join(policyDir, "policy.go"), "package mygopolicy\n")

	manifestContent := `version: v1
policies:
  - name: my-go-policy
    filePath: ./policies/my-go-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "my-go-policy", policies[0].Name)
	assert.Equal(t, "go", policies[0].Runtime)
}

func TestDiscoverPoliciesFromBuildFile_MixedGoAndPython(t *testing.T) {
	tmpDir := t.TempDir()

	// Go policy
	goDir := filepath.Join(tmpDir, "policies", "go-policy")
	testutils.CreateDir(t, goDir)
	testutils.WriteFile(t, filepath.Join(goDir, "policy-definition.yaml"), "name: go-policy\nversion: v1.0.0\n")
	testutils.WriteGoMod(t, goDir, "github.com/example/go-policy")
	testutils.WriteFile(t, filepath.Join(goDir, "policy.go"), "package gopolicy\n")

	// Python policy
	pyDir := filepath.Join(tmpDir, "policies", "py-policy")
	testutils.CreateDir(t, pyDir)
	testutils.WriteFile(t, filepath.Join(pyDir, "policy-definition.yaml"), `name: py-policy
version: v1.0.0
processingMode:
  requestHeaderMode: PROCESS
  requestBodyMode: SKIP
  responseHeaderMode: SKIP
  responseBodyMode: SKIP
`)
	testutils.WriteFile(t, filepath.Join(pyDir, "policy.py"), "def on_request(): pass\n")

	manifestContent := `version: v1
policies:
  - name: go-policy
    filePath: ./policies/go-policy
  - name: py-policy
    filePath: ./policies/py-policy
`
	manifestPath := filepath.Join(tmpDir, "build-lock.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	policies, err := DiscoverPoliciesFromBuildFile(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 2)
	assert.Equal(t, "go-policy", policies[0].Name)
	assert.Equal(t, "go", policies[0].Runtime)
	assert.Equal(t, "py-policy", policies[1].Name)
	assert.Equal(t, "python", policies[1].Runtime)
}
