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

package gateway

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wso2/api-platform/cli/utils"
)

// DockerBuildConfig holds configuration for building gateway images
type DockerBuildConfig struct {
	TempDir                    string
	GatewayBuilder             string
	GatewayControllerBaseImage string
	RouterBaseImage            string
	ImageRepository            string
	GatewayName                string
	GatewayVersion             string
	Platform                   string
	NoCache                    bool
	Push                       bool
	LogFilePath                string
	OutputCopyDir              string
}

// BuildGatewayImages executes the docker build process for gateway images
// This is the main function that orchestrates the entire build process
func BuildGatewayImages(config DockerBuildConfig) error {
	logFile, err := os.Create(config.LogFilePath)
	if err != nil {
		return fmt.Errorf("failed to create docker log file: %w", err)
	}
	defer logFile.Close()

	// Step 1: Run gateway-builder container
	fmt.Println("  → Running gateway-builder container...")
	if err := runGatewayBuilder(config, logFile); err != nil {
		return fmt.Errorf("failed to run gateway-builder: %w\n\nCheck logs at: %s", err, config.LogFilePath)
	}

	if config.OutputCopyDir != "" {
		outputSrc := filepath.Join(config.TempDir, "output")
		if _, err := os.Stat(outputSrc); err == nil {
			if err := utils.CopyDir(outputSrc, config.OutputCopyDir); err != nil {
				return fmt.Errorf("failed to copy workspace output to %s: %w", config.OutputCopyDir, err)
			}
			fmt.Printf("  ✓ Workspace output copied to: %s\n", config.OutputCopyDir)
		}
	}

	fmt.Println("  ✓ Gateway-builder completed")

	// Step 2: Build the three images
	components := []string{"policy-engine", "gateway-controller", "router"}

	if config.Platform != "" {
		// Use docker buildx for cross-platform builds
		if err := buildWithBuildx(config, components, logFile); err != nil {
			return err
		}
	} else {
		// Regular docker build
		if err := buildWithDocker(config, components, logFile); err != nil {
			return err
		}

		// Push images if requested
		if config.Push {
			if err := pushImages(config, components, logFile); err != nil {
				return err
			}
		}
	}

	return nil
}

// runGatewayBuilder runs the gateway-builder container
func runGatewayBuilder(config DockerBuildConfig, logFile *os.File) error {
	args := []string{"run", "--rm", "-v", config.TempDir + ":/workspace", config.GatewayBuilder}

	if config.GatewayControllerBaseImage != "" {
		args = append(args, "-gateway-controller-base-image", config.GatewayControllerBaseImage)
	}
	if config.RouterBaseImage != "" {
		args = append(args, "-router-base-image", config.RouterBaseImage)
	}

	cmd := exec.Command("docker", args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker command failed: %w", err)
	}
	return nil
}

// buildWithBuildx builds images using docker buildx for cross-platform
func buildWithBuildx(config DockerBuildConfig, components []string, logFile *os.File) error {
	fmt.Println("  → Building and pushing images with buildx (platform: " + config.Platform + ")...")

	for _, component := range components {
		imageTag := fmt.Sprintf("%s/%s-%s:%s", config.ImageRepository, config.GatewayName, component, config.GatewayVersion)
		fmt.Printf("    → Building %s...\n", component)

		componentDir := filepath.Join(config.TempDir, "output", component)
		args := []string{"buildx", "build", "--platform", config.Platform, "--push", "-t", imageTag}

		if config.NoCache {
			args = append(args, "--no-cache")
		}
		args = append(args, ".")

		cmd := exec.Command("docker", args...)
		cmd.Dir = componentDir
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
		}
		fmt.Printf("    ✓ Built and pushed %s\n", imageTag)
	}

	return nil
}

// buildWithDocker builds images using regular docker build
func buildWithDocker(config DockerBuildConfig, components []string, logFile *os.File) error {
	fmt.Println("  → Building images...")

	for _, component := range components {
		imageTag := fmt.Sprintf("%s/%s-%s:%s", config.ImageRepository, config.GatewayName, component, config.GatewayVersion)
		fmt.Printf("    → Building %s...\n", component)

		componentDir := filepath.Join(config.TempDir, "output", component)
		args := []string{"build", "-t", imageTag}

		if config.NoCache {
			args = append(args, "--no-cache")
		}
		args = append(args, ".")

		cmd := exec.Command("docker", args...)
		cmd.Dir = componentDir
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
		}
		fmt.Printf("    ✓ Built %s\n", imageTag)
	}

	return nil
}

// pushImages pushes the built images to the registry
func pushImages(config DockerBuildConfig, components []string, logFile *os.File) error {
	fmt.Println("  → Pushing images...")

	for _, component := range components {
		imageTag := fmt.Sprintf("%s/%s-%s:%s", config.ImageRepository, config.GatewayName, component, config.GatewayVersion)
		fmt.Printf("    → Pushing %s...\n", component)

		cmd := exec.Command("docker", "push", imageTag)
		cmd.Stdout = logFile
		cmd.Stderr = logFile

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to push %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
		}
		fmt.Printf("    ✓ Pushed %s\n", imageTag)
	}

	return nil
}
