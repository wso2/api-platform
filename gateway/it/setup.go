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
	"strings"
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
	GatewayControllerPort = "9111"

	// RouterPort is the HTTP traffic port for the router
	RouterPort = "8080"

	// EnvoyAdminPort is the admin port for Envoy
	EnvoyAdminPort = "9901"
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

	// Create compose stack using testcontainers-go
	compose, err := tc.NewDockerCompose(absPath)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create docker compose: %w", err)
	}

	cm := &ComposeManager{
		compose:     compose,
		composeFile: absPath,
		projectName: "gateway-it",
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

	// Stream logs in background
	cm.StreamLogs()

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

// RestartGatewayController restarts the gateway-controller service with specific environment variables
func (cm *ComposeManager) RestartGatewayController(ctx context.Context, envVars map[string]string) error {
	// Project name is "gateway-it", service is "gateway-controller".
	// Default naming is usually project-service-1 or project_service_1.
	// However, explicit container_name might be set in compose file.
	// Given the context, we should rely on docker compose commands rather than assuming container name for 'docker stop/rm'
	// BUT the prompt explicitly used "docker stop containerName" and "docker rm containerName".
	// I should check if I can just use `docker compose stop` and `docker compose up`.
	// The prompt suggested:
	// exec.CommandContext(ctx, "docker", "stop", containerName).Run()
	// exec.CommandContext(ctx, "docker", "rm", containerName).Run()
	// args := []string{"compose", "-f", cm.composeFile, "-p", cm.projectName, "up", "-d", "gateway-controller"}

	// I'll stick to 'docker compose' commands to be safe with names.

	log.Println("Restarting gateway-controller with new configuration...")

	// Stop and remove the service container
	stopCmd := execCommandContext(ctx, "docker", "compose", "-f", cm.composeFile, "-p", cm.projectName, "stop", "gateway-controller")
	if err := stopCmd.Run(); err != nil {
		return fmt.Errorf("failed to stop gateway-controller: %w", err)
	}

	// Force remove the container by declared name to avoid conflicts
	// We use direct docker rm because compose rm sometimes doesn't clear the name reservation fast enough
	// or behaves differently with static container_names.
	rmCmd := execCommandContext(ctx, "docker", "rm", "-f", "it-gateway-controller")
	// We ignore error here because if it doesn't exist, that's fine.
	_ = rmCmd.Run()

	// Start with new env vars
	args := []string{"compose", "-f", cm.composeFile, "-p", cm.projectName, "up", "-d", "gateway-controller"}
	cmd := execCommandContext(ctx, "docker", args...)

	// Copy existing env and append new ones
	cmd.Env = os.Environ()
	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		log.Printf("Setting env: %s=%s", k, v)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start gateway-controller: %w\nOutput: %s", err, string(output))
	}

	// Wait for health check
	return cm.WaitForGatewayControllerHealthy(ctx)
}

// WaitForGatewayControllerHealthy waits for the gateway-controller to be healthy
func (cm *ComposeManager) WaitForGatewayControllerHealthy(ctx context.Context) error {
	endpoint := fmt.Sprintf("http://localhost:%s/health", GatewayControllerPort)
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for gateway-controller to be healthy")
		case <-ticker.C:
			resp, err := client.Get(endpoint)
			if err != nil {
				continue
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
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
		cm.cancel()

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := cm.compose.Down(cleanupCtx, tc.RemoveOrphans(true), tc.RemoveImagesLocal); err != nil {
			log.Printf("Failed to stop docker compose: %v", err)
		}
	})
}

// StreamLogs streams service logs to stdout
func (cm *ComposeManager) StreamLogs() {
	go func() {
		log.Println("Streaming logs from containers...")
		cmd := execCommandContext(cm.ctx, "docker", "compose", "-f", cm.composeFile, "-p", cm.projectName, "logs", "-f")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// Don't log error on context cancellation (standard shutdown)
			if cm.ctx.Err() == nil {
				log.Printf("Background log streaming stopped: %v", err)
			}
		}
	}()
}

// CheckLogsForText checks if a container's logs contain specific text
func (cm *ComposeManager) CheckLogsForText(ctx context.Context, containerName, text string) (bool, error) {
	// Need to use the actual container name (project name + service name usually, or explicit name)
	// In our compose file, we set container_name explicitly (e.g., it-otel-collector)

	cmd := execCommandContext(ctx, "docker", "logs", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to get logs for %s: %w", containerName, err)
	}

	return strings.Contains(string(output), text), nil
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
		"5050",                // Sample backend
		"18000",               // xDS gRPC
		"18001",               // xDS gRPC
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
