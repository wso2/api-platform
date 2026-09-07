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

package platformgateway

import (
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

const (
	// EnvImagePGController overrides the gateway controller image.
	EnvImagePGController = "PG_CONTROLLER_IMAGE"
	// EnvImagePGRuntime overrides the gateway runtime image.
	EnvImagePGRuntime = "PG_RUNTIME_IMAGE"
)

const (
	svcController = "gateway-controller"
	svcRuntime    = "gateway-runtime"
)

// PlatformGateway returns the Platform Gateway component definition.
func PlatformGateway() *components.Definition {
	env := map[string]string{
		EnvImagePGController: shared.Image(shared.EnvImageGatewayController, shared.GatewayControllerRunImage()).Ref,
		EnvImagePGRuntime:    shared.Image(shared.EnvImageGatewayRuntime, shared.GatewayRuntimeRunImage()).Ref,
	}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name:  "platform-gateway",
		Alias: "platform-gateway",

		Compose: &components.ComposeSpec{
			ComposeFile:    "tests/framework/core/catalog/platformgateway/docker-compose.yaml",
			PrimaryService: svcRuntime,
			Services:       []string{svcController, svcRuntime},

			StagedFiles: map[string]string{
				"aesgcm-keys/default-aesgcm256-v1.bin": "gateway/it/it-aesgcm-keys/default-aesgcm256-v1.bin",
				"listener-certs":                       "gateway/gateway-controller/listener-certs",
				"certificates":                         "gateway/gateway-controller/certificates",
			},
			Env: env,

			CoverageServices: []components.CoverageService{
				{Name: svcRuntime, Types: []string{"go"}},
				{Name: svcController, Types: []string{"go"}},
			},
		},

		Endpoints: []components.Endpoint{
			{Name: "http", Port: 8080, Scheme: "http"},
			{Name: "https", Port: 8443, Scheme: "https"},
			{Name: "envoy-admin", Port: 9901, Scheme: "http"},
			{Name: "policy-admin", Port: 9002, Scheme: "http"},
			{Name: "rest", Port: 9090, Scheme: "http", Service: svcController},
			{Name: "admin", Port: 9092, Scheme: "http", Service: svcController},
			{Name: "metrics", Port: 9091, Scheme: "http", Service: svcController},
			{Name: "pe-metrics", Port: 9003, Scheme: "http", Service: svcRuntime},
		},

		// The controller health endpoint is available before APIs are deployed.
		Health: &components.HealthCheck{
			Service:  svcController,
			Endpoint: "admin", Path: "/api/admin/v1/health",
			ExpectStatus: 200,
			Timeout:      3 * time.Minute, Interval: 2 * time.Second,
		},

		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			Schema: map[components.DBType][]string{
				components.Postgres:  {"gateway/gateway-controller/pkg/storage/gateway-controller-db.postgres.sql"},
				components.SQLServer: {"gateway/gateway-controller/pkg/storage/gateway-controller-db.sqlserver.sql"},
			},
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          gatewayControllerDBEnv,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath:    "gateway/configs/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/overlays/gateway-controller-storage.toml",
			ExtraOverlays: []string{
				"tests/framework/core/catalog/overlays/gateway-analytics.toml",
			},
			ContainerPath: "/config.toml",
			Format:        components.TOML,
		},

		Wiring: components.TypedWiring[PlatformGatewayWiring](),

		Limits: components.ResourceLimits{CPUs: 2, MemoryMB: 3000},
	}
}

func runtimeCoverageEnvironment() map[string]string {
	if !shared.CoverageMode() {
		return nil
	}
	spec, _ := BuildSpec("")
	return cloneEnvironment(spec.Coverage.Environment)
}

func cloneEnvironment(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

// PlatformGatewayWiring is what a block may configure about the gateway.
type PlatformGatewayWiring struct {
	// ControlPlaneHost is the address of the control plane.
	ControlPlaneHost string `yaml:"controlPlaneHost"`
	// ControlPlaneToken authenticates control-plane requests.
	ControlPlaneToken string `yaml:"controlPlaneToken"`
	// LogLevel sets the gateway log level.
	LogLevel string `yaml:"logLevel"`
}
