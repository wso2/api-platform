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

package controlplane

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// util methods for tests

// makePolicyDef creates a PolicyDefinition for use in tests.
// Parameters and SystemParameters are only set for customer-managed policies,
// matching the manifest format sent to the control plane.
func makePolicyDef(name, version, managedBy string) models.PolicyDefinition {
	def := models.PolicyDefinition{
		Name:        name,
		Version:     version,
		DisplayName: name,
		ManagedBy:   managedBy,
	}
	if managedBy == "customer" {
		params := map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
		systemParams := map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
		def.Parameters = &params
		def.SystemParameters = &systemParams
	}
	return def
}

// newManifestTLSServer creates a TLS test server with the given handler and returns
// a configured Client pointing at it. The server is automatically closed when the
// test finishes via t.Cleanup.
func newManifestTLSServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	host := strings.TrimPrefix(server.URL, "https://")
	return createTestClientWithHost(t, host)
}

// testGatewayID is the gateway ID used across manifest test cases.
const testGatewayID = "gw-1"

// manifestPayload mirrors the JSON body sent by pushGatewayManifest.
type manifestPayload struct {
	Policies []models.PolicyDefinition `json:"policies"`
}

// --- pushGatewayManifest tests ---

// TestPushGatewayManifest_Success204 verifies that a manifest POST succeeds when the
// control plane responds with 204 No Content, and that the request is sent to the
// correct URL path with the expected method, auth header, content-type, and policy body.
func TestPushGatewayManifest_Success204(t *testing.T) {
	var capturedReq *http.Request
	var capturedBody []byte

	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedReq = r
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	policies := []models.PolicyDefinition{
		makePolicyDef("rate-limit", "v1.0.0", "wso2"),
	}

	err := client.pushGatewayManifest(testGatewayID, policies)
	if err != nil {
		t.Fatalf("pushGatewayManifest() unexpected error: %v", err)
	}

	// Verify URL path
	wantPath := "/api/internal/v1/gateways/gw-1/manifest"
	if capturedReq.URL.Path != wantPath {
		t.Errorf("request path = %q, want %q", capturedReq.URL.Path, wantPath)
	}

	// Verify method
	if capturedReq.Method != http.MethodPost {
		t.Errorf("request method = %q, want POST", capturedReq.Method)
	}

	// Verify auth header
	if capturedReq.Header.Get("api-key") != "test-token" {
		t.Errorf("api-key header = %q, want %q", capturedReq.Header.Get("api-key"), "test-token")
	}

	// Verify content-type
	if capturedReq.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", capturedReq.Header.Get("Content-Type"))
	}

	// Verify body
	var payload manifestPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if len(payload.Policies) != 1 || payload.Policies[0].Name != "rate-limit" {
		t.Errorf("body policies = %+v, want one policy named rate-limit", payload.Policies)
	}
}

// TestPushGatewayManifest_Success200 verifies that a manifest POST also succeeds
// when the control plane responds with 200 OK (in addition to 204).
func TestPushGatewayManifest_Success200(t *testing.T) {
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	err := client.pushGatewayManifest(testGatewayID, []models.PolicyDefinition{makePolicyDef("auth", "v1.0.0", "wso2")})
	if err != nil {
		t.Errorf("pushGatewayManifest() with 200 should succeed, got error: %v", err)
	}
}

// TestPushGatewayManifest_ErrorOnNon2xx verifies that pushGatewayManifest returns
// an error for any non-2xx HTTP response status from the control plane.
func TestPushGatewayManifest_ErrorOnNon2xx(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"500 internal server error", http.StatusInternalServerError},
		{"400 bad request", http.StatusBadRequest},
		{"401 unauthorized", http.StatusUnauthorized},
		{"404 not found", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("error body"))
			})

			err := client.pushGatewayManifest(testGatewayID, nil)
			if err == nil {
				t.Errorf("pushGatewayManifest() with status %d expected error, got nil", tc.status)
			}
		})
	}
}

// TestPushGatewayManifest_EmptyPolicies verifies that pushGatewayManifest succeeds
// and sends an empty policies array when no policies are provided.
func TestPushGatewayManifest_EmptyPolicies(t *testing.T) {
	var capturedBody []byte
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.pushGatewayManifest(testGatewayID, []models.PolicyDefinition{})
	if err != nil {
		t.Fatalf("pushGatewayManifest() with empty policies unexpected error: %v", err)
	}

	var payload manifestPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	if len(payload.Policies) != 0 {
		t.Errorf("expected empty policies array, got %d", len(payload.Policies))
	}
}

// TestPushGatewayManifest_ServerUnreachable verifies that pushGatewayManifest returns
// an error when the control plane host is unreachable.
func TestPushGatewayManifest_ServerUnreachable(t *testing.T) {
	client := createTestClientWithHost(t, "127.0.0.1:1")

	err := client.pushGatewayManifest(testGatewayID, nil)
	if err == nil {
		t.Error("pushGatewayManifest() to unreachable server expected error, got nil")
	}
}

// TestPushGatewayManifestOnConnect_ParamsOnlyForCustomerPolicies verifies that
// Parameters and SystemParameters are included in the manifest only for customer-managed
// policies, and are absent for wso2-managed policies.
func TestPushGatewayManifestOnConnect_ParamsOnlyForCustomerPolicies(t *testing.T) {
	var capturedBody []byte
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"rate-limit":    makePolicyDef("rate-limit", "v1.0.0", "wso2"),
		"custom-policy": makePolicyDef("custom-policy", "v1.0.0", "customer"),
	}

	client.pushGatewayManifestOnConnect(testGatewayID)

	var payload manifestPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	for _, p := range payload.Policies {
		if p.ManagedBy == "customer" {
			if p.Parameters == nil || p.SystemParameters == nil {
				t.Errorf("customer policy %q should have Parameters and SystemParameters set", p.Name)
			}
		} else {
			if p.Parameters != nil || p.SystemParameters != nil {
				t.Errorf("wso2 policy %q should NOT have Parameters or SystemParameters in the manifest", p.Name)
			}
		}
	}
}

// TestPushGatewayManifestOnConnect_SuccessFirstAttempt verifies that the manifest is
// sent exactly once when the control plane responds successfully on the first attempt.
func TestPushGatewayManifestOnConnect_SuccessFirstAttempt(t *testing.T) {
	var callCount int32
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusNoContent)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"rate-limit": makePolicyDef("rate-limit", "v1.0.0", "wso2"),
	}

	client.pushGatewayManifestOnConnect(testGatewayID)

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount)
	}
}

// TestPushGatewayManifestOnConnect_RetriesOnFailureThenSucceeds verifies that the
// manifest push is retried after failures and ultimately succeeds when the control
// plane returns a success response on a later attempt.
func TestPushGatewayManifestOnConnect_RetriesOnFailureThenSucceeds(t *testing.T) {
	var callCount int32
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"auth": makePolicyDef("auth", "v1.0.0", "wso2"),
	}

	// Override context to avoid the 2s/4s backoff delays in tests
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.ctx = ctx

	client.pushGatewayManifestOnConnect(testGatewayID)

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 HTTP calls (2 failures + 1 success), got %d", callCount)
	}
}

// TestPushGatewayManifestOnConnect_AllRetriesFail verifies that the manifest push
// exhausts all retry attempts without panicking when the control plane consistently
// returns an error response.
func TestPushGatewayManifestOnConnect_AllRetriesFail(t *testing.T) {
	var callCount int32
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"auth": makePolicyDef("auth", "v1.0.0", "wso2"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client.ctx = ctx
	client.pushGatewayManifestOnConnect(testGatewayID)

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

// TestPushGatewayManifestOnConnect_FilterSystemPolicies verifies that policies with
// the wso2_apip_sys_ prefix are excluded from the manifest payload, while non-system
// policies (both wso2 and customer) are included.
func TestPushGatewayManifestOnConnect_FilterSystemPolicies(t *testing.T) {
	var capturedBody []byte
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"wso2_apip_sys_auth": makePolicyDef("wso2_apip_sys_auth", "v1.0.0", "wso2"),
		"wso2_apip_sys_log":  makePolicyDef("wso2_apip_sys_log", "v1.0.0", "wso2"),
		"rate-limit":         makePolicyDef("rate-limit", "v1.0.0", "wso2"),
		"custom-policy":      makePolicyDef("custom-policy", "v1.0.0", "customer"),
	}

	client.pushGatewayManifestOnConnect(testGatewayID)

	var payload manifestPayload
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}

	if len(payload.Policies) != 2 {
		t.Errorf("expected 2 policies after filtering system policies, got %d", len(payload.Policies))
	}
	for _, p := range payload.Policies {
		if strings.HasPrefix(p.Name, "wso2_apip_sys_") {
			t.Errorf("system policy %q should have been filtered out", p.Name)
		}
	}
}

// TestPushGatewayManifestOnConnect_SkipsOnOnPrem verifies that no manifest is sent
// when the gateway is connected to an on-prem control plane (identified by a non-empty
// gatewayPath discovered from the well-known endpoint).
func TestPushGatewayManifestOnConnect_SkipsOnOnPrem(t *testing.T) {
	var callCount int32
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusNoContent)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"rate-limit": makePolicyDef("rate-limit", "v1.0.0", "wso2"),
	}

	// Simulate on-prem by setting a non-empty gatewayPath
	client.state.mu.Lock()
	client.gatewayPath = "/internal/data/v1"
	client.state.mu.Unlock()

	client.pushGatewayManifestOnConnect(testGatewayID)

	if atomic.LoadInt32(&callCount) != 0 {
		t.Errorf("expected no HTTP calls for on-prem, got %d", callCount)
	}
}

// TestPushGatewayManifestOnConnect_ContextCancelled verifies that the manifest push
// aborts its retry loop early when the context is cancelled, rather than waiting
// through all the attempts.
func TestPushGatewayManifestOnConnect_ContextCancelled(t *testing.T) {
	// Server that always fails so retries kick in
	client := newManifestTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	client.policyDefinitions = map[string]models.PolicyDefinition{
		"auth": makePolicyDef("auth", "v1.0.0", "wso2"),
	}

	// Cancel the context immediately to abort retries
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	client.ctx = ctx

	done := make(chan struct{})
	go func() {
		client.pushGatewayManifestOnConnect(testGatewayID)
		close(done)
	}()

	select {
	case <-done:
		// expected: returned without waiting through all retry backoffs
	case <-time.After(5 * time.Second):
		t.Error("pushGatewayManifestOnConnect did not return in a timely manner after the defined context cancellation")
	}
}
