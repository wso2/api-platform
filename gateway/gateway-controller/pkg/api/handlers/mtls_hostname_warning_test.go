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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

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
