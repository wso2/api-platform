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

package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/testutils"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestCreateManifest_EmptyPolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	manifest := CreateManifest("v1.0.0", policies, "/output")

	assert.Equal(t, "v1.0.0", manifest.BuilderVersion)
	assert.Equal(t, "/output", manifest.OutputDir)
	assert.Empty(t, manifest.Policies)
	assert.NotEmpty(t, manifest.BuildTimestamp)
}

func TestCreateManifest_SinglePolicy(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    "/policies/ratelimit",
		},
	}

	manifest := CreateManifest("v2.0.0", policies, "/build/output")

	assert.Equal(t, "v2.0.0", manifest.BuilderVersion)
	assert.Equal(t, "/build/output", manifest.OutputDir)
	assert.Len(t, manifest.Policies, 1)
	assert.Equal(t, "ratelimit", manifest.Policies[0].Name)
	assert.Equal(t, "v1.0.0", manifest.Policies[0].Version)
}

func TestCreateManifest_MultiplePolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0"},
		{Name: "jwt-auth", Version: "v0.1.0"},
		{Name: "cors", Version: "v2.0.0"},
	}

	manifest := CreateManifest("v1.5.0", policies, "/out")

	assert.Len(t, manifest.Policies, 3)
	assert.Equal(t, "ratelimit", manifest.Policies[0].Name)
	assert.Equal(t, "jwt-auth", manifest.Policies[1].Name)
	assert.Equal(t, "cors", manifest.Policies[2].Name)
}

func TestManifest_ToJSON_EmptyPolicies(t *testing.T) {
	manifest := &Manifest{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies:       []PolicyInfo{},
	}

	jsonStr, err := manifest.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonStr, `"builderVersion": "v1.0.0"`)
	assert.Contains(t, jsonStr, `"outputDir": "/output"`)
	assert.Contains(t, jsonStr, `"policies": []`)
}

func TestManifest_ToJSON_WithPolicies(t *testing.T) {
	manifest := &Manifest{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies: []PolicyInfo{
			{Name: "ratelimit", Version: "v1.0.0"},
			{Name: "jwt-auth", Version: "v0.1.0"},
		},
	}

	jsonStr, err := manifest.ToJSON()
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", parsed["builderVersion"])
	policies := parsed["policies"].([]interface{})
	assert.Len(t, policies, 2)
}

func TestManifest_WriteToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "manifest.json")

	manifest := &Manifest{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies: []PolicyInfo{
			{Name: "test-policy", Version: "v1.0.0"},
		},
	}

	err := manifest.WriteToFile(filePath)
	require.NoError(t, err)

	// Verify file exists and contains expected content
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-policy")
	assert.Contains(t, string(content), "v1.0.0")
}

func TestManifest_WriteToFile_DirectoryNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent-dir", "manifest.json")

	manifest := &Manifest{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies:       []PolicyInfo{},
	}

	err := manifest.WriteToFile(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write manifest file")
}

func TestWriteManifestLockWithVersions_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a manifest file
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create the policy directory structure
	policyDir := filepath.Join(tmpDir, "policies", "ratelimit")
	testutils.CreateDir(t, policyDir)

	// Create discovered policies
	discovered := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    policyDir,
		},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	// Verify lock file was created
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ratelimit")
	assert.Contains(t, string(content), "v1.0.0")
}

func TestWriteManifestLockWithVersions_ManifestNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "nonexistent.yaml")

	discovered := []*types.DiscoveredPolicy{}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read manifest file")
}

func TestWriteManifestLockWithVersions_EmptyManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with no policies (should succeed with empty lock)
	manifestContent := `version: "1.0"
policies: []
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	discovered := []*types.DiscoveredPolicy{}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	// Verify lock file was created
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	_, err = os.Stat(lockPath)
	assert.NoError(t, err)
}

func TestWriteManifestLockWithVersions_PolicyNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a manifest file with a policy that won't be discovered
	manifestContent := `version: "1.0"
policies:
  - name: unknown-policy
    filePath: ./policies/unknown
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Empty discovered policies
	discovered := []*types.DiscoveredPolicy{}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine version for policy")
}

func TestWriteManifestLockWithVersions_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a manifest file with multiple policies
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit
  - name: jwt-auth
    filePath: ./policies/jwt-auth
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create the policy directories
	ratelimitDir := filepath.Join(tmpDir, "policies", "ratelimit")
	jwtAuthDir := filepath.Join(tmpDir, "policies", "jwt-auth")
	testutils.CreateDir(t, ratelimitDir)
	testutils.CreateDir(t, jwtAuthDir)

	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: ratelimitDir},
		{Name: "jwt-auth", Version: "v0.1.0", Path: jwtAuthDir},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	// Verify lock file
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ratelimit")
	assert.Contains(t, string(content), "jwt-auth")
	assert.Contains(t, string(content), "v1.0.0")
	assert.Contains(t, string(content), "v0.1.0")
}

// ==== Phase 2a: Additional coverage for WriteManifestLockWithVersions ====

func TestWriteManifestLockWithVersions_MultipleCandidatesWithFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with a policy that has filePath
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit-v2
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create policy directories for both versions
	v1Dir := filepath.Join(tmpDir, "policies", "ratelimit-v1")
	v2Dir := filepath.Join(tmpDir, "policies", "ratelimit-v2")
	testutils.CreateDir(t, v1Dir)
	testutils.CreateDir(t, v2Dir)

	// Multiple candidates with same name but different paths
	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: v1Dir},
		{Name: "ratelimit", Version: "v2.0.0", Path: v2Dir},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	// Verify the correct version was selected (v2.0.0 based on filePath)
	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v2.0.0")
	assert.NotContains(t, string(content), "v1.0.0")
}

func TestWriteManifestLockWithVersions_GomoduleWithVersion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with gomodule containing @version
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@v1.0.0
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create policy directory with go.mod
	policyDir := filepath.Join(tmpDir, "policies", "ratelimit")
	testutils.CreateDir(t, policyDir)

	goModPath := filepath.Join(policyDir, "go.mod")
	testutils.WriteGoMod(t, policyDir, "github.com/example/ratelimit")

	discovered := []*types.DiscoveredPolicy{
		{
			Name:      "ratelimit",
			Version:   "v1.0.0",
			Path:      policyDir,
			GoModPath: goModPath,
		},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ratelimit")
	assert.Contains(t, string(content), "v1.0.0")
}

func TestWriteManifestLockWithVersions_GomoduleWithoutVersion(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with gomodule without @version
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create policy directory with go.mod
	policyDir := filepath.Join(tmpDir, "policies", "ratelimit")
	testutils.CreateDir(t, policyDir)

	goModPath := filepath.Join(policyDir, "go.mod")
	testutils.WriteGoMod(t, policyDir, "github.com/example/ratelimit")

	discovered := []*types.DiscoveredPolicy{
		{
			Name:      "ratelimit",
			Version:   "v2.0.0",
			Path:      policyDir,
			GoModPath: goModPath,
		},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v2.0.0")
}

func TestWriteManifestLockWithVersions_GomoduleNoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with gomodule
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@v1.0.0
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Empty discovered policies - no candidates at all
	discovered := []*types.DiscoveredPolicy{}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine version for policy")
}

func TestWriteManifestLockWithVersions_GomoduleMultipleCandidates(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with gomodule
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@v2.0.0
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create two policy directories with go.mod
	v1Dir := filepath.Join(tmpDir, "policies", "ratelimit-v1")
	v2Dir := filepath.Join(tmpDir, "policies", "ratelimit-v2")
	testutils.CreateDir(t, v1Dir)
	testutils.CreateDir(t, v2Dir)

	v1GoModPath := filepath.Join(v1Dir, "go.mod")
	v2GoModPath := filepath.Join(v2Dir, "go.mod")
	testutils.WriteGoMod(t, v1Dir, "github.com/example/ratelimit")
	testutils.WriteGoMod(t, v2Dir, "github.com/example/ratelimit")

	// Multiple candidates - should pick the one matching version
	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: v1Dir, GoModPath: v1GoModPath},
		{Name: "ratelimit", Version: "v2.0.0", Path: v2Dir, GoModPath: v2GoModPath},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v2.0.0")
}

func TestWriteManifestLockWithVersions_GomoduleVersionNormalization(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manifest with gomodule - version without 'v' prefix
	manifestContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@1.0.0
`
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, manifestContent)

	// Create policy directory with go.mod
	policyDir := filepath.Join(tmpDir, "policies", "ratelimit")
	testutils.CreateDir(t, policyDir)

	goModPath := filepath.Join(policyDir, "go.mod")
	testutils.WriteGoMod(t, policyDir, "github.com/example/ratelimit")

	// Discovered version has 'v' prefix
	discovered := []*types.DiscoveredPolicy{
		{
			Name:      "ratelimit",
			Version:   "v1.0.0", // Has 'v' prefix
			Path:      policyDir,
			GoModPath: goModPath,
		},
	}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "policy-manifest-lock.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v1.0.0")
}

func TestWriteManifestLockWithVersions_InvalidManifestYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid YAML manifest
	manifestPath := filepath.Join(tmpDir, "policy-manifest.yaml")
	testutils.WriteFile(t, manifestPath, "invalid: yaml: content: -")

	discovered := []*types.DiscoveredPolicy{}

	err := WriteManifestLockWithVersions(manifestPath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse manifest YAML")
}

func TestManifest_WriteToFile_InvalidPath(t *testing.T) {
	manifest := &Manifest{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies:       []PolicyInfo{},
	}

	// Try to write to a path that doesn't exist
	err := manifest.WriteToFile("/nonexistent/directory/manifest.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write manifest file")
}

func TestManifest_ToJSON_Success(t *testing.T) {
manifest := &Manifest{
BuildTimestamp: "2025-01-01T00:00:00Z",
BuilderVersion: "v2.0.0",
OutputDir:      "/output/dir",
Policies: []PolicyInfo{
{Name: "test", Version: "v1.0.0"},
},
}

jsonStr, err := manifest.ToJSON()
require.NoError(t, err)
assert.Contains(t, jsonStr, "v2.0.0")
assert.Contains(t, jsonStr, "test")
assert.Contains(t, jsonStr, "v1.0.0")

// Verify it's valid JSON
var parsed Manifest
err = json.Unmarshal([]byte(jsonStr), &parsed)
require.NoError(t, err)
assert.Equal(t, manifest.BuilderVersion, parsed.BuilderVersion)
}
