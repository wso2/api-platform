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
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
)

// MetricsMiddleware returns a Gin middleware that records HTTP request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Track concurrent requests
		metrics.ConcurrentRequests.Inc()
		defer metrics.ConcurrentRequests.Dec()

		// Start timer
		startTime := time.Now()

		// Get request size
		requestSize := c.Request.ContentLength
		if requestSize < 0 {
			requestSize = 0
		}

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Get response status and size
		status := c.Writer.Status()
		responseSize := c.Writer.Size()
		if responseSize < 0 {
			responseSize = 0
		}

		// Get endpoint pattern (use FullPath for route pattern, fallback to path)
		endpoint := c.FullPath()
		if endpoint == "" {
			endpoint = c.Request.URL.Path
		}

		// Record metrics
		statusStr := strconv.Itoa(status)
		method := c.Request.Method

		metrics.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusStr).Inc()
		metrics.HTTPRequestDurationSeconds.WithLabelValues(method, endpoint).Observe(duration.Seconds())
		metrics.HTTPRequestSizeBytes.WithLabelValues(endpoint).Observe(float64(requestSize))
		metrics.HTTPResponseSizeBytes.WithLabelValues(endpoint).Observe(float64(responseSize))
	}
}
