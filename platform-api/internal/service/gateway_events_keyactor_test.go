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

import (
	"encoding/json"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// Gateways below 1.2.0 apply an apikey.revoked / apikey.updated event only when
// its user id equals the created_by they stored from the create event; newer
// releases trust the control plane. KeyEventActor must therefore stamp the
// creator's identity (or the empty id Platform API v1 wrote for migrated keys)
// on events to old gateways, and the authorized caller otherwise.
func TestKeyEventActor(t *testing.T) {
	svc := NewGatewayEventsService(&capturingEventHub{}, newTestIdentityService(), newTestLogger())
	cases := []struct {
		name, version, createdBy, caller, want string
	}{
		{"1.2.0 trusts the platform: caller as is", "1.2.0", "creator-1", "admin-1", "admin-1"},
		{"current build (no version yet): caller as is", "", "creator-1", "admin-1", "admin-1"},
		{"STS build is a current build", "2026.09.24", "creator-1", "admin-1", "admin-1"},
		{"1.1.0: the key's creator, whoever revokes", "1.1.0", "creator-1", "admin-1", "creator-1"},
		{"1.0.0: the creator revoking their own key", "1.0.0", "creator-1", "creator-1", "creator-1"},
		{"1.1.0: migrated key (creator is the migration actor) -> empty id", "1.1.0", constants.MigrationActorSub, "admin-1", ""},
		{"1.1.0: key with no creator -> empty id", "1.1.0", "", "admin-1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := svc.KeyEventActor(tc.version, tc.createdBy, tc.caller); got != tc.want {
				t.Fatalf("KeyEventActor(%q, %q, %q) = %q, want %q", tc.version, tc.createdBy, tc.caller, got, tc.want)
			}
		})
	}
}

// Without an identity service a migrated key cannot be told apart; the
// creator id is still sent rather than the caller.
func TestKeyEventActor_NoIdentityService(t *testing.T) {
	svc := NewGatewayEventsService(&capturingEventHub{}, nil, newTestLogger())
	if got := svc.KeyEventActor("1.1.0", "creator-1", "admin-1"); got != "creator-1" {
		t.Fatalf("got %q, want creator-1", got)
	}
}

// The *For broadcasters stamp the computed actor on the envelope and keep the
// payload: an admin's revocation reaches a 1.1.0 gateway under the creator's
// identity, and a migrated key's revocation under the empty id, while a 1.2.0
// gateway sees the admin.
func TestBroadcastAPIKeyRevokedEventFor_StampsGatewayAcceptedIdentity(t *testing.T) {
	hub := &capturingEventHub{}
	svc := NewGatewayEventsService(hub, newTestIdentityService(), newTestLogger())
	event := &model.APIKeyRevokedEvent{ApiId: "api-1", KeyName: "key-1"}

	gateways := []struct {
		gw        *model.APIGatewayWithDetails
		createdBy string
		wantUser  string
	}{
		{&model.APIGatewayWithDetails{ID: "gw-110", Version: "1.1.0"}, "creator-1", "creator-1"},
		{&model.APIGatewayWithDetails{ID: "gw-110-migrated", Version: "1.1.0"}, constants.MigrationActorSub, ""},
		{&model.APIGatewayWithDetails{ID: "gw-120", Version: "1.2.0"}, "creator-1", "admin-1"},
	}
	for _, g := range gateways {
		if err := svc.BroadcastAPIKeyRevokedEventFor(g.gw, g.createdBy, "admin-1", event); err != nil {
			t.Fatalf("broadcast to %s: %v", g.gw.ID, err)
		}
	}
	if len(hub.published) != len(gateways) {
		t.Fatalf("published %d events, want %d", len(hub.published), len(gateways))
	}
	for i, g := range gateways {
		e := hub.published[i]
		if e.GatewayID != g.gw.ID {
			t.Errorf("event %d went to %q, want %q", i, e.GatewayID, g.gw.ID)
		}
		var envelope dto.GatewayEventDTO
		if err := json.Unmarshal([]byte(e.EventData), &envelope); err != nil {
			t.Fatalf("decode envelope: %v", err)
		}
		if envelope.Type != EventTypeAPIKeyRevoked {
			t.Errorf("event %d type %q, want %q", i, envelope.Type, EventTypeAPIKeyRevoked)
		}
		if envelope.UserId != g.wantUser {
			t.Errorf("event to %s carries userId %q, want %q", g.gw.ID, envelope.UserId, g.wantUser)
		}
		payload, _ := json.Marshal(envelope.Payload)
		var got model.APIKeyRevokedEvent
		if err := json.Unmarshal(payload, &got); err != nil || got.KeyName != "key-1" || got.ApiId != "api-1" {
			t.Errorf("event to %s payload %s, want key-1 on api-1 (err %v)", g.gw.ID, payload, err)
		}
	}
}
