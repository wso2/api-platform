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

// Environment variables that override the gateway service images.
const (
	EnvImagePGController = "PG_CONTROLLER_IMAGE"
	EnvImagePGRuntime    = "PG_RUNTIME_IMAGE"
)

// Compose service names. These are internals — nothing outside this file, and certainly
// no suite file, should name them.
const (
	svcController = "gateway-controller"
	svcRuntime    = "gateway-runtime"
)

// PlatformGateway is the gateway, as a single component.
//
// It is two containers underneath — a controller that owns storage and serves the
// management API, and a runtime that carries Envoy and the policy engine — but that is an
// implementation detail. A suite file names "platform-gateway" and gets a working gateway;
// it cannot accidentally compose a runtime with no controller, and if the gateway later
// becomes one container or three, no suite file changes.
//
// The endpoints below are the RUNTIME's, because that is the surface tests address: APIs
// are invoked on the data plane, and the health feature probes Envoy's admin interface.
// The controller's management API is reached through the same component via its own
// endpoints, published by the compose file.
func PlatformGateway() *components.Definition {
	env := map[string]string{
		EnvImagePGController: shared.Image(shared.EnvImageGatewayController, shared.GatewayControllerRunImage()).Ref,
		EnvImagePGRuntime:    shared.Image(shared.EnvImageGatewayRuntime, shared.GatewayRuntimeRunImage()).Ref,
	}
	for key, value := range runtimeCoverageEnvironment() {
		env[key] = value
	}
	return &components.Definition{
		Name: "platform-gateway",

		// The alias is informational for a compose-backed component: services address
		// each other by compose service name on the stack's own network. It still names
		// the component in diagnostics.
		Alias: "platform-gateway",

		Compose: &components.ComposeSpec{
			ComposeFile:    "tests/framework/core/catalog/platformgateway/docker-compose.yaml",
			PrimaryService: svcRuntime,
			Services:       []string{svcController, svcRuntime},

			// Copied next to the compose file so its relative bind mounts resolve.
			StagedFiles: map[string]string{
				"aesgcm-keys/default-aesgcm256-v1.bin": "gateway/it/it-aesgcm-keys/default-aesgcm256-v1.bin",
				"listener-certs":                       "gateway/gateway-controller/listener-certs",

				// The controller's custom-trust-store seed directory, matching both the
				// product compose and the legacy IT compose. It is NOT needed for the
				// certificate management API — those certificates live in the database, and
				// certstore treats the filesystem only as a first-run seed — but without it
				// bootstrapCertificatesFromFilesystem returns at its first os.Stat and the
				// seeding path is never executed at all. The legacy suite mounted this and so
				// exercised that path incidentally; dropping the mount quietly retired it.
				"certificates": "gateway/gateway-controller/certificates",
			},

			// config.toml is assembled per block from the product's shipped config plus
			// overlays. Database settings are resolved into the generated environment
			// file; administrative credentials come from the test overlay.

			// Image references are resolved from the source version or environment overrides.
			Env: env,

			CoverageServices: []components.CoverageService{
				{Name: svcRuntime, Types: []string{"go"}},
				{Name: svcController, Types: []string{"go"}},
			},
		},

		Endpoints: []components.Endpoint{
			// Runtime — the data plane and its admin interface.
			{Name: "http", Port: 8080, Scheme: "http"},
			{Name: "https", Port: 8443, Scheme: "https"},
			{Name: "envoy-admin", Port: 9901, Scheme: "http"},
			// The policy engine's own admin interface. Distinct from Envoy's: it reports the
			// POLICY chain the engine is running, which is what a test compares against the
			// controller to know a deploy has actually reached the data plane.
			{Name: "policy-admin", Port: 9002, Scheme: "http"},
			// Controller — management and admin APIs. On a different compose service from
			// the data plane, which is why each names its service: consolidating means a
			// caller addresses them uniformly without knowing which container answers.
			{Name: "rest", Port: 9090, Scheme: "http", Service: svcController},
			{Name: "admin", Port: 9092, Scheme: "http", Service: svcController},

			// Prometheus metrics, one endpoint per process. Published because the metrics
			// feature scrapes both directly; they are not otherwise part of the surface a
			// test addresses, and each belongs to a DIFFERENT service — which is exactly the
			// case Endpoint.Service exists for.
			{Name: "metrics", Port: 9091, Scheme: "http", Service: svcController},
			{Name: "pe-metrics", Port: 9003, Scheme: "http", Service: svcRuntime},
		},

		// Gated on the CONTROLLER's admin health, not on Envoy's /ready.
		//
		// Envoy's /ready looks like the obvious gate and is not: with no APIs deployed it
		// has no listener config, so it sits in PRE_INITIALIZING and answers 503
		// indefinitely. Measured directly — the controller was healthy and serving xDS, the
		// runtime's admin interface was live, and /ready still returned
		// "503 PRE_INITIALIZING" with 0 listeners loaded. It becomes ready only once an API
		// is deployed, which makes it a post-deployment assertion rather than a boot gate.
		// (This is also why the existing suite's compose uses the image's own
		// health-check.sh for the runtime instead.)
		//
		// The controller's admin health is genuinely ready at boot and is what a test needs
		// before it can deploy anything.
		Health: &components.HealthCheck{
			Service:  svcController,
			Endpoint: "admin", Path: "/api/admin/v1/health",
			ExpectStatus: 200,
			Timeout:      3 * time.Minute, Interval: 2 * time.Second,
		},

		// Storage belongs to the controller, but it is declared here because the gateway
		// is one component from the outside. The framework provisions one store and the
		// generated env file carries the DSN to whichever container needs it.
		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			Schema: map[components.DBType][]string{
				components.Postgres:  {"gateway/gateway-controller/pkg/storage/gateway-controller-db.postgres.sql"},
				components.SQLServer: {"gateway/gateway-controller/pkg/storage/gateway-controller-db.sqlserver.sql"},
			},
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          gatewayControllerDBEnv,
		},

		// Configuration is assembled from the product's OWN shipped file plus the storage
		// overlay, then staged as config.toml. Both containers mount the same file; the
		// controller reads its controller sections and the runtime its policy-engine ones.
		Config: &components.ConfigInjection{
			BaseConfigPath:    "gateway/configs/config.toml",
			SharedOverlayPath: "tests/framework/core/catalog/overlays/gateway-controller-storage.toml",
			ExtraOverlays:     nil,
			// Where the generated file is staged, relative to the compose directory —
			// not a container path, because the compose file owns the mounts.
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
	// ControlPlaneHost is the alias:port of a control plane the gateway syncs with, for
	// topologies where platform-api is present.
	ControlPlaneHost string `yaml:"controlPlaneHost"`
	// ControlPlaneToken authenticates that sync.
	ControlPlaneToken string `yaml:"controlPlaneToken"`
	// LogLevel overrides the log level for both containers.
	LogLevel string `yaml:"logLevel"`
}
