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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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

// fakeHealthProvider reports a fixed policy engine state.
type fakeHealthProvider bool

func (f fakeHealthProvider) IsHealthy() bool { return bool(f) }

func healthTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.PolicyEngine.Logging = config.LoggingConfig{Level: "info", Format: "text"}
	cfg.PolicyEngine.Health = config.HealthConfig{
		CacheTTL:       time.Second,
		XDSConnection:  config.HealthXDSConnectionConfig{Grace: 5 * time.Minute},
		PythonExecutor: config.HealthPythonExecutorConfig{CheckTimeout: time.Second},
	}
	return cfg
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type servedHealth struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"checks"`
}

// serveHealth answers one GET from the checker and decodes the body.
func serveHealth(t *testing.T, c *health.Checker) (int, servedHealth, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	c.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var body servedHealth
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	return rec.Code, body, raw
}

func TestBuildHealthCheckersPolicyEngineHealthy(t *testing.T) {
	liveness, readiness := buildHealthCheckers(healthTestConfig(), discardLogger(), fakeHealthProvider(true), nil, nil)

	code, live, raw := serveHealth(t, liveness)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "healthy", live.Status)
	assert.Len(t, live.Checks, 1)
	assert.Equal(t, "up", live.Checks["policy_engine"].Status)
	assert.NotContains(t, raw, "component")

	code, ready, _ := serveHealth(t, readiness)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ready", ready.Status)
	assert.Equal(t, "up", ready.Checks["policy_engine"].Status)
	assert.NotContains(t, ready.Checks, "xds_connection", "no configuration link in file mode")
	assert.NotContains(t, ready.Checks, "python_executor", "no Python executor configured")
}

func TestBuildHealthCheckersPolicyEngineNotLoaded(t *testing.T) {
	liveness, readiness := buildHealthCheckers(healthTestConfig(), discardLogger(), fakeHealthProvider(false), nil, nil)

	for name, c := range map[string]*health.Checker{"liveness": liveness, "readiness": readiness} {
		code, body, _ := serveHealth(t, c)
		assert.Equal(t, http.StatusServiceUnavailable, code, name)
		assert.Equal(t, health.UnhealthyStatus, body.Status, name)
		assert.Equal(t, "down", body.Checks["policy_engine"].Status, name)
		assert.Equal(t, "policy configuration not loaded", body.Checks["policy_engine"].Error, name)
	}
}

func TestBuildHealthCheckersWithConfigurationLink(t *testing.T) {
	metrics.Init()
	client, err := xdsclient.NewClient(xdsclient.NewDefaultConfig("localhost:18000"), kernel.NewKernel(),
		registry.GetRegistry(), resolver.DefaultRegistry())
	require.NoError(t, err)

	liveness, readiness := buildHealthCheckers(healthTestConfig(), discardLogger(), client, client, nil)

	ready := readiness.Evaluate()
	require.Contains(t, ready.Checks, "xds_connection")
	assert.Equal(t, health.StatusDown, ready.Checks["xds_connection"].Status)
	assert.Equal(t, "no configuration received yet", ready.Checks["xds_connection"].Error)
	assert.Equal(t, "policy configuration not loaded", ready.Checks["policy_engine"].Error)

	live := liveness.Evaluate()
	assert.NotContains(t, live.Checks, "xds_connection", "liveness never depends on the controller")
	assert.Equal(t, "policy configuration not loaded", live.Checks["policy_engine"].Error)
}

func TestBuildHealthCheckersWithPythonExecutor(t *testing.T) {
	executorDown := func(context.Context) error { return health.Reason("python executor unreachable") }

	liveness, readiness := buildHealthCheckers(healthTestConfig(), discardLogger(), fakeHealthProvider(true), nil, executorDown)

	ready := readiness.Evaluate()
	require.Contains(t, ready.Checks, "python_executor")
	assert.Equal(t, "python executor unreachable", ready.Checks["python_executor"].Error)
	assert.Equal(t, health.StatusUp, ready.Checks["policy_engine"].Status)
	assert.True(t, liveness.Evaluate().Healthy, "liveness never depends on the Python executor")
}

func TestBuildHealthCheckersPythonExecutorUsesItsOwnTimeout(t *testing.T) {
	cfg := healthTestConfig()
	cfg.PolicyEngine.Health.PythonExecutor.CheckTimeout = 100 * time.Millisecond
	executorHangs := func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}

	_, readiness := buildHealthCheckers(cfg, discardLogger(), fakeHealthProvider(true), nil, executorHangs)

	start := time.Now()
	ready := readiness.Evaluate()
	elapsed := time.Since(start)

	assert.Equal(t, "timed out after 100ms", ready.Checks["python_executor"].Error)
	assert.Equal(t, health.StatusUp, ready.Checks["policy_engine"].Status)
	assert.Less(t, elapsed, time.Second, "the python_executor check is bounded by its configured timeout")
}

// switchableHealthProvider lets a test change the policy engine state between evaluations.
type switchableHealthProvider struct{ healthy bool }

func (s *switchableHealthProvider) IsHealthy() bool { return s.healthy }

func TestBuildHealthCheckersLogsPolicyEngineOutageOnce(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	cfg := healthTestConfig()
	cfg.PolicyEngine.Health.CacheTTL = time.Nanosecond
	provider := &switchableHealthProvider{}
	liveness, readiness := buildHealthCheckers(cfg, logger, provider, nil, nil)

	for i := 0; i < 3; i++ {
		serveHealth(t, liveness)
		serveHealth(t, readiness)
		time.Sleep(time.Millisecond)
	}
	provider.healthy = true
	serveHealth(t, liveness)
	serveHealth(t, readiness)

	logged := out.String()
	assert.Equal(t, 1, strings.Count(logged, `msg="health check failed" check=policy_engine`), logged)
	assert.Equal(t, 1, strings.Count(logged, `msg="health check recovered" check=policy_engine`), logged)
}
