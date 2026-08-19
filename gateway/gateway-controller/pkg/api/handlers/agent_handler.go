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

package handlers

import (
	"net/http"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/go-httpkit/httputil"
)

// Agent (A2A) management endpoints.
//
// The kind, its management-API contract, and its persistence land before the
// service layer that backs them, so every endpoint answers 501 until the agent
// service is wired in. Answering 501 rather than omitting the routes keeps the
// generated ServerInterface satisfied and keeps the endpoints inert: they are
// still gated by the same role map as every other management route, so an
// unauthorised caller gets 403 here exactly as it will once these are live.

// writeAgentNotImplemented renders the single sterile response every Agent
// endpoint returns until the agent service exists.
func writeAgentNotImplemented(w http.ResponseWriter) {
	httputil.WriteJSON(w, http.StatusNotImplemented, api.ErrorResponse{
		Status:  "error",
		Message: "Agent management is not available in this build",
	})
}

// CreateAgent implements ServerInterface.CreateAgent
// (POST /agents)
func (s *APIServer) CreateAgent(w http.ResponseWriter, r *http.Request) {
	writeAgentNotImplemented(w)
}

// ListAgents implements ServerInterface.ListAgents
// (GET /agents)
func (s *APIServer) ListAgents(w http.ResponseWriter, r *http.Request, params api.ListAgentsParams) {
	writeAgentNotImplemented(w)
}

// GetAgentById implements ServerInterface.GetAgentById
// (GET /agents/{id})
func (s *APIServer) GetAgentById(w http.ResponseWriter, r *http.Request, id string) {
	writeAgentNotImplemented(w)
}

// UpdateAgent implements ServerInterface.UpdateAgent
// (PUT /agents/{id})
func (s *APIServer) UpdateAgent(w http.ResponseWriter, r *http.Request, id string) {
	writeAgentNotImplemented(w)
}

// DeleteAgent implements ServerInterface.DeleteAgent
// (DELETE /agents/{id})
func (s *APIServer) DeleteAgent(w http.ResponseWriter, r *http.Request, id string) {
	writeAgentNotImplemented(w)
}
