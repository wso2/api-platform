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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// =============================================================================
// applyFlagOverrides Tests
// =============================================================================

func TestApplyFlagOverrides_PolicyChainsFile(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			ConfigMode: config.ConfigModeConfig{
				Mode: "xds",
			},
		},
	}

	// Set flag value
	testFile := "/path/to/chains.yaml"
	oldPolicyChainsFile := *policyChainsFile
	*policyChainsFile = testFile
	defer func() { *policyChainsFile = oldPolicyChainsFile }()

	applyFlagOverrides(cfg)

	assert.Equal(t, "file", cfg.PolicyEngine.ConfigMode.Mode)
	assert.Equal(t, testFile, cfg.PolicyEngine.FileConfig.Path)
}

func TestApplyFlagOverrides_NoFlags(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			ConfigMode: config.ConfigModeConfig{
				Mode: "xds",
			},
		},
	}

	// Clear all flags
	oldPolicyChainsFile := *policyChainsFile
	oldXdsServerAddr := *xdsServerAddr
	*policyChainsFile = ""
	*xdsServerAddr = ""
	defer func() {
		*policyChainsFile = oldPolicyChainsFile
		*xdsServerAddr = oldXdsServerAddr
	}()

	applyFlagOverrides(cfg)

	// Config should remain unchanged
	assert.Equal(t, "xds", cfg.PolicyEngine.ConfigMode.Mode)
}

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
// initializeFileConfig Tests
// =============================================================================

func TestInitializeFileConfig_EmptyFile(t *testing.T) {
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	// Create temp empty config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "chains.yaml")
	yamlContent := `[]`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			FileConfig: config.FileConfigConfig{
				Path: configPath,
			},
		},
	}

	err = initializeFileConfig(context.Background(), cfg, k, reg)

	// Empty file should load successfully
	assert.NoError(t, err)
}

func TestInitializeFileConfig_FileNotFound(t *testing.T) {
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			FileConfig: config.FileConfigConfig{
				Path: "/nonexistent/path/chains.yaml",
			},
		},
	}

	err := initializeFileConfig(context.Background(), cfg, k, reg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load configuration from file")
}

func TestInitializeFileConfig_InvalidYAML(t *testing.T) {
	k := kernel.NewKernel()
	reg := registry.GetRegistry()

	// Create temp config file with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(configPath, []byte("invalid: yaml: ["), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			FileConfig: config.FileConfigConfig{
				Path: configPath,
			},
		},
	}

	err = initializeFileConfig(context.Background(), cfg, k, reg)

	assert.Error(t, err)
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

// =============================================================================
// componentPrefixWriter Tests
// =============================================================================

func TestComponentPrefixWriter_PrefixesEachWrite(t *testing.T) {
	var buf bytes.Buffer
	w := newComponentPrefixWriter(&buf, "[pol] ")

	n, err := w.Write([]byte("time=... level=INFO msg=hello\n"))

	require.NoError(t, err)
	// n must equal len(p); io.Writer callers treat any other count as a failed
	// write.
	assert.Equal(t, len("time=... level=INFO msg=hello\n"), n)
	assert.Equal(t, "[pol] time=... level=INFO msg=hello\n", buf.String())
}

func TestComponentPrefixWriter_OneUnderlyingWritePerRecord(t *testing.T) {
	counting := &countingWriter{}
	w := newComponentPrefixWriter(counting, "[pol] ")

	_, err := w.Write([]byte("first\n"))
	require.NoError(t, err)
	_, err = w.Write([]byte("second\n"))
	require.NoError(t, err)

	// One underlying write per record, so another writer on the same descriptor
	// cannot interleave between tag and line.
	assert.Equal(t, 2, counting.writes)
	assert.Equal(t, "[pol] first\n[pol] second\n", counting.buf.String())
}

func TestComponentPrefixWriter_PropagatesError(t *testing.T) {
	w := newComponentPrefixWriter(failingWriter{}, "[pol] ")

	n, err := w.Write([]byte("boom\n"))

	require.Error(t, err)
	assert.Zero(t, n)
}

func TestSetupLogger_TextFormatIsComponentTagged(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{Level: "info", Format: "text"},
		},
	}

	out := captureStdout(t, func() {
		setupLogger(cfg).Info("hello")
	})

	assert.True(t, strings.HasPrefix(out, "[pol] "), "text log line must be [pol]-tagged, got: %q", out)
	assert.Contains(t, out, "msg=hello")
}

func TestSetupLogger_JSONFormatCarriesComponentField(t *testing.T) {
	cfg := &config.Config{
		PolicyEngine: config.PolicyEngine{
			Logging: config.LoggingConfig{Level: "info", Format: "json"},
		},
	}

	out := captureStdout(t, func() {
		setupLogger(cfg).Info("hello")
	})

	assert.False(t, strings.HasPrefix(out, "[pol] "), "JSON log line must not be text-prefixed, got: %q", out)

	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &rec), "JSON log line must parse, got: %q", out)
	assert.Equal(t, "pol", rec["component"])
	assert.Equal(t, "hello", rec["msg"])
}

type countingWriter struct {
	buf    bytes.Buffer
	writes int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	c.writes++
	return c.buf.Write(p)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// captureStdout redirects os.Stdout for the duration of fn. setupLogger binds to
// os.Stdout at construction, so the swap must precede the call.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())

	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}
