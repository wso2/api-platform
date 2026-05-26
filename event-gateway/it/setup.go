// --------------------------------------------------------------------
// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
// --------------------------------------------------------------------

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

// execCommandContext is a variable for exec.CommandContext to allow mocking in tests.
var execCommandContext = exec.CommandContext

const (
	// DefaultStartupTimeout is the maximum time to wait for all services to become healthy.
	DefaultStartupTimeout = 120 * time.Second

	// HealthCheckInterval is how often to poll service health endpoints.
	HealthCheckInterval = 2 * time.Second
)

// ComposeManager manages the Docker Compose lifecycle for integration tests.
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

// NewComposeManager creates a new ComposeManager from the given docker-compose file path.
func NewComposeManager(composeFile string) (*ComposeManager, error) {
	absPath, err := filepath.Abs(composeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve compose file path: %w", err)
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("compose file not found: %s", absPath)
	}

	ctx, cancel := context.WithCancel(context.Background())

	compose, err := tc.NewDockerComposeWith(
		tc.StackIdentifier("egw-it"),
		tc.WithStackFiles(absPath),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create docker compose: %w", err)
	}

	cm := &ComposeManager{
		compose:     compose,
		composeFile: absPath,
		projectName: "egw-it",
		ctx:         ctx,
		cancel:      cancel,
		signalChan:  make(chan os.Signal, 1),
	}
	cm.setupSignalHandler()
	return cm, nil
}

func (cm *ComposeManager) setupSignalHandler() {
	signal.Notify(cm.signalChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-cm.signalChan
		if sig == nil {
			return
		}
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cm.Cleanup()
		os.Exit(1)
	}()
}

// Start brings up all Docker Compose services and waits for them to be healthy.
func (cm *ComposeManager) Start() error {
	log.Println("Starting Event Gateway integration test services...")

	if err := cm.compose.Up(cm.ctx, tc.Wait(true)); err != nil {
		return fmt.Errorf("failed to start docker compose: %w", err)
	}

	log.Println("Services started – waiting for health checks...")

	startCtx, cancel := context.WithTimeout(cm.ctx, DefaultStartupTimeout)
	defer cancel()

	if err := cm.WaitForHealthy(startCtx); err != nil {
		// Best-effort teardown on startup failure.
		downCtx, downCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer downCancel()
		if downErr := cm.composeDown(downCtx); downErr != nil {
			log.Printf("Warning: teardown after failed startup: %v", downErr)
		}
		return err
	}

	log.Println("All event-gateway services are healthy and ready")
	return nil
}

// WaitForHealthy polls the health endpoints of each service until all pass.
func (cm *ComposeManager) WaitForHealthy(ctx context.Context) error {
	services := []struct {
		name     string
		endpoint string
	}{
		{"event-gateway", fmt.Sprintf("http://localhost:%s/health", EventGatewayAdminPort)},
	}

	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("startup timeout exceeded. Check docker-compose logs for details")
		case <-ticker.C:
			allHealthy := true
			for _, svc := range services {
				resp, err := client.Get(svc.endpoint)
				if err != nil {
					log.Printf("Service %s not ready yet: %v", svc.name, err)
					allHealthy = false
					continue
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					log.Printf("Service %s returned HTTP %d", svc.name, resp.StatusCode)
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

// Cleanup stops and removes all Docker Compose services.
// Safe to call multiple times and from concurrent goroutines.
func (cm *ComposeManager) Cleanup() {
	cm.cleanupOnce.Do(func() {
		log.Println("Cleaning up event-gateway integration test services...")

		// Cancel the main context to abort any in-flight compose operations.
		cm.cancel()
		signal.Stop(cm.signalChan)
		close(cm.signalChan)

		// Run 'docker compose down --volumes --remove-orphans' directly so the
		// project name and file path are guaranteed to match what was started.
		downCtx, downCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer downCancel()
		if err := cm.composeDown(downCtx); err != nil {
			log.Printf("Warning: docker compose down failed: %v", err)
		}

		log.Println("Cleanup complete")
	})
}

// composeDown runs 'docker compose down --volumes --remove-orphans' for the
// managed stack, giving containers up to 15 s to stop gracefully.
func (cm *ComposeManager) composeDown(ctx context.Context) error {
	args := []string{
		"compose",
		"-p", cm.projectName,
		"-f", cm.composeFile,
		"down",
		"-t", "15",
		"--volumes",
		"--remove-orphans",
	}
	cmd := execCommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DumpLogs writes docker compose logs to the given file.
func (cm *ComposeManager) DumpLogs(outputFile string) error {
	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"compose", "-p", cm.projectName, "-f", cm.composeFile, "logs", "--no-color", "--timestamps"}
	cmd := execCommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if writeErr := os.WriteFile(outputFile, out, 0644); writeErr != nil {
		return fmt.Errorf("failed to write logs: %w", writeErr)
	}
	return err
}

// CheckDockerAvailable verifies that Docker is running.
func CheckDockerAvailable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := execCommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker is not available: %w", err)
	}
	return nil
}

// CheckPortsAvailable verifies that the ports used by the test stack are free.
func CheckPortsAvailable() error {
	ports := []string{
		GatewayControllerPort, // 9090
		WebSubPort,            // 8080
		WebSocketPort,         // 8081
		EventGatewayAdminPort, // 9002
		WebhookListenerPort,   // 8090
		"9092",                // Kafka external
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
		return fmt.Errorf("ports already in use: %v – stop conflicting services before running tests", conflicts)
	}
	return nil
}

// waitForEndpoint polls an HTTP endpoint until it returns 200 OK or the context expires.
func waitForEndpoint(ctx context.Context, endpoint string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		resp, err := client.Get(endpoint)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s: %w", endpoint, ctx.Err())
		case <-ticker.C:
		}
	}
}
