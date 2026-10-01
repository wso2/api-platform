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

package platformgateway

import (
	"context"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// keyGatewayChangePending is set while something changed since the gateway was last settled:
// an API, a certificate, a relay entry or a service. An HTTPS request in an @mtls scenario
// waits for the gateway to apply what the controller holds only while it is set.
const keyGatewayChangePending = "mtlsGatewayChangePending"

// markGatewayChanged records that the gateway may not yet hold what the controller holds.
func markGatewayChanged(ctx context.Context) {
	_ = tcontext.Set(ctx, keyGatewayChangePending, true)
}

// clearGatewayChange records that the gateway held what the controller holds.
func clearGatewayChange(ctx context.Context) {
	_ = tcontext.Set(ctx, keyGatewayChangePending, false)
}

// gatewayChangePending reports whether something changed since the last settle.
func gatewayChangePending(ctx context.Context) bool {
	v, ok := tcontext.Get(ctx, keyGatewayChangePending)
	pending, _ := v.(bool)
	return ok && pending
}

// settleAfterChange waits for the gateway to apply what the controller holds, but only when
// something changed since the last settle.
func (g *Gateway) settleAfterChange(ctx context.Context) error {
	if !gatewayChangePending(ctx) {
		return nil
	}
	return g.awaitGatewayApplied(ctx)
}
