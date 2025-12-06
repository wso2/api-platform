package admin

import (
	"encoding/json"
	"net/http"

	"github.com/policy-engine/policy-engine/internal/kernel"
	"github.com/policy-engine/policy-engine/internal/registry"
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
