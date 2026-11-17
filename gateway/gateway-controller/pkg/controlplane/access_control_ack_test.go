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

package controlplane

import (
	"errors"
	"fmt"
	"testing"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/templateengine"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/upstreamguard"
)

func TestDeployFailureCode(t *testing.T) {
	refused := fmt.Errorf("failed to deploy LLM provider configuration from YAML: %w",
		&templateengine.RenderError{Cause: &upstreamguard.RefusedError{FieldPath: "spec.upstream.url"}})

	if got := deployFailureCode(refused); got != ackCodeUpstreamURLNotAllowed {
		t.Fatalf("deployFailureCode(refused) = %q, want %q", got, ackCodeUpstreamURLNotAllowed)
	}
	if got := deployFailureCode(errors.New("database is locked")); got != ackCodeGatewayProcessingError {
		t.Fatalf("deployFailureCode(other) = %q, want %q", got, ackCodeGatewayProcessingError)
	}
	if got := agentAckFailureCode(refused); got != ackCodeUpstreamURLNotAllowed {
		t.Fatalf("agentAckFailureCode(refused) = %q, want %q", got, ackCodeUpstreamURLNotAllowed)
	}
	// The control plane stores the code in a VARCHAR(50) status reason.
	if len(ackCodeUpstreamURLNotAllowed) > 50 {
		t.Fatalf("ack code %q is longer than 50 characters", ackCodeUpstreamURLNotAllowed)
	}
}
