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
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/apikey"
	"github.com/wso2/api-platform/common/eventhub"
	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// seedGraphQLAPIForAPIKeyHandlerTests mirrors seedAPIForAPIKeyHandlerTests but
// stores a GraphQLApi-kind config instead of RestApi, since API key operations
// are dispatched by artifact kind (models.KindGraphQLApi).
func seedGraphQLAPIForAPIKeyHandlerTests(t *testing.T, server *APIServer, handle string) *models.StoredConfig {
	t.Helper()

	graphqlConfig := api.GraphQLAPI{
		ApiVersion: api.GraphQLAPIApiVersionGatewayApiPlatformWso2Comv1,
		Kind:       api.GraphQLAPIKindGraphQLApi,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.GraphQLAPIConfigData{
			DisplayName: "Test GraphQL API",
			Version:     "v1.0.0",
			Context:     "/test-graphql",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{
					Url: stringPtr("http://backend.example.com/graphql"),
				},
			},
		},
	}

	cfg := &models.StoredConfig{
		UUID:                "0000-test-api-id-0000-000000000000",
		Kind:                string(models.KindGraphQLApi),
		Handle:              handle,
		DisplayName:         graphqlConfig.Spec.DisplayName,
		Version:             graphqlConfig.Spec.Version,
		Configuration:       graphqlConfig,
		SourceConfiguration: graphqlConfig,
		DesiredState:        models.StateDeployed,
		Origin:              models.OriginGatewayAPI,
	}

	require.NoError(t, server.store.Add(cfg))
	require.NoError(t, server.db.SaveConfig(cfg))

	return cfg
}

// --- CreateGraphQLAPIKey ---

func TestCreateGraphQLAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"name": "test-key"}`)
	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.CreateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestCreateGraphQLAPIKeyInvalidBody(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys", []byte("invalid json {{{"), map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.CreateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateGraphQLAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	body := createTestAPIKeyRequestBody(t, "test-key", "Test Key", "external-key-123456789012345678901234567890123456")
	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	r = withCorrelationID(r, "corr-id-create-graphql-key")

	server.CreateGraphQLAPIKey(w, r, "test-handle")

	assert.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)

	createdKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, cfg.UUID, createdKey.ArtifactUUID)
	assert.Equal(t, "test-user", createdKey.CreatedBy)
	assert.Equal(t, string(api.External), createdKey.Source)

	assert.Equal(t, "test-gateway", mockHub.publishedEvents[0].gatewayID)
	assert.Equal(t, eventhub.EventTypeAPIKey, mockHub.publishedEvents[0].event.EventType)
	assert.Equal(t, "CREATE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, createdKey.UUID), mockHub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-id-create-graphql-key", mockHub.publishedEvents[0].event.EventID)
}

func TestCreateGraphQLAPIKeyDBError(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.saveErr = errors.New("db save error")

	body := createTestAPIKeyRequestBody(t, "test-key", "Test Key", "external-key-123456789012345678901234567890123456")
	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.CreateGraphQLAPIKey(w, r, "test-handle")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	_, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.Error(t, err)
}

// TestCreateGraphQLAPIKeyExpirationInPast_ReturnsBadRequest guards against a regression where
// an expiresIn duration that computes to a past timestamp — a client input error — was mapped
// to a generic 500 instead of 400; see the identical fix applied to REST's CreateAPIKey.
func TestCreateGraphQLAPIKeyExpirationInPast_ReturnsBadRequest(t *testing.T) {
	server := createTestAPIServer()
	seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")

	name := "test-key"
	request := api.APIKeyCreationRequest{
		Name: &name,
		ExpiresIn: &struct {
			Duration int                                     `json:"duration" yaml:"duration"`
			Unit     api.APIKeyCreationRequestExpiresInUnit `json:"unit" yaml:"unit"`
		}{Duration: -10, Unit: api.APIKeyCreationRequestExpiresInUnitSeconds},
	}
	body, err := json.Marshal(request)
	require.NoError(t, err)

	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{UserID: "test-user", Roles: []string{"admin"}})

	server.CreateGraphQLAPIKey(w, r, "test-handle")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "must be in the future")
}

func TestCreateGraphQLAPIKeyAPINotFound(t *testing.T) {
	server := createTestAPIServer()

	body := createTestAPIKeyRequestBody(t, "test-key", "Test Key", "external-key-123456789012345678901234567890123456")
	w, r := createTestContextWithHeader("POST", "/graphql-apis/nonexistent/api-keys", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.CreateGraphQLAPIKey(w, r, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- RevokeGraphQLAPIKey ---

func TestRevokeGraphQLAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContext("DELETE", "/graphql-apis/test-handle/api-keys/test-key", nil)
	server.RevokeGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRevokeGraphQLAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Test Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	w, r := createTestContext("DELETE", "/graphql-apis/test-handle/api-keys/test-key", nil)
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	r = withCorrelationID(r, "corr-id-revoke-graphql-key")

	server.RevokeGraphQLAPIKey(w, r, "test-handle", "test-key")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "DELETE", mockHub.publishedEvents[0].event.Action)
	assert.Equal(t, apikey.BuildAPIKeyEntityID(cfg.UUID, storeKey.UUID), mockHub.publishedEvents[0].event.EntityID)

	_, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.Error(t, err)
}

func TestRevokeGraphQLAPIKeyDBError(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.updateErr = errors.New("db update error")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Test Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	w, r := createTestContext("DELETE", "/graphql-apis/test-handle/api-keys/test-key", nil)
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.RevokeGraphQLAPIKey(w, r, "test-handle", "test-key")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	storedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, models.APIKeyStatusActive, storedKey.Status)
}

func TestRevokeGraphQLAPIKeyNotFound(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContext("DELETE", "/graphql-apis/test-handle/api-keys/nonexistent", nil)
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.RevokeGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
}

// --- RegenerateGraphQLAPIKey ---

func TestRegenerateGraphQLAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{}`)
	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys/test-key/regenerate", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.RegenerateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegenerateGraphQLAPIKeyInvalidBody(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys/test-key/regenerate", []byte("invalid {{{"), map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.RegenerateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegenerateGraphQLAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Test Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys/test-key/regenerate", []byte(`{}`), map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	r = withCorrelationID(r, "corr-id-regenerate-graphql-key")

	server.RegenerateGraphQLAPIKey(w, r, "test-handle", "test-key")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "corr-id-regenerate-graphql-key", mockHub.publishedEvents[0].event.EventID)

	regeneratedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.NotEqual(t, "apip_****old", regeneratedKey.MaskedAPIKey)
}

func TestRegenerateGraphQLAPIKeyNotFound(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContextWithHeader("POST", "/graphql-apis/test-handle/api-keys/nonexistent/regenerate", []byte(`{}`), map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.RegenerateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- UpdateGraphQLAPIKey ---

func TestUpdateGraphQLAPIKeyNoAuth(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"apiKey": "new-key-value"}`)
	w, r := createTestContextWithHeader("PUT", "/graphql-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	server.UpdateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUpdateGraphQLAPIKeyInvalidBody(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContextWithHeader("PUT", "/graphql-apis/test-handle/api-keys/test-key", []byte("invalid json {{{"), map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.UpdateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateGraphQLAPIKeyMissingAPIKey(t *testing.T) {
	server := createTestAPIServer()

	body := []byte(`{"description": "test"}`)
	w, r := createTestContextWithHeader("PUT", "/graphql-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	server.UpdateGraphQLAPIKey(w, r, "0000-test-handle-0000-000000000000", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "apiKey is required", response.Message)
}

func TestUpdateGraphQLAPIKeyWithDBAndEventHub(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Old Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	body := createTestAPIKeyRequestBody(t, "test-key", "Updated Key", "external-key-abcdef1234567890abcdef1234567890abcdef")
	w, r := createTestContextWithHeader("PUT", "/graphql-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})
	r = withCorrelationID(r, "corr-id-update-graphql-key")

	server.UpdateGraphQLAPIKey(w, r, "test-handle", "test-key")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, mockHub.publishedEvents, 1)
	assert.Equal(t, "UPDATE", mockHub.publishedEvents[0].event.Action)

	updatedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, models.APIKeyStatusActive, updatedKey.Status)
	assert.NotEqual(t, "apip_****old", updatedKey.MaskedAPIKey)
}

func TestUpdateGraphQLAPIKeyDBError(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")
	mockDB.updateErr = errors.New("db update error")

	storeKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Old Key", "test-user", "apip_****old")
	dbKey := *storeKey
	require.NoError(t, server.store.StoreAPIKey(storeKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	body := createTestAPIKeyRequestBody(t, "test-key", "Updated Key", "external-key-abcdef1234567890abcdef1234567890abcdef")
	w, r := createTestContextWithHeader("PUT", "/graphql-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.UpdateGraphQLAPIKey(w, r, "test-handle", "test-key")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Empty(t, mockHub.publishedEvents)

	storedKey, err := mockDB.GetAPIKeysByAPIAndName(cfg.UUID, "test-key")
	require.NoError(t, err)
	assert.Equal(t, "apip_****old", storedKey.MaskedAPIKey)
}

// TestUpdateGraphQLAPIKeyRejectsLocalKey guards the business rule surfaced live
// during manual verification: a locally-generated (non-external) key cannot be
// updated with a custom value — only regenerated. Confirms
// storage.IsOperationNotAllowedError is mapped to 400, matching REST's handling.
func TestUpdateGraphQLAPIKeyRejectsLocalKey(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)
	mockHub := &mockEventHub{}
	attachTestEventHub(server, mockHub, "test-gateway")

	// A locally-generated key has Source == "local", not "external".
	localKey := createStoredExternalAPIKey("0000-test-key-id-0000-000000000000", cfg.UUID, "test-key", "Local Key", "test-user", "apip_****local")
	localKey.Source = "local"
	dbKey := *localKey
	require.NoError(t, server.store.StoreAPIKey(localKey))
	require.NoError(t, mockDB.SaveAPIKey(&dbKey))

	body := createTestAPIKeyRequestBody(t, "test-key", "Updated Key", "external-key-abcdef1234567890abcdef1234567890abcdef")
	w, r := createTestContextWithHeader("PUT", "/graphql-apis/test-handle/api-keys/test-key", body, map[string]string{
		"Content-Type": "application/json",
	})
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.UpdateGraphQLAPIKey(w, r, "test-handle", "test-key")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, mockHub.publishedEvents)
}

// --- ListGraphQLAPIKeys ---

func TestListGraphQLAPIKeysNoAuth(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContext("GET", "/graphql-apis/test-handle/api-keys", nil)
	server.ListGraphQLAPIKeys(w, r, "0000-test-handle-0000-000000000000")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListGraphQLAPIKeysSuccess(t *testing.T) {
	server := createTestAPIServer()
	cfg := seedGraphQLAPIForAPIKeyHandlerTests(t, server, "test-handle")
	mockDB := server.db.(*MockStorage)

	key1 := createStoredExternalAPIKey("0000-key1-0000-000000000000", cfg.UUID, "key-1", "Key One", "test-user", "***key-1")
	key2 := createStoredExternalAPIKey("0000-key2-0000-000000000000", cfg.UUID, "key-2", "Key Two", "test-user", "***key-2")
	mockDB.apiKeys[key1.UUID] = key1
	mockDB.apiKeys[key2.UUID] = key2

	w, r := createTestContext("GET", "/graphql-apis/test-handle/api-keys", nil)
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.ListGraphQLAPIKeys(w, r, "test-handle")

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "success", response["status"])
}

func TestListGraphQLAPIKeysAPINotFound(t *testing.T) {
	server := createTestAPIServer()

	w, r := createTestContext("GET", "/graphql-apis/nonexistent/api-keys", nil)
	r = withAuthContext(r, commonmodels.AuthContext{
		UserID: "test-user",
		Roles:  []string{"admin"},
	})

	server.ListGraphQLAPIKeys(w, r, "nonexistent")

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response api.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
}
