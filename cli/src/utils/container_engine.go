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
	"strings"
)

// Container engines that can run the gateway builder and build the gateway images.
const (
	ContainerEngineDocker  = "docker"
	ContainerEnginePodman  = "podman"
	ContainerEngineBuildah = "buildah"

	DefaultContainerEngine = ContainerEngineDocker

	// EnvContainerEngine selects the container engine when --container-engine is not set.
	EnvContainerEngine = "WSO2AP_CONTAINER_ENGINE"
)

// SupportedContainerEngines lists the accepted --container-engine values.
var SupportedContainerEngines = []string{ContainerEngineDocker, ContainerEnginePodman, ContainerEngineBuildah}

// runEngineCommand runs a command and returns its combined output. Tests replace it.
var runEngineCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// ResolveContainerEngine picks the container engine from the --container-engine
// flag value, then the WSO2AP_CONTAINER_ENGINE environment variable, then the
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
// --platform build path needs.
func CheckContainerEngine(engine string, multiPlatform bool) error {
	if err := runEngineCheck(engine, "version"); err != nil {
		return fmt.Errorf("%s is not available: %w\n\nTo use %s instead, set --container-engine or %s.",
			engine, err, otherContainerEngines(engine), EnvContainerEngine)
	}

	if engine == ContainerEngineDocker && multiPlatform {
		if err := runEngineCheck(engine, "buildx", "version"); err != nil {
			return fmt.Errorf("docker buildx is not available but required for --platform: %w\n\n"+
				"Install docker buildx, remove the --platform flag, or use podman or buildah by setting --container-engine or %s.",
				err, EnvContainerEngine)
		}
	}
	return nil
}

func runEngineCheck(engine string, args ...string) error {
	out, err := runEngineCommand(engine, args...)
	if err == nil {
		return nil
	}
	// A missing binary already has a clear error; otherwise add the engine's own last line of output.
	if detail := lastLine(string(out)); detail != "" {
		return fmt.Errorf("%w: %s", err, detail)
	}
	return err
}

func otherContainerEngines(engine string) string {
	var others []string
	for _, supported := range SupportedContainerEngines {
		if supported != engine {
			others = append(others, supported)
		}
	}
	return strings.Join(others, " or ")
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
