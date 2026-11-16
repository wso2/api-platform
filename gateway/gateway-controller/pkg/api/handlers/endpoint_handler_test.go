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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/testutil/pki"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

const tenantAEndpointYAML = `apiVersion: gateway.api-platform.wso2.com/v1
kind: Endpoint
metadata:
  name: tenant-a
  labels:
    team: orders
spec:
  displayName: Orders backend for tenant A
  basePath: /v1
  upstreams:
    - url: http://orders-tenant-a:9080
  timeout:
    connect: 2s
`

// storeEndpointSyncer applies the mock database's Endpoints to the in-memory
// store, as the real publisher does.
type storeEndpointSyncer struct {
	server *APIServer
	db     *MockStorage
}

func (s storeEndpointSyncer) Sync(context.Context, string) error {
	wanted := map[string]bool{}
	for _, endpoint := range s.db.endpoints {
		wanted[endpoint.GetHandle()] = true
	}
	for _, existing := range s.server.store.GetAllEndpoints() {
		if !wanted[existing.GetHandle()] {
			s.server.store.DeleteEndpoint(existing.GetHandle())
		}
	}
	for _, endpoint := range s.db.endpoints {
		if err := s.server.store.SetEndpoint(endpoint); err != nil {
			return err
		}
	}
	return nil
}

// endpointRoutingTestDefs holds a routing policy shaped like dynamic-endpoint v1.1.
func endpointRoutingTestDefs() map[string]models.PolicyDefinition {
	return map[string]models.PolicyDefinition{
		"dynamic-endpoint|v1.1.0": {
			Name: "dynamic-endpoint", Version: "v1.1.0",
			Parameters: &map[string]interface{}{"properties": map[string]interface{}{
				"targetEndpoint": map[string]interface{}{"type": "string", config.EndpointRoutingAnnotation: config.EndpointRoutingEndpointName},
				"endpointHeader": map[string]interface{}{"type": "string", config.EndpointRoutingAnnotation: config.EndpointRoutingHeaderName},
			}},
		},
	}
}

// newEndpointTestServer returns an API server with the Endpoint service wired
// over its mock database, and the event hub the service publishes to.
func newEndpointTestServer(t *testing.T) (*APIServer, *MockStorage, *mockEventHub) {
	t.Helper()
	server := createTestAPIServer()
	db := server.db.(*MockStorage)
	hub := &mockEventHub{}
	server.SetEndpointService(utils.NewEndpointService(
		db, server.store, storeEndpointSyncer{server: server, db: db}, hub, "test-gateway",
		config.NewEndpointValidator(config.NewUpstreamTLSValidator(db, false)),
		config.NewEndpointRouting(endpointRoutingTestDefs()),
		server.logger,
	))
	return server, db, hub
}

func createTenantA(t *testing.T, server *APIServer) {
	t.Helper()
	w, r := createTestContextWithHeader(http.MethodPost, "/endpoints", []byte(tenantAEndpointYAML),
		map[string]string{"Content-Type": "application/yaml"})
	server.CreateEndpoint(w, r)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
}

func decodeJSONObject(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), "body: %s", w.Body.String())
	return body
}

func TestCreateEndpoint(t *testing.T) {
	server, db, hub := newEndpointTestServer(t)

	w, r := createTestContextWithHeader(http.MethodPost, "/endpoints", []byte(tenantAEndpointYAML),
		map[string]string{"Content-Type": "application/yaml"})
	r = withCorrelationID(r, "corr-create-endpoint")
	server.CreateEndpoint(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	body := decodeJSONObject(t, w)
	assert.Equal(t, "Endpoint", body["kind"])
	assert.Equal(t, "tenant-a", body["metadata"].(map[string]any)["name"])
	assert.Equal(t, "/v1", body["spec"].(map[string]any)["basePath"])
	status := body["status"].(map[string]any)
	assert.Equal(t, "tenant-a", status["id"])
	assert.NotEmpty(t, status["createdAt"])
	assert.NotContains(t, status, "state", "an Endpoint is not deployed or undeployed")

	require.Len(t, db.endpoints, 1)
	_, inStore := server.store.GetEndpoint("tenant-a")
	assert.True(t, inStore)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeEndpoint, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, "corr-create-endpoint", hub.publishedEvents[0].event.EventID)
}

func TestCreateEndpoint_Rejections(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"invalid name", strings.Replace(tenantAEndpointYAML, "name: tenant-a", "name: Tenant_A", 1), http.StatusBadRequest},
		{"url with a path", strings.Replace(tenantAEndpointYAML, ":9080", ":9080/api", 1), http.StatusBadRequest},
		{"not an Endpoint", strings.Replace(tenantAEndpointYAML, "kind: Endpoint", "kind: RestApi", 1), http.StatusBadRequest},
		{"unparseable", "::: not yaml", http.StatusBadRequest},
		{"oversized body", tenantAEndpointYAML + "# " + strings.Repeat("x", maxEndpointRequestBodyBytes), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server, db, hub := newEndpointTestServer(t)
			w, r := createTestContextWithHeader(http.MethodPost, "/endpoints", []byte(tc.body),
				map[string]string{"Content-Type": "application/yaml"})
			server.CreateEndpoint(w, r)

			assert.Equal(t, tc.want, w.Code, "body: %s", w.Body.String())
			assert.Equal(t, "error", decodeJSONObject(t, w)["status"])
			assert.Empty(t, db.endpoints)
			assert.Empty(t, hub.publishedEvents)
		})
	}
}

// The over-limit response must not echo the configured limit.
func TestCreateEndpoint_OversizedBodyResponseIsGeneric(t *testing.T) {
	server, _, _ := newEndpointTestServer(t)
	w, r := createTestContext(http.MethodPost, "/endpoints", []byte(strings.Repeat("x", maxEndpointRequestBodyBytes+1)))
	server.CreateEndpoint(w, r)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, "Request body is too large", decodeJSONObject(t, w)["message"])
}

func TestCreateEndpoint_DuplicateNameIsAConflict(t *testing.T) {
	server, db, _ := newEndpointTestServer(t)
	createTenantA(t, server)

	w, r := createTestContextWithHeader(http.MethodPost, "/endpoints", []byte(tenantAEndpointYAML),
		map[string]string{"Content-Type": "application/yaml"})
	server.CreateEndpoint(w, r)

	assert.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	assert.Len(t, db.endpoints, 1)
}

func TestGetAndListEndpoints(t *testing.T) {
	server, _, _ := newEndpointTestServer(t)

	w, r := createTestContext(http.MethodGet, "/endpoints", nil)
	server.ListEndpoints(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	assert.EqualValues(t, 0, decodeJSONObject(t, w)["count"])

	createTenantA(t, server)
	second := strings.Replace(tenantAEndpointYAML, "name: tenant-a", "name: tenant-0", 1)
	w, r = createTestContextWithHeader(http.MethodPost, "/endpoints", []byte(second), map[string]string{"Content-Type": "application/yaml"})
	server.CreateEndpoint(w, r)
	require.Equal(t, http.StatusCreated, w.Code)

	w, r = createTestContext(http.MethodGet, "/endpoints", nil)
	server.ListEndpoints(w, r)
	require.Equal(t, http.StatusOK, w.Code)
	list := decodeJSONObject(t, w)
	assert.EqualValues(t, 2, list["count"])
	items := list["endpoints"].([]any)
	var names []string
	for _, item := range items {
		names = append(names, item.(map[string]any)["metadata"].(map[string]any)["name"].(string))
	}
	assert.Equal(t, []string{"tenant-0", "tenant-a"}, names, "ordered by name")
	// Names and labels are what an API author reads this list for.
	assert.Equal(t, "orders", items[1].(map[string]any)["metadata"].(map[string]any)["labels"].(map[string]any)["team"])

	w, r = createTestContext(http.MethodGet, "/endpoints/tenant-a", nil)
	server.GetEndpointById(w, r, "tenant-a")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "tenant-a", decodeJSONObject(t, w)["status"].(map[string]any)["id"])

	w, r = createTestContext(http.MethodGet, "/endpoints/missing", nil)
	server.GetEndpointById(w, r, "missing")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateEndpoint(t *testing.T) {
	server, db, hub := newEndpointTestServer(t)
	createTenantA(t, server)

	moved := strings.Replace(tenantAEndpointYAML, "http://orders-tenant-a:9080", "http://orders-moved:9090", 1)
	w, r := createTestContextWithHeader(http.MethodPut, "/endpoints/tenant-a", []byte(moved), map[string]string{"Content-Type": "application/yaml"})
	server.UpdateEndpoint(w, r, "tenant-a")

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	stored, err := db.GetEndpointByHandle("tenant-a")
	require.NoError(t, err)
	assert.Equal(t, "http://orders-moved:9090", stored.Configuration.Spec.Upstreams[0].Url)
	require.Len(t, hub.publishedEvents, 2)
	assert.Equal(t, "UPDATE", hub.publishedEvents[1].event.Action)

	t.Run("unknown endpoint", func(t *testing.T) {
		w, r := createTestContextWithHeader(http.MethodPut, "/endpoints/missing", []byte(moved), map[string]string{"Content-Type": "application/yaml"})
		server.UpdateEndpoint(w, r, "missing")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
	t.Run("rename", func(t *testing.T) {
		renamed := strings.Replace(tenantAEndpointYAML, "name: tenant-a", "name: tenant-b", 1)
		w, r := createTestContextWithHeader(http.MethodPut, "/endpoints/tenant-a", []byte(renamed), map[string]string{"Content-Type": "application/yaml"})
		server.UpdateEndpoint(w, r, "tenant-a")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("invalid configuration", func(t *testing.T) {
		bad := strings.Replace(tenantAEndpointYAML, "connect: 2s", "connect: soon", 1)
		w, r := createTestContextWithHeader(http.MethodPut, "/endpoints/tenant-a", []byte(bad), map[string]string{"Content-Type": "application/yaml"})
		server.UpdateEndpoint(w, r, "tenant-a")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestDeleteEndpoint(t *testing.T) {
	server, db, hub := newEndpointTestServer(t)
	createTenantA(t, server)

	w, r := createTestContext(http.MethodDelete, "/endpoints/tenant-a", nil)
	server.DeleteEndpoint(w, r, "tenant-a")

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Equal(t, "tenant-a", decodeJSONObject(t, w)["id"])
	assert.Empty(t, db.endpoints)
	_, inStore := server.store.GetEndpoint("tenant-a")
	assert.False(t, inStore)
	assert.Equal(t, "DELETE", hub.publishedEvents[len(hub.publishedEvents)-1].event.Action)

	w, r = createTestContext(http.MethodDelete, "/endpoints/tenant-a", nil)
	server.DeleteEndpoint(w, r, "tenant-a")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Deleting an Endpoint that a deployed API names as its fixed target is
// refused with a 409 that names those APIs.
func TestDeleteEndpoint_FixedTargetOfADeployedAPIReturns409(t *testing.T) {
	server, db, hub := newEndpointTestServer(t)
	w, r := createTestContextWithHeader(http.MethodPost, "/endpoints",
		[]byte(strings.Replace(tenantAEndpointYAML, "name: tenant-a", "name: tenant-default", 1)),
		map[string]string{"Content-Type": "application/yaml"})
	server.CreateEndpoint(w, r)
	require.Equal(t, http.StatusCreated, w.Code)

	policies := []api.Policy{{Name: "dynamic-endpoint", Version: "v1", Params: &map[string]interface{}{
		"endpointHeader": "x-endpoint", "targetEndpoint": "tenant-default",
	}}}
	orders := createTestStoredConfig("orders-uuid", "Orders", "v1.0", "/orders")
	orders.Handle = "orders-v1.0"
	restAPI := orders.Configuration.(api.RestAPI)
	restAPI.Spec.Policies = &policies
	orders.Configuration, orders.SourceConfiguration = restAPI, restAPI
	orders.DesiredState = models.StateDeployed
	require.NoError(t, db.SaveConfig(orders))

	w, r = createTestContext(http.MethodDelete, "/endpoints/tenant-default", nil)
	server.DeleteEndpoint(w, r, "tenant-default")

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	body := decodeJSONObject(t, w)
	assert.Equal(t, "error", body["status"])
	assert.Equal(t, "Endpoint 'tenant-default' is the fixed target of deployed APIs: orders-v1.0", body["message"])

	require.Len(t, db.endpoints, 1, "the Endpoint must still exist")
	assert.Len(t, hub.publishedEvents, 1, "no DELETE event for a refused delete")
}

func TestEndpointOperationsWithoutAServiceReturn503(t *testing.T) {
	server := createTestAPIServer()
	calls := map[string]func(w http.ResponseWriter, r *http.Request){
		"create": server.CreateEndpoint,
		"list":   server.ListEndpoints,
		"get":    func(w http.ResponseWriter, r *http.Request) { server.GetEndpointById(w, r, "tenant-a") },
		"update": func(w http.ResponseWriter, r *http.Request) { server.UpdateEndpoint(w, r, "tenant-a") },
		"delete": func(w http.ResponseWriter, r *http.Request) { server.DeleteEndpoint(w, r, "tenant-a") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			w, r := createTestContext(http.MethodPost, "/endpoints", []byte(tenantAEndpointYAML))
			call(w, r)
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

// ─── Certificates an Endpoint's tls block names ─────────────────────────────

// storeEndpointWithTLS puts an https Endpoint naming identity and trustedCAs
// into the mock database.
func storeEndpointWithTLS(t *testing.T, db *MockStorage, name, identity string, trustedCAs ...string) {
	t.Helper()
	tls := map[string]interface{}{}
	if identity != "" {
		tls["identity"] = identity
	}
	if len(trustedCAs) > 0 {
		cas := make([]interface{}, 0, len(trustedCAs))
		for _, ca := range trustedCAs {
			cas = append(cas, ca)
		}
		tls["trustedCAs"] = cas
	}
	endpoint := &models.StoredEndpoint{
		UUID: "uuid-" + name,
		Configuration: api.EndpointConfiguration{
			ApiVersion: api.EndpointConfigurationApiVersionGatewayApiPlatformWso2Comv1,
			Kind:       api.EndpointConfigurationKindEndpoint,
			Metadata:   api.Metadata{Name: name},
			Spec:       api.EndpointConfigurationData{Tls: &tls},
		},
	}
	endpoint.Configuration.Spec.Upstreams = append(endpoint.Configuration.Spec.Upstreams, struct {
		Url    string `json:"url" yaml:"url"`
		Weight *int   `json:"weight,omitempty" yaml:"weight,omitempty"`
	}{Url: "https://" + name + ".backend:9443"})
	require.NoError(t, db.SaveEndpoint(endpoint))
}

func TestDeleteCertificate_TrustedCANamedByAnEndpoint_Returns409(t *testing.T) {
	target := &models.StoredCertificate{
		UUID: "endpoint-ca-1", Name: "partner-ca", Usage: models.CertificateUsageUpstream,
		Certificate: pki.NewRootCA(t, "Partner CA").PEM(), NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	storeEndpointWithTLS(t, mockDB, "tenant-b", "", "other-ca", "partner-ca")
	storeEndpointWithTLS(t, mockDB, "tenant-a", "", "partner-ca")
	storeEndpointWithTLS(t, mockDB, "tenant-c", "", "other-ca")

	w := httptest.NewRecorder()
	newDeleteCertHandler(server, target.UUID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil))

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	resp := decodeJSONObject(t, w)
	assert.Equal(t, "certificate 'partner-ca' is named by 2 endpoints; remove those references first", resp["message"])
	errs := resp["errors"].([]any)
	require.Len(t, errs, 2)
	assert.Equal(t, map[string]any{"field": "spec.tls.trustedCAs[0]", "message": "referenced by Endpoint 'tenant-a'"}, errs[0])
	assert.Equal(t, map[string]any{"field": "spec.tls.trustedCAs[1]", "message": "referenced by Endpoint 'tenant-b'"}, errs[1])

	_, err := mockDB.GetCertificate(target.UUID)
	assert.NoError(t, err, "the delete never reached the database")
}

func TestDeleteCertificate_IdentityNamedByAnEndpointAndAnAPI_Returns409(t *testing.T) {
	identity := gatewayIdentityLeaf(t, "endpoint-identity")
	target := &models.StoredCertificate{
		UUID: "endpoint-identity-1", Name: "out-identity-a", Certificate: identity.PEM(),
		Usage: models.CertificateUsageIdentity, KeyAlgorithm: "ECDSA", NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	require.NoError(t, server.db.SaveConfig(restAPIConfigWithUpstreamTLS("out-partner-api", "out-identity-a")))
	storeEndpointWithTLS(t, mockDB, "tenant-a", "out-identity-a")

	w := httptest.NewRecorder()
	newDeleteCertHandler(server, target.UUID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil))

	require.Equal(t, http.StatusConflict, w.Code, "body: %s", w.Body.String())
	resp := decodeJSONObject(t, w)
	assert.Equal(t, "gateway identity 'out-identity-a' is named by 1 deployed API and 1 endpoint; remove those references first", resp["message"])
	errs := resp["errors"].([]any)
	require.Len(t, errs, 2)
	assert.Equal(t, "referenced by API 'out-partner-api'", errs[0].(map[string]any)["message"])
	assert.Equal(t, map[string]any{"field": "spec.tls.identity", "message": "referenced by Endpoint 'tenant-a'"}, errs[1])
}

func TestDeleteCertificate_NotNamedByAnyEndpointIsAllowed(t *testing.T) {
	target := &models.StoredCertificate{
		UUID: "free-ca-1", Name: "free-ca", Usage: models.CertificateUsageUpstream,
		Certificate: pki.NewRootCA(t, "Free CA").PEM(), NotAfter: time.Now().Add(365 * 24 * time.Hour),
	}
	mockDB := NewMockStorage()
	mockDB.certs = []*models.StoredCertificate{target, seedUpstreamCert(t)}
	server := createTestAPIServerWithCertStore(t, mockDB)
	storeEndpointWithTLS(t, mockDB, "tenant-a", "", "another-ca")

	w := httptest.NewRecorder()
	newDeleteCertHandler(server, target.UUID).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/certificates/"+target.UUID, nil))

	assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
}

func TestPluralTLSReferrers(t *testing.T) {
	assert.Equal(t, "1 deployed API", pluralTLSReferrers(1, 0))
	assert.Equal(t, "3 deployed APIs", pluralTLSReferrers(3, 0))
	assert.Equal(t, "1 endpoint", pluralTLSReferrers(0, 1))
	assert.Equal(t, "2 endpoints", pluralTLSReferrers(0, 2))
	assert.Equal(t, "2 deployed APIs and 1 endpoint", pluralTLSReferrers(2, 1))
}
