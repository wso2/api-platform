/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package docker

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckDockerAvailable_DockerNotInPath(t *testing.T) {
	// Save original PATH and set to empty
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", originalPath)

	err := CheckDockerAvailable()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker command not found in PATH")
}

func TestExecuteDockerCommand_DockerNotInPath(t *testing.T) {
	// Save original PATH and set to empty
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", originalPath)

	err := ExecuteDockerCommand("version")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "docker version failed")
}

func TestExecuteDockerCommand_InvalidSubcommand(t *testing.T) {
	// Skip if docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not installed, skipping test")
	}

	err := ExecuteDockerCommand("nonexistent-command-that-does-not-exist")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed")
}

func TestCheckDockerAvailable_Success(t *testing.T) {
	// Skip if docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not installed, skipping test")
	}

	err := CheckDockerAvailable()

	// Accept success or daemon-not-running error; fail on unexpected errors
	if err != nil && !strings.Contains(err.Error(), "daemon") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestExecuteDockerCommand_Version(t *testing.T) {
	// Skip if docker is not available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not installed, skipping test")
	}

	// docker version should work even without daemon
	err := ExecuteDockerCommand("--version")

	// This should succeed as --version doesn't need daemon
	assert.NoError(t, err)
}

// lookupPath is a helper to check if a command exists in PATH
func lookupPath(name string) (string, error) {
	path := os.Getenv("PATH")
	if path == "" {
		return "", os.ErrNotExist
	}
	for _, dir := range splitPath(path) {
		fullPath := dir + "/" + name
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}
	return "", os.ErrNotExist
}

// splitPath splits PATH by the OS-specific separator
func splitPath(path string) []string {
	var result []string
	var current string
	for _, c := range path {
		if c == ':' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
