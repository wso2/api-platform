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
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestErrorHandlingMiddleware_NormalRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	h := ErrorHandlingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

func TestErrorHandlingMiddleware_PanicRecovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	h := ErrorHandlingMiddleware(logger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("something went wrong")
	}))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "Internal server error", response.Message)
}

func TestErrorHandlingMiddleware_PanicWithCorrelationID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// CorrelationID must wrap ErrorHandling so the logger is available in context.
	h := CorrelationIDMiddleware(logger)(
		ErrorHandlingMiddleware(logger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			panic("correlation test panic")
		})),
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	req.Header.Set("X-Correlation-ID", "test-correlation-123")
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "test-correlation-123", w.Header().Get("X-Correlation-ID"))
}

func TestErrorHandlingMiddleware_DifferentPanicTypes(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{"string panic", "string error"},
		{"error panic", assert.AnError},
		{"int panic", 123},
		{"nil panic", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			h := ErrorHandlingMiddleware(logger)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				panic(tt.panicValue)
			}))

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/panic", nil))

			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	}
}
