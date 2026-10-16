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
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/metrics"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/xdsclient"
)

func healthTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.PolicyEngine.Logging = config.LoggingConfig{Level: "info", Format: "text"}
	cfg.PolicyEngine.Health = config.HealthConfig{
		CheckTimeout:   time.Second,
		CacheTTL:       time.Second,
		LogFile:        filepath.Join(t.TempDir(), "logs", "health.log"),
		LogMaxSizeMB:   1,
		LogMaxFiles:    1,
		BackoffInitial: 10 * time.Second,
		BackoffMax:     time.Minute,
		XDSGrace:       5 * time.Minute,
	}
	return cfg
}

func TestBuildHealthCheckers(t *testing.T) {
	cfg := healthTestConfig(t)

	liveness, readiness, closer := buildHealthCheckers(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
	defer closer.Close()

	live := liveness.Evaluate()
	assert.True(t, live.Healthy)
	assert.Equal(t, "healthy", live.Status)
	assert.Equal(t, runtimeComponent, live.Component)
	assert.Empty(t, live.Checks, "liveness has no checks of its own")

	ready := readiness.Evaluate()
	assert.Equal(t, "ready", ready.Status)
	assert.Equal(t, runtimeComponent, ready.Component)
	assert.NotContains(t, ready.Checks, "xds_connection", "no configuration link in file mode")
	assert.NotContains(t, ready.Checks, "python_executor", "no Python executor configured")

	body, err := health.RenderBody(live)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"checks":{}`)

	_, err = os.Stat(cfg.PolicyEngine.Health.LogFile)
	assert.NoError(t, err, "the health log file is created when configured")
}

func TestBuildHealthCheckersWithConfigurationLink(t *testing.T) {
	metrics.Init()
	cfg := healthTestConfig(t)
	client, err := xdsclient.NewClient(xdsclient.NewDefaultConfig("localhost:18000"), kernel.NewKernel(),
		registry.GetRegistry(), resolver.DefaultRegistry())
	require.NoError(t, err)

	liveness, readiness, closer := buildHealthCheckers(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), client, nil)
	defer closer.Close()

	ready := readiness.Evaluate()
	require.Contains(t, ready.Checks, "xds_connection")
	assert.Equal(t, health.StatusDown, ready.Checks["xds_connection"].Status)
	assert.Equal(t, "no configuration received yet", ready.Checks["xds_connection"].Error)
	assert.NotContains(t, liveness.Evaluate().Checks, "xds_connection", "liveness never depends on the controller")
}

func TestBuildHealthCheckersWithPythonExecutor(t *testing.T) {
	cfg := healthTestConfig(t)
	executorDown := func(context.Context) error { return health.Reason("python executor unreachable") }

	liveness, readiness, closer := buildHealthCheckers(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, executorDown)
	defer closer.Close()

	ready := readiness.Evaluate()
	require.Contains(t, ready.Checks, "python_executor")
	assert.Equal(t, "python executor unreachable", ready.Checks["python_executor"].Error)
	assert.True(t, liveness.Evaluate().Healthy, "liveness never depends on the Python executor")
}
