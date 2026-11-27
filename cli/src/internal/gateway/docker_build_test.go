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
