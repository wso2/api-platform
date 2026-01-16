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

package steps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AssertSteps provides assertion step definitions
type AssertSteps struct {
	state TestState
}

// NewAssertSteps creates a new AssertSteps instance
func NewAssertSteps(state TestState) *AssertSteps {
	return &AssertSteps{state: state}
}

// ExitCodeShouldBe asserts the exit code equals expected
func (s *AssertSteps) ExitCodeShouldBe(expected int) error {
	actual := s.state.GetExitCode()
	if actual != expected {
		return fmt.Errorf("expected exit code %d, got %d\nStdout: %s\nStderr: %s",
			expected, actual, s.state.GetStdout(), s.state.GetStderr())
	}
	return nil
}

// OutputShouldContain asserts the combined output contains text
func (s *AssertSteps) OutputShouldContain(text string) error {
	output := s.state.GetCombinedOutput()
	if !strings.Contains(output, text) {
		return fmt.Errorf("expected output to contain %q, got:\n%s", text, output)
	}
	return nil
}

// OutputShouldNotContain asserts the combined output does not contain text
func (s *AssertSteps) OutputShouldNotContain(text string) error {
	output := s.state.GetCombinedOutput()
	if strings.Contains(output, text) {
		return fmt.Errorf("expected output to not contain %q, but it did:\n%s", text, output)
	}
	return nil
}

// StdoutShouldContain asserts stdout contains text
func (s *AssertSteps) StdoutShouldContain(text string) error {
	stdout := s.state.GetStdout()
	if !strings.Contains(stdout, text) {
		return fmt.Errorf("expected stdout to contain %q, got:\n%s", text, stdout)
	}
	return nil
}

// StderrShouldContain asserts stderr contains text
func (s *AssertSteps) StderrShouldContain(text string) error {
	stderr := s.state.GetStderr()
	if !strings.Contains(stderr, text) {
		return fmt.Errorf("expected stderr to contain %q, got:\n%s", text, stderr)
	}
	return nil
}

// CommandShouldSucceed asserts the command exited with code 0
func (s *AssertSteps) CommandShouldSucceed() error {
	return s.ExitCodeShouldBe(0)
}

// CommandShouldFail asserts the command exited with non-zero code
func (s *AssertSteps) CommandShouldFail() error {
	actual := s.state.GetExitCode()
	if actual == 0 {
		return fmt.Errorf("expected command to fail, but it succeeded\nStdout: %s", s.state.GetStdout())
	}
	return nil
}

// GatewayShouldExist asserts the gateway exists in the list
func (s *AssertSteps) GatewayShouldExist(name string) error {
	// Run gateway list and check for the name
	if err := s.state.ExecuteCLI("gateway", "list"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if !strings.Contains(output, name) {
		return fmt.Errorf("expected gateway %q to exist, but not found in:\n%s", name, output)
	}
	return nil
}

// GatewayShouldNotExist asserts the gateway does not exist
func (s *AssertSteps) GatewayShouldNotExist(name string) error {
	// Run gateway list and check for the name
	if err := s.state.ExecuteCLI("gateway", "list"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if strings.Contains(output, name) {
		return fmt.Errorf("expected gateway %q to not exist, but found in:\n%s", name, output)
	}
	return nil
}

// CurrentGatewayShouldBe asserts the current gateway matches
func (s *AssertSteps) CurrentGatewayShouldBe(name string) error {
	if err := s.state.ExecuteCLI("gateway", "current"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if !strings.Contains(output, name) {
		return fmt.Errorf("expected current gateway to be %q, got:\n%s", name, output)
	}
	return nil
}

// APIShouldBeDeployed asserts the API is deployed
func (s *AssertSteps) APIShouldBeDeployed(name string) error {
	if err := s.state.ExecuteCLI("gateway", "api", "list"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if !strings.Contains(output, name) {
		return fmt.Errorf("expected API %q to be deployed, but not found in:\n%s", name, output)
	}
	return nil
}

// APIShouldNotExist asserts the API does not exist
func (s *AssertSteps) APIShouldNotExist(name string) error {
	if err := s.state.ExecuteCLI("gateway", "api", "list"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if strings.Contains(output, name) {
		return fmt.Errorf("expected API %q to not exist, but found in:\n%s", name, output)
	}
	return nil
}

// MCPShouldBeDeployed asserts the MCP is deployed
func (s *AssertSteps) MCPShouldBeDeployed(name string) error {
	if err := s.state.ExecuteCLI("gateway", "mcp", "list"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if !strings.Contains(output, name) {
		return fmt.Errorf("expected MCP %q to be deployed, but not found in:\n%s", name, output)
	}
	return nil
}

// MCPShouldNotExist asserts the MCP does not exist
func (s *AssertSteps) MCPShouldNotExist(name string) error {
	if err := s.state.ExecuteCLI("gateway", "mcp", "list"); err != nil {
		return err
	}

	output := s.state.GetStdout()
	if strings.Contains(output, name) {
		return fmt.Errorf("expected MCP %q to not exist, but found in:\n%s", name, output)
	}
	return nil
}

// MCPConfigShouldBeGenerated asserts the MCP config was generated
func (s *AssertSteps) MCPConfigShouldBeGenerated() error {
	if s.state.GetExitCode() != 0 {
		return fmt.Errorf("MCP config generation failed: %s", s.state.GetCombinedOutput())
	}

	// Check for success message in output
	output := s.state.GetCombinedOutput()
	if !strings.Contains(strings.ToLower(output), "success") &&
		!strings.Contains(strings.ToLower(output), "generated") &&
		!strings.Contains(strings.ToLower(output), "created") {
		// Check if output file was created by looking at the output
		if !strings.Contains(output, ".yaml") && !strings.Contains(output, ".yml") {
			return fmt.Errorf("MCP config generation may have failed, output: %s", output)
		}
	}

	return nil
}

// BuildShouldComplete asserts the gateway build completed successfully
func (s *AssertSteps) BuildShouldComplete() error {
	if s.state.GetExitCode() != 0 {
		return fmt.Errorf("gateway build failed with exit code %d: %s",
			s.state.GetExitCode(), s.state.GetCombinedOutput())
	}
	return nil
}

// FileShouldExist asserts a file exists
func (s *AssertSteps) FileShouldExist(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("expected file to exist: %s", absPath)
	}
	return nil
}

// FileShouldNotExist asserts a file does not exist
func (s *AssertSteps) FileShouldNotExist(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); err == nil {
		return fmt.Errorf("expected file to not exist: %s", absPath)
	}
	return nil
}

// OutputShouldMatchPattern asserts output matches a pattern
func (s *AssertSteps) OutputShouldMatchPattern(pattern string) error {
	output := s.state.GetCombinedOutput()
	// Simple pattern matching - could be enhanced with regex
	if !strings.Contains(output, pattern) {
		return fmt.Errorf("expected output to match pattern %q, got:\n%s", pattern, output)
	}
	return nil
}
