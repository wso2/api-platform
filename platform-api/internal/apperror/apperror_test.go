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

package apperror

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefNewCapturesStack(t *testing.T) {
	e := Internal.New()
	if len(e.Stack) == 0 {
		t.Fatal("expected non-empty stack captured at construction time")
	}
	stack := e.StackString()
	if !strings.Contains(stack, "TestDefNewCapturesStack") {
		t.Errorf("expected stack trace to include the calling function, got:\n%s", stack)
	}
}

func TestDefNewFormatsMessage(t *testing.T) {
	e := ValidationFailed.New("API name is required")
	if e.Message != "API name is required" {
		t.Errorf("unexpected message %q", e.Message)
	}
	if e.Code != CodeCommonValidationFailed || e.HTTPStatus != http.StatusBadRequest {
		t.Errorf("Def did not carry its declared code/status: %+v", e)
	}

	// Fixed-message entries called with no args return the template verbatim.
	if got := RESTAPINotFound.New().Message; got != "The specified REST API could not be found." {
		t.Errorf("unexpected fixed message %q", got)
	}

	// Templates with verbs interpolate args.
	if got := DeploymentNotActive.New("API").Message; got != "No active deployment found for this API on the gateway." {
		t.Errorf("unexpected formatted message %q", got)
	}
}

func TestDefWrapAttachesCause(t *testing.T) {
	cause := errors.New("db connection refused")
	e := Internal.Wrap(cause)
	if !errors.Is(e, cause) {
		t.Error("expected errors.Is to find the wrapped cause")
	}
	if len(e.Stack) == 0 {
		t.Error("expected Wrap to capture the stack too")
	}

	// A wrapped *Error must still be discoverable via errors.As — the
	// propagation contract relies on this.
	wrapped := fmt.Errorf("service layer context: %w", e)
	var appErr *Error
	if !errors.As(wrapped, &appErr) {
		t.Fatal("expected errors.As to find *Error through a %w wrap")
	}
	if appErr.Code != CodeCommonInternalError {
		t.Errorf("unexpected code %q", appErr.Code)
	}
}

func TestDefIs(t *testing.T) {
	err := fmt.Errorf("outer: %w", ProjectNotFound.New())
	if !ProjectNotFound.Is(err) {
		t.Error("expected Def.Is to match through a %w wrap")
	}
	if RESTAPINotFound.Is(err) {
		t.Error("Def.Is matched a different catalog entry")
	}
	if ProjectNotFound.Is(errors.New("plain")) {
		t.Error("Def.Is matched a plain error")
	}
}

func TestNewValidationFallback(t *testing.T) {
	e := NewValidation(errors.New("bad json"))
	if e.HTTPStatus != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", e.HTTPStatus)
	}
	if e.Code != CodeCommonValidationFailed {
		t.Errorf("expected %s, got %s", CodeCommonValidationFailed, e.Code)
	}
	if e.Message != "Invalid input." {
		t.Errorf("unexpected message %q", e.Message)
	}
}

func TestWriteHTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteHTTP(rec, ProjectNotFound.Wrap(errors.New("sql: no rows in result set")).
		WithLogMessage("internal detail that must not leak"), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Status != "error" || body.Code != CodeProjectNotFound {
		t.Errorf("unexpected envelope: %+v", body)
	}
	if strings.Contains(rec.Body.String(), "sql:") || strings.Contains(rec.Body.String(), "internal detail") {
		t.Error("internal diagnostics leaked into the client response")
	}
	if strings.Contains(rec.Body.String(), "trackingId") {
		t.Error("empty trackingId must be omitted from the response")
	}

	rec = httptest.NewRecorder()
	WriteHTTP(rec, Internal.New(), "d2f1c0aa-0000-0000-0000-000000000000")
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.TrackingID != "d2f1c0aa-0000-0000-0000-000000000000" {
		t.Errorf("expected trackingId echoed in body, got %+v", body)
	}
}

// TestErrorIs_MatchesOnCodeNotIdentity guards the behaviour that lets
// errors.Is treat two independently constructed instances of the same catalog
// entry as equal. Def.New allocates a fresh *Error each call, so pointer
// identity would never match.
func TestErrorIs_MatchesOnCodeNotIdentity(t *testing.T) {
	a := NotFound.New()
	b := NotFound.New()
	if a == b {
		t.Fatal("Def.New must allocate a distinct *Error each call")
	}
	if !errors.Is(a, b) {
		t.Error("errors.Is should match two errors carrying the same catalog code")
	}
	if errors.Is(a, Conflict.New()) {
		t.Error("errors.Is must not match errors carrying different catalog codes")
	}
	// The code must also be found through a wrapping chain.
	wrapped := fmt.Errorf("context: %w", a)
	if !errors.Is(wrapped, NotFound.New()) {
		t.Error("errors.Is should see through fmt.Errorf %w wrapping")
	}
	if !NotFound.Is(wrapped) {
		t.Error("Def.Is should see through fmt.Errorf %w wrapping")
	}
}
