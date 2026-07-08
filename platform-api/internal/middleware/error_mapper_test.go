/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

func testLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

func TestMapErrorsAppError(t *testing.T) {
	var logBuf bytes.Buffer
	h := MapErrors(testLogger(&logBuf), func(w http.ResponseWriter, r *http.Request) error {
		return apperror.ProjectNotFound.New().
			WithLogMessage("project p1 missing in org o1")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v0.9/projects/p1", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var body utils.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Status != "error" || body.Code != utils.CodeProjectNotFound ||
		body.Message != "The specified project could not be found." {
		t.Errorf("unexpected body: %+v", body)
	}
	log := logBuf.String()
	if !strings.Contains(log, "trackingId") {
		t.Error("expected the mapper to log a trackingId")
	}
	if !strings.Contains(log, "project p1 missing in org o1") {
		t.Error("expected the mapper to log the internal detail")
	}
	if strings.Contains(rec.Body.String(), "project p1 missing") {
		t.Error("internal log message leaked into the client response")
	}
}

func TestMapErrorsSeveritySplit(t *testing.T) {
	// 4xx: WARN, no stack, no trackingId in the body.
	var warnBuf bytes.Buffer
	h := MapErrors(testLogger(&warnBuf), func(w http.ResponseWriter, r *http.Request) error {
		return apperror.ProjectNotFound.New()
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))

	log := warnBuf.String()
	if !strings.Contains(log, `"level":"WARN"`) {
		t.Errorf("expected 4xx to log at WARN, got: %s", log)
	}
	if strings.Contains(log, `"stack"`) {
		t.Error("4xx must not log a stack trace")
	}
	var body utils.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.TrackingID != "" {
		t.Error("4xx response must not carry a trackingId")
	}

	// 5xx: ERROR, stack logged, trackingId echoed in the body and matching the log.
	var errBuf bytes.Buffer
	h = MapErrors(testLogger(&errBuf), func(w http.ResponseWriter, r *http.Request) error {
		return apperror.Internal.Wrap(errors.New("db down"))
	})
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))

	log = errBuf.String()
	if !strings.Contains(log, `"level":"ERROR"`) {
		t.Errorf("expected 5xx to log at ERROR, got: %s", log)
	}
	if !strings.Contains(log, `"stack"`) {
		t.Error("5xx must log the origin stack trace")
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.TrackingID == "" {
		t.Error("5xx response must carry a trackingId for log correlation")
	}
	if !strings.Contains(log, body.TrackingID) {
		t.Error("trackingId in the response must match the logged one")
	}
}

func TestMapErrorsPlainErrorFallsBackToGeneric500(t *testing.T) {
	var logBuf bytes.Buffer
	h := MapErrors(testLogger(&logBuf), func(w http.ResponseWriter, r *http.Request) error {
		return errors.New("pq: connection reset by peer at 10.0.0.5:5432")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body utils.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Code != utils.CodeCommonInternalError || body.Message != "An unexpected error occurred." {
		t.Errorf("unexpected body: %+v", body)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Error("raw internal error leaked into the client response")
	}
	if !strings.Contains(logBuf.String(), "10.0.0.5") {
		t.Error("expected the raw cause to be logged internally")
	}
}

func TestMapErrorsPrefersInnerTypedErrorOverGenericInternalWrapper(t *testing.T) {
	// A service returns a specific typed error; a handler fallback blindly
	// wraps it in a generic Internal. The mapper must surface the specific
	// error, not the 500 wrapper.
	var logBuf bytes.Buffer
	h := MapErrors(testLogger(&logBuf), func(w http.ResponseWriter, r *http.Request) error {
		serviceErr := apperror.GatewayNotFound.New().WithLogMessage("gateway g1 missing")
		return apperror.Internal.Wrap(serviceErr).WithLogMessage("failed to get gateway")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v0.9/gateways/g1", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 from the inner typed error, got %d", rec.Code)
	}
	var body utils.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Code != utils.CodeGatewayNotFound {
		t.Errorf("expected code %q, got %q", utils.CodeGatewayNotFound, body.Code)
	}
	if !strings.Contains(logBuf.String(), "gateway g1 missing") {
		t.Error("expected the inner error's log message to be logged")
	}
}

func TestMapErrorsRecoversPanic(t *testing.T) {
	var logBuf bytes.Buffer
	h := MapErrors(testLogger(&logBuf), func(w http.ResponseWriter, r *http.Request) error {
		panic("secret internal state: token=abc123")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body utils.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Code != utils.CodeCommonInternalError {
		t.Errorf("unexpected code %q", body.Code)
	}
	if strings.Contains(rec.Body.String(), "abc123") {
		t.Error("panic value leaked into the client response")
	}
	log := logBuf.String()
	if !strings.Contains(log, "panic recovered") || !strings.Contains(log, "trackingId") {
		t.Error("expected a structured panic log with trackingId")
	}
}

func TestMapErrorsSuccessPathWritesNothingExtra(t *testing.T) {
	var logBuf bytes.Buffer
	h := MapErrors(testLogger(&logBuf), func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusNoContent)
		return nil
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("DELETE", "/x", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("expected empty body, got %q", rec.Body.String())
	}
	if strings.Contains(logBuf.String(), "request failed") {
		t.Error("mapper logged a failure for a successful request")
	}
}
