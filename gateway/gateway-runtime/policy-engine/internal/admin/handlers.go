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

	"github.com/wso2/api-platform/common/redact"
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

	// Obtain a consistent snapshot of routes and sensitive values in one lock acquisition so
	// the redaction list always matches the routes shown in the dump (same xDS generation).
	routes, rawSecrets := h.kernel.DumpRoutesAndSensitiveValues()

	dump := DumpConfig(routes, h.kernel, h.registry, h.getPolicyChainVersion())

	jsonBytes, err := json.Marshal(dump)
	if err != nil {
		http.Error(w, "Failed to marshal config dump", http.StatusInternalServerError)
		return
	}

	// Redact resolved secret values so plaintext secrets never appear in the dump.
	// json.Marshal JSON-escapes special characters (e.g. `"` → `\"`), so a
	// raw string match would miss secrets that contain those characters.
	// Build a combined set: each raw secret plus its JSON-escaped form (the
	// content that json.Marshal would emit inside a JSON string literal).
	sensitiveValues := make([]string, 0, len(rawSecrets)*2)
	for _, secret := range rawSecrets {
		sensitiveValues = append(sensitiveValues, secret)
		if escapedBytes, err2 := json.Marshal(secret); err2 == nil && len(escapedBytes) >= 2 {
			// escapedBytes is `"<content>"` — strip the surrounding quotes.
			escaped := string(escapedBytes[1 : len(escapedBytes)-1])
			if escaped != secret {
				sensitiveValues = append(sensitiveValues, escaped)
			}
		}
	}
	redacted := redact.Redact(string(jsonBytes), sensitiveValues)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(redacted))
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

	// Check policy engine health
	peHealthy := h.health == nil || h.health.IsHealthy()

	// Check Python executor health if checker is configured
	pyHealthy := true
	if h.pythonHealth != nil {
		ready, _, err := h.pythonHealth.IsPythonHealthy()
		pyHealthy = err == nil && ready
	}

	// Build response
	resp := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	statusCode := http.StatusOK

	// If any component is unhealthy, set status to unhealthy and provide reason
	if !peHealthy || !pyHealthy {
		resp.Status = "unhealthy"
		statusCode = http.StatusServiceUnavailable

		// Build reason message
		if !peHealthy && !pyHealthy {
			resp.Reason = "policy engine and python executor are unhealthy"
		} else if !peHealthy {
			resp.Reason = "policy engine is unhealthy"
		} else {
			resp.Reason = "python executor is unhealthy"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}
