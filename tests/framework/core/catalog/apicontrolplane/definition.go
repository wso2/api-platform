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

package apicontrolplane

import (
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// EnvImageAPIControlPlane names the environment variable used to override the Management
// Portal image.
const EnvImageAPIControlPlane = "ACP_IMAGE"

const svcAPIControlPlane = "api-control-plane"

// APIControlPlane returns the Management Portal component definition: the console SPA and the
// BFF that serves it, signing users in through the control plane's file-based login.
func APIControlPlane() *components.Definition {
	env := map[string]string{
		EnvImageAPIControlPlane: shared.Image(EnvImageAPIControlPlane, shared.APIControlPlaneImage()).Ref,
	}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name:         svcAPIControlPlane,
		Alias:        svcAPIControlPlane,
		AliasIsFixed: true,

		Compose: &components.ComposeSpec{
			ComposeFile: "tests/framework/core/catalog/apicontrolplane/docker-compose.yaml",

			Env:            env,
			PrimaryService: svcAPIControlPlane,
			Services:       []string{svcAPIControlPlane},
			CoverageServices: []components.CoverageService{{
				Name: svcAPIControlPlane, Types: []string{"go"},
			}},

			GeneratedFiles: map[string][]byte{
				"tls/cp-cert.pem": shared.ControlPlaneCrypto()["certs/cert.pem"],
			},
		},

		Endpoints: []components.Endpoint{
			{Name: "http", Port: 8082, Scheme: "http", AwaitListening: true},
		},

		Health: &components.HealthCheck{
			Endpoint: "http", Path: "/healthz", ExpectStatus: 200,
			Timeout: 180 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath:    "portals/api-control-plane/configs/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/overlays/api-control-plane-cp-trust.toml",
			ContainerPath:     "/config.toml",
			Format:            components.TOML,
		},

		DependsOn: []string{"platform-api"},

		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 512},
	}
}

func runtimeCoverageEnvironment() map[string]string {
	if !shared.CoverageMode() {
		return nil
	}
	spec, err := BuildSpec("")
	if err != nil {
		panic("catalog: building Management Portal coverage specification: " + err.Error())
	}
	env := make(map[string]string, len(spec.Coverage.Environment))
	for key, value := range spec.Coverage.Environment {
		env[key] = value
	}
	return env
}
