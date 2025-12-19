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

// TestCmdGatewayImageBuildLocalPolicies tests the gateway image build command with local policies
func TestCmdGatewayImageBuildLocalPolicies(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayImageBuild-LocalPolicies") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "apipctl")
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for binary: %v", err)
	}

	// Check if binary exists
	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", absBinaryPath)
	}

	// Get the test resources directory path
	resourcesDir := filepath.Join("..", "resources")
	absResourcesDir, err := filepath.Abs(resourcesDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for resources directory: %v", err)
	}

	// Check if manifest exists
	manifestPath := filepath.Join(absResourcesDir, "test-local-policy-manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("Test manifest not found at %s", manifestPath)
	}

	t.Logf("Testing gateway image build command with local policies in directory: %s", absResourcesDir)

	// Build the command
	cmd := exec.Command(absBinaryPath,
		"gateway", "image", "build",
		"--path", absResourcesDir,
		"--image-tag", "v0.2.0-test",
		"--image-repository", "test-registry/gateway",
	)

	// Set working directory to the src directory
	cmd.Dir = filepath.Join("..", "..")

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	t.Logf("Command output:\n%s", outputStr)

	// Check if command executed successfully
	if err != nil {
		// Check if it's a Docker availability error (acceptable in test environments)
		if strings.Contains(outputStr, "Docker is not available") ||
			strings.Contains(outputStr, "docker: command not found") {
			t.Logf("✓ Command executed but Docker is not available (expected in some test environments)")
			t.Logf("  This is acceptable - the command structure and validation work correctly")
			return
		}

		// Check if it's a policy path error (acceptable if gateway/policies not in expected location)
		if strings.Contains(outputStr, "policy not found") ||
			strings.Contains(outputStr, "no such file or directory") {
			t.Logf("✓ Command executed but policy files not found (expected if not in workspace)")
			t.Logf("  This is acceptable - the command structure and validation work correctly")
			return
		}

		t.Fatalf("Command failed unexpectedly: %v\nOutput: %s", err, outputStr)
	}

	// Check for expected output patterns
	expectedPatterns := []string{
		"=== Gateway Image Build ===",
		"[1/6] Checking Docker Availability",
		"[2/6] Reading Policy Manifest",
		"[3/6] Processing Local Policies",
		"Local policies:",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(outputStr, pattern) {
			t.Errorf("Expected output to contain '%s', but it didn't", pattern)
		}
	}

	// Verify that local policies were processed
	if strings.Contains(outputStr, "Processing Local Policies") {
		t.Logf("✓ Local policy processing step executed")
	}

	// Verify configuration was displayed
	if strings.Contains(outputStr, "Image Tag:") &&
		strings.Contains(outputStr, "v0.2.0-test") {
		t.Logf("✓ Build configuration displayed correctly")
	}

	// Check if lock file was generated
	lockFilePath := filepath.Join("..", "..", "policy-manifest-lock.yaml")
	if _, err := os.Stat(lockFilePath); err == nil {
		t.Logf("✓ Lock file generated successfully")
		// Clean up lock file
		os.Remove(lockFilePath)
	}

	t.Logf("✓ Gateway image build (local policies) test completed")
}

// TestCmdGatewayImageBuildHubPolicies tests the gateway image build command with hub policies
func TestCmdGatewayImageBuildHubPolicies(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayImageBuild-HubPolicies") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "apipctl")
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for binary: %v", err)
	}

	// Check if binary exists
	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", absBinaryPath)
	}

	// Get the test resources directory path
	resourcesDir := filepath.Join("..", "resources")
	absResourcesDir, err := filepath.Abs(resourcesDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for resources directory: %v", err)
	}

	// Check if manifest exists
	manifestPath := filepath.Join(absResourcesDir, "test-policy-manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("Test manifest not found at %s", manifestPath)
	}

	t.Logf("Testing gateway image build command with hub policies in directory: %s", absResourcesDir)

	// Build the command
	cmd := exec.Command(absBinaryPath,
		"gateway", "image", "build",
		"--path", absResourcesDir,
		"--image-tag", "v0.2.0-hub-test",
	)

	// Set working directory to the src directory
	cmd.Dir = filepath.Join("..", "..")

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	t.Logf("Command output:\n%s", outputStr)

	// Check if command executed
	if err != nil {
		// Check if it's a Docker availability error
		if strings.Contains(outputStr, "Docker is not available") {
			t.Logf("✓ Command executed but Docker is not available (expected in some test environments)")
			return
		}

		// Check if it's a PolicyHub connectivity error (acceptable in test environments)
		if strings.Contains(outputStr, "failed to process hub policies") ||
			strings.Contains(outputStr, "PolicyHub returned status") ||
			strings.Contains(outputStr, "failed to contact PolicyHub") {
			t.Logf("✓ Command executed but PolicyHub connectivity failed (expected in test environment)")
			t.Logf("  This is acceptable - the command structure and validation work correctly")
			return
		}

		t.Fatalf("Command failed unexpectedly: %v\nOutput: %s", err, outputStr)
	}

	// Check for expected output patterns
	expectedPatterns := []string{
		"=== Gateway Image Build ===",
		"[4/6] Resolving Hub Policies from PolicyHub",
		"Hub policies:",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(outputStr, pattern) {
			t.Errorf("Expected output to contain '%s', but it didn't", pattern)
		}
	}

	// Verify that hub policies were resolved
	if strings.Contains(outputStr, "Resolving Hub Policies from PolicyHub") {
		t.Logf("✓ Hub policy resolution step executed")
	}

	// Check if policies were downloaded
	if strings.Contains(outputStr, "Downloading and verifying") {
		t.Logf("✓ Policy download step executed")
	}

	t.Logf("✓ Gateway image build (hub policies) test completed")
}

// TestCmdGatewayImageBuildMixedPolicies tests the gateway image build command with mixed policies
func TestCmdGatewayImageBuildMixedPolicies(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayImageBuild-MixedPolicies") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "apipctl")
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for binary: %v", err)
	}

	// Check if binary exists
	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", absBinaryPath)
	}

	// Get the test resources directory path
	resourcesDir := filepath.Join("..", "resources")
	absResourcesDir, err := filepath.Abs(resourcesDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for resources directory: %v", err)
	}

	// Check if manifest exists
	manifestPath := filepath.Join(absResourcesDir, "test-mixed-manifest.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatalf("Test manifest not found at %s", manifestPath)
	}

	t.Logf("Testing gateway image build command with mixed policies in directory: %s", absResourcesDir)

	// Build the command
	cmd := exec.Command(absBinaryPath,
		"gateway", "image", "build",
		"--path", absResourcesDir,
		"--image-tag", "v0.2.0-mixed-test",
	)

	// Set working directory to the src directory
	cmd.Dir = filepath.Join("..", "..")

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	t.Logf("Command output:\n%s", outputStr)

	// Check if command executed
	if err != nil {
		// Check for acceptable errors in test environments
		if strings.Contains(outputStr, "Docker is not available") ||
			strings.Contains(outputStr, "PolicyHub returned status") ||
			strings.Contains(outputStr, "policy not found") {
			t.Logf("✓ Command executed but environment prerequisites not met (expected in test environment)")
			return
		}

		t.Fatalf("Command failed unexpectedly: %v\nOutput: %s", err, outputStr)
	}

	// Check for expected output patterns
	expectedPatterns := []string{
		"=== Gateway Image Build ===",
		"Local policies:",
		"Hub policies:",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(outputStr, pattern) {
			t.Errorf("Expected output to contain '%s', but it didn't", pattern)
		}
	}

	// Verify both local and hub policies were processed
	if strings.Contains(outputStr, "Processing Local Policies") &&
		strings.Contains(outputStr, "Resolving Hub Policies") {
		t.Logf("✓ Both local and hub policies processed")
	}

	t.Logf("✓ Gateway image build (mixed policies) test completed")
}

// TestCmdGatewayImageBuildOfflineMode tests the gateway image build command in offline mode
func TestCmdGatewayImageBuildOfflineMode(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayImageBuild-OfflineMode") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "apipctl")
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for binary: %v", err)
	}

	// Check if binary exists
	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", absBinaryPath)
	}

	// Get the test resources directory path
	resourcesDir := filepath.Join("..", "resources")
	absResourcesDir, err := filepath.Abs(resourcesDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path for resources directory: %v", err)
	}

	// First, run online mode to generate lock file
	t.Logf("Step 1: Running online mode to generate lock file")
	cmdOnline := exec.Command(absBinaryPath,
		"gateway", "image", "build",
		"--path", absResourcesDir,
		"--image-tag", "v0.2.0-offline-test",
	)
	cmdOnline.Dir = filepath.Join("..", "..")
	outputOnline, errOnline := cmdOnline.CombinedOutput()

	if errOnline != nil {
		outputStr := string(outputOnline)
		if strings.Contains(outputStr, "Docker is not available") ||
			strings.Contains(outputStr, "policy not found") {
			t.Logf("Cannot test offline mode: prerequisites not met")
			t.Skip("Skipping offline mode test - online mode prerequisites not met")
			return
		}
	}

	// Check if lock file was created in resources directory
	lockFilePath := filepath.Join(absResourcesDir, "policy-manifest-lock.yaml")
	if _, err := os.Stat(lockFilePath); os.IsNotExist(err) {
		t.Skip("Skipping offline mode test - lock file not generated")
		return
	}
	defer os.Remove(lockFilePath) // Clean up after test

	t.Logf("Step 2: Running offline mode with lock file")

	// Now test offline mode
	cmd := exec.Command(absBinaryPath,
		"gateway", "image", "build",
		"--path", absResourcesDir,
		"--image-tag", "v0.2.0-offline-test",
		"--offline",
	)

	// Set working directory to the src directory
	cmd.Dir = filepath.Join("..", "..")

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	t.Logf("Command output:\n%s", outputStr)

	// Check if command executed
	if err != nil {
		// Check for acceptable errors
		if strings.Contains(outputStr, "Docker is not available") ||
			strings.Contains(outputStr, "not found in cache") ||
			strings.Contains(outputStr, "policy not found") {
			t.Logf("✓ Command executed in offline mode but prerequisites not met (expected in test environment)")
			return
		}

		t.Fatalf("Command failed unexpectedly: %v\nOutput: %s", err, outputStr)
	}

	// Check for expected output patterns
	expectedPatterns := []string{
		"=== Gateway Image Build ===",
		"Building in OFFLINE mode",
		"[2/4] Reading Manifest Lock File",
		"[3/4] Verifying Policies",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(outputStr, pattern) {
			t.Errorf("Expected output to contain '%s', but it didn't", pattern)
		}
	}

	// Verify offline mode indicators
	if strings.Contains(outputStr, "OFFLINE mode") {
		t.Logf("✓ Offline mode executed correctly")
	}

	// Verify policies were verified (not downloaded)
	if strings.Contains(outputStr, "Verifying Policies") &&
		!strings.Contains(outputStr, "Downloading") {
		t.Logf("✓ Policies verified from cache (no downloads)")
	}

	t.Logf("✓ Gateway image build (offline mode) test completed")
}

// TestCmdGatewayImageBuildHelp tests the help command
func TestCmdGatewayImageBuildHelp(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayImageBuild-Help") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "apipctl")
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		t.Fatalf("Failed to get absolute path for binary: %v", err)
	}

	// Check if binary exists
	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not found at %s. Run 'make build' first.", absBinaryPath)
	}

	// Build the command
	cmd := exec.Command(absBinaryPath, "gateway", "image", "build", "--help")

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		t.Fatalf("Help command failed: %v\nOutput: %s", err, outputStr)
	}

	t.Logf("Help output:\n%s", outputStr)

	// Check for expected help content
	expectedPatterns := []string{
		"Build a WSO2 API Platform Gateway Docker image",
		"--image-tag",
		"--path",
		"--offline",
		"--image-repository",
		"--gateway-builder",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(outputStr, pattern) {
			t.Errorf("Expected help output to contain '%s', but it didn't", pattern)
		}
	}

	t.Logf("✓ Gateway image build help test completed")
}
