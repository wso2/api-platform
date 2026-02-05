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

package packaging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

func TestGenerateDockerfile_Success(t *testing.T) {
	tmpDir := t.TempDir()

	policies := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0"},
		{Name: "jwt-auth", Version: "v0.1.0"},
	}

	err := GenerateDockerfile(tmpDir, policies, "v1.0.0")

	require.NoError(t, err)

	// Verify Dockerfile was created
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	_, err = os.Stat(dockerfilePath)
	assert.NoError(t, err)

	// Verify content
	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "FROM")
}

func TestGenerateDockerfile_EmptyPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	policies := []*types.DiscoveredPolicy{}

	err := GenerateDockerfile(tmpDir, policies, "v1.0.0")

	require.NoError(t, err)

	// Dockerfile should still be created
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	_, err = os.Stat(dockerfilePath)
	assert.NoError(t, err)
}

func TestGenerateDockerfile_NilPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	err := GenerateDockerfile(tmpDir, nil, "v1.0.0")

	require.NoError(t, err)

	// Dockerfile should still be created
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	_, err = os.Stat(dockerfilePath)
	assert.NoError(t, err)
}

func TestGenerateDockerfile_CreatesBuildMD(t *testing.T) {
	tmpDir := t.TempDir()

	policies := []*types.DiscoveredPolicy{
		{Name: "cors", Version: "v2.0.0"},
	}

	err := GenerateDockerfile(tmpDir, policies, "v1.0.0")

	require.NoError(t, err)

	// Verify BUILD.md was created
	buildMDPath := filepath.Join(tmpDir, "BUILD.md")
	_, err = os.Stat(buildMDPath)
	assert.NoError(t, err)

	// Verify content contains policy info
	content, err := os.ReadFile(buildMDPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "cors")
	assert.Contains(t, string(content), "v2.0.0")
	assert.Contains(t, string(content), "docker build")
}

func TestGenerateDockerfile_InvalidOutputDir(t *testing.T) {
	// Use a file path to deterministically fail directory creation
	invalidPath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(invalidPath, []byte("x"), 0600))

	policies := []*types.DiscoveredPolicy{
		{Name: "test", Version: "v1.0.0"},
	}

	err := GenerateDockerfile(invalidPath, policies, "v1.0.0")

	assert.Error(t, err)
}

func TestGenerateDockerfile_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()

	policies := []*types.DiscoveredPolicy{
		{Name: "policy1", Version: "v1.0.0"},
		{Name: "policy2", Version: "v2.0.0"},
		{Name: "policy3", Version: "v3.0.0"},
		{Name: "policy4", Version: "v4.0.0"},
		{Name: "policy5", Version: "v5.0.0"},
	}

	err := GenerateDockerfile(tmpDir, policies, "v2.0.0")

	require.NoError(t, err)

	// Verify BUILD.md contains all policies
	buildMDPath := filepath.Join(tmpDir, "BUILD.md")
	content, err := os.ReadFile(buildMDPath)
	require.NoError(t, err)

	for _, p := range policies {
		assert.Contains(t, string(content), p.Name)
		assert.Contains(t, string(content), p.Version)
	}
}

func TestGenerateDockerfile_VersionInOutput(t *testing.T) {
	tmpDir := t.TempDir()

	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.2.3"},
	}

	err := GenerateDockerfile(tmpDir, policies, "builder-v3.0.0")

	require.NoError(t, err)

	// Check Dockerfile contains builder version in labels
	dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	contentStr := string(content)

	// Should contain the builder version
	assert.Contains(t, contentStr, "builder-v3.0.0")
}

func TestGenerateBuildInstructions_Success(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC),
		Policies: []types.PolicyInfo{
			{Name: "ratelimit", Version: "v1.0.0"},
			{Name: "jwt-auth", Version: "v0.1.0"},
		},
	}

	err := generateBuildInstructions(tmpDir, metadata)

	require.NoError(t, err)

	// Verify file exists
	readmePath := filepath.Join(tmpDir, "BUILD.md")
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)

	contentStr := string(content)

	// Verify content structure
	assert.Contains(t, contentStr, "# Policy Engine Runtime Build Instructions")
	assert.Contains(t, contentStr, "1. ratelimit v1.0.0")
	assert.Contains(t, contentStr, "2. jwt-auth v0.1.0")
	assert.Contains(t, contentStr, "2025-06-15T10:30:00Z")
	assert.Contains(t, contentStr, "docker build")
	assert.Contains(t, contentStr, "docker run")
}

func TestGenerateBuildInstructions_EmptyPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Now(),
		Policies:       []types.PolicyInfo{},
	}

	err := generateBuildInstructions(tmpDir, metadata)

	require.NoError(t, err)

	readmePath := filepath.Join(tmpDir, "BUILD.md")
	content, err := os.ReadFile(readmePath)
	require.NoError(t, err)

	// Should still have instructions even with no policies
	assert.Contains(t, string(content), "docker build")
}

func TestGenerateBuildInstructions_InvalidPath(t *testing.T) {
	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Now(),
		Policies:       []types.PolicyInfo{},
	}

	// Use a file path to deterministically fail directory creation
	invalidPath := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(invalidPath, []byte("x"), 0600))

	err := generateBuildInstructions(invalidPath, metadata)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write BUILD.md")
}

func TestGenerateBuildInstructions_ManyPolicies(t *testing.T) {
	tmpDir := t.TempDir()

	policies := make([]types.PolicyInfo, 10)
	for i := 0; i < 10; i++ {
		policies[i] = types.PolicyInfo{
			Name:    "policy" + string(rune('A'+i)),
			Version: "v1.0.0",
		}
	}

	metadata := &types.PackagingMetadata{
		BuildTimestamp: time.Now(),
		Policies:       policies,
	}

	err := generateBuildInstructions(tmpDir, metadata)

	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "BUILD.md"))
	require.NoError(t, err)

	// Verify all policies are numbered with full entry format
	for i, p := range policies {
		assert.Contains(t, string(content), fmt.Sprintf("%d. %s %s", i+1, p.Name, p.Version))
	}
}

func TestGenerateDockerfile_PolicyMetadataTransformation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policies with additional fields that should be ignored
	policies := []*types.DiscoveredPolicy{
		{
			Name:         "test-policy",
			Version:      "v1.0.0",
			Path:         "/some/path",
			GoModulePath: "github.com/example/policy",
		},
	}

	err := GenerateDockerfile(tmpDir, policies, "v1.0.0")

	require.NoError(t, err)

	// Only name and version should be in BUILD.md
	content, err := os.ReadFile(filepath.Join(tmpDir, "BUILD.md"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "test-policy")
	assert.Contains(t, string(content), "v1.0.0")
	// Path shouldn't be exposed in build instructions
	assert.NotContains(t, string(content), "/some/path")
}

func TestGenerateDockerfile_SpecialCharactersInPolicyName(t *testing.T) {
	tmpDir := t.TempDir()

	policies := []*types.DiscoveredPolicy{
		{Name: "my-special_policy.v2", Version: "v1.0.0-beta+build.123"},
	}

	err := GenerateDockerfile(tmpDir, policies, "v1.0.0")

	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "BUILD.md"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "my-special_policy.v2")
	assert.Contains(t, string(content), "v1.0.0-beta+build.123")
}

func TestGenerateDockerfile_BuildTimestampIsRecent(t *testing.T) {
	tmpDir := t.TempDir()

	beforeTime := time.Now().UTC().Add(-1 * time.Second)

	err := GenerateDockerfile(tmpDir, nil, "v1.0.0")

	require.NoError(t, err)

	afterTime := time.Now().UTC().Add(1 * time.Second)

	content, err := os.ReadFile(filepath.Join(tmpDir, "BUILD.md"))
	require.NoError(t, err)

	// Extract timestamp from content
	contentStr := string(content)
	timestampIdx := strings.Index(contentStr, "Build timestamp: ")
	require.NotEqual(t, -1, timestampIdx, "Build timestamp not found")
	timestampStart := timestampIdx + len("Build timestamp: ")
	timestampEnd := strings.Index(contentStr[timestampStart:], "\n")
	require.NotEqual(t, -1, timestampEnd, "Build timestamp line not found")
	timestampStr := contentStr[timestampStart : timestampStart+timestampEnd]
	parsedTime, err := time.Parse(time.RFC3339, timestampStr)
	require.NoError(t, err, "Invalid build timestamp")
	assert.True(t, parsedTime.After(beforeTime) || parsedTime.Equal(beforeTime))
	assert.True(t, parsedTime.Before(afterTime) || parsedTime.Equal(afterTime))
}

func TestGenerateDockerfile_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	err := GenerateDockerfile(tmpDir, nil, "v1.0.0")

	require.NoError(t, err)

	// Check Dockerfile permissions
	info, err := os.Stat(filepath.Join(tmpDir, "Dockerfile"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())

	// Check BUILD.md permissions
	info, err = os.Stat(filepath.Join(tmpDir, "BUILD.md"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}
