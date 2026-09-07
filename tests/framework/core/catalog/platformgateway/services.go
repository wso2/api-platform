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

const (
	aliasGatewayController = "it-gateway-controller"
	aliasGatewayRuntime    = "it-gateway-runtime"
)

// GatewayControllerWiring defines block-level controller configuration.
type GatewayControllerWiring struct {
	// ControlPlaneHost is the control-plane address.
	ControlPlaneHost string `yaml:"controlPlaneHost"`
	// ControlPlaneToken authenticates control-plane requests.
	ControlPlaneToken string `yaml:"controlPlaneToken"`
	// LogLevel sets the controller log level.
	LogLevel string `yaml:"logLevel"`
}

// GatewayController returns the gateway controller component definition.
func GatewayController() *components.Definition {
	return &components.Definition{
		Name:  "gateway-controller",
		Image: shared.Image(shared.EnvImageGatewayController, shared.GatewayControllerRunImage()),
		Alias: aliasGatewayController,
		Cmd:   []string{"-config", "/etc/gateway-controller/config.toml"},

		Endpoints: []components.Endpoint{
			{Name: "rest", Port: 9090, Scheme: "http", AwaitListening: true},
			{Name: "admin", Port: 9092, Scheme: "http", AwaitListening: true},
			{Name: "xds", Port: 18000, Scheme: "grpc"},
			{Name: "xds-alt", Port: 18001, Scheme: "grpc"},
			{Name: "metrics", Port: 9091, Scheme: "http"},
		},

		Health: &components.HealthCheck{
			Endpoint: "admin", Path: "/api/admin/v1/health",
			ExpectStatus: 200,
			Timeout:      120 * time.Second, Interval: 2 * time.Second,
		},

		Config: &components.ConfigInjection{
			BaseConfigPath:    "gateway/configs/config.toml",
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
			SelfMigrates: []components.DBType{components.SQLite},
			Env:          gatewayControllerDBEnv,
		},

		Wiring: components.TypedWiring[GatewayControllerWiring](),

		Env: map[string]string{
			"APIP_GW_CONTROLLER_LOGGING_LEVEL": "debug",
			"IT_TEMPLATE_PATH":                 "/anything",
			"IT_RATE_LIMIT":                    "5",
			"IT_ALLOW_CREDENTIALS":             "true",
		},

		Files: []components.FileMount{
			{
				HostPath:      "gateway/it/it-aesgcm-keys/default-aesgcm256-v1.bin",
				ContainerPath: "/app/data/aesgcm-keys/default-aesgcm256-v1.bin",
				Mode:          0o644,
			},
		},

		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 1000},
	}
}

// gatewayControllerDBEnv converts a database DSN to controller environment variables.
func gatewayControllerDBEnv(d components.DSN) map[string]string {
	env := map[string]string{"APIP_GW_CONTROLLER_STORAGE_TYPE": string(d.Type)}
	switch d.Type {
	case components.SQLite:
		env["APIP_GW_CONTROLLER_STORAGE_SQLITE_PATH"] = d.FilePath
	case components.Postgres:
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_HOST"] = d.Host
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_PORT"] = strconv.Itoa(d.Port)
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_DATABASE"] = d.Database
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_USER"] = d.User
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_PASSWORD"] = d.Password
		env["APIP_GW_CONTROLLER_STORAGE_POSTGRES_SSLMODE"] = d.SSLMode
	case components.SQLServer:
		env["APIP_GW_CONTROLLER_STORAGE_DATABASE_DSN"] = sqlServerDSN(d)
	}
	return env
}

// sqlServerDSN builds the SQL Server connection string used by the controller.
func sqlServerDSN(d components.DSN) string {
	return fmt.Sprintf(
		"sqlserver://%s:%s@%s:%d?database=%s&encrypt=disable&TrustServerCertificate=true&app+name=gateway-controller",
		url.QueryEscape(d.User), url.QueryEscape(d.Password), d.Host, d.Port, url.QueryEscape(d.Database),
	)
}

// GatewayRuntimeWiring defines block-level runtime configuration.
type GatewayRuntimeWiring struct {
	// ControllerHost is the controller address used for xDS.
	ControllerHost string `yaml:"controllerHost"`
	// LogLevel sets the runtime log level.
	LogLevel string `yaml:"logLevel"`
	// BedrockEndpoint overrides the Bedrock endpoint.
	BedrockEndpoint string `yaml:"bedrockEndpoint"`
}

// GatewayRuntime returns the gateway runtime component definition.
func GatewayRuntime() *components.Definition {
	return &components.Definition{
		Name:  "gateway-runtime",
		Image: shared.Image(shared.EnvImageGatewayRuntime, shared.GatewayRuntimeRunImage()),
		Alias: aliasGatewayRuntime,
		Cmd:   []string{"--pol.config", "/etc/policy-engine/config.toml"},

		Endpoints: []components.Endpoint{
			{Name: "http", Port: 8080, Scheme: "http", AwaitListening: true},
			{Name: "https", Port: 8443, Scheme: "https"},
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
			"GATEWAY_CONTROLLER_HOST":   aliasGatewayController,
			"LOG_LEVEL":                 "info",
			"ROUTER_ADMIN_ENABLED":      "true",
			"ROUTER_ADMIN_HOST":         "0.0.0.0",
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
