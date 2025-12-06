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
