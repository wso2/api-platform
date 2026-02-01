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
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	tc "github.com/testcontainers/testcontainers-go/modules/compose"
)

// execCommandContext is a variable for exec.CommandContext to allow mocking in tests
var execCommandContext = exec.CommandContext

const (
	// DefaultStartupTimeout is the maximum time to wait for services to become healthy
	DefaultStartupTimeout = 60 * time.Second

	// HealthCheckInterval is how often to check service health
	HealthCheckInterval = 2 * time.Second

	// GatewayControllerPort is the REST API port for gateway-controller
	GatewayControllerPort = "9090"

	// RouterPort is the HTTP traffic port for the router
	RouterPort = "8080"

	// EnvoyAdminPort is the admin port for Envoy
	EnvoyAdminPort = "9901"

	// SampleBackendPort is the port for sample-backend service
	SampleBackendPort = "9080"

	// EchoBackendPort is the port for echo-backend service
	EchoBackendPort = "9081"
)

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Name    string
	Healthy bool
	Error   error
}

// ComposeManager manages the Docker Compose lifecycle for integration tests
// Uses testcontainers-go compose module for reliable container management
type ComposeManager struct {
	compose       tc.ComposeStack
	composeFile   string
	projectName   string
	ctx           context.Context
	cancel        context.CancelFunc
	cleanupOnce   sync.Once
	signalChan    chan os.Signal
	shutdownMutex sync.Mutex
	isShutdown    bool
}

// NewComposeManager creates a new ComposeManager with the given compose file
func NewComposeManager(composeFile string) (*ComposeManager, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve compose file path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("compose file not found: %s", absPath)
	}

	ctx, cancel := context.WithCancel(context.Background())

	projectName := "gateway-it"

	// Create compose stack using testcontainers-go with explicit project name
	// This ensures we can later query logs using the same project identifier
	compose, err := tc.NewDockerComposeWith(
		tc.StackIdentifier(projectName),
		tc.WithStackFiles(absPath),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create docker compose: %w", err)
	}

	cm := &ComposeManager{
		compose:     compose,
		composeFile: absPath,
		projectName: projectName,
		ctx:         ctx,
		cancel:      cancel,
		signalChan:  make(chan os.Signal, 1),
	}

	// Setup signal handling for graceful cleanup
	cm.setupSignalHandler()

	return cm, nil
}

// setupSignalHandler sets up handling for SIGINT and SIGTERM
func (cm *ComposeManager) setupSignalHandler() {
	signal.Notify(cm.signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-cm.signalChan
		if sig == nil {
			// Channel was closed during normal cleanup, not a real signal
			return
		}
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cm.Cleanup()
		os.Exit(1)
	}()
}

// Start starts all Docker Compose services and waits for them to be healthy
func (cm *ComposeManager) Start() error {
	log.Println("Starting Docker Compose services...")

	// Start compose with wait for healthy services
	err := cm.compose.Up(cm.ctx, tc.Wait(true))
	if err != nil {
		return fmt.Errorf("failed to start docker compose: %w", err)
	}

	log.Println("Docker Compose services started, waiting for health checks...")

	// Wait for services to be healthy (additional verification)
	startCtx, startCancel := context.WithTimeout(cm.ctx, DefaultStartupTimeout)
	defer startCancel()

	err = cm.WaitForHealthy(startCtx)
	if err != nil {
		// Attempt cleanup on failure
		cm.Cleanup()
		return err
	}

	log.Println("All services are healthy and ready")
	return nil
}

// WaitForHealthy waits for all services to pass health checks
func (cm *ComposeManager) WaitForHealthy(ctx context.Context) error {
	services := []struct {
		name     string
		endpoint string
	}{
		{"gateway-controller", fmt.Sprintf("http://localhost:%s/health", GatewayControllerPort)},
		{"router", fmt.Sprintf("http://localhost:%s/ready", EnvoyAdminPort)},
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Collect status of all services for error message
			unhealthy := cm.checkAllServices(client, services)
			if len(unhealthy) > 0 {
				return fmt.Errorf("startup timeout exceeded (60s). Unhealthy services: %v", unhealthy)
			}
			return ctx.Err()

		case <-ticker.C:
			allHealthy := true
			for _, svc := range services {
				resp, err := client.Get(svc.endpoint)
				if err != nil {
					log.Printf("Service %s not ready: %v", svc.name, err)
					allHealthy = false
					continue
				}
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					log.Printf("Service %s returned status %d", svc.name, resp.StatusCode)
					allHealthy = false
					continue
				}

				log.Printf("Service %s is healthy", svc.name)
			}

			if allHealthy {
				return nil
			}
		}
	}
}

// checkAllServices checks health of all services and returns list of unhealthy ones
func (cm *ComposeManager) checkAllServices(client *http.Client, services []struct {
	name     string
	endpoint string
}) []string {
	var unhealthy []string

	for _, svc := range services {
		resp, err := client.Get(svc.endpoint)
		if err != nil {
			unhealthy = append(unhealthy, fmt.Sprintf("%s (error: %v)", svc.name, err))
			continue
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			unhealthy = append(unhealthy, fmt.Sprintf("%s (status: %d)", svc.name, resp.StatusCode))
		}
	}

	return unhealthy
}

// Cleanup stops and removes all Docker Compose services
func (cm *ComposeManager) Cleanup() {
	cm.cleanupOnce.Do(func() {
		cm.shutdownMutex.Lock()
		defer cm.shutdownMutex.Unlock()

		if cm.isShutdown {
			return
		}
		cm.isShutdown = true

		log.Println("Cleaning up Docker Compose services...")

		// Cancel context to stop any ongoing operations
		cm.cancel()

		// Stop signal handling
		signal.Stop(cm.signalChan)
		close(cm.signalChan)

		// First, gracefully stop containers to allow coverage data to be flushed
		// This sends SIGTERM and waits for containers to exit gracefully
		log.Println("Stopping containers gracefully (waiting for coverage flush)...")
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()

		if err := cm.gracefulStop(stopCtx); err != nil {
			log.Printf("Warning: graceful stop failed: %v", err)
		}

		// Give containers time to write coverage data after SIGTERM
		log.Println("Waiting for coverage data to be written...")
		time.Sleep(3 * time.Second)

		// Run docker compose down with cleanup context
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()

		if err := cm.compose.Down(cleanupCtx, tc.RemoveOrphans(true), tc.RemoveVolumes(true)); err != nil {
			log.Printf("Warning: error during cleanup: %v", err)
		}

		log.Println("Cleanup complete")
	})
}

// gracefulStop sends SIGTERM to containers via docker-compose stop
func (cm *ComposeManager) gracefulStop(ctx context.Context) error {
	// Use docker-compose stop which sends SIGTERM and waits for graceful shutdown
	// The -t flag specifies the timeout in seconds before sending SIGKILL
	args := []string{"compose", "-p", cm.projectName, "-f", cm.composeFile, "stop", "-t", "15"}
	cmd := execCommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// DumpLogs collects docker compose logs and writes them to the given file path
func (cm *ComposeManager) DumpLogs(outputFile string) error {
	if cm == nil {
		return fmt.Errorf("compose manager is nil")
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Use a dedicated timeout for log collection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"compose", "-p", cm.projectName, "-f", cm.composeFile, "logs", "--no-color", "--timestamps"}
	cmd := execCommandContext(ctx, "docker", args...)

	// Give some time for containers to flush logs
	time.Sleep(10 * time.Second)

	// Capture combined output to persist whatever we can even on partial failures
	out, err := cmd.CombinedOutput()

	if writeErr := os.WriteFile(outputFile, out, 0644); writeErr != nil {
		return fmt.Errorf("failed to write logs to %s: %w", outputFile, writeErr)
	}

	if err != nil {
		// Return an error but logs are still written for post-mortem
		return fmt.Errorf("failed to collect docker compose logs: %w", err)
	}

	return nil
}

// CheckDockerAvailable verifies that Docker is running and accessible
func CheckDockerAvailable() error {
	// Check Docker by running a simple command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := execCommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available. Please ensure Docker is running.\nError: %w", err)
	}

	return nil
}

// CheckPortsAvailable checks if required ports are available
func CheckPortsAvailable() error {
	ports := []string{
		GatewayControllerPort, // 9090
		RouterPort,            // 8080
		"8443",                // HTTPS
		EnvoyAdminPort,        // 9901
		"9002",                // Policy engine
		"9080",                // Sample backend
		"3001",                // MCP server backend
		"18000",               // xDS gRPC
		"18001",               // xDS gRPC
		"8082",                // Mock JWKS server
	}

	var conflicts []string
	for _, port := range ports {
		ln, err := net.Listen("tcp", ":"+port)
		if err != nil {
			conflicts = append(conflicts, port)
			continue
		}
		ln.Close()
	}

	if len(conflicts) > 0 {
		return fmt.Errorf("port conflict detected. The following ports are in use: %v\nPlease stop any services using these ports before running tests", conflicts)
	}

	return nil
}

// GetComposeFilePath returns the path to the test docker-compose file
func GetComposeFilePath() string {
	// Get the directory of this file
	_, filename, _, ok := runTimeCaller()
	if !ok {
		// Fallback to current directory
		return "docker-compose.test.yaml"
	}
	return filepath.Join(filepath.Dir(filename), "docker-compose.test.yaml")
}

// runTimeCaller is a helper to get runtime caller info
// This is a variable so it can be mocked in tests
var runTimeCaller = func() (pc uintptr, file string, line int, ok bool) {
	// Use a fixed path relative to the package
	return 0, "", 0, false
}
