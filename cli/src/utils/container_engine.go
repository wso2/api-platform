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

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Container engines that can run the gateway builder and build the gateway images.
const (
	ContainerEngineDocker  = "docker"
	ContainerEnginePodman  = "podman"
	ContainerEngineBuildah = "buildah"

	DefaultContainerEngine = ContainerEngineDocker

	// EnvContainerEngine selects the container engine when --container-engine is not set.
	EnvContainerEngine = "AP_CONTAINER_ENGINE"
)

// SupportedContainerEngines lists the accepted --container-engine values.
var SupportedContainerEngines = []string{ContainerEngineDocker, ContainerEnginePodman, ContainerEngineBuildah}

// runEngineCommand runs a command and returns its combined output. Tests replace it.
var runEngineCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// lookPath finds an engine on the PATH. Tests replace it.
var lookPath = exec.LookPath

// minBuildahMajor and minBuildahMinor are the oldest buildah release that fills
// in a name-only `buildah run -e NAME` from its own environment. Earlier releases
// drop such entries without an error, and the builder's Go settings travel that
// way so their values stay off the command line.
const (
	minBuildahMajor = 1
	minBuildahMinor = 30
)

// ResolveContainerEngine picks the container engine from the --container-engine
// flag value, then the AP_CONTAINER_ENGINE environment variable, then the
// default (docker). It returns the engine and a description of where it came from.
func ResolveContainerEngine(flagValue string) (engine string, source string, err error) {
	switch {
	case strings.TrimSpace(flagValue) != "":
		engine, source = flagValue, "--container-engine flag"
	case strings.TrimSpace(os.Getenv(EnvContainerEngine)) != "":
		engine, source = os.Getenv(EnvContainerEngine), EnvContainerEngine
	default:
		return DefaultContainerEngine, "default", nil
	}

	engine = strings.ToLower(strings.TrimSpace(engine))
	for _, supported := range SupportedContainerEngines {
		if engine == supported {
			return engine, source, nil
		}
	}
	return "", "", fmt.Errorf("unsupported container engine %q (from %s): supported engines are %s",
		engine, source, strings.Join(SupportedContainerEngines, ", "))
}

// CheckContainerEngine verifies that the engine can be used. When multiPlatform is
// true and the engine is docker, it also checks for docker buildx, which the
// --platform build path needs. buildah must be 1.30 or later.
func CheckContainerEngine(engine string, multiPlatform bool) error {
	out, err := runEngineCheck(engine, "version")
	if err != nil {
		return fmt.Errorf("%s is not available: %w%s", engine, err, otherEnginesHint(engine))
	}

	if engine == ContainerEngineBuildah {
		if err := checkBuildahVersion(out); err != nil {
			return fmt.Errorf("%w%s", err, otherEnginesHint(engine))
		}
	}

	if engine == ContainerEngineDocker && multiPlatform {
		if _, err := runEngineCheck(engine, "buildx", "version"); err != nil {
			return fmt.Errorf("docker buildx is not available but required for --platform: %w\n\n"+
				"Install docker buildx or remove the --platform flag.%s", err, otherEnginesHint(engine))
		}
	}
	return nil
}

// runEngineCheck runs an engine command and returns its output, or an error that
// ends with the engine's own last line of output.
func runEngineCheck(engine string, args ...string) ([]byte, error) {
	out, err := runEngineCommand(engine, args...)
	if err == nil {
		return out, nil
	}
	// A missing binary already has a clear error; otherwise add the engine's own last line of output.
	if detail := lastLine(string(out)); detail != "" {
		return nil, fmt.Errorf("%w: %s", err, detail)
	}
	return nil, err
}

// buildahVersion matches the "Version:" line of `buildah version`, not "Go Version:".
var buildahVersion = regexp.MustCompile(`(?m)^\s*Version:\s*v?(\d+)\.(\d+)\S*`)

// checkBuildahVersion fails for buildah releases older than 1.30, which drop the
// builder's Go settings (GOPRIVATE, GONOSUMDB, --go-env) without an error.
func checkBuildahVersion(versionOutput []byte) error {
	m := buildahVersion.FindSubmatch(versionOutput)
	if m == nil {
		return fmt.Errorf("could not read the buildah version from `buildah version`: buildah %d.%d or later is required",
			minBuildahMajor, minBuildahMinor)
	}
	major, _ := strconv.Atoi(string(m[1]))
	minor, _ := strconv.Atoi(string(m[2]))
	if major < minBuildahMajor || (major == minBuildahMajor && minor < minBuildahMinor) {
		return fmt.Errorf("buildah %d.%d is too old: buildah %d.%d or later is required, "+
			"because earlier releases do not pass the builder's Go settings (such as GOPRIVATE) into the build container",
			major, minor, minBuildahMajor, minBuildahMinor)
	}
	return nil
}

// otherEnginesHint suggests the other supported engines that are on the PATH,
// or returns "" when there are none.
func otherEnginesHint(engine string) string {
	var others []string
	for _, supported := range SupportedContainerEngines {
		if supported == engine {
			continue
		}
		if _, err := lookPath(supported); err == nil {
			others = append(others, supported)
		}
	}
	if len(others) == 0 {
		return ""
	}
	return fmt.Sprintf("\n\nTo use %s instead, set --container-engine or %s.", strings.Join(others, " or "), EnvContainerEngine)
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
