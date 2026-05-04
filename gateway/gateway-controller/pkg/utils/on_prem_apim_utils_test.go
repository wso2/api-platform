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

package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	management "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func ptrStr(s string) *string { return &s }

func buildStoredConfig(name, version, context, upstreamURL string, ops []management.Operation) *models.StoredConfig {
	return &models.StoredConfig{
		UUID:   name + "-uuid",
		Handle: name,
		Kind:   "RestApi",
		Origin: models.OriginGatewayAPI,
		Configuration: management.RestAPI{
			ApiVersion: management.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       management.RestAPIKindRestApi,
			Metadata: management.Metadata{
				Name: name,
			},
			Spec: management.APIConfigData{
				DisplayName: name,
				Version:     version,
				Context:     context,
				Upstream: struct {
					Main    management.Upstream  `json:"main" yaml:"main"`
					Sandbox *management.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
				}{
					Main: management.Upstream{Url: ptrStr(upstreamURL)},
				},
				Operations: ops,
			},
		},
	}
}

// readZipEntries returns the paths and contents of all entries in a zip buffer.
func readZipEntries(t *testing.T, buf *bytes.Buffer) map[string]string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	require.NoError(t, err, "zip buffer must be readable")

	entries := make(map[string]string, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		var content bytes.Buffer
		_, err = content.ReadFrom(rc)
		require.NoError(t, err)
		_ = rc.Close()
		entries[f.Name] = content.String()
	}
	return entries
}

// --- Tests ---

// TestExportAPIAsZip_ZipContainsThreeFiles verifies the produced zip has exactly the
// three required APIM entries (api.yaml, deployment_environments.yaml, Definitions/swagger.yaml).
func TestExportAPIAsZip_ZipContainsThreeFiles(t *testing.T) {
	api := buildStoredConfig("PetStore", "v1", "/petstore", "https://petstore.example.com",
		[]management.Operation{
			{Method: "GET", Path: "/pet/{petId}"},
		})

	buf, err := ExportAPIAsZip(api, "onprem-gw", "")
	require.NoError(t, err)
	require.NotNil(t, buf)

	entries := readZipEntries(t, buf)
	assert.Len(t, entries, 3, "zip should contain exactly 3 files")

	assert.Contains(t, entries, "PetStore-v1/api.yaml", "zip should contain api.yaml")
	assert.Contains(t, entries, "PetStore-v1/deployment_environments.yaml", "zip should contain deployment_environments.yaml")
	assert.Contains(t, entries, "PetStore-v1/Definitions/swagger.yaml", "zip should contain Definitions/swagger.yaml")
}

// TestExportAPIAsZip_APIYamlContent verifies that api.yaml carries the API name,
// version and the upstream URL in the endpoint configuration.
func TestExportAPIAsZip_APIYamlContent(t *testing.T) {
	api := buildStoredConfig("PetStore", "v1", "/petstore", "https://petstore.example.com",
		[]management.Operation{
			{Method: "GET", Path: "/pet/{petId}"},
		})

	buf, err := ExportAPIAsZip(api, "onprem-gw", "")
	require.NoError(t, err)

	entries := readZipEntries(t, buf)
	apiYaml := entries["PetStore-v1/api.yaml"]

	assert.Contains(t, apiYaml, "PetStore", "api.yaml should contain the API name")
	assert.Contains(t, apiYaml, "v1", "api.yaml should contain the API version")
	assert.Contains(t, apiYaml, "https://petstore.example.com", "api.yaml should contain the upstream URL")
}

// TestExportAPIAsZip_DeploymentEnvsYamlContent verifies that deployment_environments.yaml
// carries the gateway name supplied to ExportAPIAsZip.
func TestExportAPIAsZip_DeploymentEnvsYamlContent(t *testing.T) {
	api := buildStoredConfig("PetStore", "v1", "/petstore", "https://petstore.example.com", nil)

	buf, err := ExportAPIAsZip(api, "onprem-gw", "")
	require.NoError(t, err)

	entries := readZipEntries(t, buf)
	deployYaml := entries["PetStore-v1/deployment_environments.yaml"]

	assert.Contains(t, deployYaml, "onprem-gw", "deployment_environments.yaml should contain the gateway name")
	assert.Contains(t, deployYaml, "deploymentEnvironment", "deployment_environments.yaml should have required field")
}

// TestExportAPIAsZip_SwaggerYamlContent verifies that Definitions/swagger.yaml carries the
// API operations derived from the stored configuration.
func TestExportAPIAsZip_SwaggerYamlContent(t *testing.T) {
	api := buildStoredConfig("PetStore", "v1", "/petstore", "https://petstore.example.com",
		[]management.Operation{
			{Method: "GET", Path: "/pet/{petId}"},
			{Method: "POST", Path: "/pet"},
		})

	buf, err := ExportAPIAsZip(api, "onprem-gw", "")
	require.NoError(t, err)

	entries := readZipEntries(t, buf)
	swagger := entries["PetStore-v1/Definitions/swagger.yaml"]

	assert.Contains(t, swagger, "/pet", "swagger should contain the operation path")
	assert.Contains(t, swagger, "get", "swagger should contain GET operation")
	assert.Contains(t, swagger, "post", "swagger should contain POST operation")
}

// TestExportAPIAsZip_SwaggerOverrideUsed verifies that when swaggerOverride is provided
// it replaces the locally generated swagger in the zip.
func TestExportAPIAsZip_SwaggerOverrideUsed(t *testing.T) {
	api := buildStoredConfig("PetStore", "v1", "/petstore", "https://petstore.example.com", nil)
	override := "openapi: \"3.0.0\"\ninfo:\n  title: override\n  version: v9\n"

	buf, err := ExportAPIAsZip(api, "onprem-gw", override)
	require.NoError(t, err)

	entries := readZipEntries(t, buf)
	swagger := entries["PetStore-v1/Definitions/swagger.yaml"]

	assert.Equal(t, override, swagger, "swagger should be exactly the override content")
	assert.NotContains(t, swagger, "PetStore", "locally generated content should NOT appear when override is set")
}

// TestExportAPIAsZip_ZipPathUsesNameAndVersion verifies that the zip directory prefix
// is <apiName>-<apiVersion> for every entry.
func TestExportAPIAsZip_ZipPathUsesNameAndVersion(t *testing.T) {
	api := buildStoredConfig("MyAPI", "v2", "/myapi", "https://backend.example.com", nil)

	buf, err := ExportAPIAsZip(api, "gw", "")
	require.NoError(t, err)

	entries := readZipEntries(t, buf)
	for path := range entries {
		assert.True(t, strings.HasPrefix(path, "MyAPI-v2/"),
			"every entry must be under MyAPI-v2/, got: %s", path)
	}
}

// TestExportAPIAsZip_MissingUpstreamURL verifies that ExportAPIAsZip returns an error
// when no upstream URL is configured.
func TestExportAPIAsZip_MissingUpstreamURL(t *testing.T) {
	api := &models.StoredConfig{
		UUID:   "no-upstream-uuid",
		Handle: "NoUpstreamAPI",
		Kind:   "RestApi",
		Configuration: management.RestAPI{
			ApiVersion: management.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
			Kind:       management.RestAPIKindRestApi,
			Metadata:   management.Metadata{Name: "NoUpstreamAPI"},
			Spec: management.APIConfigData{
				DisplayName: "NoUpstreamAPI",
				Version:     "v1",
				Context:     "/no-upstream",
				// Upstream intentionally omitted
			},
		},
	}

	buf, err := ExportAPIAsZip(api, "gw", "")

	assert.Nil(t, buf, "result should be nil on error")
	assert.Error(t, err, "should return error when upstream URL is missing")
}

// TestExportAPIAsZip_InvalidConfiguration verifies that ExportAPIAsZip returns an error
func TestExportAPIAsZip_InvalidConfiguration(t *testing.T) {
	api := &models.StoredConfig{
		UUID:          "bad-config-uuid",
		Handle:        "BadAPI",
		Kind:          "RestApi",
		Configuration: "this is not a valid config type",
	}

	buf, err := ExportAPIAsZip(api, "gw", "")

	assert.Nil(t, buf)
	assert.Error(t, err, "should return error for unsupported configuration type")
}

// buildImportTestServers starts a TLS token server and a TLS import server for testing
// ImportAPIToAPIMWithConfig. Both servers are closed automatically via t.Cleanup.
// tokenBody is the raw response body the token server will return (status 200).
// importHandler handles the import request and is responsible for writing the response.
func buildImportTestServers(t *testing.T, tokenBody string, importHandler http.HandlerFunc) APIMConfig {
	t.Helper()

	tokenServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, tokenBody)
	}))
	t.Cleanup(tokenServer.Close)

	importServer := httptest.NewTLSServer(importHandler)
	t.Cleanup(importServer.Close)

	// Strip the "https://" prefix so Host contains only "host:port"
	importHost := strings.TrimPrefix(importServer.URL, "https://")

	return APIMConfig{
		Host:               importHost,
		TokenURL:           tokenServer.URL,
		ClientID:           "test-client-id",
		ClientSecret:       "test-client-secret",
		InsecureSkipVerify: true, // required for httptest TLS certificates
	}
}

// validTokenBody is a minimal OAuth2 token response accepted by APIMTokenService.
const validTokenBody = `{"access_token":"test-token","expires_in":3600}`

// TestImportAPIToAPIM_InvalidImportResponseJSON verifies that ImportAPIToAPIMWithConfig
// returns an error when the import endpoint responds with HTTP 200 but a non-JSON body.
func TestImportAPIToAPIM_InvalidImportResponseJSON(t *testing.T) {
	cfg := buildImportTestServers(t, validTokenBody, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not valid json")
	})

	zipBuf := &bytes.Buffer{}
	resp, err := ImportAPIToAPIMWithConfig(cfg, slog.Default(), "api.zip", zipBuf)

	assert.Nil(t, resp, "response should be nil when JSON parsing fails")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse import response",
		"error should indicate that the response body could not be parsed")
}

// TestImportAPIToAPIM_EmptyIDAndRevisionInResponse verifies that ImportAPIToAPIMWithConfig
// succeeds (no error) when the import endpoint returns HTTP 200 with {"id":"","revision":""}
func TestImportAPIToAPIM_EmptyIDAndRevisionInResponse(t *testing.T) {
	cfg := buildImportTestServers(t, validTokenBody, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"","revision":""}`)
	})

	zipBuf := &bytes.Buffer{}
	resp, err := ImportAPIToAPIMWithConfig(cfg, slog.Default(), "api.zip", zipBuf)

	require.NoError(t, err, "empty id/revision in a 200 response should not cause an error")
	require.NotNil(t, resp, "response struct must be available")

	assert.NotNil(t, &resp.ID, "ID field must be present on OnPremAPIMImportResponse")
	assert.NotNil(t, &resp.Revision, "Revision field must be present on OnPremAPIMImportResponse")
}
