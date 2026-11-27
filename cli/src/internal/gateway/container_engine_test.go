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
	"strings"
	"testing"
	"time"
)

func TestEngineFor(t *testing.T) {
	tests := []struct {
		name string
		want containerEngine
	}{
		{name: "", want: dockerEngine{}},
		{name: "docker", want: dockerEngine{}},
		{name: "podman", want: podmanEngine{}},
		{name: "buildah", want: buildahEngine{}},
	}
	for _, tt := range tests {
		got, err := engineFor(tt.name)
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("%q: got %T, want %T", tt.name, got, tt.want)
		}
	}
	if _, err := engineFor("nerdctl"); err == nil {
		t.Fatal("expected an error for an unsupported engine")
	}
}

func TestSELinuxRelabelOnlyOnLinux(t *testing.T) {
	withHostOS(t, "linux")
	if !selinuxRelabel() {
		t.Fatal("expected relabel on linux")
	}
	withHostOS(t, "darwin")
	if selinuxRelabel() {
		t.Fatal("expected no relabel on darwin")
	}
}

func TestBuilderRunArgsRelabel(t *testing.T) {
	config := testBuildConfig("podman")
	flags := []string{
		"-gateway-controller-base-image", "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		"-gateway-runtime-base-image", "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
	}
	tests := []struct {
		relabel    bool
		wantVolume string
	}{
		{relabel: false, wantVolume: "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace"},
		{relabel: true, wantVolume: "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace:Z"},
	}
	for _, tt := range tests {
		got := builderRunArgs(config, tt.relabel, nil)
		want := append([]string{"run", "--rm", "-v", tt.wantVolume, "ghcr.io/wso2/api-platform/gateway-builder:1.2.0"}, flags...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("relabel=%v:\ngot  %v\nwant %v", tt.relabel, got, want)
		}
	}
}

func TestBuildahRunArgsUseImageEntrypoint(t *testing.T) {
	entrypoint := []string{"/usr/local/bin/gateway-builder", "-system-build-lock", "/api-platform/gateway/system-policies/system-build-lock.yaml"}
	config := testBuildConfig("buildah")
	config.FetchTimeout = 10 * time.Minute

	got := buildahRunArgs(config, "gateway-builder-working-container", entrypoint, true, []string{"-e", "GOPRIVATE"})
	want := []string{
		"run", "--volume", "/home/u/.wso2ap/.tmp/gateway-image-build-1:/workspace:Z", "--workingdir", "/workspace",
		"-e", "GOPRIVATE",
		"gateway-builder-working-container", "--",
		"/usr/local/bin/gateway-builder", "-system-build-lock", "/api-platform/gateway/system-policies/system-build-lock.yaml",
		"-gateway-controller-base-image", "ghcr.io/wso2/api-platform/gateway-controller:1.2.0",
		"-gateway-runtime-base-image", "ghcr.io/wso2/api-platform/gateway-runtime:1.2.0",
		"-fetch-timeout", "10m0s",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestCredentialMountForm(t *testing.T) {
	withHostOS(t, "linux")
	for _, tt := range []struct {
		engine containerEngine
		flag   string
	}{
		{dockerEngine{}, "--mount"},
		{podmanEngine{}, "--mount"},
		{buildahEngine{}, "--volume"},
	} {
		args, err := tt.engine.credentialMount()("/home/u/netrc", "/root/.netrc")
		if err != nil || len(args) != 2 || args[0] != tt.flag {
			t.Fatalf("%s: args = %v, err = %v; want %s", tt.engine.binary(), args, err, tt.flag)
		}
	}
}

func TestRelabelMounts(t *testing.T) {
	withHostOS(t, "linux")
	if (dockerEngine{}).relabelMounts() || !(podmanEngine{}).relabelMounts() || !(buildahEngine{}).relabelMounts() {
		t.Fatal("on linux only podman and buildah should relabel mounts")
	}
	withHostOS(t, "darwin")
	if (podmanEngine{}).relabelMounts() || (buildahEngine{}).relabelMounts() {
		t.Fatal("no engine should relabel mounts on darwin")
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

func TestDockerMultiPlatformArgs(t *testing.T) {
	const tag = "registry.example.com/apip/my-gw-gateway-runtime:1.2.0"
	config := testBuildConfig("docker")
	config.Platform = "linux/amd64,linux/arm64"
	config.NoCache = true

	got := dockerEngine{}.multiPlatformArgs(config, tag)
	want := []string{"buildx", "build", "--platform", "linux/amd64,linux/arm64", "--push", "-t", tag, "--no-cache", "."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestManifestListCommands(t *testing.T) {
	const tag = "registry.example.com/apip/my-gw-gateway-runtime:1.2.0"
	const list = "localhost/wso2ap-gateway-build-0123456789abcdef"
	config := testBuildConfig("podman")
	config.Platform = "linux/amd64,linux/arm64"
	config.NoCache = true

	got := manifestListCommands(config, list, tag)
	want := [][]string{
		{"build", "--platform", "linux/amd64,linux/arm64", "--manifest", list, "--no-cache", "."},
		{"manifest", "push", "--all", list, "docker://" + tag},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestLocalManifestListNameIsUnique(t *testing.T) {
	a, err := localManifestListName()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := localManifestListName()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Fatalf("expected different names, got %s twice", a)
	}
	if !strings.HasPrefix(a, "localhost/wso2ap-gateway-build-") || len(a) != len("localhost/wso2ap-gateway-build-")+16 {
		t.Fatalf("unexpected name %s", a)
	}
}

func TestManifestListDigests(t *testing.T) {
	inspect := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.index.v1+json","manifests":[` +
		`{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:aaa","size":1,"platform":{"architecture":"amd64","os":"linux"}},` +
		`{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:bbb","size":1,"platform":{"architecture":"arm64","os":"linux"}}]}`)
	got := manifestListDigests(inspect)
	if want := map[string]bool{"sha256:aaa": true, "sha256:bbb": true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := manifestListDigests([]byte("not json")); len(got) != 0 {
		t.Fatalf("got %v for invalid output, want none", got)
	}
}

func TestImagesWithDigests(t *testing.T) {
	digests := map[string]bool{"sha256:aaa": true, "sha256:bbb": true}
	// buildah pads the columns; podman separates them with one space.
	output := []byte("e3a7dda8f2ab   sha256:aaa\n" +
		"2db70850d96b sha256:bbb\n" +
		"2db70850d96b sha256:bbb\n" + // listed again under a second name: removed once
		"d2482869a6b8 sha256:other\n" +
		"\n")
	got := imagesWithDigests(output, digests)
	if want := []string{"e3a7dda8f2ab", "2db70850d96b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
