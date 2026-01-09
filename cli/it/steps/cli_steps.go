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

// Package steps provides step definitions for CLI integration tests.
package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wso2/api-platform/cli/it/resources"
)

// TestState interface for accessing test state from steps package
type TestState interface {
	ExecuteCLI(args ...string) error
	GetStdout() string
	GetStderr() string
	GetExitCode() int
	GetCombinedOutput() string
	SetGatewayInfo(name, server string)
	SetAPIInfo(name, version string)
	SetMCPInfo(name, version string)
}

// CLISteps provides CLI execution step definitions
type CLISteps struct {
	state TestState
}

// NewCLISteps creates a new CLISteps instance
func NewCLISteps(state TestState) *CLISteps {
	return &CLISteps{state: state}
}

// RunCommand runs a shell command (for general commands)
func (s *CLISteps) RunCommand(command string) error {
	// Parse command - expect "ap <args>"
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	// Skip "ap" prefix if present
	if parts[0] == "ap" {
		parts = parts[1:]
	}

	return s.state.ExecuteCLI(parts...)
}

// RunWithArgs runs the CLI with the given arguments string
func (s *CLISteps) RunWithArgs(args string) error {
	parts := strings.Fields(args)

	// Resolve resource paths in arguments
	for i, part := range parts {
		if strings.HasPrefix(part, "resources/gateway/") {
			// Extract filename and get absolute path
			filename := filepath.Base(part)
			parts[i] = resources.GetResourcePath(filename)
		}
	}

	return s.state.ExecuteCLI(parts...)
}

// RunGatewayAdd runs the gateway add command
func (s *CLISteps) RunGatewayAdd(name, server, auth string) error {
	args := []string{"gateway", "add", "--display-name", name, "--server", server}
	if auth != "" && auth != "none" {
		args = append(args, "--auth", auth)
		// Add credentials for basic auth
		if auth == "basic" {
			args = append(args, "--username", "admin", "--password", "admin", "--no-interactive")
		}
	}

	err := s.state.ExecuteCLI(args...)
	if err == nil && s.state.GetExitCode() == 0 {
		s.state.SetGatewayInfo(name, server)
	}
	return err
}

// EnsureGatewayExists ensures a gateway with the given name exists
func (s *CLISteps) EnsureGatewayExists(name string) error {
	// Add gateway with basic auth (the gateway requires auth by default)
	server := resources.GatewayControllerURL
	err := s.RunGatewayAdd(name, server, "basic")
	if err != nil {
		return err
	}

	// Check if it was added (exit code 0) or already exists
	if s.state.GetExitCode() != 0 {
		// Check if it's a duplicate error (which is OK)
		if !strings.Contains(s.state.GetCombinedOutput(), "already exists") {
			return fmt.Errorf("failed to ensure gateway exists: %s", s.state.GetCombinedOutput())
		}
	}

	s.state.SetGatewayInfo(name, server)
	return nil
}

// SetCurrentGateway sets the current gateway context
func (s *CLISteps) SetCurrentGateway(name string) error {
	return s.state.ExecuteCLI("gateway", "use", "--display-name", name)
}

// ApplySampleAPI applies the sample API to the gateway
func (s *CLISteps) ApplySampleAPI() error {
	apiPath := resources.GetSampleAPIPath()
	err := s.state.ExecuteCLI("gateway", "apply", "-f", apiPath)
	if err == nil && s.state.GetExitCode() == 0 {
		s.state.SetAPIInfo(resources.TestAPIName, resources.TestAPIVersion)
	}
	return err
}

// ApplyFile applies a YAML file to the gateway
func (s *CLISteps) ApplyFile(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}
	return s.state.ExecuteCLI("gateway", "apply", "-f", absPath)
}

// DeleteAPI deletes an API from the gateway
func (s *CLISteps) DeleteAPI(name string) error {
	return s.state.ExecuteCLI("gateway", "api", "delete", name)
}

// GenerateMCPConfig generates MCP configuration from a server
func (s *CLISteps) GenerateMCPConfig(server, output string) error {
	args := []string{"gateway", "mcp", "generate", "--server", server}
	if output != "" {
		args = append(args, "--output", output)
	}
	return s.state.ExecuteCLI(args...)
}

// BuildGatewayWithManifest builds a gateway with the given manifest
func (s *CLISteps) BuildGatewayWithManifest(manifestPath string) error {
	absPath, err := filepath.Abs(manifestPath)
	if err != nil {
		// Try using the resources path
		absPath = resources.GetPolicyManifestPath()
	}

	return s.state.ExecuteCLI("gateway", "build", "-f", absPath,
		"--docker-registry", "localhost:5000",
		"--image-tag", "test-v1.0.0")
}

// RunGatewayList runs the gateway list command
func (s *CLISteps) RunGatewayList() error {
	return s.state.ExecuteCLI("gateway", "list")
}

// RunGatewayRemove runs the gateway remove command
func (s *CLISteps) RunGatewayRemove(name string) error {
	return s.state.ExecuteCLI("gateway", "remove", name)
}

// RunGatewayCurrent runs the gateway current command
func (s *CLISteps) RunGatewayCurrent() error {
	return s.state.ExecuteCLI("gateway", "current")
}

// RunGatewayHealth runs the gateway health command
func (s *CLISteps) RunGatewayHealth() error {
	return s.state.ExecuteCLI("gateway", "health")
}

// RunAPIList runs the API list command
func (s *CLISteps) RunAPIList() error {
	return s.state.ExecuteCLI("gateway", "api", "list")
}

// RunAPIGet runs the API get command
func (s *CLISteps) RunAPIGet(name string) error {
	return s.state.ExecuteCLI("gateway", "api", "get", name)
}

// RunMCPList runs the MCP list command
func (s *CLISteps) RunMCPList() error {
	return s.state.ExecuteCLI("gateway", "mcp", "list")
}

// RunMCPGet runs the MCP get command
func (s *CLISteps) RunMCPGet(name string) error {
	return s.state.ExecuteCLI("gateway", "mcp", "get", name)
}

// RunMCPDelete runs the MCP delete command
func (s *CLISteps) RunMCPDelete(name string) error {
	return s.state.ExecuteCLI("gateway", "mcp", "delete", name)
}

// EnsureGatewayExistsWithServer ensures a gateway exists with the specific server URL
func (s *CLISteps) EnsureGatewayExistsWithServer(name, server string) error {
	// Add gateway with no auth for non-standard servers (like unreachable ones)
	err := s.state.ExecuteCLI("gateway", "add", "--display-name", name, "--server", server)
	if err == nil && s.state.GetExitCode() == 0 {
		s.state.SetGatewayInfo(name, server)
		return nil
	}

	// If gateway already exists, update it
	if s.state.GetExitCode() != 0 {
		if !strings.Contains(s.state.GetCombinedOutput(), "already exists") {
			return fmt.Errorf("failed to create gateway: %s", s.state.GetCombinedOutput())
		}
	}

	s.state.SetGatewayInfo(name, server)
	return nil
}

// ResetConfiguration resets the CLI configuration to empty
func (s *CLISteps) ResetConfiguration() error {
	// Create a backup of the config and create a fresh empty config
	configDir := os.Getenv("HOME") + "/.wso2ap"
	configFile := configDir + "/config.yaml"

	// Create empty config
	emptyConfig := "# WSO2 API Platform CLI Configuration\ngateways: []\n"
	return os.WriteFile(configFile, []byte(emptyConfig), 0644)
}

// ApplyResourceFile applies a resource file to the gateway
func (s *CLISteps) ApplyResourceFile(filePath string) error {
	// Convert relative resource paths to absolute paths
	absPath := filePath
	if filepath.IsAbs(filePath) {
		absPath = filePath
	} else if filepath.HasPrefix(filePath, "resources/gateway/") {
		// Extract the filename from the relative path
		filename := filepath.Base(filePath)
		absPath = resources.GetResourcePath(filename)
	} else {
		var err error
		absPath, err = filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		}
	}
	return s.state.ExecuteCLI("gateway", "apply", "-f", absPath)
}
