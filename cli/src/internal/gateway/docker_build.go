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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

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
	BuilderAccess              BuilderAccess // Go settings and credentials for the builder container
	FetchTimeout               time.Duration // per gomodule policy download; 0 leaves the builder default
	// ContainerEngine is docker, podman or buildah. Empty means docker.
	ContainerEngine string
}

func (c DockerBuildConfig) imageTag(component string) string {
	return fmt.Sprintf("%s/%s-%s:%s", c.ImageRepository, c.GatewayName, component, c.GatewayVersion)
}

// BuildGatewayImages executes the build process for gateway images
// This is the main function that orchestrates the entire build process
func BuildGatewayImages(config DockerBuildConfig) error {
	engine, err := engineFor(config.ContainerEngine)
	if err != nil {
		return err
	}

	logFile, err := os.Create(config.LogFilePath)
	if err != nil {
		return fmt.Errorf("failed to create build log file: %w", err)
	}
	defer logFile.Close()

	// Step 1: Run gateway-builder
	fmt.Println("  → Running gateway-builder...")
	if err := runGatewayBuilder(config, engine, logFile); err != nil {
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
		return buildMultiPlatform(config, engine, components, logFile)
	}

	if err := buildImages(config, engine, components, logFile); err != nil {
		return err
	}
	if config.Push {
		return pushImages(config, engine, components, logFile)
	}
	return nil
}

// runGatewayBuilder prepares the builder's Go settings and credentials and runs
// the gateway-builder with the engine.
func runGatewayBuilder(config DockerBuildConfig, engine containerEngine, logFile *os.File) error {
	// Credentials live next to the workspace, never inside it: the workspace output
	// is copied into the gateway images and, with --output-dir, out to the user.
	scratchParent := filepath.Dir(config.TempDir)
	sweepStaleCredentialDirs(scratchParent, processAlive, time.Now())
	accessArgs, env, cleanup, err := config.BuilderAccess.prepare(scratchParent, os.Environ(), engine.credentialMount())
	if err != nil {
		return err
	}
	defer cleanup()

	return engine.runBuilder(config, builderAccessRun{
		args:   accessArgs,
		env:    env,
		access: config.BuilderAccess,
	}, logFile)
}

// builderAccessRun is what the engine adds to the builder run for the Go settings
// and credentials: extra run arguments and the environment for the engine
// process (nil to inherit).
type builderAccessRun struct {
	args   []string
	env    []string
	access BuilderAccess // for the --netrc mount hint
}

// runBuilderProcess runs the command that executes the gateway builder, with its
// output scrolled on a TTY and written to the log file. A failure that means the
// container could not be created gets the --netrc mount hint.
func runBuilderProcess(name string, args []string, access builderAccessRun, logFile *os.File) error {
	_, err := runForwarding(name, args, access, logFile, false)
	if err != nil {
		return fmt.Errorf("%w%s", err, netrcMountHint(err, access.access))
	}
	return nil
}

// runForwarding runs a command with its output scrolled on a TTY and written to
// the log file, forwarding signals to it. With captureStdout, stdout is returned
// instead of shown.
//
// Signals are always forwarded, so an interrupt never ends ap before its deferred
// cleanup: deleting a generated --git-credential netrc, and removing buildah's
// working container, which buildah run (unlike docker and podman run --rm)
// leaves behind.
func runForwarding(name string, args []string, access builderAccessRun, logFile *os.File, captureStdout bool) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = access.env

	// Setup scrolling output - auto-detects TTY internally, falls back to file-only if not TTY
	scroller := terminal.NewScrollingLogger(terminal.ScrollingLoggerConfig{
		LogFile: logFile,
		Prefix:  "    ",
	})
	var stdout bytes.Buffer
	cmd.Stdout = scroller
	if captureStdout {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = scroller
	scroller.Start()
	defer scroller.Stop()

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("%s %s failed: %w", name, args[0], err)
	}
	defer forwardSignals(cmd.Process)()
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("%s %s failed: %w", name, args[0], err)
	}

	// Clear the scrolled logs before returning success
	scroller.ClearDisplay()
	return stdout.String(), nil
}

// runCreateFailed is the exit code `docker run`, `podman run` and `buildah run`
// return when the container cannot be created, for example when a bind mount
// source does not exist.
const runCreateFailed = 125

// netrcMountHint explains the usual reason a --netrc mount fails: Docker Desktop,
// Colima and podman machine share only some host directories (the home directory
// by default) with their VM, so a file elsewhere does not exist from the engine's side.
func netrcMountHint(err error, access BuilderAccess) string {
	var exitErr *exec.ExitError
	if access.NetrcFile == "" || !errors.As(err, &exitErr) || exitErr.ExitCode() != runCreateFailed {
		return ""
	}
	return "\n\nThe container engine could not create the builder container. If the log says the --netrc file does not exist " +
		"(\"bind source path does not exist\" or \"no such file or directory\"), move it under your home directory: " +
		"Docker Desktop, Colima and podman machine share it with their VM by default, but not every other host path."
}

// forwardSignals passes interrupt, terminate and hangup (terminal closed) signals to
// p instead of letting them end ap, so deferred cleanup still runs once p exits. The
// returned func stops forwarding.
func forwardSignals(p *os.Process) (stop func()) {
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for {
			select {
			case sig := <-sigs:
				_ = p.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(sigs)
		close(done)
	}
}

// buildImages builds the images for the host platform
func buildImages(config DockerBuildConfig, engine containerEngine, components []string, logFile *os.File) error {
	fmt.Println("  → Building images...")

	for _, component := range components {
		imageTag := config.imageTag(component)
		fmt.Printf("    → Building %s...\n", component)

		componentDir := filepath.Join(config.TempDir, "output", component)
		if err := runStreaming(engine.binary(), buildArgs(config, imageTag), componentDir, logFile, "      "); err != nil {
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
func buildMultiPlatform(config DockerBuildConfig, engine containerEngine, components []string, logFile *os.File) error {
	fmt.Println("  → Building and pushing images (platform: " + config.Platform + ")...")

	for _, component := range components {
		imageTag := config.imageTag(component)
		fmt.Printf("    → Building %s...\n", component)

		componentDir := filepath.Join(config.TempDir, "output", component)
		if err := engine.buildMultiPlatform(config, imageTag, componentDir, logFile); err != nil {
			return fmt.Errorf("failed to build %s: %w\n\nCheck logs at: %s", component, err, config.LogFilePath)
		}
		fmt.Printf("    ✓ Built and pushed %s\n", imageTag)
	}

	return nil
}

// pushImages pushes the built images to the registry
func pushImages(config DockerBuildConfig, engine containerEngine, components []string, logFile *os.File) error {
	fmt.Println("  → Pushing images...")

	for _, component := range components {
		imageTag := config.imageTag(component)
		fmt.Printf("    → Pushing %s...\n", component)

		if err := runStreaming(engine.binary(), []string{"push", imageTag}, "", logFile, "      "); err != nil {
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

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
