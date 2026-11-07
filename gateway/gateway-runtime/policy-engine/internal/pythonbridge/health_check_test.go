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

package pythonbridge

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
)

type fakeExecutorHealth func(ctx context.Context) (*proto.HealthCheckResponse, error)

func (f fakeExecutorHealth) HealthCheck(ctx context.Context) (*proto.HealthCheckResponse, error) {
	return f(ctx)
}

func evaluatePythonCheck(t *testing.T, client executorHealthClient) health.CheckResult {
	t.Helper()
	c := health.NewChecker(health.Options{
		HealthyStatus: "ready",
		CacheTTL:      time.Nanosecond,
	})
	c.Register("python_executor", newHealthCheck(client), health.WithTimeout(200*time.Millisecond))
	return c.Evaluate().Checks["python_executor"]
}

func TestPythonExecutorHealthCheck(t *testing.T) {
	tests := []struct {
		name       string
		client     fakeExecutorHealth
		wantStatus health.CheckStatus
		wantError  string
	}{
		{
			name: "ready",
			client: func(context.Context) (*proto.HealthCheckResponse, error) {
				return &proto.HealthCheckResponse{Ready: true}, nil
			},
			wantStatus: health.StatusUp,
		},
		{
			name: "not ready",
			client: func(context.Context) (*proto.HealthCheckResponse, error) {
				return &proto.HealthCheckResponse{Ready: false}, nil
			},
			wantStatus: health.StatusDown,
			wantError:  "python executor not ready",
		},
		{
			name: "unreachable, without exposing the transport error",
			client: func(context.Context) (*proto.HealthCheckResponse, error) {
				return nil, errors.New("connect Python Executor: dial unix /var/run/python.sock: connection refused")
			},
			wantStatus: health.StatusDown,
			wantError:  "python executor unreachable",
		},
		{
			name: "hung",
			client: func(ctx context.Context) (*proto.HealthCheckResponse, error) {
				<-ctx.Done()
				return nil, errors.New("rpc error: context deadline exceeded")
			},
			wantStatus: health.StatusDown,
			wantError:  "timed out after 200ms",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluatePythonCheck(t, tt.client)

			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantError, got.Error)
		})
	}
}

func TestNewHealthCheckUsesTheStreamManager(t *testing.T) {
	// An executor address nothing listens on: the stream manager cannot connect.
	sm := NewStreamManager("/nonexistent/python-executor.sock", false)
	got := evaluatePythonCheck(t, sm)

	assert.Equal(t, health.StatusDown, got.Status)
	assert.Contains(t, []string{"python executor unreachable", "timed out after 200ms"}, got.Error)
}
