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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
)

// pingFunc adapts a function to the pinger interface.
type pingFunc func(ctx context.Context) error

func (f pingFunc) PingContext(ctx context.Context) error { return f(ctx) }

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestChecker returns a controller checker with only the database check, a short
// timeout and no caching, so every evaluation pings.
func newTestChecker(db pinger) *health.Checker {
	c := health.NewChecker(health.Options{
		Component:     "gateway-controller",
		HealthyStatus: "healthy",
		CheckTimeout:  200 * time.Millisecond,
		CacheTTL:      time.Nanosecond,
	})
	c.Register("database", newDatabaseCheck(db))
	return c
}

type renderedCheck struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Since  string `json:"since"`
}

type renderedBody struct {
	Status    string                   `json:"status"`
	Component string                   `json:"component"`
	Checks    map[string]renderedCheck `json:"checks"`
}

func evaluate(t *testing.T, c *health.Checker) (renderedBody, string) {
	t.Helper()
	raw, err := health.RenderBody(c.Evaluate())
	require.NoError(t, err)
	var b renderedBody
	require.NoError(t, json.Unmarshal(raw, &b))
	return b, string(raw)
}

func TestDatabaseCheckUp(t *testing.T) {
	b, _ := evaluate(t, newTestChecker(pingFunc(func(context.Context) error { return nil })))

	assert.Equal(t, "healthy", b.Status)
	assert.Equal(t, "gateway-controller", b.Component)
	assert.Equal(t, "up", b.Checks["database"].Status)
}

func TestDatabaseCheckUnreachableHidesDriverError(t *testing.T) {
	driverErr := errors.New("failed to connect to host=gw-db.internal user=gateway password=s3cret: connection refused")

	b, raw := evaluate(t, newTestChecker(pingFunc(func(context.Context) error { return driverErr })))

	assert.Equal(t, "unhealthy", b.Status)
	db := b.Checks["database"]
	assert.Equal(t, "down", db.Status)
	assert.Equal(t, "database unreachable", db.Error)
	assert.NotEmpty(t, db.Since)
	for _, leak := range []string{"gw-db.internal", "gateway password", "s3cret", "connection refused"} {
		assert.NotContains(t, raw, leak)
	}
}

func TestDatabaseCheckKeepsDriverErrorAsCause(t *testing.T) {
	driverErr := errors.New("connection refused")

	err := newDatabaseCheck(pingFunc(func(context.Context) error { return driverErr }))(context.Background())

	assert.ErrorIs(t, err, errDatabaseUnreachable)
	assert.ErrorIs(t, err, driverErr)
}

func TestDatabaseCheckTimeout(t *testing.T) {
	b, _ := evaluate(t, newTestChecker(pingFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return errors.New("driver: canceling query due to user request")
	})))

	assert.Equal(t, renderedCheck{Status: "down", Error: "timed out after 200ms", Since: b.Checks["database"].Since},
		b.Checks["database"])
}

func TestNewHealthChecker(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "logs", "health.log")
	cfg := &config.Config{}
	cfg.Controller.Logging = config.LoggingConfig{Level: "info", Format: "json"}
	cfg.Controller.Health = config.HealthConfig{
		CheckTimeout:   time.Second,
		CacheTTL:       time.Second,
		LogFile:        logFile,
		LogMaxSizeMB:   1,
		LogMaxFiles:    1,
		BackoffInitial: 10 * time.Second,
		BackoffMax:     time.Minute,
	}

	checker, closer := newHealthChecker(cfg, discardLogger())
	defer closer.Close()
	checker.Register("database", newDatabaseCheck(pingFunc(func(context.Context) error {
		return errors.New("connection refused")
	})))

	b, _ := evaluate(t, checker)
	assert.Equal(t, "gateway-controller", b.Component)
	assert.Equal(t, "unhealthy", b.Status)
	require.NoError(t, closer.Close())
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(content), "health check failed"),
		"the failure must be written to the health log file: %q", content)
}

func TestDatabaseCheckDoesNotStackPingsOnHungDatabase(t *testing.T) {
	release := make(chan struct{})
	var pings atomic.Int32
	// A driver that ignores its context: the ping blocks until released.
	check := newDatabaseCheck(pingFunc(func(context.Context) error {
		pings.Add(1)
		<-release
		return nil
	}))
	call := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		return check(ctx)
	}

	assert.ErrorIs(t, call(), context.DeadlineExceeded, "first call")
	assert.ErrorIs(t, call(), context.DeadlineExceeded, "second call while the first ping hangs")
	assert.Equal(t, int32(1), pings.Load(), "no second ping while the first is still running")

	close(release)
	require.Eventually(t, func() bool { return call() == nil }, 2*time.Second, 10*time.Millisecond,
		"the check must recover once the hung ping returns")
	assert.Equal(t, int32(2), pings.Load(), "exactly one new ping after the hung one returned")
}

func TestDatabaseCheckReportsHungPingAsTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	c := newTestChecker(pingFunc(func(context.Context) error {
		<-release
		return nil
	}))

	for i := 0; i < 3; i++ {
		b, _ := evaluate(t, c)
		assert.Equal(t, "down", b.Checks["database"].Status, "evaluation %d", i)
		assert.Equal(t, "timed out after 200ms", b.Checks["database"].Error, "evaluation %d", i)
	}
}
