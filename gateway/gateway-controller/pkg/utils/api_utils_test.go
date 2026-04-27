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

func TestAPIUtilsService_PushAPIDeployment(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Helper function to create minimal test StoredConfig
	createTestStoredConfig := func() *models.StoredConfig {
		return &models.StoredConfig{
			UUID:         "0000-test-api-0000-000000000000",
			Kind:         "RestApi",
			DesiredState: models.StateDeployed,
			Origin:       models.OriginGatewayAPI,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			// Configuration will be marshaled in the HTTP request body
			Configuration: api.RestAPI{},
		}
	}

	t.Run("Successful push", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/apis/0000-test-api-0000-000000000000/gateway-deployments", r.URL.Path)
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Verify request body contains expected fields
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var notification APIDeploymentPush
			err = json.Unmarshal(body, &notification)
			require.NoError(t, err)
			assert.Equal(t, "0000-test-api-0000-000000000000", notification.ID)

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"status": "deployed"}`))
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		}
		svc := NewAPIUtilsService(cfg, logger)

		// Actually call the method
		err := svc.PushAPIDeployment("0000-test-api-0000-000000000000", createTestStoredConfig(), "")
		assert.NoError(t, err)
	})

	t.Run("With deployment ID", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/apis/0000-test-api-0000-000000000000/gateway-deployments", r.URL.Path)
			assert.Contains(t, r.URL.RawQuery, "deploymentId=rev-123")
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		}
		svc := NewAPIUtilsService(cfg, logger)

		// Actually call the method with deployment ID
		err := svc.PushAPIDeployment("0000-test-api-0000-000000000000", createTestStoredConfig(), "rev-123")
		assert.NoError(t, err)
	})

	t.Run("HTTP error response", func(t *testing.T) {
		server := newHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		cfg := PlatformAPIConfig{
			BaseURL: server.URL,
			Token:   "test-token",
		}
		svc := NewAPIUtilsService(cfg, logger)

		// Should return error for non-success status
		err := svc.PushAPIDeployment("0000-test-api-0000-000000000000", createTestStoredConfig(), "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
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

// Test for JSON marshaling of APIDeploymentPush
func TestAPIDeploymentPush_JSON(t *testing.T) {
	now := time.Now()
	notification := APIDeploymentPush{
		ID:                "0000-test-id-0000-000000000000",
		Status:            "DEPLOYED",
		CreatedAt:         now,
		UpdatedAt:         now,
		DeployedAt:        &now,
		ProjectIdentifier: "default",
	}

	data, err := json.Marshal(notification)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"id":"0000-test-id-0000-000000000000"`)
	assert.Contains(t, string(data), `"status":"DEPLOYED"`)
}
