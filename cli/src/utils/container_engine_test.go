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

// fakeEngineCommands replaces runEngineCommand; failing maps "name args..." to the output and error to return.
func fakeEngineCommands(t *testing.T, failing map[string]string) *[]string {
	t.Helper()
	var calls []string
	original := runEngineCommand
	runEngineCommand = func(name string, args ...string) ([]byte, error) {
		call := strings.Join(append([]string{name}, args...), " ")
		calls = append(calls, call)
		if out, ok := failing[call]; ok {
			return []byte(out), errors.New("exit status 1")
		}
		return nil, nil
	}
	t.Cleanup(func() { runEngineCommand = original })
	return &calls
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

func TestCheckContainerEngineUnavailableSuggestsOtherEngines(t *testing.T) {
	fakeEngineCommands(t, map[string]string{
		"docker version": "Client: 29.1.3\nCannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n",
		"podman version": "",
	})

	err := CheckContainerEngine("docker", false)
	if err == nil {
		t.Fatal("expected an error")
	}
	want := "docker is not available: exit status 1: Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?\n\n" +
		"To use podman or buildah instead, set --container-engine or WSO2AP_CONTAINER_ENGINE."
	if err.Error() != want {
		t.Fatalf("got error:\n%s\nwant:\n%s", err, want)
	}

	err = CheckContainerEngine("podman", false)
	if err == nil || !strings.HasPrefix(err.Error(), "podman is not available: exit status 1\n\nTo use docker or buildah instead") {
		t.Fatalf("unexpected podman error: %v", err)
	}
}

func TestCheckContainerEngineMissingBuildx(t *testing.T) {
	fakeEngineCommands(t, map[string]string{"docker buildx version": "unknown command: docker buildx"})

	if err := CheckContainerEngine("docker", false); err != nil {
		t.Fatalf("buildx should not be checked without --platform: %v", err)
	}
	err := CheckContainerEngine("docker", true)
	if err == nil || !strings.Contains(err.Error(), "docker buildx is not available but required for --platform") ||
		!strings.Contains(err.Error(), "use podman or buildah") {
		t.Fatalf("unexpected error: %v", err)
	}
}
