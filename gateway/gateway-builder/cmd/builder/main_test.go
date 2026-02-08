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

package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-builder/internal/docker"
	"github.com/wso2/api-platform/gateway/gateway-builder/internal/manifest"
	"github.com/wso2/api-platform/gateway/gateway-builder/pkg/types"
)

// TestInitLogger tests the logger initialization function
func TestInitLogger_JSONFormat(t *testing.T) {
	// Capture that it doesn't panic with json format
	assert.NotPanics(t, func() {
		initLogger("json", "info")
	})
}

func TestInitLogger_TextFormat(t *testing.T) {
	// Capture that it doesn't panic with text format
	assert.NotPanics(t, func() {
		initLogger("text", "info")
	})
}

func TestInitLogger_DebugLevel(t *testing.T) {
	assert.NotPanics(t, func() {
		initLogger("json", "debug")
	})
}

func TestInitLogger_WarnLevel(t *testing.T) {
	assert.NotPanics(t, func() {
		initLogger("json", "warn")
	})
}

func TestInitLogger_ErrorLevel(t *testing.T) {
	assert.NotPanics(t, func() {
		initLogger("json", "error")
	})
}

func TestInitLogger_UnknownLevel(t *testing.T) {
	// Unknown level should default to info without panicking
	assert.NotPanics(t, func() {
		initLogger("json", "unknown")
	})
}

// TestPrintDockerfileGenerationSummary tests the summary printing function
func TestPrintDockerfileGenerationSummary(t *testing.T) {
	// Create mock docker result
	result := &docker.GenerateResult{
		Success:                       true,
		PolicyEngineDockerfile:        "/tmp/output/Dockerfile.policy-engine",
		GatewayControllerDockerfile:   "/tmp/output/Dockerfile.gateway-controller",
		RouterDockerfile:              "/tmp/output/Dockerfile.router",
	}

	// Create mock policies
	policies := []*types.DiscoveredPolicy{
		{Name: "test-policy", Version: "v1.0.0"},
	}

	// Create mock manifest
	buildManifest := manifest.CreateManifest("test-version", policies, "/tmp/output")

	// Capture stdout
	output := captureOutput(func() {
		printDockerfileGenerationSummary(result, buildManifest, "/tmp/output/build-manifest.json")
	})

	// Verify expected content in output
	assert.Contains(t, output, "Gateway Dockerfiles Generated")
	assert.Contains(t, output, "Policy Engine")
	assert.Contains(t, output, "Gateway Controller")
	assert.Contains(t, output, "Router")
	assert.Contains(t, output, "build-manifest.json")
}

func TestPrintDockerfileGenerationSummary_WithMultiplePolicies(t *testing.T) {
	result := &docker.GenerateResult{
		Success:                       true,
		PolicyEngineDockerfile:        "/output/Dockerfile.policy-engine",
		GatewayControllerDockerfile:   "/output/Dockerfile.gateway-controller",
		RouterDockerfile:              "/output/Dockerfile.router",
	}

	policies := []*types.DiscoveredPolicy{
		{Name: "auth-policy", Version: "v1.0.0"},
		{Name: "rate-limit", Version: "v2.0.0"},
		{Name: "cors", Version: "v1.5.0"},
	}

	buildManifest := manifest.CreateManifest("1.0.0", policies, "/output")

	output := captureOutput(func() {
		printDockerfileGenerationSummary(result, buildManifest, "/output/manifest.json")
	})

	// Should contain policy count info in JSON manifest
	assert.Contains(t, output, "auth-policy")
	assert.Contains(t, output, "rate-limit")
	assert.Contains(t, output, "cors")
}

// TestVersionVariables tests that version variables are set
func TestVersionVariables(t *testing.T) {
	// These are set at compile time, but we can verify they have default values
	assert.NotEmpty(t, Version, "Version should have a default value")
	assert.NotEmpty(t, GitCommit, "GitCommit should have a default value")
	assert.NotEmpty(t, BuildDate, "BuildDate should have a default value")
}

// TestDefaultConstants tests the default constant values
func TestDefaultConstants(t *testing.T) {
	assert.Equal(t, "policy-manifest.yaml", DefaultManifestFile)
	assert.Equal(t, "system-policy-manifest-lock.yaml", DefaultSystemPolicyManifestLockFile)
	assert.Equal(t, "output", DefaultOutputDir)
	assert.Equal(t, "/api-platform/gateway/gateway-runtime/policy-engine", DefaultPolicyEngineSrc)
}

// captureOutput is a helper to capture stdout during test execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}
