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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==== LoadManifest tests ====

func TestLoadManifest_Success(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	manifest, err := LoadManifest(lockPath)

	require.NoError(t, err)
	assert.Equal(t, "v1", manifest.Version)
	assert.Len(t, manifest.Policies, 1)
	assert.Equal(t, "test-policy", manifest.Policies[0].Name)
}

func TestLoadManifest_FileNotFound(t *testing.T) {
	_, err := LoadManifest("/nonexistent/path.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read manifest lock file")
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte("invalid: yaml: content: -"), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse manifest YAML")
}

func TestLoadManifest_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `policies:
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manifest version is required")
}

func TestLoadManifest_UnsupportedVersion(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v2
policies:
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported manifest version")
}

func TestLoadManifest_NoPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies: []
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manifest must declare at least one policy")
}

func TestLoadManifest_PolicyMissingName(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestLoadManifest_PolicyMissingPathAndModule(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: test-policy
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "either filePath or gomodule must be provided")
}

func TestLoadManifest_DuplicatePolicy(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: test-policy
    filePath: ./policies/test
  - name: test-policy
    filePath: ./policies/test
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	_, err = LoadManifest(lockPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate policy entry")
}

func TestLoadManifest_BothFilePathAndGomodule(t *testing.T) {
	tmpDir := t.TempDir()

	// This should succeed but warn (filePath preferred)
	manifestContent := `version: v1
policies:
  - name: test-policy
    filePath: ./policies/test
    gomodule: github.com/example/test
`
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err := os.WriteFile(lockPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	manifest, err := LoadManifest(lockPath)

	require.NoError(t, err)
	assert.Equal(t, "./policies/test", manifest.Policies[0].FilePath)
	assert.Equal(t, "github.com/example/test", manifest.Policies[0].Gomodule)
}

// ==== ParsePolicyYAML tests ====

func TestParsePolicyYAML_Success(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `name: test-policy
version: v1.0.0
description: Test policy
`
	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	err := os.WriteFile(policyPath, []byte(policyContent), 0644)
	require.NoError(t, err)

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
	err := os.WriteFile(policyPath, []byte("invalid:: yaml:"), 0644)
	require.NoError(t, err)

	_, err = ParsePolicyYAML(policyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestParsePolicyYAML_MissingName(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `version: v1.0.0`
	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	err := os.WriteFile(policyPath, []byte(policyContent), 0644)
	require.NoError(t, err)

	_, err = ParsePolicyYAML(policyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy name is required")
}

func TestParsePolicyYAML_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()

	policyContent := `name: test-policy`
	policyPath := filepath.Join(tmpDir, "policy-definition.yaml")
	err := os.WriteFile(policyPath, []byte(policyContent), 0644)
	require.NoError(t, err)

	_, err = ParsePolicyYAML(policyPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy version is required")
}

// ==== ValidateDirectoryStructure tests ====

func TestValidateDirectoryStructure_Success(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"), []byte("name: test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"), []byte("package test"), 0644)

	err := ValidateDirectoryStructure(policyDir)

	assert.NoError(t, err)
}

func TestValidateDirectoryStructure_MissingPolicyYAML(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"), []byte("package test"), 0644)

	err := ValidateDirectoryStructure(policyDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "policy-definition.yaml")
}

func TestValidateDirectoryStructure_MissingGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"), []byte("name: test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"), []byte("package test"), 0644)

	err := ValidateDirectoryStructure(policyDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "go.mod")
}

func TestValidateDirectoryStructure_NoGoFiles(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"), []byte("name: test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"), []byte("module test"), 0644)

	err := ValidateDirectoryStructure(policyDir)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no .go files found")
}

// ==== CollectSourceFiles tests ====

func TestCollectSourceFiles_Success(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy.go"), []byte("package test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "helper.go"), []byte("package test"), 0644)
	os.WriteFile(filepath.Join(policyDir, "config.yaml"), []byte("key: value"), 0644) // Non-.go file

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
	os.MkdirAll(policyDir, 0755)

	files, err := CollectSourceFiles(policyDir)

	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestCollectSourceFiles_IgnoresDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	policyDir := filepath.Join(tmpDir, "test-policy")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy.go"), []byte("package test"), 0644)
	os.MkdirAll(filepath.Join(policyDir, "subdir.go"), 0755) // Directory with .go suffix

	files, err := CollectSourceFiles(policyDir)

	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// ==== DiscoverPoliciesFromManifest tests ====

func TestDiscoverPoliciesFromManifest_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory with all required files
	policyDir := filepath.Join(tmpDir, "policies", "ratelimit", "v1.0.0")
	err := os.MkdirAll(policyDir, 0755)
	require.NoError(t, err)

	// Create policy-definition.yaml
	policyDefContent := `name: ratelimit
version: v1.0.0
displayName: Rate Limit Policy
description: Limits API request rates
`
	err = os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"), []byte(policyDefContent), 0644)
	require.NoError(t, err)

	// Create go.mod
	goModContent := `module github.com/example/policies/ratelimit

go 1.23
`
	err = os.WriteFile(filepath.Join(policyDir, "go.mod"), []byte(goModContent), 0644)
	require.NoError(t, err)

	// Create a source file
	err = os.WriteFile(filepath.Join(policyDir, "ratelimit.go"), []byte("package ratelimit\n"), 0644)
	require.NoError(t, err)

	// Create manifest lock file
	manifestContent := `version: v1
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit/v1.0.0
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	err = os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	// Discover policies
	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "ratelimit", policies[0].Name)
	assert.Equal(t, "v1.0.0", policies[0].Version)
	assert.Contains(t, policies[0].Path, "ratelimit")
	assert.Len(t, policies[0].SourceFiles, 1)
}

func TestDiscoverPoliciesFromManifest_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two policy directories
	policy1Dir := filepath.Join(tmpDir, "policies", "ratelimit")
	policy2Dir := filepath.Join(tmpDir, "policies", "jwt-auth")
	err := os.MkdirAll(policy1Dir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(policy2Dir, 0755)
	require.NoError(t, err)

	// Policy 1: ratelimit
	os.WriteFile(filepath.Join(policy1Dir, "policy-definition.yaml"),
		[]byte("name: ratelimit\nversion: v1.0.0\n"), 0644)
	os.WriteFile(filepath.Join(policy1Dir, "go.mod"),
		[]byte("module github.com/example/ratelimit\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policy1Dir, "policy.go"),
		[]byte("package ratelimit\n"), 0644)

	// Policy 2: jwt-auth
	os.WriteFile(filepath.Join(policy2Dir, "policy-definition.yaml"),
		[]byte("name: jwt-auth\nversion: v0.1.0\n"), 0644)
	os.WriteFile(filepath.Join(policy2Dir, "go.mod"),
		[]byte("module github.com/example/jwt-auth\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policy2Dir, "policy.go"),
		[]byte("package jwtauth\n"), 0644)

	// Create manifest lock
	manifestContent := `version: v1
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit
  - name: jwt-auth
    filePath: ./policies/jwt-auth
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 2)
	assert.Equal(t, "ratelimit", policies[0].Name)
	assert.Equal(t, "jwt-auth", policies[1].Name)
}

func TestDiscoverPoliciesFromManifest_ManifestNotFound(t *testing.T) {
	policies, err := DiscoverPoliciesFromManifest("/nonexistent/manifest.yaml", "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "failed to read manifest lock")
}

func TestDiscoverPoliciesFromManifest_PolicyPathNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: missing-policy
    filePath: ./nonexistent-policy
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	// The error occurs when trying to read go.mod from the non-existent policy path
	assert.Contains(t, err.Error(), "failed to read module path from go.mod for missing-policy")
}

func TestDiscoverPoliciesFromManifest_NoFilePathOrGomodule(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies:
  - name: incomplete-policy
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "either filePath or gomodule must be provided")
}

func TestDiscoverPoliciesFromManifest_NameMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory with different name in definition
	policyDir := filepath.Join(tmpDir, "policies", "my-policy")
	os.MkdirAll(policyDir, 0755)

	// Policy definition has a different name
	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"),
		[]byte("name: different-name\nversion: v1.0.0\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"),
		[]byte("module github.com/example/policy\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"),
		[]byte("package policy\n"), 0644)

	manifestContent := `version: v1
policies:
  - name: my-policy
    filePath: ./policies/my-policy
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "policy name mismatch")
}

func TestDiscoverPoliciesFromManifest_MissingVersion(t *testing.T) {
	tmpDir := t.TempDir()

	policyDir := filepath.Join(tmpDir, "policies", "noversion")
	os.MkdirAll(policyDir, 0755)

	// Policy definition without version
	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"),
		[]byte("name: noversion\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"),
		[]byte("module github.com/example/noversion\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"),
		[]byte("package noversion\n"), 0644)

	manifestContent := `version: v1
policies:
  - name: noversion
    filePath: ./policies/noversion
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "policy version is required")
}

func TestDiscoverPoliciesFromManifest_InvalidPolicyStructure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory without go.mod
	policyDir := filepath.Join(tmpDir, "policies", "invalid")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"),
		[]byte("name: invalid\nversion: v1.0.0\n"), 0644)
	// Missing go.mod

	manifestContent := `version: v1
policies:
  - name: invalid
    filePath: ./policies/invalid
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	// The error occurs when trying to read go.mod which doesn't exist
	assert.Contains(t, err.Error(), "failed to read module path from go.mod for invalid")
}

func TestDiscoverPoliciesFromManifest_WithBaseDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy in a different location
	policiesBaseDir := filepath.Join(tmpDir, "custom-policies")
	policyDir := filepath.Join(policiesBaseDir, "ratelimit")
	os.MkdirAll(policyDir, 0755)

	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"),
		[]byte("name: ratelimit\nversion: v1.0.0\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"),
		[]byte("module github.com/example/ratelimit\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"),
		[]byte("package ratelimit\n"), 0644)

	// Manifest uses relative path
	manifestContent := `version: v1
policies:
  - name: ratelimit
    filePath: ./ratelimit
`
	manifestPath := filepath.Join(tmpDir, "manifest", "policy-manifest-lock.yaml")
	os.MkdirAll(filepath.Dir(manifestPath), 0755)
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	// Provide custom baseDir
	policies, err := DiscoverPoliciesFromManifest(manifestPath, policiesBaseDir)

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "ratelimit", policies[0].Name)
}

func TestDiscoverPoliciesFromManifest_EmptyManifest(t *testing.T) {
	tmpDir := t.TempDir()

	manifestContent := `version: v1
policies: []
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	// Empty manifest is an error per the implementation
	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "must declare at least one policy")
}

func TestDiscoverPoliciesFromManifest_InvalidPolicyDefinition(t *testing.T) {
	tmpDir := t.TempDir()

	policyDir := filepath.Join(tmpDir, "policies", "baddef")
	os.MkdirAll(policyDir, 0755)

	// Invalid YAML in policy definition
	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"),
		[]byte("invalid: yaml: content:::"), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"),
		[]byte("module github.com/example/baddef\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"),
		[]byte("package baddef\n"), 0644)

	manifestContent := `version: v1
policies:
  - name: baddef
    filePath: ./policies/baddef
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.Error(t, err)
	assert.Nil(t, policies)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestDiscoverPoliciesFromManifest_WithSystemParameters(t *testing.T) {
	tmpDir := t.TempDir()

	policyDir := filepath.Join(tmpDir, "policies", "sysparam-policy")
	os.MkdirAll(policyDir, 0755)

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
	os.WriteFile(filepath.Join(policyDir, "policy-definition.yaml"),
		[]byte(policyDefContent), 0644)
	os.WriteFile(filepath.Join(policyDir, "go.mod"),
		[]byte("module github.com/example/sysparam\n\ngo 1.23\n"), 0644)
	os.WriteFile(filepath.Join(policyDir, "policy.go"),
		[]byte("package sysparam\n"), 0644)

	manifestContent := `version: v1
policies:
  - name: sysparam-policy
    filePath: ./policies/sysparam-policy
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	os.WriteFile(manifestPath, []byte(manifestContent), 0644)

	policies, err := DiscoverPoliciesFromManifest(manifestPath, "")

	require.NoError(t, err)
	require.Len(t, policies, 1)
	assert.Equal(t, "sysparam-policy", policies[0].Name)
	// SystemParameters should be extracted
	assert.NotNil(t, policies[0].SystemParameters)
}
