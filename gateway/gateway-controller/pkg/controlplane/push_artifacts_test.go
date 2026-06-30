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
	"archive/zip"
	"bytes"
	"encoding/json"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commonconstants "github.com/wso2/api-platform/common/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// decodePushedArtifactDPIDs parses a bulk import request (multipart "artifacts" zip whose
// artifacts.json holds the JSON array of pushed artifacts) and returns the dpids it carried.
func decodePushedArtifactDPIDs(t *testing.T, r *http.Request) []string {
	t.Helper()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse multipart: %v", err)
	}
	file, _, err := r.FormFile("artifacts")
	if err != nil {
		t.Fatalf("read artifacts file: %v", err)
	}
	defer file.Close()
	zipBytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "artifacts.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open artifacts.json: %v", err)
		}
		defer rc.Close()
		data, _ := io.ReadAll(rc)
		var reqs []struct {
			DPID string `json:"dpid"`
		}
		if err := json.Unmarshal(data, &reqs); err != nil {
			t.Fatalf("decode artifacts.json: %v", err)
		}
		dpids := make([]string, 0, len(reqs))
		for _, req := range reqs {
			dpids = append(dpids, req.DPID)
		}
		return dpids
	}
	t.Fatalf("multipart zip missing artifacts.json")
	return nil
}

// writeImportArtifactsResponse writes a success ImportArtifactsResponse marking every dpid as
// imported (id = "cp-"+dpid).
func writeImportArtifactsResponse(w http.ResponseWriter, dpids []string) {
	artifacts := make(map[string]map[string]string, len(dpids))
	for _, dpid := range dpids {
		artifacts[dpid] = map[string]string{"id": "cp-" + dpid, "status": "deployed"}
	}
	body, _ := json.Marshal(map[string]any{
		"total":     len(dpids),
		"success":   len(dpids),
		"failed":    0,
		"artifacts": artifacts,
	})
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// retryTestRestConfig is a minimal gateway-originated RestApi StoredConfig that carries the
// project-id annotation required for a project-scoped push to reach the control plane.
func retryTestRestConfig() *models.StoredConfig {
	cr := map[string]any{
		"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
		"kind":       models.KindRestApi,
		"metadata": map[string]any{
			"name":        "retry-api",
			"annotations": map[string]any{commonconstants.AnnotationProjectID: "default"},
		},
		"spec": map[string]any{"context": "/retry"},
	}
	return &models.StoredConfig{
		UUID:                "gw-retry-1",
		Kind:                models.KindRestApi,
		Handle:              "retry-api",
		DisplayName:         "Retry API",
		Version:             "v1.0",
		Origin:              models.OriginGatewayAPI,
		DesiredState:        models.StateDeployed,
		Configuration:       cr,
		SourceConfiguration: cr,
	}
}

// TestPushArtifactWithRetry_RetriesThenSucceeds verifies the DP->CP push retries a transient
// failure with backoff and succeeds once the control plane recovers.
func TestPushArtifactWithRetry_RetriesThenSucceeds(t *testing.T) {
	client := createTestClient(t)
	client.pushRetryBaseWait = time.Millisecond
	client.pushRetryMaxWait = 2 * time.Millisecond

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail the first two attempts, then succeed on the third.
		if atomic.AddInt32(&hits, 1) < 3 {
			http.Error(w, "transient", http.StatusInternalServerError)
			return
		}
		writeImportArtifactsResponse(w, decodePushedArtifactDPIDs(t, r))
	}))
	defer server.Close()
	client.apiUtilsService.SetBaseURL(server.URL)

	cfg := retryTestRestConfig()
	if _, err := client.pushArtifactWithRetry(cfg.UUID, cfg, ""); err != nil {
		t.Fatalf("pushArtifactWithRetry() error = %v, want nil after retrying to success", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("control plane hit %d times, want 3 (2 failures + 1 success)", got)
	}
}

// TestPushArtifactWithRetry_ExhaustsAttempts verifies the push is attempted exactly
// pushArtifactMaxAttempts (5) times and then returns the error when the control plane stays down.
func TestPushArtifactWithRetry_ExhaustsAttempts(t *testing.T) {
	client := createTestClient(t)
	client.pushRetryBaseWait = time.Millisecond
	client.pushRetryMaxWait = 2 * time.Millisecond

	var hits int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer server.Close()
	client.apiUtilsService.SetBaseURL(server.URL)

	cfg := retryTestRestConfig()
	if _, err := client.pushArtifactWithRetry(cfg.UUID, cfg, ""); err == nil {
		t.Fatal("pushArtifactWithRetry() error = nil, want error after exhausting all retries")
	}
	if got := atomic.LoadInt32(&hits); got != pushArtifactMaxAttempts {
		t.Errorf("control plane hit %d times, want %d (the default max attempts)", got, pushArtifactMaxAttempts)
	}
}

// TestPushGatewayArtifacts pushes only gateway-originated artifacts to the control
// plane via the generic import endpoint, skipping control-plane-originated ones.
func TestPushGatewayArtifacts(t *testing.T) {
	client := createTestClient(t)

	var (
		mu       sync.Mutex
		pushedID []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artifacts/import-gateway-artifacts" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		dpids := decodePushedArtifactDPIDs(t, r)
		mu.Lock()
		pushedID = append(pushedID, dpids...)
		mu.Unlock()
		writeImportArtifactsResponse(w, dpids)
	}))
	defer server.Close()

	client.apiUtilsService.SetBaseURL(server.URL)

	// Seed a pending gateway-originated artifact (should be pushed), a CP-originated artifact
	// (skipped: not gateway-originated), and an already-synced gateway artifact (skipped: only
	// cp_sync_status pending/failed is retried).
	if err := client.db.SaveConfig(&models.StoredConfig{
		UUID:         "gw-artifact-1",
		Kind:         models.KindRestApi,
		Handle:       "weather-api",
		DisplayName:  "Weather API",
		Version:      "v1.0",
		Origin:       models.OriginGatewayAPI,
		CPSyncStatus: models.CPSyncStatusPending,
		DesiredState: models.StateDeployed,
		// The configuration is the full CR; project-scoped kinds (RestApi) must
		// declare the project via the project-id metadata annotation or the push
		// is rejected before reaching the control plane.
		Configuration: map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
			"kind":       models.KindRestApi,
			"metadata": map[string]any{
				"name":        "weather-api",
				"annotations": map[string]any{commonconstants.AnnotationProjectID: "default"},
			},
			"spec": map[string]any{"context": "/weather"},
		},
		SourceConfiguration: map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
			"kind":       models.KindRestApi,
			"metadata": map[string]any{
				"name":        "weather-api",
				"annotations": map[string]any{commonconstants.AnnotationProjectID: "default"},
			},
			"spec": map[string]any{"context": "/weather"},
		},
	}); err != nil {
		t.Fatalf("seed gateway config: %v", err)
	}
	if err := client.db.SaveConfig(&models.StoredConfig{
		UUID:         "cp-artifact-1",
		Kind:         models.KindRestApi,
		Handle:       "cp-api",
		Origin:       models.OriginControlPlane,
		DesiredState: models.StateDeployed,
	}); err != nil {
		t.Fatalf("seed cp config: %v", err)
	}
	// Already-synced gateway artifact: must NOT be re-pushed on reconnect.
	if err := client.db.SaveConfig(&models.StoredConfig{
		UUID:         "gw-artifact-synced",
		Kind:         models.KindRestApi,
		Handle:       "synced-api",
		DisplayName:  "Synced API",
		Version:      "v1.0",
		Origin:       models.OriginGatewayAPI,
		CPSyncStatus: models.CPSyncStatusSuccess,
		DesiredState: models.StateDeployed,
		Configuration: map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
			"kind":       models.KindRestApi,
			"metadata": map[string]any{
				"name":        "synced-api",
				"annotations": map[string]any{commonconstants.AnnotationProjectID: "default"},
			},
			"spec": map[string]any{"context": "/synced"},
		},
		SourceConfiguration: map[string]any{
			"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
			"kind":       models.KindRestApi,
			"metadata": map[string]any{
				"name":        "synced-api",
				"annotations": map[string]any{commonconstants.AnnotationProjectID: "default"},
			},
			"spec": map[string]any{"context": "/synced"},
		},
	}); err != nil {
		t.Fatalf("seed synced gateway config: %v", err)
	}

	client.pushGatewayArtifacts()

	mu.Lock()
	defer mu.Unlock()
	if len(pushedID) != 1 {
		t.Fatalf("pushed %d artifacts, want 1 (only the gateway-originated one): %v", len(pushedID), pushedID)
	}
	if pushedID[0] != "gw-artifact-1" {
		t.Errorf("pushed artifact ID = %q, want gw-artifact-1", pushedID[0])
	}
}

// TestPushGatewayArtifacts_IncludesTemplate verifies that a pending gateway-originated LLM
// provider template is picked up by the connect/reconnect push and pushed to the control
// plane. Templates are organization-level, so this also confirms they are not rejected by the
// project-scoping check that applies to project-scoped kinds.
func TestPushGatewayArtifacts_IncludesTemplate(t *testing.T) {
	client := createTestClient(t)

	var (
		mu       sync.Mutex
		pushedID []string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/artifacts/import-gateway-artifacts" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		dpids := decodePushedArtifactDPIDs(t, r)
		mu.Lock()
		pushedID = append(pushedID, dpids...)
		mu.Unlock()
		writeImportArtifactsResponse(w, dpids)
	}))
	defer server.Close()

	client.apiUtilsService.SetBaseURL(server.URL)

	tmplCR := api.LLMProviderTemplate{
		ApiVersion: api.LLMProviderTemplateApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.LLMProviderTemplateKindLlmProviderTemplate,
		Metadata:   api.Metadata{Name: "openai-template"},
		Spec:       api.LLMProviderTemplateData{DisplayName: "OpenAI Template"},
	}
	if err := client.db.SaveConfig(&models.StoredConfig{
		UUID:                "gw-template-1",
		Kind:                models.KindLlmProviderTemplate,
		Handle:              "openai-template",
		DisplayName:         "openai-template",
		Origin:              models.OriginGatewayAPI,
		CPSyncStatus:        models.CPSyncStatusPending,
		DesiredState:        models.StateDeployed,
		Configuration:       tmplCR,
		SourceConfiguration: tmplCR,
	}); err != nil {
		t.Fatalf("seed template: %v", err)
	}

	client.pushGatewayArtifacts()

	mu.Lock()
	defer mu.Unlock()
	if len(pushedID) != 1 || pushedID[0] != "gw-template-1" {
		t.Fatalf("pushed %v, want [gw-template-1] (the pending template)", pushedID)
	}
}

// TestPushGatewayArtifactsToControlPlane_GatedOff verifies the push is a no-op when
// deployment_sync_enabled is false.
func TestPushGatewayArtifactsToControlPlane_GatedOff(t *testing.T) {
	client := createTestClient(t)
	client.config.DeploymentSyncEnabled = false

	hit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client.apiUtilsService.SetBaseURL(server.URL)

	restCR := map[string]any{
		"apiVersion": api.RestAPIApiVersionGatewayApiPlatformWso2Comv1,
		"kind":       models.KindRestApi,
		"metadata": map[string]any{
			"name": "petstore-api-v1.0",
			"annotations": map[string]any{
				commonconstants.AnnotationArtifactID: "019d953f-d386-7a64-4444-1869a28292e0",
				commonconstants.AnnotationProjectID:  "test-project",
			},
		},
		"spec": map[string]any{
			"displayName": "PetStore API test",
			"version":     "v1.0",
			"context":     "/petstoretest",
			"upstream": map[string]any{
				"main": map[string]any{"url": "http://petstore.swagger.io/v2"},
			},
			"policies": []map[string]any{
				{
					"name":    "api-key-auth",
					"version": "v1",
					"params":  map[string]any{"key": "X-API-Key", "in": "header"},
				},
				{
					"name":    "set-headers",
					"version": "v1",
					"params": map[string]any{
						"request": map[string]any{
							"headers": []map[string]any{{"name": "X-Client-Version", "value": "1.2.3"}},
						},
					},
				},
			},
			"operations": []map[string]any{
				{"method": "GET", "path": "/pet/{petId}"},
				{"method": "POST", "path": "/pet"},
				{"method": "PUT", "path": "/pet"},
				{"method": "DELETE", "path": "/pet/{petId}"},
				{"method": "GET", "path": "/store/inventory"},
				{"method": "POST", "path": "/store/order"},
			},
		},
	}

	now := time.Now()
	if err := client.db.SaveConfig(&models.StoredConfig{
		UUID:                "gw-artifact-2",
		Kind:                models.KindRestApi,
		Handle:              "petstore-api-v1.0",
		DisplayName:         "PetStore API test",
		Version:             "v1.0",
		Configuration:       restCR,
		SourceConfiguration: restCR,
		DesiredState:        models.StateDeployed,
		DeploymentID:        "deployment-petstore-1",
		Origin:              models.OriginGatewayAPI,
		CreatedAt:           now,
		UpdatedAt:           now,
		DeployedAt:          &now,
		CPSyncStatus:        models.CPSyncStatusPending,
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	client.PushGatewayArtifactsToControlPlane()

	if hit {
		t.Error("control plane was called despite deployment_sync_enabled=false")
	}
}
