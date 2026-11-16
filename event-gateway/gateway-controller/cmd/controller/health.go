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
	"net/http"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
)

// newHealthHandler answers the admin server's health probes. The event gateway
// controller does not check its dependencies, so it uses the shared health handler with
// no checks registered: every evaluation is healthy. The shared handler gives the same
// body, headers, method handling and controller.health.allowed_clients enforcement as
// the gateway controller. A nil allowed answers every client.
func newHealthHandler(allowed *health.AllowedClients) http.Handler {
	return health.NewChecker(health.Options{
		HealthyStatus:  "healthy",
		AllowedClients: allowed,
	}).Handler()
}
