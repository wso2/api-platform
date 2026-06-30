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
	"net/http"
	"strconv"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
)

// metricsResponseWriter wraps http.ResponseWriter to capture status code and
// response body size after the downstream handler has written the response.
type metricsResponseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (rw *metricsResponseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *metricsResponseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

// MetricsMiddleware records HTTP request metrics via Prometheus.
//
// When registered as a per-route middleware (inside the mux, via
// StdHTTPServerOptions.Middlewares), r.Pattern carries the route template
// (e.g. "GET /api/llm-providers/{id}"), giving low-cardinality labels.
// When registered as outer middleware, r.Pattern is empty and r.URL.Path is
// used as a fallback.
func MetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			metrics.ConcurrentRequests.Inc()
			defer metrics.ConcurrentRequests.Dec()

			startTime := time.Now()

			requestSize := r.ContentLength
			if requestSize < 0 {
				requestSize = 0
			}

			rw := &metricsResponseWriter{ResponseWriter: w, status: 0}
			next.ServeHTTP(rw, r)

			duration := time.Since(startTime)

			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			responseSize := rw.size

			// r.Pattern is set when running as per-route middleware (inside mux).
			// Fall back to URL path when running as outer middleware.
			endpoint := r.Pattern
			if endpoint == "" {
				endpoint = r.URL.Path
			}

			statusStr := strconv.Itoa(status)
			method := r.Method

			metrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusStr).Inc()
			metrics.HTTPRequestDurationSeconds.WithLabelValues(method, endpoint).Observe(duration.Seconds())
			metrics.HTTPRequestSizeBytes.WithLabelValues(endpoint).Observe(float64(requestSize))
			metrics.HTTPResponseSizeBytes.WithLabelValues(endpoint).Observe(float64(responseSize))
		})
	}
}
