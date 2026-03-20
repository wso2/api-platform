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

package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// =============================================================================
// setupLogger Tests
// =============================================================================

func TestSetupLogger_DebugLevel(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "debug",
				Format: "text",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

func TestSetupLogger_InfoLevel(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "info",
				Format: "text",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelDebug))
}

func TestSetupLogger_WarnLevel(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "warn",
				Format: "text",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

func TestSetupLogger_ErrorLevel(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "error",
				Format: "text",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	assert.True(t, logger.Enabled(context.Background(), slog.LevelError))
	assert.False(t, logger.Enabled(context.Background(), slog.LevelWarn))
}

func TestSetupLogger_DefaultLevel(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "invalid",
				Format: "text",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	// Should default to Info level
	assert.True(t, logger.Enabled(context.Background(), slog.LevelInfo))
}

func TestSetupLogger_JSONFormat(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "info",
				Format: "json",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	// Logger should be created successfully with JSON format
	assert.NotNil(t, logger)
}

func TestSetupLogger_TextFormat(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{
				Level:  "info",
				Format: "text",
			},
		},
	}

	logger := setupLogger(cfg)

	require.NotNil(t, logger)
	// Logger should be created successfully with text format
	assert.NotNil(t, logger)
}

// =============================================================================
// initializeXDSClient Tests (with valid config)
// =============================================================================

func TestInitializeXDSClient_InvalidConfig(t *testing.T) {
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	// Create config with missing required fields
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			XDS: config.XDSConfig{
				ConnectTimeout:        5 * time.Second,
				RequestTimeout:        5 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     30 * time.Second,
			},
		},
	}

	_, err := initializeXDSClient(context.Background(), cfg, "", k, reg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create xDS client")
}

func TestInitializeXDSClient_ValidConfig(t *testing.T) {
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			XDS: config.XDSConfig{
				ConnectTimeout:        1 * time.Second,
				RequestTimeout:        1 * time.Second,
				InitialReconnectDelay: 1 * time.Second,
				MaxReconnectDelay:     5 * time.Second,
				TLS: config.XDSTLSConfig{
					Enabled: false,
				},
			},
		},
	}

	// Note: This will fail to actually connect since there's no server,
	// but the client creation and start attempt should work
	client, err := initializeXDSClient(context.Background(), cfg, "localhost:18000", k, reg)

	// Client should be created successfully even if it can't connect
	require.NoError(t, err)
	require.NotNil(t, client)

	// Note: Not calling Stop/Wait due to potential issues with context in test environment
	// The client will be cleaned up when the test exits
}
