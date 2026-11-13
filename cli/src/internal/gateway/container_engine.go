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

package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/wso2/api-platform/cli/utils"
)

// containerEngine holds what differs between docker, podman and buildah.
// Building (`build -t`) and pushing (`push`) are the same for all of them and
// are run directly with binary().
type containerEngine interface {
	// binary is the executable to run.
	binary() string
	// relabelMounts reports whether bind mounts get an SELinux relabel option.
	relabelMounts() bool
	// credentialMount mounts the builder's credentials file in the engine's form.
	credentialMount() credentialMount
	// runBuilder runs the gateway-builder image with the build workspace mounted,
	// adding access's run arguments and environment.
	runBuilder(config DockerBuildConfig, access builderAccessRun, logFile *os.File) error
	// buildMultiPlatform builds and pushes one image for config.Platform.
	buildMultiPlatform(config DockerBuildConfig, imageTag, dir string, logFile *os.File) error
}

// engineFor returns the engine for a name; an empty name means docker.
func engineFor(name string) (containerEngine, error) {
	switch name {
	case "", utils.ContainerEngineDocker:
		return dockerEngine{}, nil
	case utils.ContainerEnginePodman:
		return podmanEngine{}, nil
	case utils.ContainerEngineBuildah:
		return buildahEngine{}, nil
	default:
		return nil, fmt.Errorf("unsupported container engine %q", name)
	}
}

// dockerEngine runs the builder with `docker run` and builds multi-platform
// images with buildx, which pushes them directly.
type dockerEngine struct{}

func (dockerEngine) binary() string { return utils.ContainerEngineDocker }

func (dockerEngine) relabelMounts() bool { return false }

func (e dockerEngine) credentialMount() credentialMount { return bindMount(e.relabelMounts()) }

func (e dockerEngine) runBuilder(config DockerBuildConfig, access builderAccessRun, logFile *os.File) error {
	return runBuilderProcess(e.binary(), builderRunArgs(config, e.relabelMounts(), access.args), access, logFile)
}

func (e dockerEngine) buildMultiPlatform(config DockerBuildConfig, imageTag, dir string, logFile *os.File) error {
	return runStreaming(e.binary(), e.multiPlatformArgs(config, imageTag), dir, logFile, "      ")
}

func (dockerEngine) multiPlatformArgs(config DockerBuildConfig, imageTag string) []string {
	args := []string{"buildx", "build", "--platform", config.Platform, "--push", "-t", imageTag}
	if config.NoCache {
		args = append(args, "--no-cache")
	}
	return append(args, ".")
}

// podmanEngine runs the builder with `podman run`, like docker, and builds
// multi-platform images as a manifest list.
type podmanEngine struct{}

func (podmanEngine) binary() string { return utils.ContainerEnginePodman }

func (podmanEngine) relabelMounts() bool { return selinuxRelabel() }

func (e podmanEngine) credentialMount() credentialMount { return bindMount(e.relabelMounts()) }

func (e podmanEngine) runBuilder(config DockerBuildConfig, access builderAccessRun, logFile *os.File) error {
	return runBuilderProcess(e.binary(), builderRunArgs(config, e.relabelMounts(), access.args), access, logFile)
}

func (e podmanEngine) buildMultiPlatform(config DockerBuildConfig, imageTag, dir string, logFile *os.File) error {
	return buildManifestList(e.binary(), config, imageTag, dir, logFile)
}

// buildahEngine runs the builder with buildah from/run/rm and builds
// multi-platform images as a manifest list, like podman.
type buildahEngine struct{}

func (buildahEngine) binary() string { return utils.ContainerEngineBuildah }

func (buildahEngine) relabelMounts() bool { return selinuxRelabel() }

func (e buildahEngine) credentialMount() credentialMount { return volumeMount(e.relabelMounts()) }

// runBuilder: buildah run doesn't apply the image's entrypoint, so it is read
// from the image and passed explicitly; it carries the -system-build-lock flag
// that adds the system policies.
//
// from and inspect run with the same signal handling as the builder: with
// --git-credential the credentials file already exists while from pulls the
// image, and an interrupt must not end ap before it is deleted. from shows the
// pull progress (stderr) like the docker and podman runs do.
func (e buildahEngine) runBuilder(config DockerBuildConfig, access builderAccessRun, logFile *os.File) error {
	out, err := runForwarding(e.binary(), []string{"from", config.GatewayBuilder}, access, logFile, true)
	if err != nil {
		return fmt.Errorf("failed to create a container from gateway-builder image %s: %w", config.GatewayBuilder, err)
	}
	container := lastLine(out)
	if container == "" {
		return fmt.Errorf("buildah from %s did not return a container name", config.GatewayBuilder)
	}
	// rm forwards signals too: the credentials file is deleted only after it.
	defer runQuiet(logFile, access.forwardSignals, e.binary(), "rm", container)

	// Read the full inspect JSON: older buildah releases (e.g. 1.39) don't support
	// template functions such as json in --format.
	out, err = runForwarding(e.binary(), []string{"inspect", "--type", "container", container}, access, logFile, true)
	if err != nil {
		return fmt.Errorf("failed to inspect the gateway-builder container: %w", err)
	}
	entrypoint, err := buildahEntrypoint([]byte(out))
	if err != nil {
		return fmt.Errorf("failed to read the entrypoint of gateway-builder image %s: %w", config.GatewayBuilder, err)
	}

	return runBuilderProcess(e.binary(), buildahRunArgs(config, container, entrypoint, e.relabelMounts(), access.args), access, logFile)
}

func (e buildahEngine) buildMultiPlatform(config DockerBuildConfig, imageTag, dir string, logFile *os.File) error {
	return buildManifestList(e.binary(), config, imageTag, dir, logFile)
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
func buildahRunArgs(config DockerBuildConfig, container string, entrypoint []string, relabel bool, accessArgs []string) []string {
	args := []string{"run", "--volume", workspaceVolume(config.TempDir, relabel), "--workingdir", builderWorkspace}
	args = append(args, accessArgs...)
	args = append(args, container, "--")
	args = append(args, entrypoint...)
	return append(args, builderFlags(config)...)
}

// builderRunArgs returns the `docker run` / `podman run` arguments for the gateway builder.
func builderRunArgs(config DockerBuildConfig, relabel bool, accessArgs []string) []string {
	args := []string{"run", "--rm", "-v", workspaceVolume(config.TempDir, relabel)}
	args = append(args, accessArgs...)
	args = append(args, config.GatewayBuilder)
	return append(args, builderFlags(config)...)
}

// builderFlags returns the gateway-builder's own flags, passed after the image.
func builderFlags(config DockerBuildConfig) []string {
	flags := []string{
		"-gateway-controller-base-image", config.GatewayControllerBaseImage,
		"-gateway-runtime-base-image", config.GatewayRuntimeBaseImage,
	}
	// Passed only when set: builder images older than -fetch-timeout reject unknown flags,
	// so a default build keeps working with every builder version.
	if config.FetchTimeout > 0 {
		flags = append(flags, "-fetch-timeout", config.FetchTimeout.String())
	}
	return flags
}

// workspaceVolume returns the bind mount for the build workspace, with the :Z
// SELinux relabel when relabel is set.
func workspaceVolume(dir string, relabel bool) string {
	volume := dir + ":" + builderWorkspace
	if relabel {
		volume += ":Z"
	}
	return volume
}

// selinuxRelabel reports whether podman and buildah mounts get :Z. They usually
// run on SELinux-enforcing Linux hosts (RHEL, Fedora); the option is ignored
// where SELinux is disabled, and not used on macOS or Windows.
func selinuxRelabel() bool {
	return hostOS == "linux"
}

// buildManifestList builds config.Platform into a local manifest list and
// pushes the list, with all its images, to imageTag. The local list gets a
// unique name so that concurrent builds of the same tag never add images to
// each other's list.
func buildManifestList(binary string, config DockerBuildConfig, imageTag, dir string, logFile *os.File) error {
	listName, err := localManifestListName()
	if err != nil {
		return err
	}
	defer runQuiet(logFile, false, binary, "manifest", "rm", listName)

	for _, args := range manifestListCommands(config, listName, imageTag) {
		if err := runStreaming(binary, args, dir, logFile, "      "); err != nil {
			return err
		}
	}
	return nil
}

func manifestListCommands(config DockerBuildConfig, listName, imageTag string) [][]string {
	build := []string{"build", "--platform", config.Platform, "--manifest", listName}
	if config.NoCache {
		build = append(build, "--no-cache")
	}
	build = append(build, ".")
	push := []string{"manifest", "push", "--all", listName, "docker://" + imageTag}
	return [][]string{build, push}
}

// localManifestListName returns a random name for a local manifest list.
func localManifestListName() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate a manifest list name: %w", err)
	}
	return "localhost/wso2ap-gateway-build-" + hex.EncodeToString(b), nil
}

// runQuiet runs a best-effort command with its output in the log file only,
// forwarding signals to it when forward is set.
func runQuiet(logFile *os.File, forward bool, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return
	}
	if forward {
		defer forwardSignals(cmd.Process)()
	}
	_ = cmd.Wait()
}
