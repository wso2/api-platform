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

package admin

import (
	"encoding/json"
	"net/http"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// ConfigDumpHandler handles GET /config_dump requests
type ConfigDumpHandler struct {
	kernel   *kernel.Kernel
	registry *registry.PolicyRegistry
}

// NewConfigDumpHandler creates a new config dump handler
func NewConfigDumpHandler(k *kernel.Kernel, reg *registry.PolicyRegistry) *ConfigDumpHandler {
	return &ConfigDumpHandler{
		kernel:   k,
		registry: reg,
	}
}

// ServeHTTP implements http.Handler
func (h *ConfigDumpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Dump the configuration
	dump := DumpConfig(h.kernel, h.registry)

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Encode and send response
	if err := json.NewEncoder(w).Encode(dump); err != nil {
		// If we already sent headers, we can't send an error response
		// Just log the error (logger not available here, so silent failure)
		return
	}
}
