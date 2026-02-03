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

package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	// Create the policy directory structure
	policyDir := filepath.Join(tmpDir, "policies", "ratelimit")
	err = os.MkdirAll(policyDir, 0755)
	require.NoError(t, err)

	// Create discovered policies
	discovered := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    policyDir,
		},
	}

	err = WriteManifestLockWithVersions(manifestPath, discovered)
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
	err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	discovered := []*types.DiscoveredPolicy{}

	err = WriteManifestLockWithVersions(manifestPath, discovered)
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
	err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	// Empty discovered policies
	discovered := []*types.DiscoveredPolicy{}

	err = WriteManifestLockWithVersions(manifestPath, discovered)
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
	err := os.WriteFile(manifestPath, []byte(manifestContent), 0644)
	require.NoError(t, err)

	// Create the policy directories
	ratelimitDir := filepath.Join(tmpDir, "policies", "ratelimit")
	jwtAuthDir := filepath.Join(tmpDir, "policies", "jwt-auth")
	err = os.MkdirAll(ratelimitDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(jwtAuthDir, 0755)
	require.NoError(t, err)

	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: ratelimitDir},
		{Name: "jwt-auth", Version: "v0.1.0", Path: jwtAuthDir},
	}

	err = WriteManifestLockWithVersions(manifestPath, discovered)
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
