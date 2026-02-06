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

package docker

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

// ==== PolicyEngineGenerator tests ====

func TestNewPolicyEngineGenerator(t *testing.T) {
	gen := NewPolicyEngineGenerator("/output", "/bin/policy-engine", "v1.0.0")

	assert.Equal(t, "/output", gen.outputDir)
	assert.Equal(t, "/bin/policy-engine", gen.policyEngineBin)
	assert.Equal(t, "v1.0.0", gen.builderVersion)
}

func TestPolicyEngineGenerator_Generate_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake binary
	binPath := filepath.Join(tmpDir, "policy-engine-bin")
	testutils.WriteFile(t, binPath, "#!/bin/bash\necho hello")

	outputDir := filepath.Join(tmpDir, "output")

	gen := NewPolicyEngineGenerator(outputDir, binPath, "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)
	assert.Contains(t, dockerfilePath, "Dockerfile")

	// Verify binary was copied
	copiedBin := filepath.Join(outputDir, "policy-engine", "policy-engine")
	assert.FileExists(t, copiedBin)

	// Verify Dockerfile contains expected content
	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "v1.0.0") // builder version in labels
}

func TestPolicyEngineGenerator_Generate_MissingBinary(t *testing.T) {
	tmpDir := t.TempDir()

	gen := NewPolicyEngineGenerator(
		filepath.Join(tmpDir, "output"),
		"/nonexistent/binary",
		"v1.0.0",
	)

	_, err := gen.Generate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy binary")
}

func TestPolicyEngineGenerator_Generate_OutputDirCreationFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file where the directory should be to force mkdir failure
	blockingFile := filepath.Join(tmpDir, "output", "policy-engine")
	testutils.WriteFile(t, blockingFile, "blocking")

	binPath := filepath.Join(tmpDir, "policy-engine-bin")
	testutils.WriteFile(t, binPath, "binary content")

	gen := NewPolicyEngineGenerator(
		filepath.Join(tmpDir, "output"),
		binPath,
		"v1.0.0",
	)

	_, err := gen.Generate()

	assert.Error(t, err)
}

// ==== GatewayControllerGenerator tests ====

func TestNewGatewayControllerGenerator(t *testing.T) {
	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0"},
	}

	gen := NewGatewayControllerGenerator("/output", "base:image", policies, "v1.0.0")

	assert.Equal(t, "/output", gen.outputDir)
	assert.Equal(t, "base:image", gen.baseImage)
	assert.Len(t, gen.policies, 1)
	assert.Equal(t, "v1.0.0", gen.builderVersion)
}

func TestGatewayControllerGenerator_Generate_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory with policy.yaml
	policyDir := filepath.Join(tmpDir, "policies", "test-policy")
	testutils.CreateDir(t, policyDir)

	policyYAMLPath := filepath.Join(policyDir, "policy.yaml")
	testutils.CreatePolicyYAML(t, policyDir, "test-policy", "v1.0.0")

	outputDir := filepath.Join(tmpDir, "output")

	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0", YAMLPath: policyYAMLPath},
	}

	gen := NewGatewayControllerGenerator(outputDir, "base:image", policies, "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)

	// Verify policy file was copied with correct name
	copiedPolicy := filepath.Join(outputDir, "gateway-controller", "policies", "test-policy-v1.0.0.yaml")
	assert.FileExists(t, copiedPolicy)

	// Verify Dockerfile contains expected content
	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "base:image")
}

func TestGatewayControllerGenerator_Generate_NoPolicies(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	gen := NewGatewayControllerGenerator(outputDir, "base:image", nil, "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)
}

func TestGatewayControllerGenerator_Generate_MissingPolicyYAML(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0", YAMLPath: "/nonexistent/policy.yaml"},
	}

	gen := NewGatewayControllerGenerator(outputDir, "base:image", policies, "v1.0.0")

	_, err := gen.Generate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to copy policy files")
}

func TestGatewayControllerGenerator_Generate_OutputDirCreationFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file where the directory should be to force mkdir failure
	blockingFile := filepath.Join(tmpDir, "output", "gateway-controller")
	testutils.WriteFile(t, blockingFile, "blocking")

	policies := []*types.DiscoveredPolicy{}

	gen := NewGatewayControllerGenerator(
		filepath.Join(tmpDir, "output"),
		"base:image",
		policies,
		"v1.0.0",
	)

	_, err := gen.Generate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gateway-controller directory")
}

// ==== RouterGenerator tests ====

func TestNewRouterGenerator(t *testing.T) {
	gen := NewRouterGenerator("/output", "router:base", "v1.0.0")

	assert.Equal(t, "/output", gen.outputDir)
	assert.Equal(t, "router:base", gen.baseImage)
	assert.Equal(t, "v1.0.0", gen.builderVersion)
}

func TestRouterGenerator_Generate_Success(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	gen := NewRouterGenerator(outputDir, "envoy:v1.30.0", "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)
	assert.Contains(t, dockerfilePath, "Dockerfile")

	// Verify Dockerfile contains expected content
	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "envoy:v1.30.0")
	assert.Contains(t, string(content), "v1.0.0") // builder version
	assert.Contains(t, string(content), "router") // component label
}

func TestRouterGenerator_Generate_OutputDirCreationFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file where the directory should be to force mkdir failure
	blockingFile := filepath.Join(tmpDir, "output", "router")
	testutils.WriteFile(t, blockingFile, "blocking")

	gen := NewRouterGenerator(
		filepath.Join(tmpDir, "output"),
		"envoy:v1.30.0",
		"v1.0.0",
	)

	_, err := gen.Generate()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create router directory")
}

// ==== DockerfileGenerator tests ====

func TestDockerfileGenerator_GenerateAll_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake binary
	binPath := filepath.Join(tmpDir, "policy-engine-bin")
	testutils.WriteFile(t, binPath, "#!/bin/bash\necho hello")

	// Create policy directory with policy.yaml
	policyDir := filepath.Join(tmpDir, "policies", "test-policy")
	testutils.CreateDir(t, policyDir)

	policyYAMLPath := filepath.Join(policyDir, "policy.yaml")
	testutils.CreatePolicyYAML(t, policyDir, "test-policy", "v1.0.0")

	outputDir := filepath.Join(tmpDir, "output")

	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0", YAMLPath: policyYAMLPath},
	}

	gen := &DockerfileGenerator{
		PolicyEngineBin:            binPath,
		Policies:                   policies,
		OutputDir:                  outputDir,
		GatewayControllerBaseImage: "gc:base",
		RouterBaseImage:            "router:base",
		BuilderVersion:             "v1.0.0",
	}

	result, err := gen.GenerateAll()

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, result.Errors)
	assert.FileExists(t, result.PolicyEngineDockerfile)
	assert.FileExists(t, result.GatewayControllerDockerfile)
	assert.FileExists(t, result.RouterDockerfile)
}

func TestDockerfileGenerator_GenerateAll_PolicyEngineFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Create policy directory with policy.yaml
	policyDir := filepath.Join(tmpDir, "policies", "test-policy")
	testutils.CreateDir(t, policyDir)

	policyYAMLPath := filepath.Join(policyDir, "policy.yaml")
	testutils.CreatePolicyYAML(t, policyDir, "test-policy", "v1.0.0")

	outputDir := filepath.Join(tmpDir, "output")

	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0", YAMLPath: policyYAMLPath},
	}

	gen := &DockerfileGenerator{
		PolicyEngineBin:            "/nonexistent/binary",
		Policies:                   policies,
		OutputDir:                  outputDir,
		GatewayControllerBaseImage: "gc:base",
		RouterBaseImage:            "router:base",
		BuilderVersion:             "v1.0.0",
	}

	result, err := gen.GenerateAll()

	require.NoError(t, err) // GenerateAll doesn't return error, just sets Success=false
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Error(), "policy engine generation failed")
}

func TestDockerfileGenerator_GenerateAll_GatewayControllerFails(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake binary
	binPath := filepath.Join(tmpDir, "policy-engine-bin")
	testutils.WriteFile(t, binPath, "#!/bin/bash\necho hello")

	outputDir := filepath.Join(tmpDir, "output")

	// Policy with non-existent YAML path
	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0", YAMLPath: "/nonexistent/policy.yaml"},
	}

	gen := &DockerfileGenerator{
		PolicyEngineBin:            binPath,
		Policies:                   policies,
		OutputDir:                  outputDir,
		GatewayControllerBaseImage: "gc:base",
		RouterBaseImage:            "router:base",
		BuilderVersion:             "v1.0.0",
	}

	result, err := gen.GenerateAll()

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Errors)

	hasGCError := false
	for _, e := range result.Errors {
		if strings.Contains(e.Error(), "gateway controller generation failed") {
			hasGCError = true
			break
		}
	}
	assert.True(t, hasGCError, "expected error containing 'gateway controller generation failed'")
}

// ==== Additional tests to improve coverage ====

func TestPolicyEngineGenerator_Generate_DirectoryCreationSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake binary
	binPath := filepath.Join(tmpDir, "policy-engine-bin")
	testutils.WriteFile(t, binPath, "#!/bin/bash\necho hello")

	// Use nested directory to exercise directory creation
	outputDir := filepath.Join(tmpDir, "deep", "nested", "output")

	gen := NewPolicyEngineGenerator(outputDir, binPath, "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)
}

func TestGatewayControllerGenerator_Generate_EmptyPolicies(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "output")

	// No policies - should still create Dockerfile
	gen := NewGatewayControllerGenerator(outputDir, "base:image", nil, "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)

	// Verify Dockerfile contains base image
	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "base:image")
}

func TestGatewayControllerGenerator_Generate_MultiplePolicies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple policy directories with policy.yaml
	policy1Dir := filepath.Join(tmpDir, "policies", "ratelimit")
	policy2Dir := filepath.Join(tmpDir, "policies", "jwt-auth")
	testutils.CreateDir(t, policy1Dir)
	testutils.CreateDir(t, policy2Dir)

	policy1YAML := filepath.Join(policy1Dir, "policy.yaml")
	policy2YAML := filepath.Join(policy2Dir, "policy.yaml")
	testutils.CreatePolicyYAML(t, policy1Dir, "ratelimit", "v1.0.0")
	testutils.CreatePolicyYAML(t, policy2Dir, "jwt-auth", "v0.1.0")

	outputDir := filepath.Join(tmpDir, "output")

	policies := []*types.DiscoveredPolicy{
		{Name: "ratelimit", Version: "v1.0.0", YAMLPath: policy1YAML},
		{Name: "jwt-auth", Version: "v0.1.0", YAMLPath: policy2YAML},
	}

	gen := NewGatewayControllerGenerator(outputDir, "base:image", policies, "v1.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)

	// Verify both policy files were copied
	copiedPolicy1 := filepath.Join(outputDir, "gateway-controller", "policies", "ratelimit-v1.0.0.yaml")
	copiedPolicy2 := filepath.Join(outputDir, "gateway-controller", "policies", "jwt-auth-v0.1.0.yaml")
	assert.FileExists(t, copiedPolicy1)
	assert.FileExists(t, copiedPolicy2)
}

func TestRouterGenerator_Generate_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Nested output directory
	outputDir := filepath.Join(tmpDir, "deep", "nested", "output")

	gen := NewRouterGenerator(outputDir, "envoy:latest", "v2.0.0")

	dockerfilePath, err := gen.Generate()

	require.NoError(t, err)
	assert.FileExists(t, dockerfilePath)

	content, err := os.ReadFile(dockerfilePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "envoy:latest")
	assert.Contains(t, string(content), "v2.0.0")
}

func TestDockerfileGenerator_GenerateAll_AllSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake binary
	binPath := filepath.Join(tmpDir, "policy-engine-bin")
	testutils.WriteFile(t, binPath, "#!/bin/bash\necho hello")

	outputDir := filepath.Join(tmpDir, "output")

	// No policies - simplest success case
	gen := &DockerfileGenerator{
		PolicyEngineBin:            binPath,
		Policies:                   nil,
		OutputDir:                  outputDir,
		GatewayControllerBaseImage: "gc:base",
		RouterBaseImage:            "router:base",
		BuilderVersion:             "v1.0.0",
	}

	result, err := gen.GenerateAll()

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, result.Errors)
	assert.Equal(t, outputDir, result.OutputDir)
	assert.Equal(t, binPath, result.PolicyEngineBin)
}
