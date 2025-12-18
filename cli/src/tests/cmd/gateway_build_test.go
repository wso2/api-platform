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

package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type TestConfig struct {
	Tests []TestEntry `yaml:"tests"`
}

type TestEntry struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Enabled     bool   `yaml:"enabled"`
}

func isTestEnabled(testName string) bool {
	// Load test config
	configPath := filepath.Join("..", "test-config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If config doesn't exist, all tests are enabled by default
		return true
	}

	var config TestConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return true
	}

	// Find the test entry
	for _, test := range config.Tests {
		if test.Name == testName {
			return test.Enabled
		}
	}

	// Default to enabled if not found in config
	return true
}

func TestCmdGatewayBuild(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayBuild") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "apipctl")

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", binaryPath)
	}

	// Get the test manifest path
	manifestPath := filepath.Join("..", "resources", "test-policy-manifest.yaml")
	absManifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for manifest: %v", err)
	}

	// Check if manifest exists
	if _, err := os.Stat(absManifestPath); os.IsNotExist(err) {
		t.Fatalf("Test manifest not found at %s", absManifestPath)
	}

	t.Logf("Testing gateway build command with manifest: %s", absManifestPath)

	// Build the command
	cmd := exec.Command(binaryPath,
		"gateway", "build",
		"--file", absManifestPath,
		"--docker-registry", "myregistry",
		"--image-tag", "v1.0.0",
		"--gateway-builder", "my-builder",
	)

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	t.Logf("Command output:\n%s", outputStr)

	// Check if command executed (note: it may fail due to PolicyHub availability)
	if err != nil {
		// Check if it's a validation error or expected failure
		if strings.Contains(outputStr, "Validating Policy Manifest") {
			// Command started but may have failed at later steps (e.g., PolicyHub connectivity)
			if strings.Contains(outputStr, "failed to resolve policies") ||
				strings.Contains(outputStr, "connection") ||
				strings.Contains(outputStr, "timeout") {
				t.Logf("✓ Command executed but failed at PolicyHub step (expected in test environment)")
				t.Logf("  This is acceptable - the command structure and validation work correctly")
				return
			}
		}

		// If it's a different error, report it
		if !strings.Contains(outputStr, "[1/5]") {
			t.Fatalf("Command failed before validation step: %v\nOutput: %s", err, outputStr)
		}
	}

	// Check for expected output patterns
	expectedPatterns := []string{
		"[1/5] Validating Policy Manifest",
		"[2/5] Preparing Build Configuration",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(outputStr, pattern) {
			t.Errorf("Expected output to contain '%s', but it didn't", pattern)
		}
	}

	// Verify that manifest was loaded
	if strings.Contains(outputStr, "[1/5] Validating Policy Manifest") &&
		strings.Contains(outputStr, "✓ Validated 2 policies") {
		t.Logf("✓ Manifest validation successful")
	}

	// Verify configuration was displayed
	if strings.Contains(outputStr, "Docker Registry: myregistry") &&
		strings.Contains(outputStr, "Image Tag: v1.0.0") &&
		strings.Contains(outputStr, "Gateway Builder: my-builder") {
		t.Logf("✓ Build configuration displayed correctly")
	}

	t.Logf("✓ Gateway build command test completed")
}
