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
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// The service resolves the creator through the identity service before the
// translator decides (gatewaytranslator.KeyEventActor has the version table):
// a creator that resolves to the migration actor (current or legacy subject)
// yields the empty id, and without an identity service the creator id is still
// sent, never the caller.
func TestKeyEventActor_ResolvesTheCreator(t *testing.T) {
	withIdentity := NewGatewayEventsService(&capturingEventHub{}, newTestIdentityService(), newTestLogger())
	withoutIdentity := NewGatewayEventsService(&capturingEventHub{}, nil, newTestLogger())
	cases := []struct {
		name, createdBy, want string
		svc                   *GatewayEventsService
	}{
		{"migrated key, migration actor", constants.MigrationActorSub, "", withIdentity},
		{"migrated key, legacy migration actor", constants.LegacyMigrationActorSub, "", withIdentity},
		{"own key", "creator-1", "creator-1", withIdentity},
		{"no identity service", "creator-1", "creator-1", withoutIdentity},
	}
	for _, tc := range cases {
		got, err := tc.svc.KeyEventActor("1.1.0", tc.createdBy, "admin-1")
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

// failingIdentityRepo fails every lookup, standing in for a database error.
type failingIdentityRepo struct{ passthroughIdentityRepo }

func (failingIdentityRepo) GetSubByUUID(string) (string, bool, error) {
	return "", false, errors.New("database unavailable")
}

// When the creator cannot be resolved, a gateway that re-checks ownership gets
// no event and the caller gets the error: guessing the creator's UUID would
// reach the gateway as the migration actor for a key Platform API v1 created
// without a creator, and the gateway would drop it. Gateways that trust the
// control plane need no lookup and still receive the caller.
func TestKeyEventActor_CreatorLookupFailure(t *testing.T) {
	hub := &capturingEventHub{}
	svc := NewGatewayEventsService(hub, NewIdentityService(failingIdentityRepo{}), newTestLogger())
	event := &model.APIKeyRevokedEvent{ApiId: "api-1", KeyName: "key-1"}

	if _, err := svc.KeyEventActor("1.1.0", "creator-uuid", "admin-1"); err == nil {
		t.Fatal("1.1.0: want an error when the creator cannot be resolved")
	}
	if err := svc.BroadcastAPIKeyRevokedEventFor(&model.APIGatewayWithDetails{ID: "gw-110", Version: "1.1.0"}, "creator-uuid", "admin-1", event); err == nil {
		t.Fatal("1.1.0: want the broadcast to fail")
	}
	if len(hub.published) != 0 {
		t.Fatalf("1.1.0: published %d events, want none", len(hub.published))
	}

	got, err := svc.KeyEventActor("1.2.0", "creator-uuid", "admin-1")
	if err != nil || got != "admin-1" {
		t.Fatalf("1.2.0: got (%q, %v), want (admin-1, nil) without a lookup", got, err)
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
		{&model.APIGatewayWithDetails{ID: "gw-110-migrated-legacy", Version: "1.1.0"}, constants.LegacyMigrationActorSub, ""},
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
