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

// The service resolves the creator through the identity service before the
// translator decides (gatewaytranslator.KeyEventActor has the version table):
// a creator that resolves to the migration actor yields the empty id, and
// without an identity service the creator id is still sent, never the caller.
func TestKeyEventActor_ResolvesTheCreator(t *testing.T) {
	withIdentity := NewGatewayEventsService(&capturingEventHub{}, newTestIdentityService(), newTestLogger())
	if got := withIdentity.KeyEventActor("1.1.0", constants.MigrationActorSub, "admin-1"); got != "" {
		t.Fatalf("migrated key: got %q, want the empty id", got)
	}
	if got := withIdentity.KeyEventActor("1.1.0", "creator-1", "admin-1"); got != "creator-1" {
		t.Fatalf("own key: got %q, want creator-1", got)
	}
	withoutIdentity := NewGatewayEventsService(&capturingEventHub{}, nil, newTestLogger())
	if got := withoutIdentity.KeyEventActor("1.1.0", "creator-1", "admin-1"); got != "creator-1" {
		t.Fatalf("no identity service: got %q, want creator-1", got)
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
