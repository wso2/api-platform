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

package docker

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

// ExecuteDockerCommand runs a docker command with proper error handling
func ExecuteDockerCommand(args ...string) error {
	cmd := exec.Command("docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	slog.Debug("Executing docker command",
		"command", fmt.Sprintf("docker %s", strings.Join(args, " ")))

	if err := cmd.Run(); err != nil {
		slog.Error("Docker command failed",
			"command", args[0],
			"stdout", stdout.String(),
			"stderr", stderr.String(),
			"error", err)
		return fmt.Errorf("docker %s failed: %w\nStderr: %s",
			args[0], err, stderr.String())
	}

	slog.Debug("Docker command succeeded",
		"command", args[0],
		"output", stdout.String())

	return nil
}

// CheckDockerAvailable verifies docker CLI is available and daemon is running
func CheckDockerAvailable() error {
	// Check if docker command exists
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker command not found in PATH: %w", err)
	}

	// Verify docker daemon is running
	cmd := exec.Command("docker", "info")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker daemon not running: %w\nStderr: %s", err, stderr.String())
	}

	slog.Debug("Docker is available and daemon is running")
	return nil
}
