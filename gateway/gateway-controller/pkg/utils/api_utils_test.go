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
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		result, err := svc.FetchAPIDefinition("test-api")
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

func TestAPIUtilsService_NotifyAPIDeployment(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Helper function to create minimal test StoredConfig
	createTestStoredConfig := func() *models.StoredConfig {
		return &models.StoredConfig{
			ID:        "test-api",
			Kind:      "RestApi",
			Status:    models.StatusDeployed,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			// Configuration will be marshaled in the HTTP request body
			Configuration: api.APIConfiguration{},
		}
	}

	t.Run("Successful notification", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/apis/test-api/gateway-deployments", r.URL.Path)
			assert.Equal(t, "test-token", r.Header.Get("api-key"))
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			// Verify request body contains expected fields
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var notification APIDeploymentNotification
			err = json.Unmarshal(body, &notification)
			require.NoError(t, err)
			assert.Equal(t, "test-api", notification.ID)

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
		err := svc.NotifyAPIDeployment("test-api", createTestStoredConfig(), "")
		assert.NoError(t, err)
	})

	t.Run("With revision ID", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "/apis/test-api/gateway-deployments", r.URL.Path)
			assert.Contains(t, r.URL.RawQuery, "revisionId=rev-123")
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

		// Actually call the method with revision ID
		err := svc.NotifyAPIDeployment("test-api", createTestStoredConfig(), "rev-123")
		assert.NoError(t, err)
	})

	t.Run("HTTP error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		err := svc.NotifyAPIDeployment("test-api", createTestStoredConfig(), "")
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

// Test for JSON marshaling of APIDeploymentNotification
func TestAPIDeploymentNotification_JSON(t *testing.T) {
	now := time.Now()
	notification := APIDeploymentNotification{
		ID:                "test-id",
		Status:            "DEPLOYED",
		CreatedAt:         now,
		UpdatedAt:         now,
		DeployedAt:        &now,
		DeployedVersion:   1,
		ProjectIdentifier: "default",
	}

	data, err := json.Marshal(notification)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"id":"test-id"`)
	assert.Contains(t, string(data), `"status":"DEPLOYED"`)
}
