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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wso2/api-platform/cli/internal/terminal"
	"github.com/wso2/api-platform/cli/utils"
)

// builderWorkspace is where the gateway builder expects the build workspace to be mounted.
const builderWorkspace = "/workspace"

// hostOS is the operating system the CLI runs on. Tests replace it.
var hostOS = runtime.GOOS

// DockerBuildConfig holds configuration for building gateway images
type DockerBuildConfig struct {
	TempDir                    string
	GatewayBuilder             string
	GatewayControllerBaseImage string
	GatewayRuntimeBaseImage    string
	ImageRepository            string
	GatewayName                string
	GatewayVersion             string
	Platform                   string
	NoCache                    bool
	Push                       bool
	LogFilePath                string
	OutputCopyDir              string
	// ContainerEngine is docker, podman or buildah. Empty means docker.
	ContainerEngine string
}

func (c DockerBuildConfig) engine() string {
	if c.ContainerEngine == "" {
		return utils.DefaultContainerEngine
	}
	return c.ContainerEngine
}

func (c DockerBuildConfig) imageTag(component string) string {
	return fmt.Sprintf("%s/%s-%s:%s", c.ImageRepository, c.GatewayName, component, c.GatewayVersion)
}

// BuildGatewayImages executes the build process for gateway images
// This is the main function that orchestrates the entire build process
func BuildGatewayImages(config DockerBuildConfig) error {
	logFile, err := os.Create(config.LogFilePath)
	if err != nil {
		return fmt.Errorf("failed to create build log file: %w", err)
	}
	defer logFile.Close()

	// Step 1: Run gateway-builder
	fmt.Println("  → Running gateway-builder...")
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

	// Step 2: Build the two images
	components := []string{"gateway-runtime", "gateway-controller"}

	if config.Platform != "" {
		// Cross-platform builds always push: docker buildx, or a manifest list with podman/buildah
		return buildMultiPlatform(config, components, logFile)
	}

	if err := buildImages(config, components, logFile); err != nil {
		return err
	}
	if config.Push {
		return pushImages(config, components, logFile)
	}
	return nil
}

// runGatewayBuilder runs the gateway-builder container with the workspace mounted
func runGatewayBuilder(config DockerBuildConfig, logFile *os.File) error {
	if config.engine() == utils.ContainerEngineBuildah {
		return runGatewayBuilderWithBuildah(config, logFile)
	}
	return runStreaming(config.engine(), builderRunArgs(config), "", logFile, "    ")
}

// builderRunArgs returns the `docker run` / `podman run` arguments for the gateway builder.
func builderRunArgs(config DockerBuildConfig) []string {
	args := []string{"run", "--rm", "-v", workspaceVolume(config), config.GatewayBuilder}
	return append(args, builderFlags(config)...)
}

// runGatewayBuilderWithBuildah runs the builder with buildah from/run/rm.
// buildah run doesn't apply the image's entrypoint, so it is read from the image
// and passed explicitly; it carries the -system-build-lock flag that adds the
// system policies.
func runGatewayBuilderWithBuildah(config DockerBuildConfig, logFile *os.File) error {
	out, err := runOutput(logFile, "buildah", "from", config.GatewayBuilder)
	if err != nil {
		return fmt.Errorf("buildah from %s failed: %w", config.GatewayBuilder, err)
	}
	container := lastLine(out)
	if container == "" {
		return fmt.Errorf("buildah from %s did not return a container name", config.GatewayBuilder)
	}
	defer func() {
		cmd := exec.Command("buildah", "rm", container)
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		_ = cmd.Run()
	}()

	// Read the full inspect JSON: older buildah releases (e.g. 1.39) don't support
	// template functions such as json in --format.
	out, err = runOutput(logFile, "buildah", "inspect", "--type", "container", container)
	if err != nil {
		return fmt.Errorf("failed to inspect the gateway-builder container: %w", err)
	}
	entrypoint, err := buildahEntrypoint([]byte(out))
	if err != nil {
		return fmt.Errorf("failed to read the entrypoint of gateway-builder image %s: %w", config.GatewayBuilder, err)
	}

	return runStreaming("buildah", buildahRunArgs(config, container, entrypoint), "", logFile, "    ")
}

// buildahEntrypoint returns the image entrypoint from `buildah inspect --type container` output.
func buildahEntrypoint(inspectJSON []byte) ([]string, error) {
	var info struct {
		OCIv1 struct {
			Config struct {
				Entrypoint []string `json:"Entrypoint"`
			} `json:"config"`
		} `json:"OCIv1"`
	}
	if err := json.Unmarshal(inspectJSON, &info); err != nil {
		return nil, fmt.Errorf("invalid buildah inspect output: %w", err)
	}
	if len(info.OCIv1.Config.Entrypoint) == 0 {
		return nil, fmt.Errorf("image has no entrypoint")
	}
	return info.OCIv1.Config.Entrypoint, nil
}

// buildahRunArgs returns the `buildah run` arguments for the gateway builder.
func buildahRunArgs(config DockerBuildConfig, container string, entrypoint []string) []string {
	args := []string{"run", "--volume", workspaceVolume(config), "--workingdir", builderWorkspace, container, "--"}
	args = append(args, entrypoint...)
	return append(args, builderFlags(config)...)
}

func builderFlags(config DockerBuildConfig) []string {
	return []string{
		"-gateway-controller-base-image", config.GatewayControllerBaseImage,
		"-gateway-runtime-base-image", config.GatewayRuntimeBaseImage,
	}
}

// workspaceVolume returns the bind mount for the build workspace. podman and buildah
// get the :Z SELinux relabel on Linux, where they usually run on SELinux-enforcing
// hosts (RHEL, Fedora); the option is ignored where SELinux is disabled.
func workspaceVolume(config DockerBuildConfig) string {
	volume := config.TempDir + ":" + builderWorkspace
	if hostOS == "linux" && config.engine() != utils.ContainerEngineDocker {
		volume += ":Z"
	}
	return volume
}

// buildImages builds the images for the host platform
func buildImages(config DockerBuildConfig, components []string, logFile *os.File) error {
	fmt.Println("  → Building images...")

	for _, component := range components {
		imageTag := config.imageTag(component)
		fmt.Printf("    → Building %s...\n", component)

		componentDir := filepath.Join(config.TempDir, "output", component)
		if err := runStreaming(config.engine(), buildArgs(config, imageTag), componentDir, logFile, "      "); err != nil {
			return fmt.Errorf("failed to build %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
		}
		fmt.Printf("    ✓ Built %s\n", imageTag)
	}

	return nil
}

// buildArgs returns the image build arguments; docker, podman and buildah share them.
func buildArgs(config DockerBuildConfig, imageTag string) []string {
	args := []string{"build", "-t", imageTag}
	if config.NoCache {
		args = append(args, "--no-cache")
	}
	return append(args, ".")
}

// buildMultiPlatform builds and pushes images for the --platform list
func buildMultiPlatform(config DockerBuildConfig, components []string, logFile *os.File) error {
	fmt.Println("  → Building and pushing images (platform: " + config.Platform + ")...")

	for _, component := range components {
		imageTag := config.imageTag(component)
		fmt.Printf("    → Building %s...\n", component)

		componentDir := filepath.Join(config.TempDir, "output", component)
		if config.engine() != utils.ContainerEngineDocker {
			// --manifest adds to an existing list of the same name, so drop any stale one first
			cmd := exec.Command(config.engine(), "manifest", "rm", imageTag)
			cmd.Stdout = logFile
			cmd.Stderr = logFile
			_ = cmd.Run()
		}

		for _, args := range multiPlatformCommands(config, imageTag) {
			if err := runStreaming(config.engine(), args, componentDir, logFile, "      "); err != nil {
				return fmt.Errorf("failed to build %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
			}
		}
		fmt.Printf("    ✓ Built and pushed %s\n", imageTag)
	}

	return nil
}

// multiPlatformCommands returns the commands that build and push one image for
// config.Platform: docker buildx pushes directly; podman and buildah build a
// manifest list and push it with all its images.
func multiPlatformCommands(config DockerBuildConfig, imageTag string) [][]string {
	if config.engine() == utils.ContainerEngineDocker {
		args := []string{"buildx", "build", "--platform", config.Platform, "--push", "-t", imageTag}
		if config.NoCache {
			args = append(args, "--no-cache")
		}
		return [][]string{append(args, ".")}
	}

	build := []string{"build", "--platform", config.Platform, "--manifest", imageTag}
	if config.NoCache {
		build = append(build, "--no-cache")
	}
	build = append(build, ".")
	push := []string{"manifest", "push", "--all", imageTag, "docker://" + imageTag}
	return [][]string{build, push}
}

// pushImages pushes the built images to the registry
func pushImages(config DockerBuildConfig, components []string, logFile *os.File) error {
	fmt.Println("  → Pushing images...")

	for _, component := range components {
		imageTag := config.imageTag(component)
		fmt.Printf("    → Pushing %s...\n", component)

		if err := runStreaming(config.engine(), []string{"push", imageTag}, "", logFile, "      "); err != nil {
			return fmt.Errorf("failed to push %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
		}
		fmt.Printf("    ✓ Pushed %s\n", imageTag)
	}

	return nil
}

// runStreaming runs a command with its output scrolled on a TTY and written to the log file.
func runStreaming(name string, args []string, dir string, logFile *os.File, prefix string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	// Setup scrolling output - auto-detects TTY internally, falls back to file-only if not TTY
	scroller := terminal.NewScrollingLogger(terminal.ScrollingLoggerConfig{
		LogFile: logFile,
		Prefix:  prefix,
	})
	cmd.Stdout = scroller
	cmd.Stderr = scroller
	scroller.Start()

	err := cmd.Run()
	scroller.Stop()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w", name, args[0], err)
	}

	// Clear the scrolled logs before the caller prints its success message
	scroller.ClearDisplay()
	return nil
}

// runOutput runs a command and returns its stdout; stderr goes to the log file.
func runOutput(logFile *os.File, name string, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = logFile
	err := cmd.Run()
	return stdout.String(), err
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
