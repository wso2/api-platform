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

package buildfile

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

func TestCreateBuildInfo_EmptyPolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{}

	info := CreateBuildInfo("v1.0.0", policies, "/output")

	assert.Equal(t, "v1.0.0", info.BuilderVersion)
	assert.Equal(t, "/output", info.OutputDir)
	assert.Empty(t, info.Policies)
	assert.NotEmpty(t, info.BuildTimestamp)
}

func TestCreateBuildInfo_SinglePolicy(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    "/policies/ratelimit",
		},
	}

	info := CreateBuildInfo("v2.0.0", policies, "/build/output")

	assert.Equal(t, "v2.0.0", info.BuilderVersion)
	assert.Equal(t, "/build/output", info.OutputDir)
	assert.Len(t, info.Policies, 1)
	assert.Equal(t, "ratelimit", info.Policies[0].Name)
	assert.Equal(t, "v1.0.0", info.Policies[0].Version)
}

func TestCreateBuildInfo_MultiplePolicies(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0"},
		{Name: "jwt-auth", Version: "v0.1.0"},
		{Name: "cors", Version: "v2.0.0"},
	}

	info := CreateBuildInfo("v1.5.0", policies, "/out")

	assert.Len(t, info.Policies, 3)
	assert.Equal(t, "ratelimit", info.Policies[0].Name)
	assert.Equal(t, "jwt-auth", info.Policies[1].Name)
	assert.Equal(t, "cors", info.Policies[2].Name)
}

func TestBuildInfo_ToJSON_EmptyPolicies(t *testing.T) {
	info := &BuildInfo{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies:       []PolicyInfo{},
	}

	jsonStr, err := info.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonStr, `"builderVersion": "v1.0.0"`)
	assert.Contains(t, jsonStr, `"outputDir": "/output"`)
	assert.Contains(t, jsonStr, `"policies": []`)
}

func TestBuildInfo_ToJSON_WithPolicies(t *testing.T) {
	info := &BuildInfo{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies: []PolicyInfo{
			{Name: "ratelimit", Version: "v1.0.0"},
			{Name: "jwt-auth", Version: "v0.1.0"},
		},
	}

	jsonStr, err := info.ToJSON()
	require.NoError(t, err)

	// Verify it's valid JSON
	var parsed map[string]any
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "v1.0.0", parsed["builderVersion"])
	policies := parsed["policies"].([]any)
	assert.Len(t, policies, 2)
}

func TestBuildInfo_WriteToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "build-info.json")

	info := &BuildInfo{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies: []PolicyInfo{
			{Name: "test-policy", Version: "v1.0.0"},
		},
	}

	err := info.WriteToFile(filePath)
	require.NoError(t, err)

	// Verify file exists and contains expected content
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-policy")
	assert.Contains(t, string(content), "v1.0.0")
}

func TestBuildInfo_WriteToFile_DirectoryNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent-dir", "build-info.json")

	info := &BuildInfo{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies:       []PolicyInfo{},
	}

	err := info.WriteToFile(filePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write build info file")
}

func TestWriteBuildManifestWithVersions_Success(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	policyDir := filepath.Join(tmpDir, "policies", "ratelimit")
	testutils.CreateDir(t, policyDir)

	discovered := []*types.DiscoveredPolicy{
		{
			Name:    "ratelimit",
			Version: "v1.0.0",
			Path:    policyDir,
		},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ratelimit")
	assert.Contains(t, string(content), "v1.0.0")
}

func TestWriteBuildManifestWithVersions_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	buildFilePath := filepath.Join(tmpDir, "nonexistent.yaml")

	discovered := []*types.DiscoveredPolicy{}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read build file")
}

func TestWriteBuildManifestWithVersions_EmptyBuildFile(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies: []
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	_, err = os.Stat(lockPath)
	assert.NoError(t, err)
}

func TestWriteBuildManifestWithVersions_PolicyNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: unknown-policy
    filePath: ./policies/unknown
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine version for policy")
}

func TestWriteBuildManifestWithVersions_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit
  - name: jwt-auth
    filePath: ./policies/jwt-auth
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	ratelimitDir := filepath.Join(tmpDir, "policies", "ratelimit")
	jwtAuthDir := filepath.Join(tmpDir, "policies", "jwt-auth")
	testutils.CreateDir(t, ratelimitDir)
	testutils.CreateDir(t, jwtAuthDir)

	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: ratelimitDir},
		{Name: "jwt-auth", Version: "v0.1.0", Path: jwtAuthDir},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ratelimit")
	assert.Contains(t, string(content), "jwt-auth")
	assert.Contains(t, string(content), "v1.0.0")
	assert.Contains(t, string(content), "v0.1.0")
}

func TestWriteBuildManifestWithVersions_MultipleCandidatesWithFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    filePath: ./policies/ratelimit-v2
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	v1Dir := filepath.Join(tmpDir, "policies", "ratelimit-v1")
	v2Dir := filepath.Join(tmpDir, "policies", "ratelimit-v2")
	testutils.CreateDir(t, v1Dir)
	testutils.CreateDir(t, v2Dir)

	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: v1Dir},
		{Name: "ratelimit", Version: "v2.0.0", Path: v2Dir},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v2.0.0")
	assert.NotContains(t, string(content), "v1.0.0")
}

func TestWriteBuildManifestWithVersions_GomoduleWithVersion(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@v1.0.0
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

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

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "ratelimit")
	assert.Contains(t, string(content), "v1.0.0")
}

func TestWriteBuildManifestWithVersions_GomoduleWithoutVersion(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

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

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v2.0.0")
}

func TestWriteBuildManifestWithVersions_GomoduleNoMatch(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@v1.0.0
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine version for policy")
}

func TestWriteBuildManifestWithVersions_GomoduleMultipleCandidates(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@v2.0.0
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	v1Dir := filepath.Join(tmpDir, "policies", "ratelimit-v1")
	v2Dir := filepath.Join(tmpDir, "policies", "ratelimit-v2")
	testutils.CreateDir(t, v1Dir)
	testutils.CreateDir(t, v2Dir)

	v1GoModPath := filepath.Join(v1Dir, "go.mod")
	v2GoModPath := filepath.Join(v2Dir, "go.mod")
	testutils.WriteGoMod(t, v1Dir, "github.com/example/ratelimit")
	testutils.WriteGoMod(t, v2Dir, "github.com/example/ratelimit")

	discovered := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", Path: v1Dir, GoModPath: v1GoModPath},
		{Name: "ratelimit", Version: "v2.0.0", Path: v2Dir, GoModPath: v2GoModPath},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v2.0.0")
}

func TestWriteBuildManifestWithVersions_GomoduleVersionNormalization(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: ratelimit
    gomodule: github.com/example/ratelimit@1.0.0
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

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

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v1.0.0")
}

func TestWriteBuildManifestWithVersions_PipPackageUsesResolvedSpec(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `
version: "1.0"
policies:
  - name: prompt-compressor
    pipPackage: "prompt-compressor~=0.0"
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{
		{
			Name:         "prompt-compressor",
			Version:      "v0.1.0",
			Runtime:      "python",
			IsPipPackage: true,
			PipSpec:      "prompt-compressor==0.1.0",
		},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), `pipPackage: prompt-compressor==0.1.0`)
	assert.NotContains(t, string(content), "prompt-compressor~=0.0")
}

func TestWriteBuildManifestWithVersions_PipPackageMultipleCandidatesUseOriginalSpec(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: prompt-compressor
    pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v1
  - name: prompt-compressor
    pipPackage: github.com/wso2/gateway-controllers/policies/prompt-compressor@v2
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{
		{
			Name:            "prompt-compressor",
			Version:         "v1.0.0",
			Runtime:         "python",
			IsPipPackage:    true,
			OriginalPipSpec: "github.com/wso2/gateway-controllers/policies/prompt-compressor@v1",
			PipSpec:         "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1.0.0#subdirectory=policies/prompt-compressor",
		},
		{
			Name:            "prompt-compressor",
			Version:         "v2.0.8",
			Runtime:         "python",
			IsPipPackage:    true,
			OriginalPipSpec: "github.com/wso2/gateway-controllers/policies/prompt-compressor@v2",
			PipSpec:         "git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v2.0.8#subdirectory=policies/prompt-compressor",
		},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)

	lockContent := string(content)
	assert.Contains(t, lockContent, "version: v1.0.0")
	assert.Contains(t, lockContent, "version: v2.0.8")
	assert.Contains(t, lockContent, "pipPackage: git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1.0.0#subdirectory=policies/prompt-compressor")
	assert.Contains(t, lockContent, "pipPackage: git+https://github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v2.0.8#subdirectory=policies/prompt-compressor")
	assert.NotContains(t, lockContent, "@policies/prompt-compressor/v1#subdirectory")
	assert.NotContains(t, lockContent, "@policies/prompt-compressor/v2#subdirectory")
}

func TestWriteBuildManifestWithVersions_PipPackageFallbackSkipsNonPipCandidates(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: prompt-compressor
    pipPackage: "prompt-compressor~=1.0"
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	localPolicyDir := filepath.Join(tmpDir, "policies", "prompt-compressor")
	testutils.CreateDir(t, localPolicyDir)

	discovered := []*types.DiscoveredPolicy{
		{
			Name:            "prompt-compressor",
			Version:         "v9.9.9",
			Runtime:         "python",
			Path:            localPolicyDir,
			PythonSourceDir: localPolicyDir,
		},
		{
			Name:         "prompt-compressor",
			Version:      "v1.2.3",
			Runtime:      "python",
			IsPipPackage: true,
			PipSpec:      "prompt-compressor==1.2.3@https://private.pypi.org/simple/",
		},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)

	lockContent := string(content)
	assert.Contains(t, lockContent, "version: v1.2.3")
	assert.Contains(t, lockContent, "pipPackage: prompt-compressor==1.2.3@https://private.pypi.org/simple/")
	assert.NotContains(t, lockContent, "version: v9.9.9")
}

func TestWriteBuildManifestWithVersions_PipPackageRedactsIndexCredentials(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: prompt-compressor
    pipPackage: "prompt-compressor~=1.0"
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{
		{
			Name:         "prompt-compressor",
			Version:      "v1.2.3",
			Runtime:      "python",
			IsPipPackage: true,
			PipSpec:      "prompt-compressor==1.2.3@https://deploy-token:secret123@private.pypi.org/simple/",
		},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)

	lockContent := string(content)
	assert.Contains(t, lockContent, "version: v1.2.3")
	// Credentials must be redacted in the manifest
	assert.NotContains(t, lockContent, "secret123")
	assert.NotContains(t, lockContent, "deploy-token")
	assert.Contains(t, lockContent, "<redacted-credentials>@private.pypi.org/simple/")
}

func TestWriteBuildManifestWithVersions_PipPackageRedactsVCSCredentials(t *testing.T) {
	tmpDir := t.TempDir()

	buildFileContent := `version: "1.0"
policies:
  - name: prompt-compressor
    pipPackage: "github.com/wso2/gateway-controllers/policies/prompt-compressor@v1"
`
	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, buildFileContent)

	discovered := []*types.DiscoveredPolicy{
		{
			Name:            "prompt-compressor",
			Version:         "v1.0.0",
			Runtime:         "python",
			IsPipPackage:    true,
			OriginalPipSpec: "github.com/wso2/gateway-controllers/policies/prompt-compressor@v1",
			PipSpec:         "git+https://ghp_secrettoken@github.com/wso2/gateway-controllers.git@policies/prompt-compressor/v1.0.0#subdirectory=policies/prompt-compressor",
		},
	}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	require.NoError(t, err)

	lockPath := filepath.Join(tmpDir, "build-manifest.yaml")
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)

	lockContent := string(content)
	assert.Contains(t, lockContent, "version: v1.0.0")
	// VCS credentials must be redacted in the manifest
	assert.NotContains(t, lockContent, "ghp_secrettoken")
	assert.Contains(t, lockContent, "<redacted-credentials>@github.com/wso2/gateway-controllers.git")
	assert.Contains(t, lockContent, "#subdirectory=policies/prompt-compressor")
}

func TestWriteBuildManifestWithVersions_InvalidBuildFileYAML(t *testing.T) {
	tmpDir := t.TempDir()

	buildFilePath := filepath.Join(tmpDir, "build.yaml")
	testutils.WriteFile(t, buildFilePath, "invalid: yaml: content: -")

	discovered := []*types.DiscoveredPolicy{}

	err := WriteBuildManifestWithVersions(buildFilePath, discovered)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse build file YAML")
}

func TestBuildInfo_WriteToFile_InvalidPath(t *testing.T) {
	info := &BuildInfo{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v1.0.0",
		OutputDir:      "/output",
		Policies:       []PolicyInfo{},
	}

	err := info.WriteToFile("/nonexistent/directory/build-info.json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write build info file")
}

func TestBuildInfo_ToJSON_Success(t *testing.T) {
	info := &BuildInfo{
		BuildTimestamp: "2025-01-01T00:00:00Z",
		BuilderVersion: "v2.0.0",
		OutputDir:      "/output/dir",
		Policies: []PolicyInfo{
			{Name: "test", Version: "v1.0.0"},
		},
	}

	jsonStr, err := info.ToJSON()
	require.NoError(t, err)
	assert.Contains(t, jsonStr, "v2.0.0")
	assert.Contains(t, jsonStr, "test")
	assert.Contains(t, jsonStr, "v1.0.0")

	// Verify it's valid JSON
	var parsed BuildInfo
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)
	assert.Equal(t, info.BuilderVersion, parsed.BuilderVersion)
}
