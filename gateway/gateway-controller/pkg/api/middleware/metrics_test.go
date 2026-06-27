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

package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
)

// setupMetrics initialises the Prometheus registry for a test.
func setupMetrics(t *testing.T) {
	t.Helper()
	metrics.SetEnabled(true)
	metrics.Init()
}

// metricsMux registers a single route and wraps it with MetricsMiddleware running
// INSIDE the mux handler so r.Pattern is available.
func metricsMux(method, pattern string, status int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(method+" "+pattern, func(w http.ResponseWriter, r *http.Request) {
		MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
		})).ServeHTTP(w, r)
	})
	return mux
}

func TestMetricsMiddleware_BasicRequest(t *testing.T) {
	setupMetrics(t)

	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "gateway_controller_http_requests_total" {
			found = true
			assert.Greater(t, len(mf.GetMetric()), 0)
		}
	}
	assert.True(t, found, "http_requests_total metric should exist")
}

func TestMetricsMiddleware_DifferentHTTPMethods(t *testing.T) {
	setupMetrics(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/test"},
		{http.MethodPost, "/api/test"},
		{http.MethodPut, "/api/test"},
		{http.MethodDelete, "/api/test"},
		{http.MethodPatch, "/api/test"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			assert.Equal(t, http.StatusOK, w.Code)

			mfs, err := metrics.GetRegistry().Gather()
			assert.NoError(t, err)

			found := false
			for _, mf := range mfs {
				if mf.GetName() == "gateway_controller_http_requests_total" {
					for _, m := range mf.GetMetric() {
						for _, l := range m.GetLabel() {
							if l.GetName() == "method" && l.GetValue() == tt.method {
								found = true
							}
						}
					}
				}
			}
			assert.True(t, found, "Metric should contain method label: %s", tt.method)
		})
	}
}

func TestMetricsMiddleware_DifferentStatusCodes(t *testing.T) {
	setupMetrics(t)

	tests := []struct {
		name       string
		statusCode int
	}{
		{"OK 200", http.StatusOK},
		{"Created 201", http.StatusCreated},
		{"Bad Request 400", http.StatusBadRequest},
		{"Not Found 404", http.StatusNotFound},
		{"Internal Server Error 500", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))
			assert.Equal(t, tt.statusCode, w.Code)

			mfs, err := metrics.GetRegistry().Gather()
			assert.NoError(t, err)

			found := false
			for _, mf := range mfs {
				if mf.GetName() == "gateway_controller_http_requests_total" {
					for _, m := range mf.GetMetric() {
						for _, l := range m.GetLabel() {
							if l.GetName() == "status_code" {
								found = true
							}
						}
					}
				}
			}
			assert.True(t, found, "Metric should contain status_code label")
		})
	}
}

func TestMetricsMiddleware_RequestDuration(t *testing.T) {
	setupMetrics(t)

	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/slow", nil))
	assert.GreaterOrEqual(t, time.Since(start).Milliseconds(), int64(10))

	assert.Equal(t, http.StatusOK, w.Code)

	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "gateway_controller_http_request_duration_seconds" {
			found = true
			assert.Equal(t, dto.MetricType_HISTOGRAM, mf.GetType())
			assert.Greater(t, len(mf.GetMetric()), 0)
		}
	}
	assert.True(t, found, "http_request_duration_seconds metric should exist")
}

func TestMetricsMiddleware_RequestResponseSizes(t *testing.T) {
	setupMetrics(t)

	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"received": data})
	}))

	requestBody := []byte(`{"test": "data", "number": 123}`)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(requestBody))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Greater(t, w.Body.Len(), 0)

	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)

	foundReq, foundResp := false, false
	for _, mf := range mfs {
		switch mf.GetName() {
		case "gateway_controller_http_request_size_bytes":
			foundReq = true
			assert.Equal(t, dto.MetricType_HISTOGRAM, mf.GetType())
		case "gateway_controller_http_response_size_bytes":
			foundResp = true
			assert.Equal(t, dto.MetricType_HISTOGRAM, mf.GetType())
		}
	}
	assert.True(t, foundReq, "http_request_size_bytes metric should exist")
	assert.True(t, foundResp, "http_response_size_bytes metric should exist")
}

// TestMetricsMiddleware_EndpointLabelingPerRoute checks that r.Pattern is used
// when the middleware runs inside a mux handler (per-route).
func TestMetricsMiddleware_EndpointLabelingPerRoute(t *testing.T) {
	setupMetrics(t)

	tests := []struct {
		name             string
		pattern          string
		requestPath      string
		expectedEndpoint string
	}{
		{
			name:             "Route with parameter",
			pattern:          "/api/{id}",
			requestPath:      "/api/123",
			expectedEndpoint: "GET /api/{id}",
		},
		{
			name:             "Static route",
			pattern:          "/api/health",
			requestPath:      "/api/health",
			expectedEndpoint: "GET /api/health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := metricsMux("GET", tt.pattern, http.StatusOK)

			req := httptest.NewRequest(http.MethodGet, tt.requestPath, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)

			mfs, err := metrics.GetRegistry().Gather()
			assert.NoError(t, err)

			found := false
			for _, mf := range mfs {
				if mf.GetName() == "gateway_controller_http_requests_total" {
					for _, m := range mf.GetMetric() {
						for _, l := range m.GetLabel() {
							if l.GetName() == "endpoint" && l.GetValue() == tt.expectedEndpoint {
								found = true
							}
						}
					}
				}
			}
			assert.True(t, found, "Metric should have endpoint label: %s", tt.expectedEndpoint)
		})
	}
}

// TestMetricsMiddleware_EndpointLabeling_FallbackToPath checks that r.URL.Path
// is used as the endpoint label when the middleware runs outside the mux (r.Pattern is empty).
func TestMetricsMiddleware_EndpointLabeling_FallbackToPath(t *testing.T) {
	setupMetrics(t)

	// Middleware wrapping a 404 — r.Pattern is empty.
	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)

	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "gateway_controller_http_requests_total" {
			for _, m := range mf.GetMetric() {
				for _, l := range m.GetLabel() {
					if l.GetName() == "endpoint" && l.GetValue() == "/nonexistent" {
						found = true
					}
				}
			}
		}
	}
	assert.True(t, found, "Metric should fall back to URL.Path when r.Pattern is empty")
}

func TestMetricsMiddleware_ConcurrentRequests(t *testing.T) {
	setupMetrics(t)

	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	initialCount := getGaugeValue(t, "gateway_controller_concurrent_requests")

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/concurrent", nil))
			assert.Equal(t, http.StatusOK, w.Code)
		}()
	}

	time.Sleep(10 * time.Millisecond)
	finalCount := getGaugeValue(t, "gateway_controller_concurrent_requests")

	wg.Wait()

	afterCount := getGaugeValue(t, "gateway_controller_concurrent_requests")
	assert.Equal(t, initialCount, afterCount, "Concurrent requests gauge should return to initial value")
	assert.GreaterOrEqual(t, finalCount, float64(0))
}

func TestMetricsMiddleware_NegativeContentLength(t *testing.T) {
	setupMetrics(t)

	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.ContentLength = -1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "gateway_controller_http_request_size_bytes" {
			found = true
			assert.Greater(t, len(mf.GetMetric()), 0)
		}
	}
	assert.True(t, found, "Request size metric should handle negative content length")
}

func TestMetricsMiddleware_ZeroResponseSize(t *testing.T) {
	setupMetrics(t)

	h := MetricsMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/empty", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "gateway_controller_http_response_size_bytes" {
			found = true
			assert.Greater(t, len(mf.GetMetric()), 0)
		}
	}
	assert.True(t, found, "Response size metric should handle zero size")
}

// getGaugeValue reads a gauge metric by name from the Prometheus registry.
func getGaugeValue(t *testing.T, metricName string) float64 {
	t.Helper()
	mfs, err := metrics.GetRegistry().Gather()
	assert.NoError(t, err)
	for _, mf := range mfs {
		if mf.GetName() == metricName && len(mf.GetMetric()) > 0 {
			return mf.GetMetric()[0].GetGauge().GetValue()
		}
	}
	return 0
}
