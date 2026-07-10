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

package utils

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	commonconstants "github.com/wso2/api-platform/common/constants"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func newHTTPTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprint(r)
			if strings.Contains(msg, "failed to listen on a port") || strings.Contains(msg, "bind: operation not permitted") {
				t.Skipf("skipping test: local listener unavailable in this environment: %v", r)
			}
			panic(r)
		}
	}()

	return httptest.NewServer(handler)
}

func TestNewAPIUtilsService(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("With default timeout", func(t *testing.T) {
		cfg := PlatformAPIConfig{
			BaseURL: "http://localhost:8080",
			Token:   "test-token",
		}
		svc := NewAPIUtilsService(cfg, logger)
		assert.NotNil(t, svc)
		assert.Equal(t, 30*time.Second, svc.config.Timeout)
	})

	t.Run("With custom timeout", func(t *testing.T) {
		cfg := PlatformAPIConfig{
			BaseURL: "http://localhost:8080",
			Token:   "test-token",
			Timeout: 60 * time.Second,
		}
		svc := NewAPIUtilsService(cfg, logger)
		assert.NotNil(t, svc)
		assert.Equal(t, 60*time.Second, svc.config.Timeout)
	})
}

func TestAPIUtilsService_FetchAPIDefinition(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Successful fetch", func(t *testing.T) {
		expectedData := []byte("test zip content")
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/apis/test-api-123", r.URL.Path)
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.Equal(t, "application/zip", r.Header.Get("Accept"))
			w.WriteHeader(http.StatusOK)
			w.Write(expectedData)
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchAPIDefinition("test-api-123")
		assert.NoError(t, err)
		assert.Equal(t, expectedData, result)
	})

	t.Run("Server returns error", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("API not found"))
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchAPIDefinition("nonexistent")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("Connection error", func(t *testing.T) {
		cfg := PlatformAPIConfig{
			BaseURL: "http://localhost:99999",
			Token:   "test-token",
			Timeout: 1 * time.Second,
		}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchAPIDefinition("0000-test-api-0000-000000000000")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAPIUtilsService_ExtractYAMLFromZip(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := PlatformAPIConfig{BaseURL: "http://localhost"}
	svc := NewAPIUtilsService(cfg, logger)

	t.Run("Extract YAML file", func(t *testing.T) {
		// Create a zip with a YAML file
		yamlContent := []byte("apiVersion: v1\nkind: API")
		zipData := createTestZip(t, map[string][]byte{
			"api.yaml": yamlContent,
		})

		result, err := svc.ExtractYAMLFromZip(zipData)
		assert.NoError(t, err)
		assert.Equal(t, yamlContent, result)
	})

	t.Run("Extract YML file", func(t *testing.T) {
		yamlContent := []byte("apiVersion: v1")
		zipData := createTestZip(t, map[string][]byte{
			"api.yml": yamlContent,
		})

		result, err := svc.ExtractYAMLFromZip(zipData)
		assert.NoError(t, err)
		assert.Equal(t, yamlContent, result)
	})

	t.Run("No YAML file in zip", func(t *testing.T) {
		zipData := createTestZip(t, map[string][]byte{
			"readme.txt": []byte("No YAML here"),
		})

		result, err := svc.ExtractYAMLFromZip(zipData)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "no YAML file found")
	})

	t.Run("Invalid zip data", func(t *testing.T) {
		result, err := svc.ExtractYAMLFromZip([]byte("not a zip"))
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAPIUtilsService_SaveAPIDefinition(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := PlatformAPIConfig{BaseURL: "http://localhost"}
	svc := NewAPIUtilsService(cfg, logger)

	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "api-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	t.Run("Save successfully", func(t *testing.T) {
		zipData := []byte("test zip content")
		err := svc.SaveAPIDefinition("test-api-123", zipData)
		assert.NoError(t, err)

		// Verify file was created
		savedPath := filepath.Join("data", "apis", "test-api-123.zip")
		savedData, err := os.ReadFile(savedPath)
		assert.NoError(t, err)
		assert.Equal(t, zipData, savedData)
	})
}

// readPushedArtifacts parses a bulk import request: the multipart "artifacts" zip part, whose
// artifacts.json entry holds the JSON array of ImportArtifactRequest.
func readPushedArtifacts(t *testing.T, r *http.Request) []ImportArtifactRequest {
	t.Helper()
	require.NoError(t, r.ParseMultipartForm(10<<20))
	file, _, err := r.FormFile("artifacts")
	require.NoError(t, err)
	defer file.Close()
	zipBytes, err := io.ReadAll(file)
	require.NoError(t, err)
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	require.NoError(t, err)
	for _, f := range zr.File {
		if f.Name != gatewayArtifactsZipEntry {
			continue
		}
		rc, err := f.Open()
		require.NoError(t, err)
		defer rc.Close()
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		var reqs []ImportArtifactRequest
		require.NoError(t, json.Unmarshal(data, &reqs))
		return reqs
	}
	t.Fatalf("multipart zip missing %s", gatewayArtifactsZipEntry)
	return nil
}

func TestAPIUtilsService_PushArtifact(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Helper function to create a minimal test StoredConfig whose Configuration is the
	// gateway artifact CR (apiVersion/kind/metadata/spec), as actually stored.
	createTestStoredConfig := func(kind string) *models.StoredConfig {
		md := api.Metadata{Name: "weather-api"}
		// A well-formed project-scoped CR carries the project as a metadata annotation.
		if kind != models.KindLlmProvider && kind != models.KindLlmProviderTemplate {
			md.Annotations = &map[string]string{commonconstants.AnnotationProjectID: "weather-project"}
		}
		return &models.StoredConfig{
			UUID:         "0000-test-api-0000-000000000000",
			Kind:         kind,
			Handle:       "weather-api",
			DisplayName:  "Weather API",
			Version:      "v1.0",
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Configuration: api.RestAPI{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.RestAPIKindRestApi,
				Metadata:   md,
				Spec:       api.APIConfigData{Version: "v1.0"},
			},
			SourceConfiguration: api.RestAPI{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.RestAPIKindRestApi,
				Metadata:   md,
				Spec:       api.APIConfigData{Version: "v1.0"},
			},
		}
	}

	t.Run("Successful push targets the bulk import endpoint with a zipped multipart body", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/artifacts/import-gateway-artifacts", r.URL.Path)
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"),
				"expected multipart/form-data, got %q", r.Header.Get("Content-Type"))

			reqs := readPushedArtifacts(t, r)
			require.Len(t, reqs, 1)
			req := reqs[0]
			assert.Equal(t, "0000-test-api-0000-000000000000", req.DPID)
			// configuration is the gateway artifact CR itself.
			assert.Equal(t, "RestApi", req.Configuration["kind"])
			md, _ := req.Configuration["metadata"].(map[string]interface{})
			assert.Equal(t, "weather-api", md["name"])
			// Project-scoped kind carries its project via the project-id annotation (from the CR; never defaulted).
			anns, _ := md["annotations"].(map[string]interface{})
			assert.Equal(t, "weather-project", anns[commonconstants.AnnotationProjectID])
			assert.Equal(t, "deployed", req.Status)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"total":1,"success":1,"failed":0,"artifacts":{"0000-test-api-0000-000000000000":{"id":"0000-test-api-0000-000000000000","origin":"gateway_api","status":"deployed"}}}`))
		}))
		defer server.Close()

		svc := NewAPIUtilsService(PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}, logger)
		cpArtifactID, err := svc.PushArtifact("0000-test-api-0000-000000000000", createTestStoredConfig("RestApi"), "")
		assert.NoError(t, err)
		// The CP-minted artifact UUID from the per-dpid result is returned to the caller.
		assert.Equal(t, "0000-test-api-0000-000000000000", cpArtifactID)
	})

	t.Run("Org-level kind omits project", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/artifacts/import-gateway-artifacts", r.URL.Path)
			reqs := readPushedArtifacts(t, r)
			require.Len(t, reqs, 1)
			req := reqs[0]
			// Organization-level kinds carry no project annotation.
			md, _ := req.Configuration["metadata"].(map[string]interface{})
			anns, _ := md["annotations"].(map[string]interface{})
			_, hasProject := anns[commonconstants.AnnotationProjectID]
			assert.False(t, hasProject, "org-level kind must not carry a project annotation")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"total":1,"success":1,"failed":0,"artifacts":{"0000-test-api-0000-000000000000":{"id":"cp-llm-1","origin":"gateway_api","status":"deployed"}}}`))
		}))
		defer server.Close()

		svc := NewAPIUtilsService(PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}, logger)
		_, err := svc.PushArtifact("0000-test-api-0000-000000000000", createTestStoredConfig("LlmProvider"), "")
		assert.NoError(t, err)
	})

	t.Run("Template push carries UpdatedAt as the deployedAt watermark", func(t *testing.T) {
		// Templates have no deployment lifecycle so DeployedAt is nil; the push must
		// still send a non-nil deployedAt (= UpdatedAt), otherwise the CP treats
		// every template UPDATE as stale (IsNewerDeployment(nil, …) == false) and
		// silently skips it.
		var got ImportArtifactRequest
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqs := readPushedArtifacts(t, r)
			require.Len(t, reqs, 1)
			got = reqs[0]
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"total":1,"success":1,"failed":0,"artifacts":{"0000-test-api-0000-000000000000":{"id":"cp-tmpl-1","origin":"gateway_api","status":"deployed"}}}`))
		}))
		defer server.Close()

		cfg := createTestStoredConfig(models.KindLlmProviderTemplate)
		cfg.DeployedAt = nil // templates never set DeployedAt

		svc := NewAPIUtilsService(PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}, logger)
		_, err := svc.PushArtifact(cfg.UUID, cfg, "")
		require.NoError(t, err)

		require.NotNil(t, got.DeployedAt, "template push must carry a deployedAt watermark")
		assert.True(t, got.DeployedAt.Equal(cfg.UpdatedAt.UTC()),
			"template deployedAt should equal UpdatedAt; got %v want %v", got.DeployedAt, cfg.UpdatedAt.UTC())
	})

	t.Run("HTTP error response", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		svc := NewAPIUtilsService(PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}, logger)
		_, err := svc.PushArtifact("0000-test-api-0000-000000000000", createTestStoredConfig("RestApi"), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("Project-scoped kind without a project annotation fails the push (no defaulting)", func(t *testing.T) {
		// Server must never be hit: the push fails before any HTTP request.
		hit := false
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := &models.StoredConfig{
			UUID:         "0000-test-api-0000-000000000000",
			Kind:         models.KindRestApi,
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			Configuration: api.RestAPI{
				ApiVersion: "gateway.api-platform.wso2.com/v1alpha1",
				Kind:       api.RestAPIKindRestApi,
				Metadata:   api.Metadata{Name: "no-project-api"}, // no project annotation
				Spec:       api.APIConfigData{Version: "v1.0"},
			},
		}

		svc := NewAPIUtilsService(PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}, logger)
		_, err := svc.PushArtifact(cfg.UUID, cfg, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "project")
		assert.False(t, hit, "no request should be sent when the project annotation is missing")
	})
}

func TestMapToStruct(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	t.Run("Successful conversion", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "test",
			"count": 42,
		}
		var out TestStruct
		err := MapToStruct(data, &out)
		assert.NoError(t, err)
		assert.Equal(t, "test", out.Name)
		assert.Equal(t, 42, out.Count)
	})

	t.Run("Type mismatch", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  123, // wrong type
			"count": "not a number",
		}
		var out TestStruct
		err := MapToStruct(data, &out)
		assert.Error(t, err)
	})

	t.Run("Empty map", func(t *testing.T) {
		data := map[string]interface{}{}
		var out TestStruct
		err := MapToStruct(data, &out)
		assert.NoError(t, err)
		assert.Equal(t, "", out.Name)
		assert.Equal(t, 0, out.Count)
	})

	t.Run("Nested struct", func(t *testing.T) {
		type Nested struct {
			Inner struct {
				Value string `json:"value"`
			} `json:"inner"`
		}
		data := map[string]interface{}{
			"inner": map[string]interface{}{
				"value": "nested_value",
			},
		}
		var out Nested
		err := MapToStruct(data, &out)
		assert.NoError(t, err)
		assert.Equal(t, "nested_value", out.Inner.Value)
	})

	t.Run("With extra fields", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "test",
			"count": 42,
			"extra": "ignored",
		}
		var out TestStruct
		err := MapToStruct(data, &out)
		assert.NoError(t, err)
		assert.Equal(t, "test", out.Name)
	})

	t.Run("Nil map", func(t *testing.T) {
		var out TestStruct
		err := MapToStruct(nil, &out)
		assert.NoError(t, err)
	})
}

// Helper to create a test zip file
func createTestZip(t *testing.T, files map[string][]byte) []byte {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for name, content := range files {
		writer, err := zipWriter.Create(name)
		require.NoError(t, err)
		_, err = writer.Write(content)
		require.NoError(t, err)
	}

	err := zipWriter.Close()
	require.NoError(t, err)

	return buf.Bytes()
}

func createTestTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0644,
		}
		err := tarWriter.WriteHeader(header)
		require.NoError(t, err)
		_, err = tarWriter.Write(content)
		require.NoError(t, err)
	}

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())
	return buf.Bytes()
}

func TestAPIUtilsService_FetchControlPlaneDeployments(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Full sync (no since parameter)", func(t *testing.T) {
		deployedAt := time.Date(2026, 3, 4, 10, 30, 0, 0, time.UTC)
		expectedResponse := models.ControlPlaneDeploymentsResponse{
			Deployments: []models.ControlPlaneDeployment{
				{
					ArtifactID:   "api-123",
					DeploymentID: "dep-789",
					Kind:         "RestApi",
					State:        "DEPLOYED",
					DeployedAt:   deployedAt,
				},
				{
					ArtifactID:   "llm-001",
					DeploymentID: "dep-111",
					Kind:         "LlmProvider",
					State:        "DEPLOYED",
					DeployedAt:   deployedAt,
				},
			},
		}

		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/deployments", r.URL.Path)
			assert.Empty(t, r.URL.Query().Get("since"))
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.Equal(t, "application/json", r.Header.Get("Accept"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(expectedResponse)
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchControlPlaneDeployments(nil)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "api-123", result[0].ArtifactID)
		assert.Equal(t, "dep-789", result[0].DeploymentID)
		assert.Equal(t, "RestApi", result[0].Kind)
		assert.Equal(t, "DEPLOYED", result[0].State)
	})

	t.Run("Incremental sync (with since parameter)", func(t *testing.T) {
		since := time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/deployments", r.URL.Path)
			sinceParam := r.URL.Query().Get("since")
			assert.NotEmpty(t, sinceParam)
			// Verify the since parameter is a valid RFC3339 timestamp
			parsedSince, err := time.Parse(time.RFC3339, sinceParam)
			assert.NoError(t, err)
			assert.True(t, parsedSince.Equal(since))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(models.ControlPlaneDeploymentsResponse{
				Deployments: []models.ControlPlaneDeployment{},
			})
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchControlPlaneDeployments(&since)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Server returns error", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchControlPlaneDeployments(nil)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("Connection error", func(t *testing.T) {
		cfg := PlatformAPIConfig{BaseURL: "http://localhost:99999", Token: "test-token", Timeout: 1 * time.Second}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.FetchControlPlaneDeployments(nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAPIUtilsService_BatchFetchDeployments(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("Successful batch fetch", func(t *testing.T) {
		expectedTarGz := createTestTarGz(t, map[string][]byte{
			"dep-789/api-abc.yaml": []byte("apiVersion: v1"),
		})

		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/deployments/fetch-batch", r.URL.Path)
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "application/x-tar+gzip", r.Header.Get("Accept"))

			// Verify request body
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var req models.BatchFetchRequest
			err = json.Unmarshal(body, &req)
			require.NoError(t, err)
			assert.Equal(t, []string{"dep-789", "dep-456"}, req.DeploymentIDs)

			w.Header().Set("Content-Type", "application/gzip")
			w.WriteHeader(http.StatusOK)
			w.Write(expectedTarGz)
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.BatchFetchDeployments([]string{"dep-789", "dep-456"})
		assert.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("Server returns error", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request"))
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}
		svc := NewAPIUtilsService(cfg, logger)

		result, err := svc.BatchFetchDeployments([]string{"dep-789"})
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "400")
	})
}

func TestAPIUtilsService_FetchAPIKeysByKind_WebSubAPI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/websub-apis/api-keys", r.URL.Path)
		assert.Equal(t, "test-token", r.Header.Get("api-key"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode([]map[string]any{
			{
				"etag":         "etag-1",
				"uuid":         "key-1",
				"name":         "test-key",
				"maskedApiKey": "***key",
				"apiKeyHashes": map[string]string{"sha256": "abc123"},
				"artifactUuid": "api-1",
				"status":       "active",
				"createdAt":    time.Now().UTC(),
				"createdBy":    "test-user",
				"updatedAt":    time.Now().UTC(),
				"source":       "external",
			},
		})
		require.NoError(t, err)
	}))
	defer server.Close()

	cfg := PlatformAPIConfig{BaseURL: server.URL, Token: "test-token"}
	svc := NewAPIUtilsService(cfg, logger)

	keys, err := svc.FetchAPIKeysByKind(models.KindWebSubApi, "")
	require.NoError(t, err)
	if assert.Len(t, keys, 1) {
		assert.Equal(t, "key-1", keys[0].UUID)
		assert.Equal(t, "api-1", keys[0].ArtifactUUID)
		assert.Equal(t, "abc123", keys[0].APIKey)
	}
}

func TestAPIUtilsService_ExtractDeploymentsFromBatchZip(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := PlatformAPIConfig{BaseURL: "http://localhost"}
	svc := NewAPIUtilsService(cfg, logger)

	t.Run("Extract multiple deployments", func(t *testing.T) {
		yamlContent1 := []byte("apiVersion: v1\nkind: RestApi\nmetadata:\n  name: api-1")
		yamlContent2 := []byte("apiVersion: v1\nkind: LlmProvider\nmetadata:\n  name: provider-1")

		tarGzData := createTestTarGz(t, map[string][]byte{
			"dep-789/api-abc.yaml":          yamlContent1,
			"dep-456/llm-provider-xyz.yaml": yamlContent2,
		})

		result, err := svc.ExtractDeploymentsFromBatchZip(tarGzData)
		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, yamlContent1, result["dep-789"])
		assert.Equal(t, yamlContent2, result["dep-456"])
	})

	t.Run("Skip non-YAML files", func(t *testing.T) {
		tarGzData := createTestTarGz(t, map[string][]byte{
			"dep-789/api-abc.yaml": []byte("valid yaml"),
			"dep-789/readme.txt":   []byte("not yaml"),
		})

		result, err := svc.ExtractDeploymentsFromBatchZip(tarGzData)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, []byte("valid yaml"), result["dep-789"])
	})

	t.Run("Empty archive", func(t *testing.T) {
		tarGzData := createTestTarGz(t, map[string][]byte{})

		result, err := svc.ExtractDeploymentsFromBatchZip(tarGzData)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Invalid archive data", func(t *testing.T) {
		result, err := svc.ExtractDeploymentsFromBatchZip([]byte("not a tar.gz"))
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Path traversal entries are skipped", func(t *testing.T) {
		tarGzData := createTestTarGz(t, map[string][]byte{
			"../../../etc/malicious.yaml": []byte("malicious"),
			"dep-789/../dep-456/api.yaml": []byte("sneaky overwrite"),
			"dep-789/api-abc.yaml":        []byte("valid content"),
		})

		result, err := svc.ExtractDeploymentsFromBatchZip(tarGzData)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, []byte("valid content"), result["dep-789"])
		assert.NotContains(t, result, "dep-456", "internal ../ path should not create a dep-456 entry")
	})

	t.Run("Files at root level are skipped", func(t *testing.T) {
		tarGzData := createTestTarGz(t, map[string][]byte{
			"root-file.yaml": []byte("should be skipped"),
		})

		result, err := svc.ExtractDeploymentsFromBatchZip(tarGzData)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

// Test for JSON marshaling of the generic artifact import request body, where the
// configuration is the gateway artifact CR.
func TestImportArtifactRequest_JSON(t *testing.T) {
	now := time.Now()
	req := ImportArtifactRequest{
		DPID:   "0000-test-id-0000-000000000000",
		Status: "deployed",
		Configuration: map[string]interface{}{
			"apiVersion": "gateway.api-platform.wso2.com/v1alpha1",
			"kind":       "RestApi",
			"metadata": map[string]interface{}{
				"name":        "weather-api",
				"annotations": map[string]interface{}{commonconstants.AnnotationProjectID: "default"},
			},
		},
		CreatedAt:  now,
		UpdatedAt:  now,
		DeployedAt: &now,
	}

	data, err := json.Marshal(req)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"dpid":"0000-test-id-0000-000000000000"`)
	assert.Contains(t, string(data), `"status":"deployed"`)
	assert.Contains(t, string(data), `"kind":"RestApi"`)
	assert.Contains(t, string(data), `"name":"weather-api"`)
}
