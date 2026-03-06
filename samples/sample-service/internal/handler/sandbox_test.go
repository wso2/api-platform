// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSandboxWhoAmIReturnsSandboxResponse(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/sandbox/whoami", nil)
	rec := httptest.NewRecorder()

	h.SandboxWhoAmI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["environment"] != "sandbox" {
		t.Fatalf("expected environment sandbox, got %q", resp["environment"])
	}
	if resp["source"] != "sample-backend" {
		t.Fatalf("expected source sample-backend, got %q", resp["source"])
	}
	if resp["path"] != "/sandbox/whoami" {
		t.Fatalf("expected path /sandbox/whoami, got %q", resp["path"])
	}
}
