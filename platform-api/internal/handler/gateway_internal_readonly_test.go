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

// TEMP-READ-ONLY-MODE: this whole file is part of the temporary organization-scoped
// read-only mode used while Bijira migrates from Platform API v1 to v2. Delete it
// when the mode is removed.

package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
)

const (
	readOnlyTestOrgA = "11111111-1111-1111-1111-111111111111"
	readOnlyTestOrgB = "22222222-2222-2222-2222-222222222222"
)

func TestGatewayInternalAPIHandler_rejectIfReadOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// The constructor never dereferences the services, so nil is fine here.
	h := NewGatewayInternalAPIHandler(nil, nil, nil, nil, logger)

	newCtx := func() (*httptest.ResponseRecorder, *http.Request) {
		return httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/internal/v1/gateways/gw/manifest", nil)
	}

	// Never wired: the mode is off for this handler.
	rec, req := newCtx()
	if h.rejectIfReadOnly(rec, req, readOnlyTestOrgA, "gw") {
		t.Fatal("unwired handler must not reject")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("unwired handler wrote status %d, want the recorder left untouched (200)", rec.Code)
	}

	h.SetReadOnly(&config.ReadOnly{Enabled: true, WritableOrganizations: []string{readOnlyTestOrgB}})

	rec, req = newCtx()
	if !h.rejectIfReadOnly(rec, req, readOnlyTestOrgA, "gw") {
		t.Fatal("expected the write for a frozen organization to be rejected")
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	// The Gateway Internal API's own error shape: {code, message, description}.
	var body struct {
		Code        int    `json:"code"`
		Message     string `json:"message"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if body.Code != http.StatusServiceUnavailable || body.Message != "Service Unavailable" {
		t.Errorf("body = %+v, want code 503 / message \"Service Unavailable\"", body)
	}
	if !strings.Contains(body.Description, "read-only") {
		t.Errorf("description should explain the read-only condition, got %q", body.Description)
	}

	rec, req = newCtx()
	if h.rejectIfReadOnly(rec, req, readOnlyTestOrgB, "gw") {
		t.Fatal("expected the write for a writable organization to be allowed")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("writable organization: status %d, want the recorder left untouched (200)", rec.Code)
	}
}
