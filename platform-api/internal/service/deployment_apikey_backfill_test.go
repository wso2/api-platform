/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package service

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// capturingEventHub records every published event so tests can assert what was broadcast.
type capturingEventHub struct {
	published []eventhub.Event
}

func (h *capturingEventHub) Initialize() error                    { return nil }
func (h *capturingEventHub) RegisterGateway(string) error         { return nil }
func (h *capturingEventHub) PublishEvent(_ string, e eventhub.Event) error {
	h.published = append(h.published, e)
	return nil
}
func (h *capturingEventHub) Subscribe(string) (<-chan eventhub.Event, error) { return nil, nil }
func (h *capturingEventHub) Unsubscribe(string, <-chan eventhub.Event) error { return nil }
func (h *capturingEventHub) UnsubscribeAll(string) error                     { return nil }
func (h *capturingEventHub) CleanUpEvents() error                            { return nil }
func (h *capturingEventHub) Close() error                                    { return nil }

// stubBackfillAPIKeyRepo returns a fixed key list from ListByArtifact.
type stubBackfillAPIKeyRepo struct {
	repository.APIKeyRepository
	keys []*model.APIKey
	err  error
}

func (r *stubBackfillAPIKeyRepo) ListByArtifact(string) ([]*model.APIKey, error) {
	return r.keys, r.err
}

// decodeKeyName extracts the API key name from a captured apikey.created event.
func decodeKeyName(t *testing.T, e eventhub.Event) string {
	t.Helper()
	var envelope dto.GatewayEventDTO
	if err := json.Unmarshal([]byte(e.EventData), &envelope); err != nil {
		t.Fatalf("failed to decode event envelope: %v", err)
	}
	if envelope.Type != EventTypeAPIKeyCreated {
		t.Fatalf("unexpected event type %q, want %q", envelope.Type, EventTypeAPIKeyCreated)
	}
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		t.Fatalf("failed to re-marshal payload: %v", err)
	}
	var key model.APIKeyCreatedEvent
	if err := json.Unmarshal(payloadBytes, &key); err != nil {
		t.Fatalf("failed to decode api key payload: %v", err)
	}
	return key.Name
}

func TestBackfillAPIKeysToGateway(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)

	keys := []*model.APIKey{
		{UUID: "k-active", ArtifactUUID: "api-1", Name: "active-key", Status: constants.APIKeyStatusActive},
		{UUID: "k-future", ArtifactUUID: "api-1", Name: "active-future-key", Status: constants.APIKeyStatusActive, ExpiresAt: &future},
		{UUID: "k-expired", ArtifactUUID: "api-1", Name: "expired-key", Status: constants.APIKeyStatusActive, ExpiresAt: &past},
		{UUID: "k-revoked", ArtifactUUID: "api-1", Name: "revoked-key", Status: "revoked"},
		nil,
	}

	hub := &capturingEventHub{}
	svc := &DeploymentService{
		apiKeyRepo:           &stubBackfillAPIKeyRepo{keys: keys},
		gatewayEventsService: NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger()),
		slogger:              newTestLogger(),
	}

	svc.backfillAPIKeysToGateway("api-1", "gw-B", "actor-1")

	// Only the two active, non-expired keys should be broadcast.
	if len(hub.published) != 2 {
		t.Fatalf("expected 2 broadcast events, got %d", len(hub.published))
	}

	got := map[string]bool{}
	for _, e := range hub.published {
		if e.GatewayID != "gw-B" {
			t.Errorf("event broadcast to gateway %q, want gw-B", e.GatewayID)
		}
		got[decodeKeyName(t, e)] = true
	}
	for _, want := range []string{"active-key", "active-future-key"} {
		if !got[want] {
			t.Errorf("expected key %q to be backfilled, but it was not", want)
		}
	}
	for _, notWant := range []string{"expired-key", "revoked-key"} {
		if got[notWant] {
			t.Errorf("key %q should not have been backfilled", notWant)
		}
	}
}

// TestBackfillAPIKeysToGateway_NoKeys ensures an empty artifact broadcasts nothing and does not panic.
func TestBackfillAPIKeysToGateway_NoKeys(t *testing.T) {
	hub := &capturingEventHub{}
	svc := &DeploymentService{
		apiKeyRepo:           &stubBackfillAPIKeyRepo{keys: nil},
		gatewayEventsService: NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger()),
		slogger:              newTestLogger(),
	}

	svc.backfillAPIKeysToGateway("api-1", "gw-B", "actor-1")

	if len(hub.published) != 0 {
		t.Fatalf("expected no broadcast events, got %d", len(hub.published))
	}
}

// TestBackfillAPIKeysToGateway_NilDeps is a no-op when dependencies are unset (e.g. events disabled).
func TestBackfillAPIKeysToGateway_NilDeps(t *testing.T) {
	svc := &DeploymentService{slogger: newTestLogger()}
	// Must not panic despite nil apiKeyRepo / gatewayEventsService.
	svc.backfillAPIKeysToGateway("api-1", "gw-B", "actor-1")
}

// TestBackfillAPIKeysToGateway_ExportedSharedByAllKinds exercises the exported helper
// directly — the single implementation the REST, LLM provider/proxy, MCP, WebSub and
// WebBroker deploy paths all delegate to. Also asserts nil-slogger safety (some call
// sites may pass a nil logger).
func TestBackfillAPIKeysToGateway_ExportedSharedByAllKinds(t *testing.T) {
	future := time.Now().Add(time.Hour)
	repo := &stubBackfillAPIKeyRepo{keys: []*model.APIKey{
		{UUID: "k1", ArtifactUUID: "artifact-1", Name: "k1", Status: constants.APIKeyStatusActive},
		{UUID: "k2", ArtifactUUID: "artifact-1", Name: "k2", Status: constants.APIKeyStatusActive, ExpiresAt: &future},
	}}
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())

	// nil slogger must not panic and must still broadcast.
	BackfillAPIKeysToGateway(repo, events, nil, "artifact-1", "gw-X", "")

	if len(hub.published) != 2 {
		t.Fatalf("expected 2 broadcast events, got %d", len(hub.published))
	}
	for _, e := range hub.published {
		if e.GatewayID != "gw-X" {
			t.Errorf("event broadcast to %q, want gw-X", e.GatewayID)
		}
	}
}

// TestBackfillAPIKeysToGateway_RepoError stops silently (best-effort) when the key
// lookup fails — the deployment must not be blocked by a backfill error.
func TestBackfillAPIKeysToGateway_RepoError(t *testing.T) {
	repo := &stubBackfillAPIKeyRepo{err: fmt.Errorf("db down")}
	hub := &capturingEventHub{}
	events := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())

	BackfillAPIKeysToGateway(repo, events, newTestLogger(), "artifact-1", "gw-X", "")

	if len(hub.published) != 0 {
		t.Fatalf("expected no broadcasts on repo error, got %d", len(hub.published))
	}
}
