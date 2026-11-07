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

package xdsclient

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
)

func markConfigReceived(c *Client) {
	c.mu.Lock()
	c.configReceived = true
	c.mu.Unlock()
}

func TestConnectionCheck(t *testing.T) {
	recordConnectionMetrics(t)
	const grace = 5 * time.Minute
	client, clock := newTestClientWithClock(t)
	check := client.ConnectionCheck(grace)
	start := clock.Now()

	err := check(context.Background())
	var reason health.Reason
	require.ErrorAs(t, err, &reason)
	assert.Equal(t, "no configuration received yet", string(reason), "never received configuration")

	markConfigReceived(client)
	client.setState(StateConnected)
	assert.NoError(t, check(context.Background()), "connected")

	lost := start.Add(time.Hour)
	clock.set(lost)
	client.setState(StateReconnecting)
	clock.set(lost.Add(grace - time.Second))
	assert.NoError(t, check(context.Background()), "down for less than the grace window")

	clock.set(lost.Add(grace + 70*time.Second))
	err = check(context.Background())
	require.ErrorAs(t, err, &reason)
	assert.Equal(t, "disconnected from the controller for 6m10s", string(reason))

	client.setState(StateConnecting)
	client.setState(StateConnected)
	assert.NoError(t, check(context.Background()), "reconnected")
}

func TestConnectionCheckThroughHealthChecker(t *testing.T) {
	recordConnectionMetrics(t)
	const grace = 5 * time.Minute
	client, clock := newTestClientWithClock(t)
	markConfigReceived(client)
	client.setState(StateConnected)

	var logs strings.Builder
	checker := health.NewChecker(health.Options{
		HealthyStatus: "ready",
		CacheTTL:      time.Nanosecond,
		Logger:        slog.New(slog.NewTextHandler(&logs, nil)),
	})
	checker.Register("xds_connection", client.ConnectionCheck(grace))
	require.True(t, checker.Evaluate().Healthy)

	lost := clock.Now().Add(time.Hour)
	clock.set(lost)
	client.setState(StateReconnecting)
	clock.set(lost.Add(grace + 70*time.Second))

	res := checker.Evaluate()
	require.False(t, res.Healthy)
	xds := res.Checks["xds_connection"]
	assert.Equal(t, health.StatusDown, xds.Status)
	assert.Equal(t, "disconnected from the controller for 6m10s", xds.Error)
	assert.Equal(t, lost.Add(grace), xds.Since, "the failure is dated from when the grace expired")

	client.setState(StateConnecting)
	client.setState(StateConnected)
	assert.True(t, checker.Evaluate().Healthy)

	out := logs.String()
	assert.Equal(t, 1, strings.Count(out, `msg="health check failed" check=xds_connection`), out)
	assert.Equal(t, 1, strings.Count(out, `msg="health check recovered" check=xds_connection`), out)
}
