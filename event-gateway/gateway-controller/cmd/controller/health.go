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
	"encoding/json"
	"net/http"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
)

// forbiddenBody is the answer to a client outside controller.health.allowed_clients, the
// same one the gateway controller gives.
var forbiddenBody = []byte(`{"status":"forbidden"}`)

// newFixedHealthHandler answers the admin server's health probe with a fixed healthy
// response: the event gateway controller does not check its dependencies. It still
// honours controller.health.allowed_clients, judged by the connection's peer address:
// any other client gets 403. A nil allowed answers every client.
func newFixedHealthHandler(allowed *health.AllowedClients) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed.Allows(r.RemoteAddr) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write(forbiddenBody)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})
}
