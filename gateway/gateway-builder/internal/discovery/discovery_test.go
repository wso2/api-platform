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
