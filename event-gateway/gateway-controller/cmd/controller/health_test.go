/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/health"
)

func TestHealthHandlerAnswersHealthy(t *testing.T) {
	rec := httptest.NewRecorder()
	newHealthHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/v1/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var body struct {
		Status    string         `json:"status"`
		Timestamp string         `json:"timestamp"`
		Checks    map[string]any `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Status != "healthy" {
		t.Fatalf("status field = %q, want healthy", body.Status)
	}
	if _, err := time.Parse(time.RFC3339, body.Timestamp); err != nil {
		t.Fatalf("timestamp %q is not RFC 3339: %v", body.Timestamp, err)
	}
	if len(body.Checks) != 0 {
		t.Fatalf("checks = %v, want none", body.Checks)
	}
}

// Only GET and HEAD are answered, as on the gateway controller.
func TestHealthHandlerRejectsOtherMethods(t *testing.T) {
	rec := httptest.NewRecorder()
	newHealthHandler(nil).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin/v1/health", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// A client outside controller.health.allowed_clients gets 403; an allowed one the healthy
// answer.
func TestHealthHandlerHonoursAllowedClients(t *testing.T) {
	allowed, err := health.ParseAllowedClients("controller.health", []string{"10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	h := newHealthHandler(allowed)

	for addr, want := range map[string]int{"10.1.2.3:4000": http.StatusOK, "192.168.1.5:4000": http.StatusForbidden} {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/ready", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("client %s: status = %d, want %d", addr, rec.Code, want)
		}
	}
}
