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
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// createLLMDeletionTestClient creates a test client with llmDeploymentService wired up.
// The eventHub is set on the LLMDeploymentService so that DELETE events are published
// through the same mock hub that the test verifies.
func createLLMDeletionTestClient() (*Client, *storage.ConfigStore, *mockStorageForDeletion, *mockControlPlaneEventHub) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockControlPlaneEventHub{}

	routerConfig := &config.RouterConfig{ListenerPort: 8080}
	apiDeploymentService := utils.NewAPIDeploymentService(store, db, nil, nil, routerConfig, nil)
	llmService := utils.NewLLMDeploymentService(store, db, nil, nil, nil, apiDeploymentService, routerConfig, nil, nil)
	llmService.SetEventHub(hub, "test-gateway")

	client := &Client{
		logger:               logger,
		store:                store,
		db:                   db,
		eventHub:             hub,
		gatewayID:            "test-gateway",
		llmDeploymentService: llmService,
	}

	return client, store, db, hub
}

// --- LLM Provider Deletion Tests ---

func TestHandleLLMProviderDeletedEvent_FullDeletion(t *testing.T) {
	client, store, db, hub := createLLMDeletionTestClient()

	providerID := "test-provider-delete"
	config := &models.StoredConfig{
		UUID:         providerID,
		Handle:       "test-provider-handle",
		DisplayName:  "Test Provider",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProvider",
	}
	db.SaveConfig(config)
	store.Add(config)

	event := map[string]interface{}{
		"type": "llmprovider.deleted",
		"payload": map[string]interface{}{
			"providerId": providerID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-provider-delete",
	}

	client.handleLLMProviderDeletedEvent(event)

	// Verify DB deletion
	assert.Equal(t, 1, db.deleteCallCount, "DeleteConfig should be called once")
	assert.Equal(t, 1, db.removeKeyCallCount, "RemoveAPIKeysAPI should be called once")

	// Verify config removed from DB
	_, err := db.GetConfig(providerID)
	assert.True(t, storage.IsNotFoundError(err), "config should be removed from DB")

	// Verify eventHub event published
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeLLMProvider, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, providerID, hub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-provider-delete", hub.publishedEvents[0].event.EventID)
}

func TestHandleLLMProviderDeletedEvent_NotFound(t *testing.T) {
	client, _, _, hub := createLLMDeletionTestClient()

	event := map[string]interface{}{
		"type": "llmprovider.deleted",
		"payload": map[string]interface{}{
			"providerId": "non-existent-provider",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-provider-notfound",
	}

	client.handleLLMProviderDeletedEvent(event)

	// Not found — no events published, no errors
	assert.Empty(t, hub.publishedEvents, "no events should be published when provider not found")
}

func TestHandleLLMProviderDeletedEvent_DBOnlyConfig(t *testing.T) {
	client, _, db, hub := createLLMDeletionTestClient()

	providerID := "test-provider-db-only"
	config := &models.StoredConfig{
		UUID:         providerID,
		Handle:       "test-provider-db-handle",
		DisplayName:  "Test Provider DB Only",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProvider",
	}
	db.SaveConfig(config)

	event := map[string]interface{}{
		"type": "llmprovider.deleted",
		"payload": map[string]interface{}{
			"providerId": providerID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-provider-db",
	}

	client.handleLLMProviderDeletedEvent(event)

	assert.Equal(t, 1, db.deleteCallCount)
	assert.Equal(t, 1, db.removeKeyCallCount)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, providerID, hub.publishedEvents[0].event.EntityID)
}

func TestHandleLLMProviderDeletedEvent_InvalidPayload(t *testing.T) {
	client, _, _, hub := createLLMDeletionTestClient()

	tests := []struct {
		name  string
		event map[string]interface{}
	}{
		{
			name: "Empty provider ID",
			event: map[string]interface{}{
				"type": "llmprovider.deleted",
				"payload": map[string]interface{}{
					"providerId": "",
				},
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid",
			},
		},
		{
			name: "Missing provider ID",
			event: map[string]interface{}{
				"type":          "llmprovider.deleted",
				"payload":       map[string]interface{}{},
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub.publishedEvents = nil
			client.handleLLMProviderDeletedEvent(tt.event)
			assert.Empty(t, hub.publishedEvents, "no events should be published for invalid payload")
		})
	}
}

func TestHandleLLMProviderDeletedEvent_StorageErrors(t *testing.T) {
	client, store, db, hub := createLLMDeletionTestClient()

	providerID := "test-provider-error"
	cfg := &models.StoredConfig{
		UUID:         providerID,
		Handle:       "test-provider-error-handle",
		DisplayName:  "Test Provider Error",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProvider",
	}
	db.SaveConfig(cfg)
	store.Add(cfg)

	// Simulate DB errors — DeleteLLMProvider will fail on RemoveAPIKeysAPI
	db.removeKeyErr = errors.New("failed to remove keys")

	event := map[string]interface{}{
		"type": "llmprovider.deleted",
		"payload": map[string]interface{}{
			"providerId": providerID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-provider-error",
	}

	// DeleteLLMProvider returns an error when RemoveAPIKeysAPI fails
	client.handleLLMProviderDeletedEvent(event)

	// The handler should log the error; RemoveAPIKeysAPI was attempted
	assert.Equal(t, 1, db.removeKeyCallCount, "RemoveAPIKeysAPI should be attempted")
	// No events published because the service returned an error before reaching publish
	assert.Empty(t, hub.publishedEvents)
}

func TestHandleLLMProviderDeletedEvent_StorageLookupError(t *testing.T) {
	client, _, db, hub := createLLMDeletionTestClient()

	providerID := "test-provider-lookup-err"
	cfg := &models.StoredConfig{
		UUID:         providerID,
		Handle:       "test-provider-lookup-handle",
		DisplayName:  "Test Provider",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProvider",
	}
	db.SaveConfig(cfg)

	// Simulate a real storage error (not "not found")
	db.getErr = errors.New("database connection failed")

	event := map[string]interface{}{
		"type": "llmprovider.deleted",
		"payload": map[string]interface{}{
			"providerId": providerID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-provider-lookup-err",
	}

	client.handleLLMProviderDeletedEvent(event)

	// Should abort — no delete attempted, no events published
	assert.Equal(t, 0, db.deleteCallCount, "DeleteConfig should not be called on lookup error")
	assert.Empty(t, hub.publishedEvents, "no events should be published on lookup error")
}

// --- LLM Proxy Deletion Tests ---

func TestHandleLLMProxyDeletedEvent_FullDeletion(t *testing.T) {
	client, store, db, hub := createLLMDeletionTestClient()

	proxyID := "test-proxy-delete"
	config := &models.StoredConfig{
		UUID:         proxyID,
		Handle:       "test-proxy-handle",
		DisplayName:  "Test Proxy",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProxy",
	}
	db.SaveConfig(config)
	store.Add(config)

	event := map[string]interface{}{
		"type": "llmproxy.deleted",
		"payload": map[string]interface{}{
			"proxyId": proxyID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-proxy-delete",
	}

	client.handleLLMProxyDeletedEvent(event)

	assert.Equal(t, 1, db.deleteCallCount, "DeleteConfig should be called once")
	assert.Equal(t, 1, db.removeKeyCallCount, "RemoveAPIKeysAPI should be called once")

	_, err := db.GetConfig(proxyID)
	assert.True(t, storage.IsNotFoundError(err), "config should be removed from DB")

	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeLLMProxy, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "DELETE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, proxyID, hub.publishedEvents[0].event.EntityID)
	assert.Equal(t, "corr-proxy-delete", hub.publishedEvents[0].event.EventID)
}

func TestHandleLLMProxyDeletedEvent_NotFound(t *testing.T) {
	client, _, _, hub := createLLMDeletionTestClient()

	event := map[string]interface{}{
		"type": "llmproxy.deleted",
		"payload": map[string]interface{}{
			"proxyId": "non-existent-proxy",
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-proxy-notfound",
	}

	client.handleLLMProxyDeletedEvent(event)

	assert.Empty(t, hub.publishedEvents, "no events should be published when proxy not found")
}

func TestHandleLLMProxyDeletedEvent_DBOnlyConfig(t *testing.T) {
	client, _, db, hub := createLLMDeletionTestClient()

	proxyID := "test-proxy-db-only"
	config := &models.StoredConfig{
		UUID:         proxyID,
		Handle:       "test-proxy-db-handle",
		DisplayName:  "Test Proxy DB Only",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProxy",
	}
	db.SaveConfig(config)

	event := map[string]interface{}{
		"type": "llmproxy.deleted",
		"payload": map[string]interface{}{
			"proxyId": proxyID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-proxy-db",
	}

	client.handleLLMProxyDeletedEvent(event)

	assert.Equal(t, 1, db.deleteCallCount)
	assert.Equal(t, 1, db.removeKeyCallCount)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, proxyID, hub.publishedEvents[0].event.EntityID)
}

func TestHandleLLMProxyDeletedEvent_InvalidPayload(t *testing.T) {
	client, _, _, hub := createLLMDeletionTestClient()

	tests := []struct {
		name  string
		event map[string]interface{}
	}{
		{
			name: "Empty proxy ID",
			event: map[string]interface{}{
				"type": "llmproxy.deleted",
				"payload": map[string]interface{}{
					"proxyId": "",
				},
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid",
			},
		},
		{
			name: "Missing proxy ID",
			event: map[string]interface{}{
				"type":          "llmproxy.deleted",
				"payload":       map[string]interface{}{},
				"timestamp":     time.Now().Format(time.RFC3339),
				"correlationId": "corr-invalid-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub.publishedEvents = nil
			client.handleLLMProxyDeletedEvent(tt.event)
			assert.Empty(t, hub.publishedEvents, "no events should be published for invalid payload")
		})
	}
}

func TestHandleLLMProxyDeletedEvent_StorageErrors(t *testing.T) {
	client, store, db, hub := createLLMDeletionTestClient()

	proxyID := "test-proxy-error"
	cfg := &models.StoredConfig{
		UUID:         proxyID,
		Handle:       "test-proxy-error-handle",
		DisplayName:  "Test Proxy Error",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProxy",
	}
	db.SaveConfig(cfg)
	store.Add(cfg)

	db.removeKeyErr = errors.New("failed to remove keys")

	event := map[string]interface{}{
		"type": "llmproxy.deleted",
		"payload": map[string]interface{}{
			"proxyId": proxyID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-proxy-error",
	}

	client.handleLLMProxyDeletedEvent(event)

	assert.Equal(t, 1, db.removeKeyCallCount, "RemoveAPIKeysAPI should be attempted")
	assert.Empty(t, hub.publishedEvents)
}

func TestHandleLLMProxyDeletedEvent_StorageLookupError(t *testing.T) {
	client, _, db, hub := createLLMDeletionTestClient()

	proxyID := "test-proxy-lookup-err"
	cfg := &models.StoredConfig{
		UUID:         proxyID,
		Handle:       "test-proxy-lookup-handle",
		DisplayName:  "Test Proxy",
		Version:      "v1",
		DesiredState: models.StateDeployed,
		Origin:       models.OriginControlPlane,
		Kind:         "LlmProxy",
	}
	db.SaveConfig(cfg)

	db.getErr = errors.New("database connection failed")

	event := map[string]interface{}{
		"type": "llmproxy.deleted",
		"payload": map[string]interface{}{
			"proxyId": proxyID,
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-proxy-lookup-err",
	}

	client.handleLLMProxyDeletedEvent(event)

	assert.Equal(t, 0, db.deleteCallCount, "DeleteConfig should not be called on lookup error")
	assert.Empty(t, hub.publishedEvents, "no events should be published on lookup error")
}
