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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// TestState holds the state for a test scenario
type TestState struct {
	// CLI execution state
	CLIBinaryPath string
	LastCommand   []string
	LastStdout    string
	LastStderr    string
	LastExitCode  int

	// Test context
	TestID     string
	TestName   string
	TempDir    string
	ConfigDir  string
	WorkingDir string

	// Gateway state
	GatewayName   string
	GatewayServer string

	// API state
	APIName    string
	APIVersion string

	// MCP state
	MCPName    string
	MCPVersion string

	// Coverage directory for CLI coverage collection
	CLICoverDir string

	// Timing
	StartTime time.Time
	EndTime   time.Time

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	mu sync.RWMutex
}

// NewTestState creates a new test state
func NewTestState() *TestState {
	ctx, cancel := context.WithCancel(context.Background())
	return &TestState{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Reset resets the test state for a new scenario
func (s *TestState) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up previous temp directory
	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
	}

	// Create new temp directory for this test
	tempDir, err := os.MkdirTemp("", "cli-it-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create config directory within temp
	configDir := filepath.Join(tempDir, ".ap")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create a fresh context for this test. If a previous context exists,
	// cancel it to avoid leaking resources and ensure subsequent
	// ExecuteCLI calls use a live context.
	if s.cancel != nil {
		s.cancel()
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.TempDir = tempDir
	s.ConfigDir = configDir
	s.WorkingDir = tempDir
	s.LastCommand = nil
	s.LastStdout = ""
	s.LastStderr = ""
	s.LastExitCode = 0
	s.GatewayName = ""
	s.GatewayServer = ""
	s.APIName = ""
	s.APIVersion = ""
	s.MCPName = ""
	s.MCPVersion = ""
	s.StartTime = time.Now()

	return nil
}

// Cleanup cleans up the test state
func (s *TestState) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.EndTime = time.Now()
	s.cancel()

	if s.TempDir != "" {
		os.RemoveAll(s.TempDir)
		s.TempDir = ""
	}
}

// ExecuteCLI executes a CLI command and captures the output
func (s *TestState) ExecuteCLI(args ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.CLIBinaryPath == "" {
		return fmt.Errorf("CLI binary path not set")
	}

	s.LastCommand = args

	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.CLIBinaryPath, args...)
	cmd.Dir = s.WorkingDir

	cmd.Env = append(os.Environ(), "AP_NO_COLOR=true")

	// Add GOCOVERDIR for coverage collection if configured
	if s.CLICoverDir != "" {
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+s.CLICoverDir)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	s.LastStdout = stdout.String()
	s.LastStderr = stderr.String()

	if exitErr, ok := err.(*exec.ExitError); ok {
		s.LastExitCode = exitErr.ExitCode()
		// Process ran and exited with non-zero status; return nil so
		// callers can inspect `LastExitCode` to determine expected failures.
		return nil
	}

	if err != nil {
		// Process failed to start or was killed (timeout, context cancel,
		// spawn error). Record an indicative exit code and return the error
		// so the test step fails fast instead of proceeding with bogus state.
		s.LastExitCode = -1
		return err
	}

	s.LastExitCode = 0
	return nil
}

// GetStdout returns the last command's stdout
func (s *TestState) GetStdout() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastStdout
}

// GetStderr returns the last command's stderr
func (s *TestState) GetStderr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastStderr
}

// GetExitCode returns the last command's exit code
func (s *TestState) GetExitCode() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastExitCode
}

// GetCombinedOutput returns stdout and stderr combined
func (s *TestState) GetCombinedOutput() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastStdout + s.LastStderr
}

// GetDuration returns the test duration
func (s *TestState) GetDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// SetTestInfo sets the current test information
func (s *TestState) SetTestInfo(testID, testName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TestID = testID
	s.TestName = testName
}

// SetCLIBinaryPath sets the path to the CLI binary
func (s *TestState) SetCLIBinaryPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CLIBinaryPath = path
}

// SetCLICoverDir sets the directory for CLI coverage data
func (s *TestState) SetCLICoverDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CLICoverDir = dir
}

// SetGatewayInfo sets the gateway information
func (s *TestState) SetGatewayInfo(name, server string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GatewayName = name
	s.GatewayServer = server
}

// SetAPIInfo sets the API information
func (s *TestState) SetAPIInfo(name, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.APIName = name
	s.APIVersion = version
}

// SetMCPInfo sets the MCP information
func (s *TestState) SetMCPInfo(name, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MCPName = name
	s.MCPVersion = version
}

// GetConfigDir returns the path to the isolated test config directory
func (s *TestState) GetConfigDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ConfigDir
}
