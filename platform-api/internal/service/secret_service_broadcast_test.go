/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// Covers the platform-api-side publish path left uncovered when secret.updated /
// secret.deprecated broadcasting was added (see docs/specs/secrets-management-test-scenarios.csv
// rows 121-122, previously marked GAP): SecretService.Update / Delete calling
// broadcastSecretEvent -> GatewayEventsService -> EventHub once WithGatewayBroadcast is
// wired. The receiving side (gateway-controller's handleSecretUpdatedEvent /
// handleSecretDeprecatedEvent) already has full coverage in
// gateway/gateway-controller/pkg/controlplane/sync_secrets_test.go (rows 124-132).

import (
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/wso2/api-platform/common/eventhub"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// ---- fake EventHub -----------------------------------------------------------

// fakeEventHub is a minimal eventhub.EventHub double that records every published
// event per gateway. Only PublishEvent is exercised by GatewayEventsService's
// broadcastEvent path; the rest satisfy the interface as no-ops.
type fakeEventHub struct {
	publishFn func(gatewayID string, event eventhub.Event) error
	published map[string][]eventhub.Event
}

func newFakeEventHub() *fakeEventHub {
	return &fakeEventHub{published: make(map[string][]eventhub.Event)}
}

func (f *fakeEventHub) Initialize() error                { return nil }
func (f *fakeEventHub) RegisterGateway(_ string) error    { return nil }
func (f *fakeEventHub) UnsubscribeAll(_ string) error     { return nil }
func (f *fakeEventHub) CleanUpEvents() error              { return nil }
func (f *fakeEventHub) Close() error                      { return nil }
func (f *fakeEventHub) Subscribe(_ string) (<-chan eventhub.Event, error) {
	return nil, nil
}
func (f *fakeEventHub) Unsubscribe(_ string, _ <-chan eventhub.Event) error {
	return nil
}

func (f *fakeEventHub) PublishEvent(gatewayID string, event eventhub.Event) error {
	if f.publishFn != nil {
		if err := f.publishFn(gatewayID, event); err != nil {
			return err
		}
	}
	f.published[gatewayID] = append(f.published[gatewayID], event)
	return nil
}

// decodedPayload unmarshals the dto.GatewayEventDTO carried in a hub event's
// EventData, returning the event's Type string and its raw Payload for a
// caller-supplied struct to further unmarshal.
func decodedPayload(t *testing.T, evt eventhub.Event) (string, json.RawMessage) {
	t.Helper()
	var wrapper struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal([]byte(evt.EventData), &wrapper); err != nil {
		t.Fatalf("failed to unmarshal event data: %v", err)
	}
	return wrapper.Type, wrapper.Payload
}

// ---- fake gateway repo --------------------------------------------------------

// mockGatewayRepoForBroadcast embeds the full GatewayRepository interface (nil by
// default for everything) and overrides only GetByOrganizationID, the one method
// broadcastSecretEvent actually calls.
type mockGatewayRepoForBroadcast struct {
	repository.GatewayRepository
	getByOrgFn func(orgID string) ([]*model.Gateway, error)
}

func (m *mockGatewayRepoForBroadcast) GetByOrganizationID(orgID string) ([]*model.Gateway, error) {
	return m.getByOrgFn(orgID)
}

// ---- helpers -------------------------------------------------------------------

func newBroadcastWiredSecretService(repo *mockSecretRepo, gwRepo repository.GatewayRepository, hub eventhub.EventHub) *SecretService {
	events := NewGatewayEventsService(hub, nil, slog.Default())
	return NewSecretService(repo, &mockVault{}, newTestIdentityService()).
		WithGatewayBroadcast(gwRepo, events)
}

func twoGateways() *mockGatewayRepoForBroadcast {
	return &mockGatewayRepoForBroadcast{
		getByOrgFn: func(orgID string) ([]*model.Gateway, error) {
			return []*model.Gateway{{ID: "gw-a"}, {ID: "gw-b"}}, nil
		},
	}
}

// ---- Update -> secret.updated --------------------------------------------------

func TestSecretService_Update_BroadcastsSecretUpdatedEventToAllGateways(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["openai-key"] = &model.Secret{Handle: "openai-key", DisplayName: "OpenAI Key", Status: model.SecretStatusActive}

	hub := newFakeEventHub()
	svc := newBroadcastWiredSecretService(repo, twoGateways(), hub)

	if _, err := svc.Update("org1", "openai-key", "alice", &dto.UpdateSecretRequest{Value: "sk-rotated"}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	for _, gw := range []string{"gw-a", "gw-b"} {
		events := hub.published[gw]
		if len(events) != 1 {
			t.Fatalf("gateway %s: got %d published events, want 1", gw, len(events))
		}
		eventType, payload := decodedPayload(t, events[0])
		if eventType != EventTypeSecretUpdated {
			t.Errorf("gateway %s: event type = %q, want %q", gw, eventType, EventTypeSecretUpdated)
		}
		var updated model.SecretUpdatedEvent
		if err := json.Unmarshal(payload, &updated); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if updated.Handle != "openai-key" {
			t.Errorf("gateway %s: payload handle = %q, want %q", gw, updated.Handle, "openai-key")
		}
		if updated.Hash != repo.secrets["openai-key"].Hash {
			t.Errorf("gateway %s: payload hash = %q, want the freshly rotated hash %q", gw, updated.Hash, repo.secrets["openai-key"].Hash)
		}
		wantAction := "UPDATE"
		if events[0].Action != wantAction {
			t.Errorf("gateway %s: hub action = %q, want %q", gw, events[0].Action, wantAction)
		}
	}
}

// ---- Delete -> secret.deprecated ------------------------------------------------

func TestSecretService_Delete_BroadcastsSecretDeprecatedEventToAllGateways(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["unused-key"] = &model.Secret{Handle: "unused-key", Status: model.SecretStatusActive}

	hub := newFakeEventHub()
	svc := newBroadcastWiredSecretService(repo, twoGateways(), hub)

	if err := svc.Delete("org1", "unused-key", "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	for _, gw := range []string{"gw-a", "gw-b"} {
		events := hub.published[gw]
		if len(events) != 1 {
			t.Fatalf("gateway %s: got %d published events, want 1", gw, len(events))
		}
		eventType, payload := decodedPayload(t, events[0])
		if eventType != EventTypeSecretDeprecated {
			t.Errorf("gateway %s: event type = %q, want %q", gw, eventType, EventTypeSecretDeprecated)
		}
		var deprecated model.SecretDeprecatedEvent
		if err := json.Unmarshal(payload, &deprecated); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if deprecated.Handle != "unused-key" {
			t.Errorf("gateway %s: payload handle = %q, want %q", gw, deprecated.Handle, "unused-key")
		}
		wantAction := "DELETE"
		if events[0].Action != wantAction {
			t.Errorf("gateway %s: hub action = %q, want %q", gw, events[0].Action, wantAction)
		}
	}
}

// TestSecretService_Delete_BlockedWhenInUse_DoesNotBroadcast confirms the 409
// in-use path returns before broadcastSecretEvent is ever reached — a secret that
// was not actually deprecated must not tell any gateway to evict it.
func TestSecretService_Delete_BlockedWhenInUse_DoesNotBroadcast(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["in-use"] = &model.Secret{Handle: "in-use", Status: model.SecretStatusActive}
	repo.findRefsFn = func(orgID, handle string) ([]model.SecretReference, error) {
		return []model.SecretReference{{Handle: "my-api", Name: "My API", Type: "RestApi"}}, nil
	}

	hub := newFakeEventHub()
	svc := newBroadcastWiredSecretService(repo, twoGateways(), hub)

	err := svc.Delete("org1", "in-use", "alice")
	var inUseErr *SecretInUseError
	if !errors.As(err, &inUseErr) {
		t.Fatalf("expected SecretInUseError, got %T: %v", err, err)
	}

	if total := len(hub.published["gw-a"]) + len(hub.published["gw-b"]); total != 0 {
		t.Errorf("expected no events published when delete is blocked, got %d", total)
	}
}

// ---- Best-effort: broadcast failures never fail the caller's request -----------

func TestSecretService_Update_GatewayRepoError_DoesNotFailUpdate(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["k1"] = &model.Secret{Handle: "k1", Status: model.SecretStatusActive}

	gwRepo := &mockGatewayRepoForBroadcast{
		getByOrgFn: func(orgID string) ([]*model.Gateway, error) {
			return nil, errors.New("db unavailable")
		},
	}
	svc := newBroadcastWiredSecretService(repo, gwRepo, newFakeEventHub())

	if _, err := svc.Update("org1", "k1", "alice", &dto.UpdateSecretRequest{Value: "new-val"}); err != nil {
		t.Fatalf("Update must succeed even when loading gateways for broadcast fails, got: %v", err)
	}
	if repo.secrets["k1"].Status != model.SecretStatusActive {
		t.Error("secret's own state must be unaffected by a broadcast-side failure")
	}
}

func TestSecretService_Update_PublishEventError_DoesNotFailUpdate_OtherGatewayStillNotified(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["k1"] = &model.Secret{Handle: "k1", Status: model.SecretStatusActive}

	hub := newFakeEventHub()
	hub.publishFn = func(gatewayID string, _ eventhub.Event) error {
		if gatewayID == "gw-a" {
			return errors.New("websocket delivery failed")
		}
		return nil
	}
	svc := newBroadcastWiredSecretService(repo, twoGateways(), hub)

	if _, err := svc.Update("org1", "k1", "alice", &dto.UpdateSecretRequest{Value: "new-val"}); err != nil {
		t.Fatalf("Update must succeed even when one gateway's publish fails, got: %v", err)
	}
	if len(hub.published["gw-a"]) != 0 {
		t.Errorf("gw-a's failed publish must not be recorded as delivered, got %d", len(hub.published["gw-a"]))
	}
	if len(hub.published["gw-b"]) != 1 {
		t.Errorf("gw-b must still receive its event independently of gw-a's failure, got %d", len(hub.published["gw-b"]))
	}
}

// TestSecretService_Update_SkipsNilOrEmptyIDGateways confirms a malformed gateway
// entry (nil, or missing its ID) is skipped rather than passed to the event hub with
// an empty gateway ID.
func TestSecretService_Update_SkipsNilOrEmptyIDGateways(t *testing.T) {
	repo := newMockRepo()
	repo.secrets["k1"] = &model.Secret{Handle: "k1", Status: model.SecretStatusActive}

	hub := newFakeEventHub()
	gwRepo := &mockGatewayRepoForBroadcast{
		getByOrgFn: func(orgID string) ([]*model.Gateway, error) {
			return []*model.Gateway{nil, {ID: ""}, {ID: "gw-a"}}, nil
		},
	}
	svc := newBroadcastWiredSecretService(repo, gwRepo, hub)

	if _, err := svc.Update("org1", "k1", "alice", &dto.UpdateSecretRequest{Value: "new-val"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(hub.published[""]) != 0 {
		t.Errorf("no event should be published under an empty gateway ID, got %d", len(hub.published[""]))
	}
	if len(hub.published["gw-a"]) != 1 {
		t.Errorf("the one well-formed gateway must still receive its event, got %d", len(hub.published["gw-a"]))
	}
}
