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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a valid configuration for testing
func validConfig() *Config {
	return &Config{
		PolicyEngine: PolicyEngine{
			Server: ServerConfig{
				ExtProcPort: 9001,
			},
			Admin: AdminConfig{
				Enabled:    true,
				Port:       9002,
				AllowedIPs: []string{"127.0.0.1"},
			},
			Metrics: MetricsConfig{
				Enabled: false,
				Port:    9003,
			},
			ConfigMode: ConfigModeConfig{
				Mode: "file",
			},
			XDS: XDSConfig{
				Enabled:               false,
				ServerAddress:         "localhost:18000",
				NodeID:                "policy-engine",
				Cluster:               "policy-engine-cluster",
				ConnectTimeout:        10 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     60 * time.Second,
			},
			FileConfig: FileConfigConfig{
				Path: "configs/policy-chains.yaml",
			},
			Logging: LoggingConfig{
				Level:  "info",
				Format: "json",
			},
		},
		Analytics: AnalyticsConfig{
			Enabled: false,
		},
		TracingConfig: TracingConfig{
			Enabled: false,
		},
	}
}

// TestValidate_ValidConfig tests that a valid configuration passes validation
func TestValidate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

// TestValidate_ExtProcPort tests extproc port validation (TCP mode only)
func TestValidate_ExtProcPort(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid port",
			port:      9001,
			expectErr: false,
		},
		{
			name:      "minimum valid port",
			port:      1,
			expectErr: false,
		},
		{
			name:      "maximum valid port",
			port:      65535,
			expectErr: false,
		},
		{
			name:      "zero port",
			port:      0,
			expectErr: true,
			errMsg:    "invalid extproc_port",
		},
		{
			name:      "negative port",
			port:      -1,
			expectErr: true,
			errMsg:    "invalid extproc_port",
		},
		{
			name:      "port exceeds maximum",
			port:      65536,
			expectErr: true,
			errMsg:    "invalid extproc_port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.PolicyEngine.Server.Mode = "tcp" // Port validation only applies in TCP mode
			cfg.PolicyEngine.Server.ExtProcPort = tt.port

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_ServerMode tests server mode validation (same pattern as gateway-controller)
func TestValidate_ServerMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		port      int
		expectErr bool
		errMsg    string
	}{
		{
			name:      "UDS mode explicit",
			mode:      "uds",
			port:      0, // Port ignored in UDS mode
			expectErr: false,
		},
		{
			name:      "UDS mode default (empty string)",
			mode:      "",
			port:      0, // Port ignored in UDS mode
			expectErr: false,
		},
		{
			name:      "TCP mode with valid port",
			mode:      "tcp",
			port:      9001,
			expectErr: false,
		},
		{
			name:      "TCP mode with invalid port - zero",
			mode:      "tcp",
			port:      0,
			expectErr: true,
			errMsg:    "invalid extproc_port",
		},
		{
			name:      "TCP mode with invalid port - too high",
			mode:      "tcp",
			port:      70000,
			expectErr: true,
			errMsg:    "invalid extproc_port",
		},
		{
			name:      "invalid mode",
			mode:      "invalid",
			port:      9001,
			expectErr: true,
			errMsg:    "server.mode must be 'uds' or 'tcp'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.PolicyEngine.Server.Mode = tt.mode
			cfg.PolicyEngine.Server.ExtProcPort = tt.port

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_UDS_PortConflict tests that UDS mode skips port conflict checks
func TestValidate_UDS_PortConflict(t *testing.T) {
	t.Run("UDS mode - admin port conflict with extproc port ignored", func(t *testing.T) {
		cfg := validConfig()
		cfg.PolicyEngine.Server.Mode = "uds"
		cfg.PolicyEngine.Server.ExtProcPort = 9002 // Same as admin port but should be ignored
		cfg.PolicyEngine.Admin.Enabled = true
		cfg.PolicyEngine.Admin.Port = 9002

		err := cfg.Validate()
		assert.NoError(t, err, "Port conflict check should be skipped when UDS mode is used")
	})

	t.Run("TCP mode - admin port conflict with extproc port detected", func(t *testing.T) {
		cfg := validConfig()
		cfg.PolicyEngine.Server.Mode = "tcp"
		cfg.PolicyEngine.Server.ExtProcPort = 9002
		cfg.PolicyEngine.Admin.Enabled = true
		cfg.PolicyEngine.Admin.Port = 9002

		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "admin.port cannot be same as server.extproc_port")
	})

	t.Run("UDS mode - metrics port conflict with extproc port ignored", func(t *testing.T) {
		cfg := validConfig()
		cfg.PolicyEngine.Server.Mode = "uds"
		cfg.PolicyEngine.Server.ExtProcPort = 9003 // Same as metrics port but should be ignored
		cfg.PolicyEngine.Metrics.Enabled = true
		cfg.PolicyEngine.Metrics.Port = 9003

		err := cfg.Validate()
		assert.NoError(t, err, "Port conflict check should be skipped when UDS mode is used")
	})
}

// TestValidate_AdminConfig tests admin configuration validation
func TestValidate_AdminConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "admin disabled - no validation",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Admin.Enabled = false
				cfg.PolicyEngine.Admin.Port = 0 // invalid but should pass since disabled
			},
			expectErr: false,
		},
		{
			name: "admin enabled - valid config",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Admin.Enabled = true
				cfg.PolicyEngine.Admin.Port = 9002
				cfg.PolicyEngine.Admin.AllowedIPs = []string{"127.0.0.1"}
			},
			expectErr: false,
		},
		{
			name: "admin enabled - invalid port zero",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Admin.Enabled = true
				cfg.PolicyEngine.Admin.Port = 0
				cfg.PolicyEngine.Admin.AllowedIPs = []string{"127.0.0.1"}
			},
			expectErr: true,
			errMsg:    "invalid admin.port",
		},
		{
			name: "admin enabled - port exceeds max",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Admin.Enabled = true
				cfg.PolicyEngine.Admin.Port = 70000
				cfg.PolicyEngine.Admin.AllowedIPs = []string{"127.0.0.1"}
			},
			expectErr: true,
			errMsg:    "invalid admin.port",
		},
		{
			name: "admin port conflicts with extproc port (TCP mode)",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Server.Mode = "tcp" // Port conflict only checked in TCP mode
				cfg.PolicyEngine.Admin.Enabled = true
				cfg.PolicyEngine.Admin.Port = 9001 // same as extproc
				cfg.PolicyEngine.Server.ExtProcPort = 9001
				cfg.PolicyEngine.Admin.AllowedIPs = []string{"127.0.0.1"}
			},
			expectErr: true,
			errMsg:    "admin.port cannot be same as server.extproc_port",
		},
		{
			name: "admin enabled - empty allowed IPs",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Admin.Enabled = true
				cfg.PolicyEngine.Admin.Port = 9002
				cfg.PolicyEngine.Admin.AllowedIPs = []string{}
			},
			expectErr: true,
			errMsg:    "admin.allowed_ips cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_MetricsConfig tests metrics configuration validation
func TestValidate_MetricsConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "metrics disabled - no validation",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Metrics.Enabled = false
			},
			expectErr: false,
		},
		{
			name: "metrics enabled - valid config",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Metrics.Enabled = true
				cfg.PolicyEngine.Metrics.Port = 9003
			},
			expectErr: false,
		},
		{
			name: "metrics enabled - invalid port",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Metrics.Enabled = true
				cfg.PolicyEngine.Metrics.Port = 0
			},
			expectErr: true,
			errMsg:    "invalid metrics.port",
		},
		{
			name: "metrics port conflicts with extproc port (TCP mode)",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Server.Mode = "tcp" // Port conflict only checked in TCP mode
				cfg.PolicyEngine.Metrics.Enabled = true
				cfg.PolicyEngine.Metrics.Port = 9001
				cfg.PolicyEngine.Server.ExtProcPort = 9001
			},
			expectErr: true,
			errMsg:    "metrics.port cannot be same as server.extproc_port",
		},
		{
			name: "metrics port conflicts with admin port",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.Metrics.Enabled = true
				cfg.PolicyEngine.Metrics.Port = 9002
				cfg.PolicyEngine.Admin.Enabled = true
				cfg.PolicyEngine.Admin.Port = 9002
			},
			expectErr: true,
			errMsg:    "metrics.port cannot be same as admin.port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_ConfigMode tests config mode validation
func TestValidate_ConfigMode(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid file mode",
			mode:      "file",
			expectErr: false,
		},
		{
			name:      "valid xds mode",
			mode:      "xds",
			expectErr: false,
		},
		{
			name:      "invalid mode",
			mode:      "invalid",
			expectErr: true,
			errMsg:    "invalid config_mode.mode",
		},
		{
			name:      "empty mode",
			mode:      "",
			expectErr: true,
			errMsg:    "invalid config_mode.mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.PolicyEngine.ConfigMode.Mode = tt.mode

			// For xds mode, enable xDS with valid config
			if tt.mode == "xds" {
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			}

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_FileConfig tests file config validation
func TestValidate_FileConfig(t *testing.T) {
	cfg := validConfig()
	cfg.PolicyEngine.ConfigMode.Mode = "file"
	cfg.PolicyEngine.FileConfig.Path = ""

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file_config.path is required")
}

// TestValidate_XDSConfig tests xDS configuration validation
func TestValidate_XDSConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "xds mode without xds enabled",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = false
			},
			expectErr: true,
			errMsg:    "xds.enabled must be true when config_mode.mode is 'xds'",
		},
		{
			name: "xds enabled - missing server address",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = ""
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			},
			expectErr: true,
			errMsg:    "xds.server_address is required",
		},
		{
			name: "xds enabled - missing node ID",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = ""
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			},
			expectErr: true,
			errMsg:    "xds.node_id is required",
		},
		{
			name: "xds enabled - missing cluster",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = ""
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			},
			expectErr: true,
			errMsg:    "xds.cluster is required",
		},
		{
			name: "xds enabled - invalid connect timeout",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 0
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			},
			expectErr: true,
			errMsg:    "xds.connect_timeout must be positive",
		},
		{
			name: "xds enabled - invalid request timeout",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 0
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			},
			expectErr: true,
			errMsg:    "xds.request_timeout must be positive",
		},
		{
			name: "xds enabled - invalid initial reconnect delay",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 0
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
			},
			expectErr: true,
			errMsg:    "xds.initial_reconnect_delay must be positive",
		},
		{
			name: "xds enabled - invalid max reconnect delay",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 0
			},
			expectErr: true,
			errMsg:    "xds.max_reconnect_delay must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_XDSTLSConfig tests xDS TLS configuration validation
func TestValidate_XDSTLSConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "TLS disabled - no validation",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
				cfg.PolicyEngine.XDS.TLS.Enabled = false
			},
			expectErr: false,
		},
		{
			name: "TLS enabled - missing cert path",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
				cfg.PolicyEngine.XDS.TLS.Enabled = true
				cfg.PolicyEngine.XDS.TLS.CertPath = ""
				cfg.PolicyEngine.XDS.TLS.KeyPath = "/path/to/key"
				cfg.PolicyEngine.XDS.TLS.CAPath = "/path/to/ca"
			},
			expectErr: true,
			errMsg:    "xds.tls.cert_path is required",
		},
		{
			name: "TLS enabled - missing key path",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
				cfg.PolicyEngine.XDS.TLS.Enabled = true
				cfg.PolicyEngine.XDS.TLS.CertPath = "/path/to/cert"
				cfg.PolicyEngine.XDS.TLS.KeyPath = ""
				cfg.PolicyEngine.XDS.TLS.CAPath = "/path/to/ca"
			},
			expectErr: true,
			errMsg:    "xds.tls.key_path is required",
		},
		{
			name: "TLS enabled - missing CA path",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
				cfg.PolicyEngine.XDS.TLS.Enabled = true
				cfg.PolicyEngine.XDS.TLS.CertPath = "/path/to/cert"
				cfg.PolicyEngine.XDS.TLS.KeyPath = "/path/to/key"
				cfg.PolicyEngine.XDS.TLS.CAPath = ""
			},
			expectErr: true,
			errMsg:    "xds.tls.ca_path is required",
		},
		{
			name: "TLS enabled - valid config",
			setup: func(cfg *Config) {
				cfg.PolicyEngine.ConfigMode.Mode = "xds"
				cfg.PolicyEngine.XDS.Enabled = true
				cfg.PolicyEngine.XDS.ServerAddress = "localhost:18000"
				cfg.PolicyEngine.XDS.NodeID = "test-node"
				cfg.PolicyEngine.XDS.Cluster = "test-cluster"
				cfg.PolicyEngine.XDS.ConnectTimeout = 10 * time.Second
				cfg.PolicyEngine.XDS.RequestTimeout = 5 * time.Second
				cfg.PolicyEngine.XDS.InitialReconnectDelay = 1 * time.Second
				cfg.PolicyEngine.XDS.MaxReconnectDelay = 60 * time.Second
				cfg.PolicyEngine.XDS.TLS.Enabled = true
				cfg.PolicyEngine.XDS.TLS.CertPath = "/path/to/cert"
				cfg.PolicyEngine.XDS.TLS.KeyPath = "/path/to/key"
				cfg.PolicyEngine.XDS.TLS.CAPath = "/path/to/ca"
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_LoggingConfig tests logging configuration validation
func TestValidate_LoggingConfig(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		format    string
		expectErr bool
		errMsg    string
	}{
		{
			name:      "valid debug level json format",
			level:     "debug",
			format:    "json",
			expectErr: false,
		},
		{
			name:      "valid info level text format",
			level:     "info",
			format:    "text",
			expectErr: false,
		},
		{
			name:      "valid warn level",
			level:     "warn",
			format:    "json",
			expectErr: false,
		},
		{
			name:      "valid error level",
			level:     "error",
			format:    "json",
			expectErr: false,
		},
		{
			name:      "invalid level",
			level:     "invalid",
			format:    "json",
			expectErr: true,
			errMsg:    "invalid logging.level",
		},
		{
			name:      "invalid format",
			level:     "info",
			format:    "xml",
			expectErr: true,
			errMsg:    "invalid logging.format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			cfg.PolicyEngine.Logging.Level = tt.level
			cfg.PolicyEngine.Logging.Format = tt.format

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_TracingConfig tests tracing configuration validation
func TestValidate_TracingConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "tracing disabled - no validation",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = false
			},
			expectErr: false,
		},
		{
			name: "tracing enabled - valid config",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = "otel-collector:4317"
				cfg.TracingConfig.BatchTimeout = 1 * time.Second
				cfg.TracingConfig.MaxExportBatchSize = 512
				cfg.TracingConfig.SamplingRate = 1.0
			},
			expectErr: false,
		},
		{
			name: "tracing enabled - missing endpoint",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = ""
				cfg.TracingConfig.BatchTimeout = 1 * time.Second
				cfg.TracingConfig.MaxExportBatchSize = 512
				cfg.TracingConfig.SamplingRate = 1.0
			},
			expectErr: true,
			errMsg:    "tracing.endpoint is required",
		},
		{
			name: "tracing enabled - invalid batch timeout",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = "otel-collector:4317"
				cfg.TracingConfig.BatchTimeout = 0
				cfg.TracingConfig.MaxExportBatchSize = 512
				cfg.TracingConfig.SamplingRate = 1.0
			},
			expectErr: true,
			errMsg:    "tracing.batch_timeout must be positive",
		},
		{
			name: "tracing enabled - invalid max export batch size",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = "otel-collector:4317"
				cfg.TracingConfig.BatchTimeout = 1 * time.Second
				cfg.TracingConfig.MaxExportBatchSize = 0
				cfg.TracingConfig.SamplingRate = 1.0
			},
			expectErr: true,
			errMsg:    "tracing.max_export_batch_size must be positive",
		},
		{
			name: "tracing enabled - sampling rate zero",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = "otel-collector:4317"
				cfg.TracingConfig.BatchTimeout = 1 * time.Second
				cfg.TracingConfig.MaxExportBatchSize = 512
				cfg.TracingConfig.SamplingRate = 0
			},
			expectErr: true,
			errMsg:    "tracing.sampling_rate must be > 0.0 and <= 1.0",
		},
		{
			name: "tracing enabled - sampling rate exceeds 1.0",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = "otel-collector:4317"
				cfg.TracingConfig.BatchTimeout = 1 * time.Second
				cfg.TracingConfig.MaxExportBatchSize = 512
				cfg.TracingConfig.SamplingRate = 1.5
			},
			expectErr: true,
			errMsg:    "tracing.sampling_rate must be > 0.0 and <= 1.0",
		},
		{
			name: "tracing enabled - valid sampling rate 0.5",
			setup: func(cfg *Config) {
				cfg.TracingConfig.Enabled = true
				cfg.TracingConfig.Endpoint = "otel-collector:4317"
				cfg.TracingConfig.BatchTimeout = 1 * time.Second
				cfg.TracingConfig.MaxExportBatchSize = 512
				cfg.TracingConfig.SamplingRate = 0.5
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_AnalyticsConfig tests analytics configuration validation
func TestValidate_AnalyticsConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "analytics disabled - no validation",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = false
			},
			expectErr: false,
		},
		{
			name: "analytics enabled - valid config",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
			},
			expectErr: false,
		},
		{
			name: "analytics enabled - invalid ALS port",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         0,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
			},
			expectErr: true,
			errMsg:    "als_server_port must be between 1 and 65535",
		},
		{
			name: "analytics enabled - invalid shutdown timeout",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       0,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
			},
			expectErr: true,
			errMsg:    "shutdown_timeout must be positive",
		},
		{
			name: "analytics enabled - invalid max message size",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 0,
					ExtProcMaxHeaderLimit: 8192,
				}
			},
			expectErr: true,
			errMsg:    "max_message_size must be positive",
		},
		{
			name: "analytics enabled - invalid max header limit",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 0,
				}
			},
			expectErr: true,
			errMsg:    "max_header_limit must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidate_AnalyticsPublishers tests analytics publisher validation
func TestValidate_AnalyticsPublishers(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Config)
		expectErr bool
		errMsg    string
	}{
		{
			name: "publisher disabled - no validation",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{Enabled: false, Type: ""},
				}
			},
			expectErr: false,
		},
		{
			name: "publisher enabled - missing type",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{Enabled: true, Type: ""},
				}
			},
			expectErr: true,
			errMsg:    "type is required when enabled",
		},
		{
			name: "publisher enabled - unknown type",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{Enabled: true, Type: "unknown"},
				}
			},
			expectErr: true,
			errMsg:    "unknown publisher type",
		},
		{
			name: "moesif publisher - missing settings",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{Enabled: true, Type: "moesif", Settings: nil},
				}
			},
			expectErr: true,
			errMsg:    "settings is required for type 'moesif'",
		},
		{
			name: "moesif publisher - missing application_id",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{Enabled: true, Type: "moesif", Settings: map[string]interface{}{}},
				}
			},
			expectErr: true,
			errMsg:    "application_id is required",
		},
		{
			name: "moesif publisher - valid config",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{
						Enabled: true,
						Type:    "moesif",
						Settings: map[string]interface{}{
							"application_id": "test-app-id",
						},
					},
				}
			},
			expectErr: false,
		},
		{
			name: "moesif publisher - invalid publish_interval (zero)",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{
						Enabled: true,
						Type:    "moesif",
						Settings: map[string]interface{}{
							"application_id":   "test-app-id",
							"publish_interval": 0,
						},
					},
				}
			},
			expectErr: true,
			errMsg:    "publish_interval must be > 0",
		},
		{
			name: "moesif publisher - valid publish_interval",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{
						Enabled: true,
						Type:    "moesif",
						Settings: map[string]interface{}{
							"application_id":   "test-app-id",
							"publish_interval": 30,
						},
					},
				}
			},
			expectErr: false,
		},
		{
			name: "moesif publisher - invalid publish_interval type",
			setup: func(cfg *Config) {
				cfg.Analytics.Enabled = true
				cfg.Analytics.AccessLogsServiceCfg = AccessLogsServiceConfig{
					ALSServerPort:         18090,
					ShutdownTimeout:       600 * time.Second,
					ExtProcMaxMessageSize: 1000000,
					ExtProcMaxHeaderLimit: 8192,
				}
				cfg.Analytics.Publishers = []PublisherConfig{
					{
						Enabled: true,
						Type:    "moesif",
						Settings: map[string]interface{}{
							"application_id":   "test-app-id",
							"publish_interval": "30s",
						},
					},
				}
			},
			expectErr: true,
			errMsg:    "publish_interval must be an integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.setup(cfg)

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLoad_ValidConfigFile tests loading a valid configuration file
func TestLoad_ValidConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[policy_engine.server]
extproc_port = 9001

[policy_engine.admin]
enabled = true
port = 9002
allowed_ips = ["127.0.0.1"]

[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"

[policy_engine.logging]
level = "info"
format = "json"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, 9001, cfg.PolicyEngine.Server.ExtProcPort)
	assert.Equal(t, 9002, cfg.PolicyEngine.Admin.Port)
	assert.True(t, cfg.PolicyEngine.Admin.Enabled)
}

// TestLoad_EmptyPath tests loading with empty path (defaults only)
func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should have default values
	assert.Equal(t, 9001, cfg.PolicyEngine.Server.ExtProcPort)
	assert.Equal(t, 9002, cfg.PolicyEngine.Admin.Port)
	assert.Equal(t, "xds", cfg.PolicyEngine.ConfigMode.Mode)
}

// TestLoad_NonExistentFile tests loading a non-existent file
func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to load config file")
}

// TestLoad_InvalidYAML tests loading an invalid YAML file
func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	invalidYAML := `
policy_engine:
  server:
    extproc_port: "not a number"  # Invalid - should be int
    - invalid yaml structure
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

// TestLoad_InvalidConfig tests loading a file with invalid configuration
func TestLoad_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Use an invalid server mode to trigger validation error
	invalidConfig := `
[policy_engine.server]
mode = "invalid"
extproc_port = 9001

[policy_engine.admin]
enabled = true
port = 9002
allowed_ips = ["127.0.0.1"]

[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"

[policy_engine.logging]
level = "info"
format = "json"
`
	err := os.WriteFile(configPath, []byte(invalidConfig), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid configuration")
}

// TestLoad_RawConfigPopulated tests that RawConfig is populated after loading
func TestLoad_RawConfigPopulated(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[policy_engine.server]
extproc_port = 9001

[policy_engine.admin]
enabled = true
port = 9002
allowed_ips = ["127.0.0.1"]

[policy_engine.config_mode]
mode = "file"

[policy_engine.file_config]
path = "configs/policy-chains.yaml"

[policy_engine.logging]
level = "info"
format = "json"

[policy_configurations]
custom_key = "custom_value"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)
	assert.NotNil(t, cfg.PolicyEngine.RawConfig)
	assert.NotEmpty(t, cfg.PolicyEngine.RawConfig)
}

// TestDefaultConfig tests that default configuration is valid
func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}
