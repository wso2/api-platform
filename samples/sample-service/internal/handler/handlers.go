// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/wso2/api-platform/samples/sample-service/internal/types"
)

// Handler holds configuration for HTTP handlers.
type Handler struct {
	Pretty      bool
	lastRequest *types.RequestInfo
	mu          sync.RWMutex
}

// RegisterRoutes registers all routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /sandbox/whoami", h.SandboxWhoAmI)
	mux.HandleFunc("GET /captured-request", h.GetCapturedRequest)
	mux.HandleFunc("/", h.Request)
}

func (h *Handler) writeJSON(w http.ResponseWriter, v any) {
	enc := json.NewEncoder(w)
	if h.Pretty {
		enc.SetIndent("", "  ")
	}
	enc.Encode(v)
}
