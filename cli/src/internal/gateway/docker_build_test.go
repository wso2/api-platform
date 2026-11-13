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
	"reflect"
	"testing"
)

func testBuildConfig(engine string) DockerBuildConfig {
	return DockerBuildConfig{
		TempDir:                    "/home/u/.wso2ap/.tmp/gateway-image-build-1",
		GatewayBuilder:             "ghcr.io/wso2/api-platform/gateway-builder:1.2.0",
		GatewayControllerBaseImage: "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		GatewayRuntimeBaseImage:    "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
		ImageRepository:            "registry.example.com/apip",
		GatewayName:                "my-gw",
		GatewayVersion:             "1.2.0",
		ContainerEngine:            engine,
	}
}

func withHostOS(t *testing.T, goos string) {
	t.Helper()
	original := hostOS
	hostOS = goos
	t.Cleanup(func() { hostOS = original })
}

func TestBuilderRunArgs(t *testing.T) {
	baseFlags := []string{
		"-gateway-controller-base-image", "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		"-gateway-runtime-base-image", "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
	}
	tests := []struct {
		name       string
		engine     string
		goos       string
		wantVolume string
	}{
		{name: "empty engine is docker", engine: "", goos: "linux", wantVolume: "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace"},
		{name: "docker on linux has no relabel", engine: "docker", goos: "linux", wantVolume: "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace"},
		{name: "podman on linux relabels for SELinux", engine: "podman", goos: "linux", wantVolume: "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace:Z"},
		{name: "podman on macOS has no relabel", engine: "podman", goos: "darwin", wantVolume: "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withHostOS(t, tt.goos)
			got := builderRunArgs(testBuildConfig(tt.engine))
			want := append([]string{"run", "--rm", "-v", tt.wantVolume, "ghcr.io/wso2/api-platform/gateway-builder:1.2.0"}, baseFlags...)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got  %v\nwant %v", got, want)
			}
		})
	}
}

func TestBuildahRunArgsUseImageEntrypoint(t *testing.T) {
	withHostOS(t, "linux")
	entrypoint := []string{"/usr/local/bin/gateway-builder", "-system-build-lock", "/api-platform/gateway/system-policies/system-build-lock.yaml"}

	got := buildahRunArgs(testBuildConfig("buildah"), "gateway-builder-working-container", entrypoint)
	want := []string{
		"run", "--volume", "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace:Z", "--workingdir", "/workspace",
		"gateway-builder-working-container", "--",
		"/usr/local/bin/gateway-builder", "-system-build-lock", "/api-platform/gateway/system-policies/system-build-lock.yaml",
		"-gateway-controller-base-image", "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		"-gateway-runtime-base-image", "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestBuildahEntrypoint(t *testing.T) {
	// Trimmed `buildah inspect --type container` output
	inspect := `{
		"Type": "buildah 0.0.1",
		"FromImage": "ghcr.io/wso2/api-platform/gateway-builder:1.2.0",
		"Docker": {"config": {"Entrypoint": ["/docker-only"]}},
		"OCIv1": {
			"architecture": "arm64",
			"config": {
				"WorkingDir": "/workspace",
				"Entrypoint": ["/usr/local/bin/gateway-builder", "-system-build-lock", "/api-platform/gateway/system-policies/system-build-lock.yaml"]
			}
		}
	}`
	got, err := buildahEntrypoint([]byte(inspect))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"/usr/local/bin/gateway-builder", "-system-build-lock", "/api-platform/gateway/system-policies/system-build-lock.yaml"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if _, err := buildahEntrypoint([]byte(`{"OCIv1": {"config": {}}}`)); err == nil {
		t.Fatal("expected an error for an image without an entrypoint")
	}
	if _, err := buildahEntrypoint([]byte(`not json`)); err == nil {
		t.Fatal("expected an error for invalid inspect output")
	}
}

func TestBuildArgs(t *testing.T) {
	config := testBuildConfig("podman")
	tag := config.imageTag("gateway-runtime")
	if tag != "registry.example.com/apip/my-gw-gateway-runtime:1.2.0" {
		t.Fatalf("unexpected image tag %s", tag)
	}

	if got, want := buildArgs(config, tag), []string{"build", "-t", tag, "."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	config.NoCache = true
	if got, want := buildArgs(config, tag), []string{"build", "-t", tag, "--no-cache", "."}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMultiPlatformCommands(t *testing.T) {
	const tag = "registry.example.com/apip/my-gw-gateway-runtime:1.2.0"
	tests := []struct {
		engine string
		want   [][]string
	}{
		{
			engine: "docker",
			want: [][]string{
				{"buildx", "build", "--platform", "linux/amd64,linux/arm64", "--push", "-t", tag, "--no-cache", "."},
			},
		},
		{
			engine: "podman",
			want: [][]string{
				{"build", "--platform", "linux/amd64,linux/arm64", "--manifest", tag, "--no-cache", "."},
				{"manifest", "push", "--all", tag, "docker://" + tag},
			},
		},
		{
			engine: "buildah",
			want: [][]string{
				{"build", "--platform", "linux/amd64,linux/arm64", "--manifest", tag, "--no-cache", "."},
				{"manifest", "push", "--all", tag, "docker://" + tag},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.engine, func(t *testing.T) {
			config := testBuildConfig(tt.engine)
			config.Platform = "linux/amd64,linux/arm64"
			config.NoCache = true
			if got := multiPlatformCommands(config, tag); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got  %v\nwant %v", got, tt.want)
			}
		})
	}
}
