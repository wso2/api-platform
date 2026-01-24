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
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCorrelationIDMiddleware_ExistingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should not be empty")
		}
		if correlationID != "test-correlation-id-123" {
			t.Errorf("Expected correlation ID 'test-correlation-id-123', got '%s'", correlationID)
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(CorrelationIDHeader, "test-correlation-id-123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID != "test-correlation-id-123" {
		t.Errorf("Expected response header to contain 'test-correlation-id-123', got '%s'", responseCorrelationID)
	}
}

func TestCorrelationIDMiddleware_GenerateNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should be auto-generated when not provided")
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID == "" {
		t.Error("Response header should contain auto-generated correlation ID")
	}
}

func TestGetLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	baseLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := gin.New()
	router.Use(CorrelationIDMiddleware(baseLogger))

	router.GET("/test", func(c *gin.Context) {
		logger := GetLogger(c, baseLogger)
		if logger == nil {
			t.Error("Logger should not be nil")
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetLogger_Fallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fallbackLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Create a context without the middleware
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		logger := GetLogger(c, fallbackLogger)
		if logger != fallbackLogger {
			t.Error("Should return fallback logger when no logger in context")
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCorrelationIDMiddleware_LowercaseHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should not be empty")
		}
		if correlationID != "lowercase-correlation-id-456" {
			t.Errorf("Expected correlation ID 'lowercase-correlation-id-456', got '%s'", correlationID)
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// Test with lowercase header name
	req.Header.Set("x-correlation-id", "lowercase-correlation-id-456")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID != "lowercase-correlation-id-456" {
		t.Errorf("Expected response header to contain 'lowercase-correlation-id-456', got '%s'", responseCorrelationID)
	}
}

func TestCorrelationIDMiddleware_MixedCaseHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	router := gin.New()
	router.Use(CorrelationIDMiddleware(logger))

	router.GET("/test", func(c *gin.Context) {
		correlationID := GetCorrelationID(c)
		if correlationID == "" {
			t.Error("Correlation ID should not be empty")
		}
		if correlationID != "mixed-case-id-789" {
			t.Errorf("Expected correlation ID 'mixed-case-id-789', got '%s'", correlationID)
		}
		c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// Test with mixed case header name
	req.Header.Set("X-CoRrElAtIoN-Id", "mixed-case-id-789")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	responseCorrelationID := w.Header().Get(CorrelationIDHeader)
	if responseCorrelationID != "mixed-case-id-789" {
		t.Errorf("Expected response header to contain 'mixed-case-id-789', got '%s'", responseCorrelationID)
	}
}
