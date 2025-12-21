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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wso2/api-platform/cli/utils"
)

func TestCmdGatewayMcpGenerate(t *testing.T) {
	if !isTestEnabled("Cmd-GatewayMcpGenerate") {
		t.Skip("Test disabled in test-config.yaml")
		return
	}

	// Check if Docker is available
	if err := utils.IsDockerAvailable(); err != nil {
		t.Logf("Docker is not available: %v", err)
		t.Skip("Skipping MCP generate test - Docker is not available in this environment")
		return
	}

	// Get the binary path
	binaryPath := filepath.Join("..", "..", "build", "ap")

	// Check if binary exists; if not, try to build it here so CI doesn't
	// need a separate build step.
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Logf("Binary not found at %s. Attempting to build...", binaryPath)
		if err := os.MkdirAll(filepath.Dir(binaryPath), 0755); err != nil {
			t.Fatalf("Failed to create build directory: %v", err)
		}
		buildCmd := exec.Command("go", "build", "-o", binaryPath, filepath.Join("..", "..", "main.go"))
		out, err := buildCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to build binary: %v\nOutput: %s", err, string(out))
		}
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			t.Fatalf("Binary still not found at %s after build", binaryPath)
		}
	}

	t.Log("Starting MCP server Docker container...")

	containerName := "everything"
	imageName := "mcp/everything:latest"

	// Clean up any existing container with the same name
	cleanupCmd := exec.Command("docker", "rm", "-f", containerName)
	cleanupCmd.Run() // Ignore errors if container doesn't exist

	// Start Docker container
	dockerCmd := exec.Command("docker", "run", "-d",
		"-p", "3001:3001",
		"--name", containerName,
		imageName,
	)

	output, err := dockerCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to start Docker container: %v\nOutput: %s", err, string(output))
	}

	containerID := strings.TrimSpace(string(output))
	t.Logf("✓ Container started: %s", containerID)

	// Ensure container is cleaned up after test
	defer func() {
		t.Log("Cleaning up Docker container...")
		stopCmd := exec.Command("docker", "stop", containerName)
		stopCmd.Run()
		removeCmd := exec.Command("docker", "rm", containerName)
		removeCmd.Run()
		t.Log("✓ Container cleaned up")
	}()

	// Wait for container to be ready using HTTP health check
	t.Log("Waiting for MCP server to be ready (HTTP health check)...")
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 0; i < 30; i++ {
		resp, err := client.Get("http://localhost:3001/mcp")
		if err == nil {
			// Consider the server ready only on HTTP 2xx responses.
			// Reject 3xx/4xx/5xx during startup to avoid false positives (e.g., 404 or 401).
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				resp.Body.Close()
				t.Log("✓ MCP server is ready (health check passed)")
				break
			}
			resp.Body.Close()
		}

		// Not ready yet; if last iteration, dump logs
		if i == 29 {
			checkCmd := exec.Command("docker", "logs", containerName)
			logs, _ := checkCmd.CombinedOutput()
			logsStr := string(logs)

			if err := os.MkdirAll("logs", 0755); err != nil {
				t.Logf("Warning: failed to create logs directory: %v", err)
			} else if err := os.WriteFile("logs/mcp-container.log", logs, 0644); err != nil {
				t.Logf("Warning: failed to write container logs: %v", err)
			}

			t.Logf("Container logs:\n%s", logsStr)
			t.Fatal("MCP server did not start within 30 seconds")
		}
		time.Sleep(1 * time.Second)
	}

	// Give the server a bit more time to fully initialize
	time.Sleep(2 * time.Second)

	// Create output directory
	outputDir := filepath.Join("..", "target", "mcp-test")
	os.RemoveAll(outputDir) // Clean up any previous test output
	defer os.RemoveAll(outputDir)

	t.Log("Running MCP generate command...")

	// Run the MCP generate command
	mcpCmd := exec.Command(binaryPath,
		"gateway", "mcp", "generate",
		"--server", "http://localhost:3001/mcp",
		"--output", outputDir,
	)

	mcpOutput, err := mcpCmd.CombinedOutput()
	mcpOutputStr := string(mcpOutput)

	t.Logf("Command output:\n%s", mcpOutputStr)

	if err != nil {
		t.Fatalf("MCP generate command failed: %v\nOutput: %s", err, mcpOutputStr)
	}

	// Check if output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Fatalf("Output directory was not created at %s", outputDir)
	}

	t.Logf("✓ Output directory created at %s", outputDir)

	// Verify at least some files were generated
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("No files were generated in output directory")
	}

	t.Logf("✓ Generated %d files/directories", len(entries))

	// List generated files for debugging
	for _, entry := range entries {
		t.Logf("  - %s", entry.Name())
	}

	t.Logf("✓ Gateway MCP generate command test completed")
}
