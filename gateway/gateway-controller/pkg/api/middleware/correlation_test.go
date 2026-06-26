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
)

func TestCorrelationIDMiddleware_ExistingHeader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var got string
	h := CorrelationIDMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetCorrelationID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(CorrelationIDHeader, "test-correlation-id-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if got != "test-correlation-id-123" {
		t.Errorf("Expected correlation ID 'test-correlation-id-123', got '%s'", got)
	}
	if w.Header().Get(CorrelationIDHeader) != "test-correlation-id-123" {
		t.Errorf("Expected response header 'test-correlation-id-123', got '%s'", w.Header().Get(CorrelationIDHeader))
	}
}

func TestCorrelationIDMiddleware_GenerateNew(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var got string
	h := CorrelationIDMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetCorrelationID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if got == "" {
		t.Error("Correlation ID should be auto-generated when not provided")
	}
	if w.Header().Get(CorrelationIDHeader) == "" {
		t.Error("Response header should contain auto-generated correlation ID")
	}
}

func TestGetLogger(t *testing.T) {
	baseLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	h := CorrelationIDMiddleware(baseLogger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetLogger(r, baseLogger) == nil {
			t.Error("Logger should not be nil")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetLogger_Fallback(t *testing.T) {
	fallbackLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// No correlation middleware — should return fallback.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := GetLogger(r, fallbackLogger)
		if logger != fallbackLogger {
			t.Error("Should return fallback logger when no logger in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestCorrelationIDMiddleware_LowercaseHeader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var got string
	h := CorrelationIDMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetCorrelationID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("x-correlation-id", "lowercase-correlation-id-456")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if got != "lowercase-correlation-id-456" {
		t.Errorf("Expected correlation ID 'lowercase-correlation-id-456', got '%s'", got)
	}
	if w.Header().Get(CorrelationIDHeader) != "lowercase-correlation-id-456" {
		t.Errorf("Expected response header 'lowercase-correlation-id-456', got '%s'", w.Header().Get(CorrelationIDHeader))
	}
}

func TestCorrelationIDMiddleware_MixedCaseHeader(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var got string
	h := CorrelationIDMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = GetCorrelationID(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-CoRrElAtIoN-Id", "mixed-case-id-789")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
	if got != "mixed-case-id-789" {
		t.Errorf("Expected correlation ID 'mixed-case-id-789', got '%s'", got)
	}
	if w.Header().Get(CorrelationIDHeader) != "mixed-case-id-789" {
		t.Errorf("Expected response header 'mixed-case-id-789', got '%s'", w.Header().Get(CorrelationIDHeader))
	}
}
