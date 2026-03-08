/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package config

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// validConfig returns a valid configuration for testing
func validConfig() *Config {
	return &Config{
		Controller: Controller{
			Server: ServerConfig{
				APIPort:   8080,
				XDSPort:   18000,
				GatewayID: constants.PlatformGatewayId,
			},
			Storage: StorageConfig{
				Type: "memory",
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
			},
			ControlPlane: ControlPlaneConfig{
				Host:             "localhost",
				ReconnectInitial: 1 * time.Second,
				ReconnectMax:     30 * time.Second,
				PollingInterval:  5 * time.Second,
			},
			Metrics: MetricsConfig{
				Enabled: false,
			},
		},
		APIKey: APIKeyConfig{
			APIKeysPerUserPerAPI: 5,
			Algorithm:            constants.HashingAlgorithmSHA256,
		},
		Router: RouterConfig{
			ListenerPort: 9090,
			HTTPSEnabled: false,
			AccessLogs: AccessLogsConfig{
				Enabled:    true,
				Format:     "json",
				JSONFields: map[string]string{"status": "%RESPONSE_CODE%"},
			},
			Upstream: RouterUpstream{
				TLS: UpstreamTLS{
					MinimumProtocolVersion: constants.TLSVersion12,
					MaximumProtocolVersion: constants.TLSVersion13,
					DisableSslVerification: true,
				},
				Timeouts: UpstreamTimeouts{
					RouteTimeoutMs:     60000,
					RouteIdleTimeoutMs: 30000,
					ConnectTimeoutMs:   5000,
				},
			},
			PolicyEngine: PolicyEngineConfig{
				RouteCacheAction: "DEFAULT",
				TimeoutMs:        1000,
				MessageTimeoutMs: 500,
			},
			VHosts: VHostsConfig{
				Main:    VHostEntry{Default: "localhost"},
				Sandbox: VHostEntry{Default: "sandbox.localhost"},
			},
			HTTPListener: HTTPListenerConfig{
				ServerHeaderTransformation: commonconstants.OVERWRITE,
			},
		},
	}
}

func TestConfig_Validate_StorageType(t *testing.T) {
	tests := []struct {
		name        string
		storageType string
		wantErr     bool
		errContains string
	}{
		{name: "Valid memory", storageType: "memory", wantErr: false},
		{name: "Valid sqlite", storageType: "sqlite", wantErr: true, errContains: "storage.sqlite.path is required"},
		{name: "Valid postgres", storageType: "postgres", wantErr: true, errContains: "storage.postgres.host is required"},
		{name: "Invalid type", storageType: "invalid", wantErr: true, errContains: "storage.type must be one of"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Storage.Type = tt.storageType
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_SQLiteConfig(t *testing.T) {
	cfg := validConfig()
	cfg.Controller.Storage.Type = "sqlite"
	cfg.Controller.Storage.SQLite.Path = "/tmp/test.db"
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_PostgresConfig(t *testing.T) {
	tests := []struct {
		name        string
		configure   func(*Config)
		wantErr     bool
		errContains string
		wantSSLMode string
	}{
		{
			name: "Valid postgres with fields",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
			},
			wantErr: false,
		},
		{
			name: "Valid postgres with dsn",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.DSN = "postgres://user:pass@localhost:5432/testdb?sslmode=require"
			},
			wantErr: false,
		},
		{
			name: "Missing host when dsn empty",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
			},
			wantErr:     true,
			errContains: "storage.postgres.host is required",
		},
		{
			name: "Missing database when dsn empty",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.User = "user"
			},
			wantErr:     true,
			errContains: "storage.postgres.database is required",
		},
		{
			name: "Missing user when dsn empty",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
			},
			wantErr:     true,
			errContains: "storage.postgres.user is required",
		},
		{
			name: "Invalid sslmode",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.SSLMode = "bad"
			},
			wantErr:     true,
			errContains: "storage.postgres.sslmode must be one of",
		},
		{
			name: "Valid sslmode allow",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.SSLMode = "allow"
			},
			wantErr:     false,
			wantSSLMode: "allow",
		},
		{
			name: "Valid sslmode prefer case-insensitive",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.SSLMode = "PREFER"
			},
			wantErr:     false,
			wantSSLMode: "prefer",
		},
		{
			name: "Invalid max open conns",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.MaxOpenConns = -1
			},
			wantErr:     true,
			errContains: "storage.postgres.max_open_conns must be >= 1",
		},
		{
			name: "Invalid max idle conns",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.MaxIdleConns = -1
			},
			wantErr:     true,
			errContains: "storage.postgres.max_idle_conns must be >= 0",
		},
		{
			name: "Invalid postgres port",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.Port = 70000
			},
			wantErr:     true,
			errContains: "storage.postgres.port must be between 1 and 65535",
		},
		{
			name: "Invalid conn max lifetime",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.ConnMaxLifetime = -1 * time.Second
			},
			wantErr:     true,
			errContains: "storage.postgres.conn_max_lifetime must be >= 0",
		},
		{
			name: "Invalid conn max idle time",
			configure: func(cfg *Config) {
				cfg.Controller.Storage.Postgres.Host = "localhost"
				cfg.Controller.Storage.Postgres.Database = "testdb"
				cfg.Controller.Storage.Postgres.User = "user"
				cfg.Controller.Storage.Postgres.ConnMaxIdleTime = -1 * time.Second
			},
			wantErr:     true,
			errContains: "storage.postgres.conn_max_idle_time must be >= 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Storage.Type = "postgres"
			tt.configure(cfg)
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.wantSSLMode != "" {
					assert.Equal(t, tt.wantSSLMode, cfg.Controller.Storage.Postgres.SSLMode)
				}
			}
		})
	}
}

func TestConfig_Validate_AccessLogFormat(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		wantErr     bool
		errContains string
	}{
		{name: "Valid json", format: "json", wantErr: false},
		{name: "Valid text", format: "text", wantErr: true, errContains: "text_format must be configured"},
		{name: "Invalid format", format: "xml", wantErr: true, errContains: "format must be either 'json' or 'text'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.AccessLogs.Format = tt.format
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_AccessLogFields(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		format      string
		jsonFields  map[string]string
		textFormat  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "JSON with fields",
			enabled:    true,
			format:     "json",
			jsonFields: map[string]string{"status": "%RESPONSE_CODE%"},
			wantErr:    false,
		},
		{
			name:        "JSON without fields",
			enabled:     true,
			format:      "json",
			jsonFields:  nil,
			wantErr:     true,
			errContains: "json_fields must be configured",
		},
		{
			name:       "Text with format",
			enabled:    true,
			format:     "text",
			textFormat: "[%START_TIME%]",
			wantErr:    false,
		},
		{
			name:        "Text without format",
			enabled:     true,
			format:      "text",
			textFormat:  "",
			wantErr:     true,
			errContains: "text_format must be configured",
		},
		{
			name:       "Disabled - no validation",
			enabled:    false,
			format:     "json",
			jsonFields: nil,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.AccessLogs.Enabled = tt.enabled
			cfg.Router.AccessLogs.Format = tt.format
			cfg.Router.AccessLogs.JSONFields = tt.jsonFields
			cfg.Router.AccessLogs.TextFormat = tt.textFormat
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_LogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		wantErr bool
	}{
		{name: "debug", level: "debug", wantErr: false},
		{name: "info", level: "info", wantErr: false},
		{name: "warn", level: "warn", wantErr: false},
		{name: "warning", level: "warning", wantErr: false},
		{name: "error", level: "error", wantErr: false},
		{name: "DEBUG uppercase", level: "DEBUG", wantErr: false},
		{name: "invalid", level: "trace", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Logging.Level = tt.level
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "logging.level must be one of")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_LogFormat(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{name: "json", format: "json", wantErr: false},
		{name: "text", format: "text", wantErr: false},
		{name: "invalid", format: "xml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Logging.Format = tt.format
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "logging.format must be either")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_Ports(t *testing.T) {
	tests := []struct {
		name        string
		apiPort     int
		xdsPort     int
		adminPort   int
		adminEnable bool
		wantErr     bool
		errContains string
	}{
		{name: "Valid ports", apiPort: 8080, xdsPort: 18000, adminPort: 9092, adminEnable: true, wantErr: false},
		{name: "API port too low", apiPort: 0, xdsPort: 18000, adminPort: 9092, adminEnable: true, wantErr: true, errContains: "server.api_port must be between"},
		{name: "API port too high", apiPort: 70000, xdsPort: 18000, adminPort: 9092, adminEnable: true, wantErr: true, errContains: "server.api_port must be between"},
		{name: "XDS port too low", apiPort: 8080, xdsPort: 0, adminPort: 9092, adminEnable: true, wantErr: true, errContains: "server.xds_port must be between"},
		{name: "XDS port too high", apiPort: 8080, xdsPort: 70000, adminPort: 9092, adminEnable: true, wantErr: true, errContains: "server.xds_port must be between"},
		{name: "Admin port too low", apiPort: 8080, xdsPort: 18000, adminPort: 0, adminEnable: true, wantErr: true, errContains: "admin_server.port must be between"},
		{name: "Admin port too high", apiPort: 8080, xdsPort: 18000, adminPort: 70000, adminEnable: true, wantErr: true, errContains: "admin_server.port must be between"},
		{name: "Admin conflicts with API port", apiPort: 8080, xdsPort: 18000, adminPort: 8080, adminEnable: true, wantErr: true, errContains: "admin_server.port cannot be same as server.api_port"},
		{name: "Admin conflicts with xDS port", apiPort: 8080, xdsPort: 18000, adminPort: 18000, adminEnable: true, wantErr: true, errContains: "admin_server.port cannot be same as server.xds_port"},
		{name: "Admin disabled ignores admin port", apiPort: 8080, xdsPort: 18000, adminPort: 0, adminEnable: false, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Server.APIPort = tt.apiPort
			cfg.Controller.Server.XDSPort = tt.xdsPort
			cfg.Controller.AdminServer.Enabled = tt.adminEnable
			cfg.Controller.AdminServer.Port = tt.adminPort
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_MetricsConfig(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		port        int
		apiPort     int
		xdsPort     int
		adminPort   int
		adminEnable bool
		wantErr     bool
		errContains string
	}{
		{name: "Metrics disabled", enabled: false, port: 0, adminEnable: true, adminPort: 9092, wantErr: false},
		{name: "Valid metrics config", enabled: true, port: 9091, apiPort: 8080, xdsPort: 18000, adminEnable: true, adminPort: 9092, wantErr: false},
		{name: "Invalid metrics port", enabled: true, port: 0, adminEnable: true, adminPort: 9092, wantErr: true, errContains: "metrics.port must be between"},
		{name: "Port too high", enabled: true, port: 70000, adminEnable: true, adminPort: 9092, wantErr: true, errContains: "metrics.port must be between"},
		{name: "Same as API port", enabled: true, port: 8080, apiPort: 8080, xdsPort: 18000, adminEnable: true, adminPort: 9092, wantErr: true, errContains: "metrics.port cannot be same as server.api_port"},
		{name: "Same as XDS port", enabled: true, port: 18000, apiPort: 8080, xdsPort: 18000, adminEnable: true, adminPort: 9092, wantErr: true, errContains: "metrics.port cannot be same as server.xds_port"},
		{name: "Same as admin port", enabled: true, port: 9092, apiPort: 8080, xdsPort: 18000, adminEnable: true, adminPort: 9092, wantErr: true, errContains: "metrics.port cannot be same as admin_server.port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Metrics.Enabled = tt.enabled
			cfg.Controller.Metrics.Port = tt.port
			if tt.apiPort != 0 {
				cfg.Controller.Server.APIPort = tt.apiPort
			}
			if tt.xdsPort != 0 {
				cfg.Controller.Server.XDSPort = tt.xdsPort
			}
			cfg.Controller.AdminServer.Enabled = tt.adminEnable
			if tt.adminPort != 0 {
				cfg.Controller.AdminServer.Port = tt.adminPort
			}
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_RouterListenerPort(t *testing.T) {
	tests := []struct {
		name        string
		port        int
		wantErr     bool
		errContains string
	}{
		{name: "Valid port", port: 9090, wantErr: false},
		{name: "Port too low", port: 0, wantErr: true, errContains: "router.listener_port must be between"},
		{name: "Port too high", port: 70000, wantErr: true, errContains: "router.listener_port must be between"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.ListenerPort = tt.port
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig_AdminServerDefaults(t *testing.T) {
	cfg := defaultConfig()
	assert.True(t, cfg.Controller.AdminServer.Enabled)
	assert.Equal(t, 9092, cfg.Controller.AdminServer.Port)
	assert.Equal(t, []string{"*"}, cfg.Controller.AdminServer.AllowedIPs)
}

func TestConfig_Validate_HTTPSPort(t *testing.T) {
	tests := []struct {
		name         string
		httpsEnabled bool
		httpsPort    int
		wantErr      bool
		errContains  string
	}{
		{name: "HTTPS disabled", httpsEnabled: false, httpsPort: 0, wantErr: false},
		{name: "Valid HTTPS port", httpsEnabled: true, httpsPort: 8443, wantErr: true, errContains: "cert_path is required"},
		{name: "Invalid HTTPS port", httpsEnabled: true, httpsPort: 0, wantErr: true, errContains: "router.https_port must be between"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.HTTPSEnabled = tt.httpsEnabled
			cfg.Router.HTTPSPort = tt.httpsPort
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateEventGatewayConfig(t *testing.T) {
	tests := []struct {
		name                  string
		webSubHubPort         int
		webSubHubListenerPort int
		timeoutSeconds        int
		wantErr               bool
		errContains           string
	}{
		{name: "Valid ports", webSubHubPort: 9500, webSubHubListenerPort: 9501, timeoutSeconds: 30, wantErr: false},
		{name: "Invalid hub port low", webSubHubPort: 0, webSubHubListenerPort: 9501, timeoutSeconds: 30, wantErr: true, errContains: "websub_hub_port must be between"},
		{name: "Invalid hub port high", webSubHubPort: 70000, webSubHubListenerPort: 9501, timeoutSeconds: 30, wantErr: true, errContains: "websub_hub_port must be between"},
		{name: "Invalid listener port low", webSubHubPort: 9500, webSubHubListenerPort: 0, timeoutSeconds: 30, wantErr: true, errContains: "websub_hub_listener_port must be between"},
		{name: "Invalid listener port high", webSubHubPort: 9500, webSubHubListenerPort: 70000, timeoutSeconds: 30, wantErr: true, errContains: "websub_hub_listener_port must be between"},
		{name: "Invalid timeout", webSubHubPort: 9500, webSubHubListenerPort: 9501, timeoutSeconds: 0, wantErr: true, errContains: "timeout_seconds must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.EventGateway.Enabled = true
			cfg.Router.EventGateway.WebSubHubPort = tt.webSubHubPort
			cfg.Router.EventGateway.WebSubHubListenerPort = tt.webSubHubListenerPort
			cfg.Router.EventGateway.TimeoutSeconds = tt.timeoutSeconds
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateControlPlaneConfig(t *testing.T) {
	tests := []struct {
		name             string
		host             string
		token            string
		reconnectInitial time.Duration
		reconnectMax     time.Duration
		pollingInterval  time.Duration
		wantErr          bool
		errContains      string
	}{
		{
			name:             "Valid config",
			host:             "localhost",
			reconnectInitial: 1 * time.Second,
			reconnectMax:     30 * time.Second,
			pollingInterval:  5 * time.Second,
			wantErr:          false,
		},
		{
			name:             "Missing host (standalone mode)",
			host:             "",
			reconnectInitial: 1 * time.Second,
			reconnectMax:     30 * time.Second,
			pollingInterval:  5 * time.Second,
			wantErr:          false,
		},
		{
			name:        "Token set without host",
			host:        "",
			token:       "some-token",
			wantErr:     true,
			errContains: "controlplane.host is required when controlplane.token is set",
		},
		{
			name:             "Non-positive reconnect initial",
			host:             "localhost",
			reconnectInitial: 0,
			reconnectMax:     30 * time.Second,
			pollingInterval:  5 * time.Second,
			wantErr:          true,
			errContains:      "controlplane.reconnect_initial must be positive",
		},
		{
			name:             "Non-positive reconnect max",
			host:             "localhost",
			reconnectInitial: 1 * time.Second,
			reconnectMax:     0,
			pollingInterval:  5 * time.Second,
			wantErr:          true,
			errContains:      "controlplane.reconnect_max must be positive",
		},
		{
			name:             "Initial greater than max",
			host:             "localhost",
			reconnectInitial: 60 * time.Second,
			reconnectMax:     30 * time.Second,
			pollingInterval:  5 * time.Second,
			wantErr:          true,
			errContains:      "reconnect_initial",
		},
		{
			name:             "Non-positive polling interval",
			host:             "localhost",
			reconnectInitial: 1 * time.Second,
			reconnectMax:     30 * time.Second,
			pollingInterval:  0,
			wantErr:          true,
			errContains:      "controlplane.polling_interval must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.ControlPlane.Host = tt.host
			cfg.Controller.ControlPlane.Token = tt.token
			cfg.Controller.ControlPlane.ReconnectInitial = tt.reconnectInitial
			cfg.Controller.ControlPlane.ReconnectMax = tt.reconnectMax
			cfg.Controller.ControlPlane.PollingInterval = tt.pollingInterval
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		minVersion  string
		maxVersion  string
		wantErr     bool
		errContains string
	}{
		{name: "Valid TLS 1.2 to 1.3", minVersion: constants.TLSVersion12, maxVersion: constants.TLSVersion13, wantErr: false},
		{name: "Valid TLS 1.0 to 1.3", minVersion: constants.TLSVersion10, maxVersion: constants.TLSVersion13, wantErr: false},
		{name: "Valid same version", minVersion: constants.TLSVersion12, maxVersion: constants.TLSVersion12, wantErr: false},
		{name: "Empty min version", minVersion: "", maxVersion: constants.TLSVersion13, wantErr: true, errContains: "minimum_protocol_version is required"},
		{name: "Empty max version", minVersion: constants.TLSVersion12, maxVersion: "", wantErr: true, errContains: "maximum_protocol_version is required"},
		{name: "Invalid min version", minVersion: "TLS1.5", maxVersion: constants.TLSVersion13, wantErr: true, errContains: "minimum_protocol_version must be one of"},
		{name: "Invalid max version", minVersion: constants.TLSVersion12, maxVersion: "TLS1.5", wantErr: true, errContains: "maximum_protocol_version must be one of"},
		{name: "Min greater than max", minVersion: constants.TLSVersion13, maxVersion: constants.TLSVersion12, wantErr: true, errContains: "cannot be greater than"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.Upstream.TLS.MinimumProtocolVersion = tt.minVersion
			cfg.Router.Upstream.TLS.MaximumProtocolVersion = tt.maxVersion
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateTLSCiphers(t *testing.T) {
	tests := []struct {
		name        string
		ciphers     string
		wantErr     bool
		errContains string
	}{
		{name: "Valid ciphers", ciphers: "ECDHE-RSA-AES256-GCM-SHA384,ECDHE-RSA-AES128-GCM-SHA256", wantErr: false},
		{name: "Empty ciphers", ciphers: "", wantErr: false},
		{name: "Whitespace only", ciphers: "   ", wantErr: true, errContains: "cannot be empty or whitespace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.Upstream.TLS.Ciphers = tt.ciphers
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateTLSTrustedCertPath(t *testing.T) {
	cfg := validConfig()
	cfg.Router.Upstream.TLS.DisableSslVerification = false
	cfg.Router.Upstream.TLS.TrustedCertPath = ""
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trusted_cert_path is required when SSL verification is enabled")

	// With path set
	cfg.Router.Upstream.TLS.TrustedCertPath = "/path/to/ca.crt"
	err = cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_ValidateTimeoutConfig(t *testing.T) {
	tests := []struct {
		name           string
		routeTimeout   uint32
		idleTimeout    uint32
		connectTimeout uint32
		wantErr        bool
		errContains    string
	}{
		{name: "Valid timeouts", routeTimeout: 60000, idleTimeout: 30000, connectTimeout: 5000, wantErr: false},
		{name: "Zero route timeout", routeTimeout: 0, idleTimeout: 30000, connectTimeout: 5000, wantErr: true, errContains: "route_timeout_ms must be positive"},
		{name: "Zero idle timeout", routeTimeout: 60000, idleTimeout: 0, connectTimeout: 5000, wantErr: true, errContains: "route_idle_timeout_ms must be positive"},
		{name: "Zero connect timeout", routeTimeout: 60000, idleTimeout: 30000, connectTimeout: 0, wantErr: true, errContains: "connect_timeout_ms must be positive"},
		{name: "Route exceeds max reasonable", routeTimeout: constants.MaxReasonableTimeoutMs + 1, idleTimeout: 30000, connectTimeout: 5000, wantErr: true, errContains: "exceeds maximum reasonable timeout"},
		{name: "Idle exceeds max reasonable", routeTimeout: 60000, idleTimeout: constants.MaxReasonableTimeoutMs + 1, connectTimeout: 5000, wantErr: true, errContains: "route_idle_timeout_ms"},
		{name: "Connect exceeds max reasonable", routeTimeout: 60000, idleTimeout: 30000, connectTimeout: constants.MaxReasonableTimeoutMs + 1, wantErr: true, errContains: "connect_timeout_ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.Upstream.Timeouts.RouteTimeoutMs = tt.routeTimeout
			cfg.Router.Upstream.Timeouts.RouteIdleTimeoutMs = tt.idleTimeout
			cfg.Router.Upstream.Timeouts.ConnectTimeoutMs = tt.connectTimeout
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidatePolicyEngineConfig(t *testing.T) {
	tests := []struct {
		name             string
		mode             string
		host             string
		port             uint32
		timeoutMs        uint32
		messageTimeoutMs uint32
		routeCacheAction string
		wantErr          bool
		errContains      string
	}{
		{name: "Valid UDS mode (default)", mode: "uds", timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "DEFAULT", wantErr: false},
		{name: "Valid TCP mode", mode: "tcp", host: "localhost", port: 50051, timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "DEFAULT", wantErr: false},
		{name: "TCP missing host", mode: "tcp", host: "", port: 50051, timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "DEFAULT", wantErr: true, errContains: "host is required"},
		{name: "TCP zero port", mode: "tcp", host: "localhost", port: 0, timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "DEFAULT", wantErr: true, errContains: "port is required"},
		{name: "TCP port too high", mode: "tcp", host: "localhost", port: 70000, timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "DEFAULT", wantErr: true, errContains: "port must be between"},
		{name: "Zero timeout", mode: "uds", timeoutMs: 0, messageTimeoutMs: 500, routeCacheAction: "DEFAULT", wantErr: true, errContains: "timeout_ms must be positive"},
		{name: "Zero message timeout", mode: "uds", timeoutMs: 1000, messageTimeoutMs: 0, routeCacheAction: "DEFAULT", wantErr: true, errContains: "message_timeout_ms must be positive"},
		{name: "Invalid route cache action", mode: "uds", timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "INVALID", wantErr: true, errContains: "route_cache_action must be one of"},
		{name: "Valid RETAIN action", mode: "uds", timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "RETAIN", wantErr: false},
		{name: "Valid CLEAR action", mode: "uds", timeoutMs: 1000, messageTimeoutMs: 500, routeCacheAction: "CLEAR", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.PolicyEngine.Mode = tt.mode
			cfg.Router.PolicyEngine.Host = tt.host
			cfg.Router.PolicyEngine.Port = tt.port
			cfg.Router.PolicyEngine.TimeoutMs = tt.timeoutMs
			cfg.Router.PolicyEngine.MessageTimeoutMs = tt.messageTimeoutMs
			cfg.Router.PolicyEngine.RouteCacheAction = tt.routeCacheAction
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidatePolicyEngineTLS(t *testing.T) {
	tests := []struct {
		name        string
		tlsEnabled  bool
		certPath    string
		keyPath     string
		wantErr     bool
		errContains string
	}{
		{name: "TLS disabled", tlsEnabled: false, wantErr: false},
		{name: "TLS enabled no certs", tlsEnabled: true, certPath: "", keyPath: "", wantErr: false},
		{name: "Cert without key", tlsEnabled: true, certPath: "/path/cert.pem", keyPath: "", wantErr: true, errContains: "key_path is required when cert_path is provided"},
		{name: "Key without cert", tlsEnabled: true, certPath: "", keyPath: "/path/key.pem", wantErr: true, errContains: "cert_path is required when key_path is provided"},
		{name: "Both cert and key", tlsEnabled: true, certPath: "/path/cert.pem", keyPath: "/path/key.pem", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.PolicyEngine.Mode = "tcp" // TLS only supported in TCP mode
			cfg.Router.PolicyEngine.Host = "localhost"
			cfg.Router.PolicyEngine.Port = 50051
			cfg.Router.PolicyEngine.TimeoutMs = 1000
			cfg.Router.PolicyEngine.MessageTimeoutMs = 500
			cfg.Router.PolicyEngine.RouteCacheAction = "DEFAULT"
			cfg.Router.PolicyEngine.TLS.Enabled = tt.tlsEnabled
			cfg.Router.PolicyEngine.TLS.CertPath = tt.certPath
			cfg.Router.PolicyEngine.TLS.KeyPath = tt.keyPath
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidatePolicyEngineMode(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		host        string
		port        uint32
		tlsEnabled  bool
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid UDS mode (default)",
			mode: "uds",
		},
		{
			name: "Valid UDS mode (empty defaults to uds)",
			mode: "",
		},
		{
			name: "Valid TCP mode",
			mode: "tcp",
			host: "localhost",
			port: 50051,
		},
		{
			name:        "Invalid mode",
			mode:        "invalid",
			wantErr:     true,
			errContains: "mode must be 'uds' or 'tcp'",
		},
		{
			name:        "UDS with TLS enabled",
			mode:        "uds",
			tlsEnabled:  true,
			wantErr:     true,
			errContains: "tls cannot be enabled when using Unix domain socket",
		},
		{
			name:        "TCP mode missing host",
			mode:        "tcp",
			host:        "",
			port:        50051,
			wantErr:     true,
			errContains: "host is required",
		},
		{
			name:        "TCP mode missing port",
			mode:        "tcp",
			host:        "localhost",
			port:        0,
			wantErr:     true,
			errContains: "port is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.PolicyEngine.Mode = tt.mode
			cfg.Router.PolicyEngine.Host = tt.host
			cfg.Router.PolicyEngine.Port = tt.port
			cfg.Router.PolicyEngine.TimeoutMs = 1000
			cfg.Router.PolicyEngine.MessageTimeoutMs = 500
			cfg.Router.PolicyEngine.RouteCacheAction = "DEFAULT"
			cfg.Router.PolicyEngine.TLS.Enabled = tt.tlsEnabled
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateVHostsConfig(t *testing.T) {
	tests := []struct {
		name           string
		mainDefault    string
		sandboxDefault string
		mainDomains    []string
		sandboxDomains []string
		wantErr        bool
		errContains    string
	}{
		{name: "Valid defaults", mainDefault: "localhost", sandboxDefault: "sandbox.localhost", wantErr: false},
		{name: "Empty main default", mainDefault: "", sandboxDefault: "sandbox.localhost", wantErr: true, errContains: "vhosts.main.default must be a non-empty"},
		{name: "Whitespace main default", mainDefault: "   ", sandboxDefault: "sandbox.localhost", wantErr: true, errContains: "vhosts.main.default must be a non-empty"},
		{name: "Empty sandbox default", mainDefault: "localhost", sandboxDefault: "", wantErr: true, errContains: "vhosts.sandbox.default must be a non-empty"},
		{name: "Valid with domains", mainDefault: "localhost", sandboxDefault: "sandbox.localhost", mainDomains: []string{"api.example.com"}, wantErr: false},
		{name: "Empty domain in list", mainDefault: "localhost", sandboxDefault: "sandbox.localhost", mainDomains: []string{"api.example.com", ""}, wantErr: true, errContains: "must be a non-empty string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.VHosts.Main.Default = tt.mainDefault
			cfg.Router.VHosts.Sandbox.Default = tt.sandboxDefault
			cfg.Router.VHosts.Main.Domains = tt.mainDomains
			cfg.Router.VHosts.Sandbox.Domains = tt.sandboxDomains
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateAnalyticsConfig(t *testing.T) {
	tests := []struct {
		name        string
		enabled     bool
		setupConfig func(*Config)
		wantErr     bool
		errContains string
	}{
		{name: "Analytics disabled", enabled: false, wantErr: false},
		{
			name:    "Analytics enabled with valid UDS config (default mode)",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = "uds"
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 1000
				cfg.Analytics.GRPCEventServerCfg.BufferSizeBytes = 16384
				cfg.Analytics.GRPCEventServerCfg.GRPCRequestTimeout = 5000
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 18090
			},
			wantErr: false,
		},
		{
			name:    "Analytics enabled with valid TCP config",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = "tcp"
				cfg.Analytics.GRPCEventServerCfg.Port = 18090
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 1000
				cfg.Analytics.GRPCEventServerCfg.BufferSizeBytes = 16384
				cfg.Analytics.GRPCEventServerCfg.GRPCRequestTimeout = 5000
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 18090
			},
			wantErr: false,
		},
		{
			name:    "Analytics enabled with empty mode defaults to UDS",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = ""
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 1000
				cfg.Analytics.GRPCEventServerCfg.BufferSizeBytes = 16384
				cfg.Analytics.GRPCEventServerCfg.GRPCRequestTimeout = 5000
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 18090
			},
			wantErr: false,
		},
		{
			name:    "Invalid mode value",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = "invalid"
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 1000
				cfg.Analytics.GRPCEventServerCfg.BufferSizeBytes = 16384
				cfg.Analytics.GRPCEventServerCfg.GRPCRequestTimeout = 5000
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 18090
			},
			wantErr:     true,
			errContains: "grpc_event_server.mode must be 'uds' or 'tcp'",
		},
		{
			name:    "TCP mode - invalid port",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = "tcp"
				cfg.Analytics.GRPCEventServerCfg.Port = 0
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 1000
				cfg.Analytics.GRPCEventServerCfg.BufferSizeBytes = 16384
				cfg.Analytics.GRPCEventServerCfg.GRPCRequestTimeout = 5000
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 18090
			},
			wantErr:     true,
			errContains: "grpc_event_server.port must be between 1 and 65535",
		},
		{
			name:    "Invalid server port",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = "uds"
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 1000
				cfg.Analytics.GRPCEventServerCfg.BufferSizeBytes = 16384
				cfg.Analytics.GRPCEventServerCfg.GRPCRequestTimeout = 5000
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 0
			},
			wantErr:     true,
			errContains: "grpc_event_server.server_port must be between 1 and 65535",
		},
		{
			name:    "Invalid buffer flush interval",
			enabled: true,
			setupConfig: func(cfg *Config) {
				cfg.Analytics.GRPCEventServerCfg.Mode = "uds"
				cfg.Analytics.GRPCEventServerCfg.BufferFlushInterval = 0
				cfg.Analytics.GRPCEventServerCfg.ServerPort = 18090
			},
			wantErr:     true,
			errContains: "invalid gRPC event server configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Analytics.Enabled = tt.enabled
			if tt.setupConfig != nil {
				tt.setupConfig(cfg)
			}
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateAuthConfig(t *testing.T) {
	tests := []struct {
		name        string
		idpEnabled  bool
		roleMapping map[string][]string
		wantErr     bool
		errContains string
	}{
		{name: "IDP disabled", idpEnabled: false, wantErr: false},
		{name: "IDP enabled no role mapping", idpEnabled: true, roleMapping: nil, wantErr: false},
		{name: "IDP enabled single wildcard", idpEnabled: true, roleMapping: map[string][]string{"admin": {"*"}}, wantErr: false},
		{name: "IDP enabled multiple wildcards", idpEnabled: true, roleMapping: map[string][]string{"admin": {"*"}, "user": {"*"}}, wantErr: true, errContains: "multiple wildcard ('*') mappings detected"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Controller.Auth.IDP.Enabled = tt.idpEnabled
			cfg.Controller.Auth.IDP.RoleMapping = tt.roleMapping
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateAPIKeyConfig(t *testing.T) {
	tests := []struct {
		name        string
		keysPerUser int
		algorithm   string
		wantErr     bool
		errContains string
	}{
		{name: "Valid config", keysPerUser: 5, algorithm: constants.HashingAlgorithmSHA256, wantErr: false},
		{name: "Empty algorithm defaults", keysPerUser: 5, algorithm: "", wantErr: false},
		{name: "Invalid algorithm", keysPerUser: 5, algorithm: "md5", wantErr: true, errContains: "api_key.algorithm must be sha256"},
		{name: "Zero keys per user", keysPerUser: 0, algorithm: constants.HashingAlgorithmSHA256, wantErr: true, errContains: "api_keys_per_user_per_api must be a positive integer"},
		{name: "Negative keys per user", keysPerUser: -1, algorithm: constants.HashingAlgorithmSHA256, wantErr: true, errContains: "api_keys_per_user_per_api must be a positive integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.APIKey.APIKeysPerUserPerAPI = tt.keysPerUser
			cfg.APIKey.Algorithm = tt.algorithm
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateHTTPListenerConfig(t *testing.T) {
	tests := []struct {
		name                       string
		serverHeaderTransformation string
		wantErr                    bool
		errContains                string
	}{
		{name: "Valid OVERWRITE", serverHeaderTransformation: commonconstants.OVERWRITE, wantErr: false},
		{name: "Valid APPEND_IF_ABSENT", serverHeaderTransformation: commonconstants.APPEND_IF_ABSENT, wantErr: false},
		{name: "Valid PASS_THROUGH", serverHeaderTransformation: commonconstants.PASS_THROUGH, wantErr: false},
		{name: "Empty defaults to OVERWRITE", serverHeaderTransformation: "", wantErr: false},
		{name: "Invalid transformation", serverHeaderTransformation: "INVALID", wantErr: true, errContains: "server_header_transformation must be one of"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.HTTPListener.ServerHeaderTransformation = tt.serverHeaderTransformation
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ResolvedEventHubConnectionPool_InheritsPrimaryStorage(t *testing.T) {
	cfg := validConfig()
	cfg.Controller.Storage.Type = "postgres"
	cfg.Controller.Storage.Postgres.Host = "primary-db"
	cfg.Controller.Storage.Postgres.Database = "gateway"
	cfg.Controller.Storage.Postgres.User = "gateway_user"
	cfg.Controller.Storage.Postgres.MaxOpenConns = 14
	cfg.Controller.Storage.Postgres.MaxIdleConns = 6
	cfg.Controller.Storage.Postgres.ConnMaxLifetime = 45 * time.Minute
	cfg.Controller.Storage.Postgres.ConnMaxIdleTime = 7 * time.Minute

	err := cfg.Validate()
	assert.NoError(t, err)
	if err != nil {
		return
	}

	poolCfg := cfg.ResolvedEventHubConnectionPool()
	assert.Equal(t, 14, poolCfg.MaxOpenConns)
	assert.Equal(t, 6, poolCfg.MaxIdleConns)
	assert.Equal(t, 45*time.Minute, poolCfg.ConnMaxLifetime)
	assert.Equal(t, 7*time.Minute, poolCfg.ConnMaxIdleTime)
}

func TestConfig_ResolvedEventHubConnectionPool_AllowsOverrides(t *testing.T) {
	cfg := validConfig()
	cfg.Controller.Storage.Type = "postgres"
	cfg.Controller.Storage.Postgres.Host = "primary-db"
	cfg.Controller.Storage.Postgres.Database = "gateway"
	cfg.Controller.Storage.Postgres.User = "gateway_user"
	cfg.Controller.Storage.Postgres.MaxOpenConns = 14
	cfg.Controller.Storage.Postgres.MaxIdleConns = 6
	cfg.Controller.Storage.Postgres.ConnMaxLifetime = 45 * time.Minute
	cfg.Controller.Storage.Postgres.ConnMaxIdleTime = 7 * time.Minute
	cfg.Controller.EventHub.ConnectionPool.MaxOpenConns = 3
	cfg.Controller.EventHub.ConnectionPool.MaxIdleConns = 2
	cfg.Controller.EventHub.ConnectionPool.ConnMaxIdleTime = 2 * time.Minute

	err := cfg.Validate()
	assert.NoError(t, err)
	if err != nil {
		return
	}

	poolCfg := cfg.ResolvedEventHubConnectionPool()
	assert.Equal(t, 3, poolCfg.MaxOpenConns)
	assert.Equal(t, 2, poolCfg.MaxIdleConns)
	assert.Equal(t, 45*time.Minute, poolCfg.ConnMaxLifetime)
	assert.Equal(t, 2*time.Minute, poolCfg.ConnMaxIdleTime)
}

func TestConfig_ResolvedEventHubConnectionPool_UsesSQLiteDefaults(t *testing.T) {
	cfg := validConfig()
	cfg.Controller.Storage.Type = "sqlite"
	cfg.Controller.Storage.SQLite.Path = "/tmp/controller.db"

	err := cfg.Validate()
	assert.NoError(t, err)
	if err != nil {
		return
	}

	poolCfg := cfg.ResolvedEventHubConnectionPool()
	assert.Equal(t, 1, poolCfg.MaxOpenConns)
	assert.Equal(t, 1, poolCfg.MaxIdleConns)
	assert.Equal(t, time.Duration(0), poolCfg.ConnMaxLifetime)
	assert.Equal(t, time.Duration(0), poolCfg.ConnMaxIdleTime)
}

func TestConfig_Validate_EventHubConnectionPoolRejectsInvalidValues(t *testing.T) {
	cfg := validConfig()
	cfg.Controller.Storage.Type = "sqlite"
	cfg.Controller.Storage.SQLite.Path = "/tmp/controller.db"
	cfg.Controller.EventHub.ConnectionPool.MaxOpenConns = -1

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "eventhub.connection_pool.max_open_conns must be >= 1")
}

func TestConfig_HelperMethods(t *testing.T) {
	t.Run("IsPersistentMode", func(t *testing.T) {
		cfg := validConfig()
		cfg.Controller.Storage.Type = "sqlite"
		assert.True(t, cfg.IsPersistentMode())

		cfg.Controller.Storage.Type = "postgres"
		assert.True(t, cfg.IsPersistentMode())

		cfg.Controller.Storage.Type = "memory"
		assert.False(t, cfg.IsPersistentMode())
	})

	t.Run("IsMemoryOnlyMode", func(t *testing.T) {
		cfg := validConfig()
		cfg.Controller.Storage.Type = "memory"
		assert.True(t, cfg.IsMemoryOnlyMode())

		cfg.Controller.Storage.Type = "sqlite"
		assert.False(t, cfg.IsMemoryOnlyMode())
	})

	t.Run("IsAccessLogsEnabled", func(t *testing.T) {
		cfg := validConfig()
		cfg.Router.AccessLogs.Enabled = true
		assert.True(t, cfg.IsAccessLogsEnabled())

		cfg.Router.AccessLogs.Enabled = false
		assert.False(t, cfg.IsAccessLogsEnabled())
	})
}

func TestValidateDomains(t *testing.T) {
	tests := []struct {
		name    string
		domains []string
		wantErr bool
	}{
		{name: "nil domains", domains: nil, wantErr: false},
		{name: "valid domains", domains: []string{"api.example.com", "*.example.com"}, wantErr: false},
		{name: "empty domain", domains: []string{"api.example.com", ""}, wantErr: true},
		{name: "whitespace domain", domains: []string{"   "}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDomains("test.field", tt.domains)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ValidateDownstreamTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		certPath    string
		keyPath     string
		minVersion  string
		maxVersion  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Valid config",
			certPath:   "/path/to/cert.pem",
			keyPath:    "/path/to/key.pem",
			minVersion: constants.TLSVersion12,
			maxVersion: constants.TLSVersion13,
			wantErr:    false,
		},
		{
			name:        "Missing cert path",
			certPath:    "",
			keyPath:     "/path/to/key.pem",
			minVersion:  constants.TLSVersion12,
			maxVersion:  constants.TLSVersion13,
			wantErr:     true,
			errContains: "cert_path is required",
		},
		{
			name:        "Missing key path",
			certPath:    "/path/to/cert.pem",
			keyPath:     "",
			minVersion:  constants.TLSVersion12,
			maxVersion:  constants.TLSVersion13,
			wantErr:     true,
			errContains: "key_path is required",
		},
		{
			name:        "Invalid min version",
			certPath:    "/path/to/cert.pem",
			keyPath:     "/path/to/key.pem",
			minVersion:  "invalid",
			maxVersion:  constants.TLSVersion13,
			wantErr:     true,
			errContains: "minimum_protocol_version must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.Router.HTTPSEnabled = true
			cfg.Router.HTTPSPort = 8443
			cfg.Router.DownstreamTLS.CertPath = tt.certPath
			cfg.Router.DownstreamTLS.KeyPath = tt.keyPath
			cfg.Router.DownstreamTLS.MinimumProtocolVersion = tt.minVersion
			cfg.Router.DownstreamTLS.MaximumProtocolVersion = tt.maxVersion
			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_CompleteValidConfig(t *testing.T) {
	cfg := validConfig()
	err := cfg.Validate()
	assert.NoError(t, err, "A complete valid configuration should pass validation")
}

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, "sqlite", cfg.Controller.Storage.Type)
	assert.Equal(t, "info", cfg.Controller.Logging.Level)
	assert.Equal(t, uint32(5000), cfg.Router.Upstream.Timeouts.ConnectTimeoutMs, "default router.upstream.timeouts.connect_timeout_ms should be 5s (5000 ms)")
}

func TestConfig_CaseInsensitiveAlgorithm(t *testing.T) {
	cfg := validConfig()
	cfg.APIKey.Algorithm = strings.ToUpper(constants.HashingAlgorithmSHA256)
	err := cfg.Validate()
	assert.NoError(t, err, "Algorithm validation should be case insensitive")
}
