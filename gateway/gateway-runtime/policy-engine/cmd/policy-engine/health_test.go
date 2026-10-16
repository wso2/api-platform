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

	liveness, readiness, closer := buildHealthCheckers(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer closer.Close()

	live := liveness.Evaluate()
	assert.True(t, live.Healthy)
	assert.Equal(t, "healthy", live.Status)
	assert.Equal(t, runtimeComponent, live.Component)
	assert.Empty(t, live.Checks, "liveness has no checks of its own")

	ready := readiness.Evaluate()
	assert.Equal(t, "ready", ready.Status)
	assert.Equal(t, runtimeComponent, ready.Component)

	body, err := health.RenderBody(live)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"checks":{}`)

	_, err = os.Stat(cfg.PolicyEngine.Health.LogFile)
	assert.NoError(t, err, "the health log file is created when configured")
}
