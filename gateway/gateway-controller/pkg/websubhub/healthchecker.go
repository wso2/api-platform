package websubhub

import (
	"net/http"
	"sync"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// HealthChecker periodically pings a target and maintains health state
// Use Start() to begin health checks and IsHealthy() to query state
type HealthChecker struct {
	url                   string
	interval              time.Duration
	mu                    sync.RWMutex
	healthy               bool
	quit                  chan struct{}
	websubhubUtilsService *utils.WebsubhubUtilsService
}

// NewHealthChecker creates a new HealthChecker for the given URL and interval
func NewHealthChecker(url string, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		url:                   url,
		interval:              interval,
		healthy:               false,
		quit:                  make(chan struct{}),
		websubhubUtilsService: utils.NewWebSubHubUtilsService(),
	}
}

// Start begins the periodic health checks in a goroutine
func (h *HealthChecker) Start() {
	go func() {
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.check()
			case <-h.quit:
				return
			}
		}
	}()
}

// Stop halts the health checking goroutine
func (h *HealthChecker) Stop() {
	close(h.quit)
}

// IsHealthy returns the last known health state
func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.healthy
}

// check pings the target URL and updates the health state
func (h *HealthChecker) check() {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(h.url)
	healthy := err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300
	if resp != nil {
		resp.Body.Close()
	}
	h.mu.Lock()
	h.healthy = healthy
	h.mu.Unlock()
}
