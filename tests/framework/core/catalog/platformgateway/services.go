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
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// Gateway image references live in version.go, built from gateway/VERSION.

// Network aliases, kept identical to the compose files because products reference each
// other by these names in configuration they read at boot.
const (
	aliasGatewayController = "it-gateway-controller"
	aliasGatewayRuntime    = "it-gateway-runtime"
)

// GatewayControllerWiring is what a block may configure about the controller.
//
// Typed rather than a free-form map: a mistyped key here would otherwise leave the
// controller talking to the wrong control plane, and the test would fail somewhere far
// from the cause.
type GatewayControllerWiring struct {
	// ControlPlaneHost is the alias and port of the control plane this controller syncs with.
	ControlPlaneHost string `yaml:"controlPlaneHost"`

	// ControlPlaneToken authenticates that sync.
	ControlPlaneToken string `yaml:"controlPlaneToken"`

	// LogLevel overrides the controller's log level.
	LogLevel string `yaml:"logLevel"`
}

// GatewayController is the gateway's control plane: management REST API, xDS server and
// artifact storage.
func GatewayController() *components.Definition {
	return &components.Definition{
		Name:  "gateway-controller",
		Image: shared.Image(shared.EnvImageGatewayController, shared.GatewayControllerRunImage()),
		Alias: aliasGatewayController,
		Cmd:   []string{"-config", "/etc/gateway-controller/config.toml"},

		Endpoints: []components.Endpoint{
			// Only the two ports the controller always binds are awaited. xDS and
			// metrics are excluded from the wait set on purpose: every awaited port
			// must come up or startup fails, so including an optional listener would
			// hang the full timeout against a healthy container.
			{Name: "rest", Port: 9090, Scheme: "http", AwaitListening: true},
			{Name: "admin", Port: 9092, Scheme: "http", AwaitListening: true},
			{Name: "xds", Port: 18000, Scheme: "grpc"},
			{Name: "xds-alt", Port: 18001, Scheme: "grpc"},
			{Name: "metrics", Port: 9091, Scheme: "http"},
		},

		Health: &components.HealthCheck{
			Endpoint: "admin", Path: "/api/admin/v1/health",
			ExpectStatus: 200,
			// The compose file carries this same check but commented out, so the
			// existing suites gate only on the port. That is the partial-boot hole this
			// framework closes: it is enabled here.
			Timeout: 120 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath: "gateway/configs/config.toml",
			// The shipped config declares only the embedded storage engine, so this
			// overlay adds the external-database sections. Without it the controller
			// refuses to start on postgres or sqlserver no matter what environment is
			// set, because the keys are not templated anywhere.
			SharedOverlayPath: "tests/framework/core/catalog/overlays/gateway-controller-storage.toml",
			ContainerPath:     "/etc/gateway-controller/config.toml",
			Format:            components.TOML,
		},

		DB: &components.DBContract{
			Supported: []components.DBType{components.SQLite, components.Postgres, components.SQLServer},
			Schema: map[components.DBType][]string{
				components.Postgres:  {"gateway/gateway-controller/pkg/storage/gateway-controller-db.postgres.sql"},
				components.SQLServer: {"gateway/gateway-controller/pkg/storage/gateway-controller-db.sqlserver.sql"},
			},
			// The controller applies its own DDL for the embedded engine but expects a
			// pre-provisioned schema on an external one, so the suites exercise the same
			// path operators use.
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          gatewayControllerDBEnv,
		},

		Wiring: components.TypedWiring[GatewayControllerWiring](),

		Env: map[string]string{
			"APIP_GW_CONTROLLER_LOGGING_LEVEL": "debug",
			// Consumed by the template-functions feature, which asserts that
			// {{ env "..." }} resolves inside spec fields.
			//
			// These values are mirrored in the Compose definition used by the integration suite.
			"IT_TEMPLATE_PATH":     "/anything",
			"IT_RATE_LIMIT":        "5",
			"IT_ALLOW_CREDENTIALS": "true",
		},

		Files: []components.FileMount{
			{
				HostPath:      "gateway/it/it-aesgcm-keys/default-aesgcm256-v1.bin",
				ContainerPath: "/app/data/aesgcm-keys/default-aesgcm256-v1.bin",
				// Readable by the container's own user, which is not root even though
				// the copy lands as root-owned.
				Mode: 0o644,
			},
		},

		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 1000},
	}
}

// gatewayControllerDBEnv renders a DSN into the controller's environment vocabulary.
func gatewayControllerDBEnv(d components.DSN) map[string]string {
	env := map[string]string{"APIP_GW_CONTROLLER_STORAGE_TYPE": string(d.Type)}
	switch d.Type {
	case components.SQLite:
		env["APIP_GW_CONTROLLER_STORAGE_SQLITE_PATH"] = d.FilePath
	case components.Postgres:
		// Note USER, not USERNAME. Discrete fields, unlike SQL Server below.
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_HOST"] = d.Host
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_PORT"] = strconv.Itoa(d.Port)
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_DATABASE"] = d.Database
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_USER"] = d.User
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_PASSWORD"] = d.Password
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_SSLMODE"] = d.SSLMode
	case components.SQLServer:
		// SQL Server takes a single assembled DSN rather than discrete fields — a
		// different shape from Postgres, under a different key prefix (STORAGE_DATABASE,
		// not STORAGE_SQLSERVER).
		env["APIP_GW_CONTROLLER_STORAGE_DATABASE_DSN"] = sqlServerDSN(d)
	}
	return env
}

// sqlServerDSN assembles the connection string the controller expects.
//
// encrypt=disable and TrustServerCertificate cover the image's self-signed certificate;
// asserting the product trusts a test certificate proves nothing about the product. The
// application name makes the connection attributable in sys.dm_exec_sessions, which is
// what the existing suites assert on to prove the controller really used this engine.
func sqlServerDSN(d components.DSN) string {
	return fmt.Sprintf(
		"sqlserver://%s:%s@%s:%d?database=%s&encrypt=disable&TrustServerCertificate=true&app+name=gateway-controller",
		url.QueryEscape(d.User), url.QueryEscape(d.Password), d.Host, d.Port, url.QueryEscape(d.Database),
	)
}

// GatewayRuntimeWiring is what a block may configure about the runtime.
type GatewayRuntimeWiring struct {
	// ControllerHost is the alias of the controller serving this runtime's xDS.
	ControllerHost string `yaml:"controllerHost"`
	// LogLevel overrides the runtime's log level.
	LogLevel string `yaml:"logLevel"`
	// BedrockEndpoint redirects AWS Bedrock traffic at a mock.
	BedrockEndpoint string `yaml:"bedrockEndpoint"`
}

// GatewayRuntime is the data plane: the Envoy router plus the policy engine.
func GatewayRuntime() *components.Definition {
	return &components.Definition{
		Name:  "gateway-runtime",
		Image: shared.Image(shared.EnvImageGatewayRuntime, shared.GatewayRuntimeRunImage()),
		Alias: aliasGatewayRuntime,
		Cmd:   []string{"--pol.config", "/etc/policy-engine/config.toml"},

		Endpoints: []components.Endpoint{
			{Name: "http", Port: 8080, Scheme: "http", AwaitListening: true},
			{Name: "https", Port: 8443, Scheme: "https"},
			// Envoy's admin interface. The health feature probes /ready on it directly,
			// which is why the runtime is configured to bind it on the container
			// interface rather than loopback.
			{Name: "envoy-admin", Port: 9901, Scheme: "http"},
			{Name: "admin", Port: 9002, Scheme: "http", AwaitListening: true},
			{Name: "metrics", Port: 9003, Scheme: "http"},
		},

		Health: &components.HealthCheck{
			Endpoint: "envoy-admin", Path: "/ready",
			ExpectStatus: 200,
			Timeout:      120 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath: "gateway/configs/config.toml",
			ContainerPath:  "/etc/policy-engine/config.toml",
			Format:         components.TOML,
		},

		Wiring: components.TypedWiring[GatewayRuntimeWiring](),

		Env: map[string]string{
			"GATEWAY_CONTROLLER_HOST": aliasGatewayController,
			"LOG_LEVEL":               "info",
			"ROUTER_ADMIN_ENABLED":    "true",
			// Bound on the container interface so the admin endpoint is reachable from
			// the test process. The deployment's own network boundary is what restricts
			// access; this is a test topology, not a shipped default.
			"ROUTER_ADMIN_HOST": "0.0.0.0",
			// Must match the compose catalog's value, and for the same reason: enabling the
			// admin interface above turns on the entrypoint's shutdown drain, which defaults
			// to 15s and therefore cannot finish inside docker's 10s stop timeout. The
			// container was SIGKILLed mid-drain every time, costing a forced 10s per teardown
			// and discarding the coverage counters Go only writes on a clean exit.
			"ROUTER_DRAIN_TIME_SECONDS": "2",
		},

		DependsOn: []string{"gateway-controller"},

		Limits: components.ResourceLimits{CPUs: 1, MemoryMB: 2000},
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var d []byte
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}
