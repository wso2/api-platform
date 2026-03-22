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

package eventlistener

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/eventhub"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/metrics"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

type mockEventHub struct {
	subscribeCh       chan eventhub.Event
	subscribeErr      error
	subscribedGateway []string
}

func (m *mockEventHub) Initialize() error {
	return nil
}

func (m *mockEventHub) RegisterGateway(string) error {
	return nil
}

func (m *mockEventHub) PublishEvent(string, eventhub.Event) error {
	return nil
}

func (m *mockEventHub) Subscribe(gatewayID string) (<-chan eventhub.Event, error) {
	m.subscribedGateway = append(m.subscribedGateway, gatewayID)
	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}
	return m.subscribeCh, nil
}

func (m *mockEventHub) Unsubscribe(string, <-chan eventhub.Event) error {
	return nil
}

func (m *mockEventHub) UnsubscribeAll(string) error {
	return nil
}

func (m *mockEventHub) CleanUpEvents() error {
	return nil
}

func (m *mockEventHub) Close() error {
	return nil
}

type mockAPIKeyXDSManager struct {
	storeCalls  []storeCall
	revokeCalls []revokeCall
	removeCalls []removeCall
}

type storeCall struct {
	apiID         string
	apiName       string
	apiVersion    string
	apiKeyID      string
	correlationID string
}

type revokeCall struct {
	apiID         string
	apiName       string
	apiVersion    string
	apiKeyName    string
	correlationID string
}

type removeCall struct {
	apiID         string
	apiName       string
	apiVersion    string
	correlationID string
}

func (m *mockAPIKeyXDSManager) StoreAPIKey(apiID, apiName, apiVersion string, apiKey *models.APIKey, correlationID string) error {
	m.storeCalls = append(m.storeCalls, storeCall{
		apiID:         apiID,
		apiName:       apiName,
		apiVersion:    apiVersion,
		apiKeyID:      apiKey.UUID,
		correlationID: correlationID,
	})
	return nil
}

func (m *mockAPIKeyXDSManager) RevokeAPIKey(apiID, apiName, apiVersion, apiKeyName, correlationID string) error {
	m.revokeCalls = append(m.revokeCalls, revokeCall{
		apiID:         apiID,
		apiName:       apiName,
		apiVersion:    apiVersion,
		apiKeyName:    apiKeyName,
		correlationID: correlationID,
	})
	return nil
}

func (m *mockAPIKeyXDSManager) RemoveAPIKeysByAPI(apiID, apiName, apiVersion, correlationID string) error {
	m.removeCalls = append(m.removeCalls, removeCall{
		apiID:         apiID,
		apiName:       apiName,
		apiVersion:    apiVersion,
		correlationID: correlationID,
	})
	return nil
}

func (m *mockAPIKeyXDSManager) RefreshSnapshot() error {
	return nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupSQLiteDBForEventListenerTests(t *testing.T) storage.Storage {
	t.Helper()

	metrics.Init()

	dbPath := filepath.Join(t.TempDir(), "eventlistener-test.db")
	db, err := storage.NewStorage(storage.BackendConfig{
		Type:       "sqlite",
		SQLitePath: dbPath,
		GatewayID:  "platform-gateway-id",
	}, newTestLogger())
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}

func testRestStoredConfig(uuid, handle, displayName, version string, status models.ConfigStatus) *models.StoredConfig {
	restAPI := api.RestAPI{
		ApiVersion: api.RestAPIApiVersionGatewayApiPlatformWso2Comv1alpha1,
		Kind:       api.RestApi,
		Metadata: api.Metadata{
			Name: handle,
		},
		Spec: api.APIConfigData{
			DisplayName: displayName,
			Version:     version,
			Context:     "/test",
			Upstream: struct {
				Main    api.Upstream  `json:"main" yaml:"main"`
				Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
			}{
				Main: api.Upstream{Url: stringPtr("https://example.com")},
			},
			Operations: []api.Operation{
				{
					Method: api.OperationMethodGET,
					Path:   "/",
				},
			},
		},
	}

	now := time.Now()
	return &models.StoredConfig{
		UUID:                uuid,
		Kind:                string(api.RestApi),
		Handle:              handle,
		DisplayName:         displayName,
		Version:             version,
		Configuration:       restAPI,
		SourceConfiguration: restAPI,
		Status:              status,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

func testRestStoredConfigWithPolicies(
	uuid, handle, displayName, version string,
	status models.ConfigStatus,
	policies []api.Policy,
) *models.StoredConfig {
	cfg := testRestStoredConfig(uuid, handle, displayName, version, status)
	restAPI := cfg.Configuration.(api.RestAPI)
	restAPI.Spec.Policies = &policies
	cfg.Configuration = restAPI
	cfg.SourceConfiguration = restAPI
	return cfg
}

func testAPIKey(uuid, name, displayName, artifactUUID string) *models.APIKey {
	now := time.Now()
	return &models.APIKey{
		UUID:         uuid,
		Name:         name,
		APIKey:       "hashed-" + uuid,
		MaskedAPIKey: "***" + name,
		ArtifactUUID: artifactUUID,
		Status:       models.APIKeyStatusActive,
		CreatedAt:    now,
		CreatedBy:    "test-user",
		UpdatedAt:    now,
		Source:       "external",
	}
}

func stringPtr(v string) *string {
	return &v
}

func TestStart_RequiresSystemConfig(t *testing.T) {
	listener := &EventListener{
		logger: newTestLogger(),
	}

	err := listener.Start()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "system configuration")
}

func TestStart_RequiresGatewayID(t *testing.T) {
	listener := &EventListener{
		eventHub: &mockEventHub{subscribeCh: make(chan eventhub.Event)},
		logger:   newTestLogger(),
		systemConfig: &config.Config{
			Controller: config.Controller{},
		},
	}

	err := listener.Start()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "controller.server.gateway_id")
}

func TestStart_SubscribesWithTrimmedGatewayID(t *testing.T) {
	hub := &mockEventHub{subscribeCh: make(chan eventhub.Event)}
	listener := NewEventListener(
		hub,
		storage.NewConfigStore(),
		nil,
		nil,
		nil,
		nil,
		nil,
		newTestLogger(),
		&config.Config{
			Controller: config.Controller{
				Server: config.ServerConfig{
					GatewayID: "  gateway-a  ",
				},
			},
		},
		nil,
	)

	require.NoError(t, listener.Start())
	assert.Equal(t, []string{"gateway-a"}, hub.subscribedGateway)

	listener.Stop()
}

func TestHandleEvent_AcceptsKnownNoopTypesAndUnknown(t *testing.T) {
	var logBuf bytes.Buffer
	listener := &EventListener{
		logger: slog.New(slog.NewTextHandler(&logBuf, nil)),
	}

	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeCertificate,
		EntityID:  "cert-1",
	})
	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventTypeLLMTemplate,
		EntityID:  "tmpl-1",
	})
	listener.handleEvent(eventhub.Event{
		EventType: eventhub.EventType("UNKNOWN"),
		EntityID:  "mystery-1",
	})

	logs := logBuf.String()
	assert.Contains(t, logs, "Certificate event received")
	assert.Contains(t, logs, "LLM template event received")
	assert.Contains(t, logs, "Unknown event type received")
}

func TestProcessEvents_RecoversFromPanicAndContinues(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eventCh := make(chan eventhub.Event, 2)
	listener := &EventListener{
		logger:  logger,
		eventCh: eventCh,
		ctx:     ctx,
		cancel:  cancel,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		listener.processEvents()
	}()

	eventCh <- eventhub.Event{
		EventType: eventhub.EventTypeAPI,
		Action:    "DELETE",
		EntityID:  "panic-api-id",
		EventID:   "corr-panic",
	}
	eventCh <- eventhub.Event{
		EventType: eventhub.EventType("UNKNOWN"),
		Action:    "UPDATE",
		EntityID:  "safe-event-id",
		EventID:   "corr-safe",
	}
	close(eventCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for processEvents to exit")
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "Recovered from panic while processing event") {
		t.Fatalf("expected panic recovery log, got: %s", logs)
	}
	if !strings.Contains(logs, "Unknown event type received") {
		t.Fatalf("expected processing to continue after panic, got: %s", logs)
	}
}
