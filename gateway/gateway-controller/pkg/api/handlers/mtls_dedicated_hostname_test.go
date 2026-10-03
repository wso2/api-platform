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

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const dedicatedHostnameMainMessage = "this gateway requires every mtls-auth API to have its own hostname " +
	"(an exact name or a leading *.); set vhosts.main"

// createDedicatedHostnameTestServer returns a test server whose deploy
// validator carries the mtls-auth validator the controller builds, with a
// client authority in the pool and HTTPS enabled.
func createDedicatedHostnameTestServer(t *testing.T, required bool) (*APIServer, *MockStorage) {
	t.Helper()
	return createDedicatedHostnameTestServerFor(t, required, config.ClientCertificateRequestMtlsHostnames)
}

// createDedicatedHostnameTestServerFor is createDedicatedHostnameTestServer
// with router.downstream_tls.client_certificate_request set to request.
func createDedicatedHostnameTestServerFor(t *testing.T, required bool, request string) (*APIServer, *MockStorage) {
	t.Helper()
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	attachTestEventHub(server, &mockEventHub{}, "test-gateway")
	server.routerConfig.HTTPSEnabled = true
	server.routerConfig.DownstreamTLS.MtlsRequiresDedicatedHostname = required
	server.routerConfig.DownstreamTLS.ClientCertificateRequest = request
	require.NoError(t, mockDB.SaveCertificate(&models.StoredCertificate{
		UUID: "partner-a", Name: "partner-a",
		Usage: models.CertificateUsageDownstream, Role: models.CertificateRoleClient,
	}))

	defs := map[string]models.PolicyDefinition{
		"mtls-auth|v1.0.0": {Name: config.MtlsAuthPolicyName, Version: "v1.0.0"},
	}
	mtlsAuthValidator := config.NewMtlsAuthValidator(mockDB, true, false, nil).
		WithVHosts(server.routerConfig.VHosts).
		WithDedicatedHostnameRequired(required).
		WithAllConnectionsAsked(server.routerConfig.DownstreamTLS.AsksAllConnections())
	validator, ok := server.validator.(*config.APIValidator)
	require.True(t, ok)
	validator.SetPolicyValidator(config.NewPolicyValidator(defs, mtlsAuthValidator))
	return server, mockDB
}

// mtlsAuthRestAPIBody builds a RestAPI body attaching mtls-auth, served on
// vhostsMain when it is not empty.
func mtlsAuthRestAPIBody(t *testing.T, handle, vhostsMain string) []byte {
	t.Helper()
	var cfg api.RestAPI
	require.NoError(t, json.Unmarshal(createTestRestAPIRequestBody(t, handle, handle, "v1.0.0", "/"+handle), &cfg))
	cfg.Spec.Policies = &[]api.Policy{{Name: "mtls-auth", Version: "v1"}}
	if vhostsMain != "" {
		cfg.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: vhostsMain}
	}
	body, err := json.Marshal(cfg)
	require.NoError(t, err)
	return body
}

func createMtlsAuthRestAPI(t *testing.T, server *APIServer, handle, vhostsMain string) *httptest.ResponseRecorder {
	t.Helper()
	w, r := createTestContextWithHeader("POST", "/rest-apis", mtlsAuthRestAPIBody(t, handle, vhostsMain),
		map[string]string{"Content-Type": "application/json"})
	server.CreateRestAPI(w, r)
	return w
}

func updateMtlsAuthRestAPI(t *testing.T, server *APIServer, mockDB *MockStorage, handle, vhostsMain string) *httptest.ResponseRecorder {
	t.Helper()
	existing := createTestStoredConfig("0000-"+handle+"-0000-000000000000", handle, "v1.0.0", "/"+handle)
	existing.Handle = handle
	require.NoError(t, mockDB.SaveConfig(existing))
	w, r := createTestContextWithHeader("PUT", "/rest-apis/"+handle, mtlsAuthRestAPIBody(t, handle, vhostsMain),
		map[string]string{"Content-Type": "application/json"})
	server.UpdateRestAPI(w, r, handle)
	return w
}

func assertDedicatedHostnameRefusal(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	entry := firstFieldError(t, w.Body.Bytes(), "spec.vhosts.main")
	assert.Equal(t, dedicatedHostnameMainMessage, entry["message"])
}

func TestCreateRestAPI_MtlsRequiresDedicatedHostname(t *testing.T) {
	t.Run("refused without its own hostname", func(t *testing.T) {
		server, _ := createDedicatedHostnameTestServer(t, true)
		assertDedicatedHostnameRefusal(t, createMtlsAuthRestAPI(t, server, "mtls-create", ""))
	})
	t.Run("accepted with its own hostname", func(t *testing.T) {
		server, _ := createDedicatedHostnameTestServer(t, true)
		w := createMtlsAuthRestAPI(t, server, "mtls-create", "pay.example.com")
		assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	})
	t.Run("accepted with the warning when the setting is off", func(t *testing.T) {
		server, _ := createDedicatedHostnameTestServer(t, false)
		w := createMtlsAuthRestAPI(t, server, "mtls-create", "")
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), config.WarningCodeMTLSHostnameNotScoped)
	})
}

func TestUpdateRestAPI_MtlsRequiresDedicatedHostname(t *testing.T) {
	t.Run("refused without its own hostname", func(t *testing.T) {
		server, mockDB := createDedicatedHostnameTestServer(t, true)
		assertDedicatedHostnameRefusal(t, updateMtlsAuthRestAPI(t, server, mockDB, "mtls-update", ""))
	})
	t.Run("accepted with its own hostname", func(t *testing.T) {
		server, mockDB := createDedicatedHostnameTestServer(t, true)
		w := updateMtlsAuthRestAPI(t, server, mockDB, "mtls-update", "pay.example.com")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), config.WarningCodeMTLSHostnameNotScoped)
	})
	t.Run("accepted with the warning when the setting is off", func(t *testing.T) {
		server, mockDB := createDedicatedHostnameTestServer(t, false)
		w := updateMtlsAuthRestAPI(t, server, mockDB, "mtls-update", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), config.WarningCodeMTLSHostnameNotScoped)
	})
}

// The refusal follows the rendered vhost, not the template as written.
func TestRestAPI_MtlsRequiresDedicatedHostname_TemplatedVhost(t *testing.T) {
	const templated = `{{ env "MTLS_DEDICATED_HOSTNAME_TEST_HOST" }}`

	t.Run("create rendering to an IP address is refused", func(t *testing.T) {
		t.Setenv("MTLS_DEDICATED_HOSTNAME_TEST_HOST", "10.0.0.5")
		server, _ := createDedicatedHostnameTestServer(t, true)
		assertDedicatedHostnameRefusal(t, createMtlsAuthRestAPI(t, server, "mtls-templated", templated))
	})
	t.Run("update rendering to an IP address is refused", func(t *testing.T) {
		t.Setenv("MTLS_DEDICATED_HOSTNAME_TEST_HOST", "10.0.0.5")
		server, mockDB := createDedicatedHostnameTestServer(t, true)
		assertDedicatedHostnameRefusal(t, updateMtlsAuthRestAPI(t, server, mockDB, "mtls-templated", templated))
	})
	t.Run("rendering to its own hostname is accepted", func(t *testing.T) {
		t.Setenv("MTLS_DEDICATED_HOSTNAME_TEST_HOST", "pay.example.com")
		server, mockDB := createDedicatedHostnameTestServer(t, true)
		w := updateMtlsAuthRestAPI(t, server, mockDB, "mtls-templated", templated)
		assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	})
}

// With all_connections the requirement has no effect: an API without its own
// hostname deploys, and carries no hostname warning.
func TestRestAPI_MtlsRequiresDedicatedHostname_AllConnections(t *testing.T) {
	t.Run("create accepted without its own hostname", func(t *testing.T) {
		server, _ := createDedicatedHostnameTestServerFor(t, true, config.ClientCertificateRequestAllConnections)
		w := createMtlsAuthRestAPI(t, server, "mtls-create", "")
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), config.WarningCodeMTLSHostnameNotScoped)
	})
	t.Run("update accepted without its own hostname", func(t *testing.T) {
		server, mockDB := createDedicatedHostnameTestServerFor(t, true, config.ClientCertificateRequestAllConnections)
		w := updateMtlsAuthRestAPI(t, server, mockDB, "mtls-update", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), config.WarningCodeMTLSHostnameNotScoped)
	})
}

// hostnameWarningFields updates an mtls-auth API over PUT with HTTPS enabled
// and returns the fields of the MTLS_HOSTNAME_NOT_SCOPED warnings in the
// response.
func hostnameWarningFields(t *testing.T, vhostsMain string) []string {
	t.Helper()
	return hostnameWarningFieldsFor(t, vhostsMain, config.ClientCertificateRequestMtlsHostnames)
}

// hostnameWarningFieldsFor is hostnameWarningFields with
// router.downstream_tls.client_certificate_request set to request.
func hostnameWarningFieldsFor(t *testing.T, vhostsMain, request string) []string {
	t.Helper()
	server := createTestAPIServer()
	mockDB := server.db.(*MockStorage)
	attachTestEventHub(server, &mockEventHub{}, "test-gateway")
	server.routerConfig.HTTPSEnabled = true
	server.routerConfig.DownstreamTLS.ClientCertificateRequest = request

	existing := createTestStoredConfig("0000-mtls-host-0000-000000000000", "mtls-host", "v1.0.0", "/mtls-host")
	existing.Handle = "mtls-host"
	require.NoError(t, mockDB.SaveConfig(existing))

	var cfg api.RestAPI
	require.NoError(t, json.Unmarshal(createTestRestAPIRequestBody(t, "mtls-host", "mtls-host", "v1.0.0", "/mtls-host"), &cfg))
	cfg.Spec.Policies = &[]api.Policy{{Name: "mtls-auth", Version: "v1"}}
	if vhostsMain != "" {
		cfg.Spec.Vhosts = &struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{Main: vhostsMain}
	}
	body, err := json.Marshal(cfg)
	require.NoError(t, err)

	w, r := createTestContextWithHeader("PUT", "/rest-apis/mtls-host", body, map[string]string{"Content-Type": "application/json"})
	server.UpdateRestAPI(w, r, "mtls-host")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp struct {
		Status struct {
			Warnings []api.Warning `json:"warnings"`
		} `json:"status"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	var fields []string
	for _, warning := range resp.Status.Warnings {
		if warning.Code != nil && string(*warning.Code) == config.WarningCodeMTLSHostnameNotScoped {
			require.NotNil(t, warning.Field)
			fields = append(fields, *warning.Field)
		}
	}
	return fields
}

func TestUpdateRestAPI_MTLSHostnameNotScopedWarning(t *testing.T) {
	t.Run("gateway default hostname", func(t *testing.T) {
		assert.Equal(t, []string{"spec.vhosts.main"}, hostnameWarningFields(t, ""))
	})
	t.Run("own hostname", func(t *testing.T) {
		assert.Empty(t, hostnameWarningFields(t, "pay.example.com"))
	})
}

// The warning follows the rendered vhost the HTTPS listener is built from,
// not the template as written.
func TestUpdateRestAPI_MTLSHostnameNotScopedWarning_TemplatedVhost(t *testing.T) {
	const templated = `{{ env "MTLS_HOSTNAME_TEST_HOST" }}`

	t.Run("renders to its own hostname", func(t *testing.T) {
		t.Setenv("MTLS_HOSTNAME_TEST_HOST", "pay.example.com")
		assert.Empty(t, hostnameWarningFields(t, templated))
	})
	t.Run("renders to an IP address", func(t *testing.T) {
		t.Setenv("MTLS_HOSTNAME_TEST_HOST", "10.0.0.5")
		assert.Equal(t, []string{"spec.vhosts.main"}, hostnameWarningFields(t, templated))
	})
}

// With all_connections every connection is asked, so no hostname is warned
// about.
func TestUpdateRestAPI_MTLSHostnameNotScopedWarning_AllConnections(t *testing.T) {
	for _, vhostsMain := range []string{"", "10.0.0.5", "pay.example.com"} {
		t.Run("vhosts.main="+vhostsMain, func(t *testing.T) {
			assert.Empty(t, hostnameWarningFieldsFor(t, vhostsMain, config.ClientCertificateRequestAllConnections))
		})
	}
}
