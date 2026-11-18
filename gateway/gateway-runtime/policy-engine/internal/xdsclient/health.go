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
	"fmt"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/health"
)

// errNoConfigurationYet is reported until the first configuration has been accepted.
var errNoConfigurationYet = health.Reason("no configuration received yet")

// ConnectionCheck returns the "xds_connection" health check, which belongs to
// readiness, not liveness. It fails until configuration has been received, and once
// the connection to the controller has been down for longer than grace. A shorter
// outage, such as a routine controller restart, does not fail it: the last
// configuration keeps being served meanwhile. When it fails after an outage, the
// failure is dated from the moment the grace expired.
func (c *Client) ConnectionCheck(grace time.Duration) health.CheckFunc {
	return func(context.Context) error {
		status := c.ConnectionStatus()
		if !status.ConfigReceived {
			return errNoConfigurationYet
		}
		if status.Connected {
			return nil
		}
		down := c.clock()().Sub(status.DisconnectedSince)
		if down <= grace {
			return nil
		}
		return health.FailingSince(
			health.Reason(fmt.Sprintf("disconnected from the controller for %s", down.Round(time.Second))),
			status.DisconnectedSince.Add(grace))
	}
}

// clock returns the clock used for connection state.
func (c *Client) clock() func() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}
