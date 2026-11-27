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
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestResolveContainerEngine(t *testing.T) {
	tests := []struct {
		name       string
		flag       string
		env        string
		wantEngine string
		wantSource string
		wantErr    string
	}{
		{name: "default is docker", wantEngine: "docker", wantSource: "default"},
		{name: "env var", env: "podman", wantEngine: "podman", wantSource: EnvContainerEngine},
		{name: "flag", flag: "buildah", wantEngine: "buildah", wantSource: "--container-engine flag"},
		{name: "flag wins over env var", flag: "buildah", env: "podman", wantEngine: "buildah", wantSource: "--container-engine flag"},
		{name: "case and spaces are normalized", flag: " Podman ", wantEngine: "podman", wantSource: "--container-engine flag"},
		{name: "blank env var means default", env: "  ", wantEngine: "docker", wantSource: "default"},
		{name: "unsupported flag value", flag: "nerdctl", wantErr: `unsupported container engine "nerdctl" (from --container-engine flag): supported engines are docker, podman, buildah`},
		{name: "unsupported env value", env: "/usr/bin/podman", wantErr: `unsupported container engine "/usr/bin/podman" (from ` + EnvContainerEngine + `)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvContainerEngine, tt.env)

			engine, source, err := ResolveContainerEngine(tt.flag)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if engine != tt.wantEngine || source != tt.wantSource {
				t.Fatalf("got (%q, %q), want (%q, %q)", engine, source, tt.wantEngine, tt.wantSource)
			}
		})
	}
}

// buildahVersionOutput is `buildah version` output for the given release.
func buildahVersionOutput(version string) string {
	return "Version:         " + version + "\nGo Version:      go1.24.4\nImage Spec:      1.1.1\nRuntime Spec:    1.2.1\n"
}

// fakeEngineCommands replaces runEngineCommand; failing maps "name args..." to the output and error to return.
// Successful commands return nothing, except `buildah version`, which reports a supported release.
func fakeEngineCommands(t *testing.T, failing map[string]string) *[]string {
	t.Helper()
	return fakeEngineCommandsWithOutput(t, failing, map[string]string{"buildah version": buildahVersionOutput("1.46.1")})
}

// fakeEngineCommandsWithOutput is fakeEngineCommands with the output of successful commands in outputs.
func fakeEngineCommandsWithOutput(t *testing.T, failing, outputs map[string]string) *[]string {
	t.Helper()
	var calls []string
	original := runEngineCommand
	runEngineCommand = func(name string, args ...string) ([]byte, error) {
		call := strings.Join(append([]string{name}, args...), " ")
		calls = append(calls, call)
		if out, ok := failing[call]; ok {
			return []byte(out), errors.New("exit status 1")
		}
		return []byte(outputs[call]), nil
	}
	t.Cleanup(func() { runEngineCommand = original })
	return &calls
}

// fakeInstalledEngines replaces lookPath so that only the given engines are on the PATH.
func fakeInstalledEngines(t *testing.T, installed ...string) {
	t.Helper()
	original := lookPath
	lookPath = func(file string) (string, error) {
		for _, name := range installed {
			if name == file {
				return "/usr/bin/" + file, nil
			}
		}
		return "", errors.New("executable file not found in $PATH")
	}
	t.Cleanup(func() { lookPath = original })
}

func TestCheckContainerEngineCommands(t *testing.T) {
	tests := []struct {
		engine        string
		multiPlatform bool
		wantCalls     []string
	}{
		{engine: "docker", wantCalls: []string{"docker version"}},
		{engine: "docker", multiPlatform: true, wantCalls: []string{"docker version", "docker buildx version"}},
		{engine: "podman", multiPlatform: true, wantCalls: []string{"podman version"}},
		{engine: "buildah", multiPlatform: true, wantCalls: []string{"buildah version"}},
	}

	for _, tt := range tests {
		calls := fakeEngineCommands(t, nil)
		if err := CheckContainerEngine(tt.engine, tt.multiPlatform); err != nil {
			t.Fatalf("%s: unexpected error: %v", tt.engine, err)
		}
		if !reflect.DeepEqual(*calls, tt.wantCalls) {
			t.Fatalf("%s (multiPlatform=%v): got calls %v, want %v", tt.engine, tt.multiPlatform, *calls, tt.wantCalls)
		}
	}
}

func TestCheckContainerEngineUnavailableSuggestsInstalledEngines(t *testing.T) {
	fakeEngineCommands(t, map[string]string{
		"docker version": "Client: 29.1.3\nCannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n",
		"podman version": "",
	})
	fakeInstalledEngines(t, "docker", "podman", "buildah")

	err := CheckContainerEngine("docker", false)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "docker is not available: exit status 1: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n\n" +
		"To use podman or buildah instead, set --container-engine or AP_CONTAINER_ENGINE."
	if err.Error() != want {
		t.Fatalf("got error:\n%s\nwant:\n%s", err, want)
	}

	err = CheckContainerEngine("podman", false)
	if err == nil || !strings.HasPrefix(err.Error(), "podman is not available: exit status 1\n\nTo use docker or buildah instead") {
		t.Fatalf("unexpected podman error: %v", err)
	}
}

// Engines that are not installed are not suggested, for example buildah on macOS.
func TestCheckContainerEngineUnavailableSkipsMissingEngines(t *testing.T) {
	fakeEngineCommands(t, map[string]string{"docker version": ""})

	fakeInstalledEngines(t, "docker", "podman")
	err := CheckContainerEngine("docker", false)
	if err == nil || !strings.HasSuffix(err.Error(), "\n\nTo use podman instead, set --container-engine or AP_CONTAINER_ENGINE.") {
		t.Fatalf("got %v, want only podman suggested", err)
	}

	fakeInstalledEngines(t, "docker")
	err = CheckContainerEngine("docker", false)
	if err == nil || err.Error() != "docker is not available: exit status 1" {
		t.Fatalf("got %v, want no suggestion when no other engine is installed", err)
	}
}

func TestCheckContainerEngineMissingBuildx(t *testing.T) {
	fakeEngineCommands(t, map[string]string{"docker buildx version": "unknown command: docker buildx"})
	fakeInstalledEngines(t, "docker", "podman", "buildah")

	if err := CheckContainerEngine("docker", false); err != nil {
		t.Fatalf("buildx should not be checked without --platform: %v", err)
	}
	err := CheckContainerEngine("docker", true)
	if err == nil || !strings.Contains(err.Error(), "docker buildx is not available but required for --platform") ||
		!strings.Contains(err.Error(), "To use podman or buildah instead") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// buildah before 1.30 drops name-only `run -e NAME` entries, so the builder
// would silently lose GOPRIVATE and the --go-env settings.
func TestCheckContainerEngineBuildahVersion(t *testing.T) {
	tests := []struct {
		output  string
		wantErr string
	}{
		{output: buildahVersionOutput("1.23.1"), wantErr: "buildah 1.23 is too old: buildah 1.30 or later is required"},
		{output: buildahVersionOutput("1.29.0"), wantErr: "buildah 1.29 is too old"},
		{output: buildahVersionOutput("1.30.0")},
		{output: buildahVersionOutput("1.46.1")},
		{output: buildahVersionOutput("1.47.0-dev")},
		{output: buildahVersionOutput("2.0.0")},
		{output: "Go Version: go1.24.4\n", wantErr: "could not read the buildah version"},
		{output: "", wantErr: "could not read the buildah version"},
	}

	for _, tt := range tests {
		fakeEngineCommandsWithOutput(t, nil, map[string]string{"buildah version": tt.output})
		fakeInstalledEngines(t)
		err := CheckContainerEngine("buildah", false)
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("%q: unexpected error: %v", tt.output, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%q: got %v, want an error containing %q", tt.output, err, tt.wantErr)
		}
	}
}
