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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestErrorHandlingMiddleware_NormalRequest tests middleware with normal request (no panic)
func TestErrorHandlingMiddleware_NormalRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create router with middleware
	router := gin.New()
	router.Use(ErrorHandlingMiddleware(logger))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	})

	// Test normal request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "success", response["status"])
}

// TestErrorHandlingMiddleware_PanicRecovery tests middleware recovers from panic
func TestErrorHandlingMiddleware_PanicRecovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create router with middleware
	router := gin.New()
	router.Use(ErrorHandlingMiddleware(logger))
	router.GET("/panic", func(c *gin.Context) {
		panic("something went wrong")
	})

	// Test panic recovery
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "Internal server error", response.Message)
}

// TestErrorHandlingMiddleware_PanicWithCorrelationID tests panic recovery with correlation ID
func TestErrorHandlingMiddleware_PanicWithCorrelationID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create router with correlation middleware first, then error handling
	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))
	router.Use(ErrorHandlingMiddleware(logger))
	router.GET("/panic", func(c *gin.Context) {
		panic("correlation test panic")
	})

	// Test panic recovery with correlation ID
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	req.Header.Set("X-Correlation-ID", "test-correlation-123")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response api.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response.Status)

	// Verify correlation ID was set in response
	assert.Equal(t, "test-correlation-123", w.Header().Get("X-Correlation-ID"))
}

// TestErrorHandlingMiddleware_DifferentPanicTypes tests recovery from different panic types
func TestErrorHandlingMiddleware_DifferentPanicTypes(t *testing.T) {
	tests := []struct {
		name       string
		panicValue interface{}
	}{
		{
			name:       "string panic",
			panicValue: "string error",
		},
		{
			name:       "error panic",
			panicValue: assert.AnError,
		},
		{
			name:       "int panic",
			panicValue: 123,
		},
		{
			name:       "nil panic",
			panicValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

			router := gin.New()
			router.Use(ErrorHandlingMiddleware(logger))
			router.GET("/panic", func(c *gin.Context) {
				panic(tt.panicValue)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/panic", nil)
			router.ServeHTTP(w, req)

			// All panics should result in 500 error
			assert.Equal(t, http.StatusInternalServerError, w.Code)
		})
	}
}
