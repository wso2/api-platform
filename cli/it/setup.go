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

package it

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultStartupTimeout is the maximum time to wait for services to become healthy
	DefaultStartupTimeout = 120 * time.Second

	// HealthCheckInterval is how often to check service health
	HealthCheckInterval = 5 * time.Second

	// GatewayControllerPort is the REST API port for gateway-controller
	GatewayControllerPort = "9090"

	// MCPServerPort is the port for MCP server
	MCPServerPort = "3001"

	// ComposeProjectName is the Docker Compose project name for CLI IT tests
	ComposeProjectName = "cli-it"
)

// InfrastructureManager manages the lifecycle of test infrastructure
type InfrastructureManager struct {
	composeFile     string
	cliBinaryPath   string
	ctx             context.Context
	cancel          context.CancelFunc
	startedServices map[InfrastructureID]bool
	mu              sync.Mutex
	reporter        *TestReporter
}

// NewInfrastructureManager creates a new infrastructure manager
func NewInfrastructureManager(reporter *TestReporter) *InfrastructureManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &InfrastructureManager{
		ctx:             ctx,
		cancel:          cancel,
		startedServices: make(map[InfrastructureID]bool),
		reporter:        reporter,
	}
}

// SetupInfrastructure starts the required infrastructure components
func (m *InfrastructureManager) SetupInfrastructure(required []InfrastructureID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range required {
		if m.startedServices[id] {
			continue
		}

		switch id {
		case InfraCLI:
			if err := m.buildCLI(); err != nil {
				return fmt.Errorf("failed to build CLI: %w", err)
			}
		case InfraGatewayImages:
			if err := m.buildGatewayImages(); err != nil {
				return fmt.Errorf("failed to build gateway images: %w", err)
			}
		case InfraGateway:
			// Ensure gateway images are built first
			if !m.startedServices[InfraGatewayImages] {
				if err := m.buildGatewayImages(); err != nil {
					return fmt.Errorf("failed to build gateway images: %w", err)
				}
				m.startedServices[InfraGatewayImages] = true
			}
			if err := m.startGatewayStack(); err != nil {
				return fmt.Errorf("failed to start gateway stack: %w", err)
			}
		case InfraMCPServer:
			// MCP server is part of the same compose file as gateway
			// It will be started with the gateway stack
			if !m.startedServices[InfraGateway] {
				if err := m.startGatewayStack(); err != nil {
					return fmt.Errorf("failed to start gateway stack for MCP: %w", err)
				}
			}
		}

		m.startedServices[id] = true
	}

	return nil
}

// buildCLI builds the CLI binary
func (m *InfrastructureManager) buildCLI() error {
	m.reporter.LogPhase1("CLI", "Building CLI binary...")

	// Get the CLI source directory
	cliSrcDir, err := filepath.Abs("../src")
	if err != nil {
		return fmt.Errorf("failed to resolve CLI source directory: %w", err)
	}

	m.reporter.LogPhase1Detail("Compiling Go source files...")
	m.reporter.LogPhase1Detail("Running: make build-skip-tests")

	// Run make build-skip-tests to build the CLI
	cmd := exec.CommandContext(m.ctx, "make", "build-skip-tests")
	cmd.Dir = cliSrcDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.reporter.LogPhase1Fail("CLI", "Build failed", string(output))
		return fmt.Errorf("failed to build CLI: %w\nOutput: %s", err, output)
	}

	// Set the CLI binary path
	m.cliBinaryPath = filepath.Join(cliSrcDir, "build", "ap")

	// Verify the binary exists
	if _, err := os.Stat(m.cliBinaryPath); os.IsNotExist(err) {
		return fmt.Errorf("CLI binary not found at %s", m.cliBinaryPath)
	}

	m.reporter.LogPhase1Detail("Verifying CLI binary...")

	// Verify the binary runs
	verifyCmd := exec.CommandContext(m.ctx, m.cliBinaryPath, "version")
	if err := verifyCmd.Run(); err != nil {
		return fmt.Errorf("CLI binary verification failed: %w", err)
	}

	m.reporter.LogPhase1Pass("CLI", "CLI binary ready")
	return nil
}

// buildGatewayImages builds the gateway Docker images with coverage instrumentation
func (m *InfrastructureManager) buildGatewayImages() error {
	m.reporter.LogPhase1("GATEWAY_IMAGES", "Building gateway Docker images...")

	// Get the gateway directory
	gatewayDir, err := filepath.Abs("../../gateway")
	if err != nil {
		return fmt.Errorf("failed to resolve gateway directory: %w", err)
	}

	m.reporter.LogPhase1Detail("Building gateway-controller-coverage image...")
	m.reporter.LogPhase1Detail("Building gateway-builder image...")
	m.reporter.LogPhase1Detail("Building policy-engine image...")
	m.reporter.LogPhase1Detail("Building router image...")
	m.reporter.LogPhase1Detail("This may take several minutes on first run...")

	// Run make build-coverage to build the gateway images
	cmd := exec.CommandContext(m.ctx, "make", "build-coverage")
	cmd.Dir = gatewayDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.reporter.LogPhase1Fail("GATEWAY_IMAGES", "Build failed", string(output))
		return fmt.Errorf("failed to build gateway images: %w\nOutput: %s", err, output)
	}

	m.reporter.LogPhase1Pass("GATEWAY_IMAGES", "Docker images ready")
	return nil
}

// startGatewayStack starts the gateway Docker Compose stack using native docker compose
func (m *InfrastructureManager) startGatewayStack() error {
	m.reporter.LogPhase1("GATEWAY", "Starting gateway stack...")

	// Resolve the compose file path
	composeFile, err := filepath.Abs("../../gateway/it/docker-compose.test.yaml")
	if err != nil {
		return fmt.Errorf("failed to resolve compose file path: %w", err)
	}

	if _, err := os.Stat(composeFile); os.IsNotExist(err) {
		return fmt.Errorf("compose file not found: %s", composeFile)
	}

	m.composeFile = composeFile

	// Create coverage directory with proper permissions to avoid Docker mount issues
	m.reporter.LogPhase1Detail("Creating coverage directory...")
	coverageDir := filepath.Join(filepath.Dir(composeFile), "coverage", "gateway-controller")
	if err := os.MkdirAll(coverageDir, 0755); err != nil {
		log.Printf("Warning: Could not create coverage directory: %v", err)
	}

	// Stop any existing containers from previous runs
	m.reporter.LogPhase1Detail("Cleaning up previous containers...")
	m.stopGatewayStack()

	m.reporter.LogPhase1Detail("Starting gateway-controller container...")
	m.reporter.LogPhase1Detail("Starting policy-engine container...")
	m.reporter.LogPhase1Detail("Starting router container...")
	m.reporter.LogPhase1Detail("Starting sample-backend container...")
	m.reporter.LogPhase1Detail("Starting mcp-server-backend container...")

	// Start the compose stack using native docker compose command
	cmd := exec.CommandContext(m.ctx, "docker", "compose",
		"-f", composeFile,
		"-p", ComposeProjectName,
		"up", "-d", "--wait",
	)
	cmd.Dir = filepath.Dir(composeFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.reporter.LogPhase1Fail("GATEWAY", "Failed to start stack", string(output))
		return fmt.Errorf("failed to start docker compose: %w\nOutput: %s", err, output)
	}

	// Wait for gateway controller to be healthy
	m.reporter.LogPhase1Detail("Waiting for gateway-controller health check...")
	if err := m.waitForGatewayHealth(); err != nil {
		m.reporter.LogPhase1Fail("GATEWAY", "Health check failed", err.Error())
		return err
	}

	// Wait for MCP server to be ready
	m.reporter.LogPhase1Detail("Waiting for mcp-server-backend to be ready...")
	if err := m.waitForMCPServer(); err != nil {
		m.reporter.LogPhase1Detail("Warning: MCP server not ready (continuing anyway)")
	}

	m.reporter.LogPhase1Pass("GATEWAY", "Gateway stack ready")
	m.startedServices[InfraMCPServer] = true // MCP is in the same compose file

	return nil
}

// stopGatewayStack stops the gateway Docker Compose stack
func (m *InfrastructureManager) stopGatewayStack() {
	if m.composeFile == "" {
		return
	}

	cmd := exec.CommandContext(m.ctx, "docker", "compose",
		"-f", m.composeFile,
		"-p", ComposeProjectName,
		"down", "-v", "--remove-orphans",
	)
	cmd.Dir = filepath.Dir(m.composeFile)
	_ = cmd.Run() // Ignore errors during cleanup
}

// waitForGatewayHealth waits for the gateway controller to be healthy
func (m *InfrastructureManager) waitForGatewayHealth() error {
	healthURL := fmt.Sprintf("http://localhost:%s/health", GatewayControllerPort)
	client := &http.Client{Timeout: 5 * time.Second}

	deadline := time.Now().Add(DefaultStartupTimeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(HealthCheckInterval)
	}

	return fmt.Errorf("gateway controller health check timed out after %v", DefaultStartupTimeout)
}

// waitForMCPServer waits for the MCP server to be available
func (m *InfrastructureManager) waitForMCPServer() error {
	deadline := time.Now().Add(DefaultStartupTimeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%s", MCPServerPort), 5*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(HealthCheckInterval)
	}

	return fmt.Errorf("MCP server health check timed out after %v", DefaultStartupTimeout)
}

// GetCLIBinaryPath returns the path to the CLI binary
func (m *InfrastructureManager) GetCLIBinaryPath() string {
	return m.cliBinaryPath
}

// Teardown stops all infrastructure components
func (m *InfrastructureManager) Teardown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Println("Tearing down infrastructure...")

	// Stop compose stack using native docker compose
	if m.composeFile != "" {
		cmd := exec.Command("docker", "compose",
			"-f", m.composeFile,
			"-p", ComposeProjectName,
			"down", "-v", "--remove-orphans",
		)
		cmd.Dir = filepath.Dir(m.composeFile)
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Warning: Failed to stop compose stack: %v\nOutput: %s", err, output)
		}
	}

	m.cancel()
	return nil
}

// CheckDockerAvailable verifies Docker is running
func CheckDockerAvailable() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available: %w", err)
	}
	return nil
}

// CheckPortsAvailable verifies required ports are free
func CheckPortsAvailable() error {
	ports := []string{
		GatewayControllerPort,
		"8080",  // Router HTTP
		"18000", // xDS
		MCPServerPort,
	}

	for _, port := range ports {
		ln, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return fmt.Errorf("port %s is already in use", port)
		}
		ln.Close()
	}

	return nil
}
