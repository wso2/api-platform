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

// Package resources provides shared test values and constants for CLI integration tests.
package resources

import (
	"path/filepath"
	"runtime"
)

// Infrastructure endpoints
const (
	// GatewayControllerURL is the gateway controller REST API endpoint
	GatewayControllerURL = "http://localhost:9090"

	// GatewayControllerHealthURL is the health check endpoint
	GatewayControllerHealthURL = "http://localhost:9090/health"

	// RouterURL is the gateway router HTTP endpoint
	RouterURL = "http://localhost:8080"

	// MCPServerURL is the MCP server backend endpoint
	MCPServerURL = "http://localhost:3001"

	// MCPServerMCPEndpoint is the MCP protocol endpoint
	MCPServerMCPEndpoint = "http://localhost:3001/mcp"
)

// Timeouts and intervals
const (
	// DefaultStartupTimeout is the maximum time to wait for infrastructure
	DefaultStartupTimeout = 120 // seconds

	// HealthCheckInterval is how often to check service health
	HealthCheckInterval = 5 // seconds

	// CLITimeout is the maximum time to wait for CLI commands
	CLITimeout = 30 // seconds
)

// Test gateway configuration
const (
	// TestGatewayName is the default gateway name for tests
	TestGatewayName = "test-gateway"

	// TestGatewayDisplayName is the display name for test gateway
	TestGatewayDisplayName = "Test Gateway"

	// TestGatewayServer is the server URL for test gateway
	TestGatewayServer = GatewayControllerURL

	// TestGatewayAuth is the authentication type for test gateway
	TestGatewayAuth = "none"
)

// Test API configuration
const (
	// TestAPIName is the name of the test API
	TestAPIName = "petstore-api"

	// TestAPIVersion is the version of the test API
	TestAPIVersion = "v1.0"

	// TestAPIContext is the context path of the test API
	TestAPIContext = "/petstore"
)

// Test MCP configuration
const (
	// TestMCPName is the name of the test MCP
	TestMCPName = "test-mcp"

	// TestMCPVersion is the version of the test MCP
	TestMCPVersion = "v1.0"

	// TestMCPContext is the context path of the test MCP
	TestMCPContext = "/test-mcp"
)

// Error messages for validation
const (
	// ErrMissingNameFlag is the expected error for missing name flag
	ErrMissingNameFlag = "required flag(s) \"display-name\" not set"

	// ErrInvalidServerURL is the expected error for invalid server URL
	ErrInvalidServerURL = "invalid server URL"

	// ErrGatewayNotFound is the expected error when gateway doesn't exist
	ErrGatewayNotFound = "gateway not found"

	// ErrAPINotFound is the expected error when API doesn't exist
	ErrAPINotFound = "API not found"

	// ErrMCPNotFound is the expected error when MCP doesn't exist
	ErrMCPNotFound = "MCP not found"

	// ErrFileNotFound is the expected error when file doesn't exist
	ErrFileNotFound = "no such file or directory"
)

// GetResourcePath returns the absolute path to a resource file in the gateway resources folder
func GetResourcePath(filename string) string {
	_, currentFile, _, _ := runtime.Caller(0)
	resourcesDir := filepath.Dir(currentFile)
	return filepath.Join(resourcesDir, "gateway", filename)
}

// GetSampleAPIPath returns the path to the sample API YAML file
func GetSampleAPIPath() string {
	return GetResourcePath("sample-api.yaml")
}

// GetSampleMCPConfigPath returns the path to the sample MCP config file
func GetSampleMCPConfigPath() string {
	return GetResourcePath("sample-mcp-config.yaml")
}

// GetPolicyManifestPath returns the path to the policy manifest file (build.yaml)
func GetPolicyManifestPath() string {
	return GetResourcePath("build.yaml")
}

// GetInvalidYAMLPath returns the path to an invalid YAML file for testing
func GetInvalidYAMLPath() string {
	return GetResourcePath("invalid.yaml")
}
