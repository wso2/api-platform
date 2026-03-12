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
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/kernel"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
)

// XDSSyncStatusProvider exposes the latest ACKed policy chain version.
type XDSSyncStatusProvider interface {
	GetPolicyChainVersion() string
}

// HealthProvider reports whether the policy engine is ready to process traffic.
type HealthProvider interface {
	IsHealthy() bool
}

// PythonHealthChecker checks the health of the Python executor via gRPC.
type PythonHealthChecker interface {
	IsPythonHealthy() (ready bool, loadedPolicies int32, err error)
}

// ConfigDumpHandler handles GET /config_dump requests
type ConfigDumpHandler struct {
	kernel   *kernel.Kernel
	registry *registry.PolicyRegistry
	xds      XDSSyncStatusProvider
}

// NewConfigDumpHandler creates a new config dump handler
func NewConfigDumpHandler(k *kernel.Kernel, reg *registry.PolicyRegistry, xds XDSSyncStatusProvider) *ConfigDumpHandler {
	return &ConfigDumpHandler{
		kernel:   k,
		registry: reg,
		xds:      xds,
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
	dump := DumpConfig(h.kernel, h.registry, h.getPolicyChainVersion())

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

func (h *ConfigDumpHandler) getPolicyChainVersion() string {
	if h.xds == nil {
		return ""
	}
	return h.xds.GetPolicyChainVersion()
}

// XDSSyncStatusHandler handles GET /xds_sync_status requests.
type XDSSyncStatusHandler struct {
	xds XDSSyncStatusProvider
}

// NewXDSSyncStatusHandler creates a new xDS sync status handler.
func NewXDSSyncStatusHandler(xds XDSSyncStatusProvider) *XDSSyncStatusHandler {
	return &XDSSyncStatusHandler{xds: xds}
}

// ServeHTTP implements http.Handler for xDS sync status.
func (h *XDSSyncStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	policyChainVersion := ""
	if h.xds != nil {
		policyChainVersion = h.xds.GetPolicyChainVersion()
	}

	resp := XDSSyncStatusResponse{
		Component:          "policy-engine",
		Timestamp:          time.Now(),
		PolicyChainVersion: policyChainVersion,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// HealthHandler handles GET /health requests.
type HealthHandler struct {
	health       HealthProvider
	pythonHealth PythonHealthChecker
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(health HealthProvider, pythonHealth PythonHealthChecker) *HealthHandler {
	return &HealthHandler{health: health, pythonHealth: pythonHealth}
}

// ServeHTTP implements http.Handler for health checks.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peStatus := "healthy"
	statusCode := http.StatusOK
	if h.health != nil && !h.health.IsHealthy() {
		peStatus = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	resp := HealthResponse{
		Status:    peStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Check Python executor health if checker is configured
	if h.pythonHealth != nil {
		ready, loadedPolicies, err := h.pythonHealth.IsPythonHealthy()
		if err != nil || !ready {
			resp.PythonExecutor = &PythonExecutorHealth{
				Status:         "unhealthy",
				LoadedPolicies: loadedPolicies,
			}
			statusCode = http.StatusServiceUnavailable
			resp.Status = "unhealthy"
		} else {
			resp.PythonExecutor = &PythonExecutorHealth{
				Status:         "healthy",
				LoadedPolicies: loadedPolicies,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}
