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
	"io"
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

type mockEventHubForUndeploy struct {
	publishedEvents []eventhub.Event
	publishErr      error
}

func (m *mockEventHubForUndeploy) Initialize() error {
	return nil
}

func (m *mockEventHubForUndeploy) RegisterGateway(gatewayID string) error {
	return nil
}

func (m *mockEventHubForUndeploy) PublishEvent(orgID string, event eventhub.Event) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.publishedEvents = append(m.publishedEvents, event)
	return nil
}

func (m *mockEventHubForUndeploy) Subscribe(orgID string) (<-chan eventhub.Event, error) {
	ch := make(chan eventhub.Event)
	return ch, nil
}

func (m *mockEventHubForUndeploy) Unsubscribe(orgID string, subscriber <-chan eventhub.Event) error {
	return nil
}

func (m *mockEventHubForUndeploy) UnsubscribeAll(orgID string) error {
	return nil
}

func (m *mockEventHubForUndeploy) CleanUpEvents() error {
	return nil
}

func (m *mockEventHubForUndeploy) Close() error {
	return nil
}

func TestClient_handleAPIUndeployedEvent_UpdatesDBAndPublishesEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockEventHubForUndeploy{}

	apiID := "api-undeploy-1"

	dbConfig := createTestAPIConfigForDeletion(apiID)
	dbConfig.Status = models.StatusDeployed
	db.configs[apiID] = dbConfig

	memConfig := createTestAPIConfigForDeletion(apiID)
	memConfig.Status = models.StatusDeployed
	if err := store.Add(memConfig); err != nil {
		t.Fatalf("failed to seed in-memory config: %v", err)
	}

	client := &Client{
		logger:       logger,
		store:        store,
		db:           db,
		eventHub:     hub,
		systemConfig: testControlplaneSystemConfig(),
	}

	event := map[string]interface{}{
		"type": "api.undeployed",
		"payload": map[string]interface{}{
			"apiId":       apiID,
			"environment": "production",
			"vhost":       "api.example.com",
		},
		"timestamp":     "2026-01-01T00:00:00Z",
		"correlationId": "corr-undeploy-1",
	}

	client.handleAPIUndeployedEvent(event)

	updatedConfig, err := db.GetConfig(apiID)
	if err != nil {
		t.Fatalf("expected API config in database: %v", err)
	}
	if updatedConfig.Status != models.StatusUndeployed {
		t.Fatalf("database status = %s, want %s", updatedConfig.Status, models.StatusUndeployed)
	}

	inMemoryConfig, err := store.Get(apiID)
	if err != nil {
		t.Fatalf("expected API config in memory: %v", err)
	}
	if inMemoryConfig.Status != models.StatusDeployed {
		t.Fatalf("in-memory status = %s, want %s", inMemoryConfig.Status, models.StatusDeployed)
	}

	if len(hub.publishedEvents) != 1 {
		t.Fatalf("published event count = %d, want 1", len(hub.publishedEvents))
	}

	published := hub.publishedEvents[0]
	if published.EventType != eventhub.EventTypeAPI {
		t.Fatalf("published event type = %s, want %s", published.EventType, eventhub.EventTypeAPI)
	}
	if published.Action != "UPDATE" {
		t.Fatalf("published action = %s, want UPDATE", published.Action)
	}
	if published.EntityID != apiID {
		t.Fatalf("published entity id = %s, want %s", published.EntityID, apiID)
	}
	if published.EventID != "corr-undeploy-1" {
		t.Fatalf("published event id = %s, want corr-undeploy-1", published.EventID)
	}
}

func TestClient_handleAPIUndeployedEvent_NotFoundDoesNotPublish(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := storage.NewConfigStore()
	db := newMockStorageForDeletion()
	hub := &mockEventHubForUndeploy{}

	client := &Client{
		logger:       logger,
		store:        store,
		db:           db,
		eventHub:     hub,
		systemConfig: testControlplaneSystemConfig(),
	}

	event := map[string]interface{}{
		"type": "api.undeployed",
		"payload": map[string]interface{}{
			"apiId": "missing-api",
		},
		"timestamp":     "2026-01-01T00:00:00Z",
		"correlationId": "corr-undeploy-missing",
	}

	client.handleAPIUndeployedEvent(event)

	if len(hub.publishedEvents) != 0 {
		t.Fatalf("published event count = %d, want 0", len(hub.publishedEvents))
	}
}
