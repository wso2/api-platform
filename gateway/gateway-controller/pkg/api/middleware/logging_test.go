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
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// okHandler returns a simple JSON 200 handler for test cases.
func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func TestLoggingMiddleware_BasicRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?param=value", nil)
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "method=GET")
	assert.Contains(t, logOutput, "path=/test")
	assert.Contains(t, logOutput, "status=200")
	assert.Contains(t, logOutput, "latency")
}

func TestLoggingMiddleware_WithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := CorrelationIDMiddleware(logger)(LoggingMiddleware(logger)(http.HandlerFunc(okHandler)))

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Correlation-ID", "test-corr-123")
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, buf.String(), "test-corr-123")
}

func TestLoggingMiddleware_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"200 OK", http.StatusOK},
		{"201 Created", http.StatusCreated},
		{"400 Bad Request", http.StatusBadRequest},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

			h := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Contains(t, buf.String(), "status=")
		})
	}
}

func TestLoggingMiddleware_WithQuery(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test?key1=value1&key2=value2", nil))

	assert.Contains(t, buf.String(), "query=")
	assert.Contains(t, buf.String(), "key1=value1")
}

func TestLoggingMiddleware_UserAgent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "TestClient/1.0")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Contains(t, buf.String(), "user_agent=TestClient/1.0")
}

func TestLoggingMiddleware_LatencyMeasurement(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	assert.Contains(t, buf.String(), "latency=")
}

func TestLoggingMiddleware_DifferentHTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

			h := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(method, "/test", nil))

			assert.Contains(t, buf.String(), "method="+method)
		})
	}
}

func TestLoggingMiddleware_ClientIP(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	assert.Contains(t, buf.String(), "client_ip=")
}

func TestLoggingMiddleware_EmptyPath(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	assert.Contains(t, buf.String(), "path=/")
}

func TestLoggingMiddleware_FallbackToBaseLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Logging without correlation middleware — should still log.
	h := LoggingMiddleware(logger)(http.HandlerFunc(okHandler))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, len(buf.String()) > 0)
}
