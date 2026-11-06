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
	"fmt"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/pythonbridge/proto"
)

var (
	// errExecutorUnreachable is reported when the Python executor cannot be asked for
	// its health. The underlying error stays the wrapped cause, for debug logs only.
	errExecutorUnreachable = health.Reason("python executor unreachable")
	// errExecutorNotReady is reported when the Python executor answers but is not ready.
	errExecutorNotReady = health.Reason("python executor not ready")
)

// executorHealthClient is the part of the stream manager the health check needs.
type executorHealthClient interface {
	HealthCheck(ctx context.Context) (*proto.HealthCheckResponse, error)
}

// NewHealthCheck returns the "python_executor" health check, which asks the Python
// executor whether it is ready. It is bounded by the health checker's deadline, so a
// hung executor reports a timeout instead of holding the health answer.
func NewHealthCheck(sm *StreamManager) health.CheckFunc {
	return newHealthCheck(sm)
}

func newHealthCheck(client executorHealthClient) health.CheckFunc {
	return func(ctx context.Context) error {
		resp, err := client.HealthCheck(ctx)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("%w: %w", errExecutorUnreachable, err)
		}
		if !resp.GetReady() {
			return errExecutorNotReady
		}
		return nil
	}
}
