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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func boolPtr(b bool) *bool { return &b }

// --- REST API Undeploy Tests ---

func TestHandleAPIUndeployedEvent_Success(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "api-001",
		Handle:       "test-api",
		DisplayName:  "Test API",
		Version:      "1.0.0",
		Kind:         "RestApi",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-1",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	event := map[string]interface{}{
		"type": "api.undeployed",
		"payload": map[string]interface{}{
			"apiId":        "api-001",
			"deploymentId": "dep-1",
			"performedAt":  now.Add(1 * time.Minute).Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-api-001",
	}
	client.handleAPIUndeployedEvent(event)

	// Verify DB was updated with undeployed state
	updated := db.configs["api-001"]
	require.NotNil(t, updated)
	assert.Equal(t, models.StateUndeployed, updated.DesiredState)
	assert.Equal(t, "dep-1", updated.DeploymentID)

	// Verify event was published (event-driven mode)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeAPI, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, "api-001", hub.publishedEvents[0].event.EntityID)
}

func TestHandleAPIUndeployedEvent_Stale(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "api-002",
		Handle:       "test-api",
		DisplayName:  "Test API",
		Version:      "1.0.0",
		Kind:         "RestApi",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-2",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	// Simulate stale: UpsertConfig returns affected=false
	db.upsertAffected = boolPtr(false)

	event := map[string]interface{}{
		"type": "api.undeployed",
		"payload": map[string]interface{}{
			"apiId":        "api-002",
			"deploymentId": "dep-2",
			"performedAt":  now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-api-002",
	}
	client.handleAPIUndeployedEvent(event)

	// UpsertConfig was called but returned not-affected
	assert.Equal(t, 1, db.upsertCallCount)

	// No event published (stale — handler returns early)
	assert.Empty(t, hub.publishedEvents)
}

func TestHandleAPIUndeployedEvent_DeploymentIDMismatch(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "api-003",
		Handle:       "test-api",
		DisplayName:  "Test API",
		Version:      "1.0.0",
		Kind:         "RestApi",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-current",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	event := map[string]interface{}{
		"type": "api.undeployed",
		"payload": map[string]interface{}{
			"apiId":        "api-003",
			"deploymentId": "dep-old",
			"performedAt":  now.Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-api-003",
	}
	client.handleAPIUndeployedEvent(event)

	// UpsertConfig should NOT have been called (rejected before DB write)
	assert.Equal(t, 0, db.upsertCallCount)

	// No event published
	assert.Empty(t, hub.publishedEvents)

	// State unchanged
	assert.Equal(t, models.StateDeployed, db.configs["api-003"].DesiredState)
	assert.Equal(t, "dep-current", db.configs["api-003"].DeploymentID)
}

func TestHandleAPIUndeployedEvent_NotFound(t *testing.T) {
	client, _, _, hub := createDeletionTestClient()

	event := map[string]interface{}{
		"type": "api.undeployed",
		"payload": map[string]interface{}{
			"apiId":        "api-nonexistent",
			"deploymentId": "dep-1",
			"performedAt":  time.Now().Format(time.RFC3339Nano),
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-api-004",
	}
	client.handleAPIUndeployedEvent(event)

	// No event published (nothing to undeploy)
	assert.Empty(t, hub.publishedEvents)
}

// --- LLM Provider Undeploy Tests ---

func TestHandleLLMProviderUndeployedEvent_Success(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "provider-001",
		Handle:       "test-provider",
		DisplayName:  "Test Provider",
		Version:      "1.0.0",
		Kind:         "LlmProvider",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-1",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	event := map[string]interface{}{
		"type": "llmprovider.undeployed",
		"payload": map[string]interface{}{
			"providerId":   "provider-001",
			"deploymentId": "dep-1",
			"performedAt":  now.Add(1 * time.Minute).Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-001",
	}
	client.handleLLMProviderUndeployedEvent(event)

	// Verify DB was updated with undeployed state
	updated := db.configs["provider-001"]
	require.NotNil(t, updated)
	assert.Equal(t, models.StateUndeployed, updated.DesiredState)
	assert.Equal(t, "dep-1", updated.DeploymentID)

	// Verify event was published (event-driven mode)
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeLLMProvider, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, "provider-001", hub.publishedEvents[0].event.EntityID)
}

func TestHandleLLMProviderUndeployedEvent_Stale(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "provider-002",
		Handle:       "test-provider",
		DisplayName:  "Test Provider",
		Version:      "1.0.0",
		Kind:         "LlmProvider",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-2",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	// Simulate stale: UpsertConfig returns affected=false
	db.upsertAffected = boolPtr(false)

	event := map[string]interface{}{
		"type": "llmprovider.undeployed",
		"payload": map[string]interface{}{
			"providerId":   "provider-002",
			"deploymentId": "dep-2",
			"performedAt":  now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-002",
	}
	client.handleLLMProviderUndeployedEvent(event)

	// UpsertConfig was called but returned not-affected
	assert.Equal(t, 1, db.upsertCallCount)

	// No event published (stale — handler returns early)
	assert.Empty(t, hub.publishedEvents)
}

func TestHandleLLMProviderUndeployedEvent_DeploymentIDMismatch(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "provider-003",
		Handle:       "test-provider",
		DisplayName:  "Test Provider",
		Version:      "1.0.0",
		Kind:         "LlmProvider",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-current",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	event := map[string]interface{}{
		"type": "llmprovider.undeployed",
		"payload": map[string]interface{}{
			"providerId":   "provider-003",
			"deploymentId": "dep-old",
			"performedAt":  now.Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-003",
	}
	client.handleLLMProviderUndeployedEvent(event)

	// UpsertConfig should NOT have been called (rejected before DB write)
	assert.Equal(t, 0, db.upsertCallCount)

	// No event published
	assert.Empty(t, hub.publishedEvents)

	// State unchanged
	assert.Equal(t, models.StateDeployed, db.configs["provider-003"].DesiredState)
	assert.Equal(t, "dep-current", db.configs["provider-003"].DeploymentID)
}

func TestHandleLLMProviderUndeployedEvent_NotFound(t *testing.T) {
	client, _, _, hub := createDeletionTestClient()

	event := map[string]interface{}{
		"type": "llmprovider.undeployed",
		"payload": map[string]interface{}{
			"providerId":   "provider-nonexistent",
			"deploymentId": "dep-1",
			"performedAt":  time.Now().Format(time.RFC3339Nano),
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-004",
	}
	client.handleLLMProviderUndeployedEvent(event)

	// No event published (nothing to undeploy)
	assert.Empty(t, hub.publishedEvents)
}

// --- LLM Proxy Undeploy Tests ---

func TestHandleLLMProxyUndeployedEvent_Success(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "proxy-001",
		Handle:       "test-proxy",
		DisplayName:  "Test Proxy",
		Version:      "1.0.0",
		Kind:         "LlmProxy",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-1",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	event := map[string]interface{}{
		"type": "llmproxy.undeployed",
		"payload": map[string]interface{}{
			"proxyId":      "proxy-001",
			"deploymentId": "dep-1",
			"performedAt":  now.Add(1 * time.Minute).Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-101",
	}
	client.handleLLMProxyUndeployedEvent(event)

	// Verify DB was updated with undeployed state
	updated := db.configs["proxy-001"]
	require.NotNil(t, updated)
	assert.Equal(t, models.StateUndeployed, updated.DesiredState)
	assert.Equal(t, "dep-1", updated.DeploymentID)

	// Verify event was published
	require.Len(t, hub.publishedEvents, 1)
	assert.Equal(t, eventhub.EventTypeLLMProxy, hub.publishedEvents[0].event.EventType)
	assert.Equal(t, "UPDATE", hub.publishedEvents[0].event.Action)
	assert.Equal(t, "proxy-001", hub.publishedEvents[0].event.EntityID)
}

func TestHandleLLMProxyUndeployedEvent_Stale(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "proxy-002",
		Handle:       "test-proxy",
		DisplayName:  "Test Proxy",
		Version:      "1.0.0",
		Kind:         "LlmProxy",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-2",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	// Simulate stale
	db.upsertAffected = boolPtr(false)

	event := map[string]interface{}{
		"type": "llmproxy.undeployed",
		"payload": map[string]interface{}{
			"proxyId":      "proxy-002",
			"deploymentId": "dep-2",
			"performedAt":  now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-102",
	}
	client.handleLLMProxyUndeployedEvent(event)

	// UpsertConfig was called but returned not-affected
	assert.Equal(t, 1, db.upsertCallCount)

	// No event published (stale — handler returns early)
	assert.Empty(t, hub.publishedEvents)
}

func TestHandleLLMProxyUndeployedEvent_DeploymentIDMismatch(t *testing.T) {
	client, store, db, hub := createDeletionTestClient()

	now := time.Now()
	cfg := &models.StoredConfig{
		UUID:         "proxy-003",
		Handle:       "test-proxy",
		DisplayName:  "Test Proxy",
		Version:      "1.0.0",
		Kind:         "LlmProxy",
		DesiredState: models.StateDeployed,
		DeploymentID: "dep-current",
		DeployedAt:   &now,
		Origin:       models.OriginControlPlane,
	}
	db.configs[cfg.UUID] = cfg
	require.NoError(t, store.Add(cfg))

	event := map[string]interface{}{
		"type": "llmproxy.undeployed",
		"payload": map[string]interface{}{
			"proxyId":      "proxy-003",
			"deploymentId": "dep-old",
			"performedAt":  now.Format(time.RFC3339Nano),
		},
		"timestamp":     now.Format(time.RFC3339),
		"correlationId": "corr-103",
	}
	client.handleLLMProxyUndeployedEvent(event)

	// UpsertConfig should NOT have been called
	assert.Equal(t, 0, db.upsertCallCount)

	// No event published
	assert.Empty(t, hub.publishedEvents)

	// State unchanged
	assert.Equal(t, models.StateDeployed, db.configs["proxy-003"].DesiredState)
	assert.Equal(t, "dep-current", db.configs["proxy-003"].DeploymentID)
}

func TestHandleLLMProxyUndeployedEvent_NotFound(t *testing.T) {
	client, _, _, hub := createDeletionTestClient()

	event := map[string]interface{}{
		"type": "llmproxy.undeployed",
		"payload": map[string]interface{}{
			"proxyId":      "proxy-nonexistent",
			"deploymentId": "dep-1",
			"performedAt":  time.Now().Format(time.RFC3339Nano),
		},
		"timestamp":     time.Now().Format(time.RFC3339),
		"correlationId": "corr-104",
	}
	client.handleLLMProxyUndeployedEvent(event)

	// No event published
	assert.Empty(t, hub.publishedEvents)
}
