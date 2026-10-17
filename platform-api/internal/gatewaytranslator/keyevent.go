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

package gatewaytranslator

import (
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
)

// KeyEventActor returns the user id an apikey.revoked or apikey.updated event
// must carry for a gateway that reported gatewayVersion.
//
// Platform API decides who may revoke or update a key (the creator, a holder
// of ap:api_key:all:manage, or a trusted origin) before broadcasting, and
// gateways from gwversion.MinTrustedKeyEventsGatewayVersion apply the event on
// that authority: they get callerUserID, the authorized caller. Older releases
// re-check ownership themselves: the event's user id has to equal the
// created_by they stored from the apikey.created event, and an event carries
// no roles, so no caller is ever an admin there. For those gateways the event
// is stamped with what they hold: keyCreatedBy for a key created here (the
// create event carried the creator's identity, which the gateway stored), and
// the empty id for a key Platform API v1 created without a creator (no
// x-user-id header) — the migration records the migration actor as its
// creator, so in v2 it resolves to the migration actor (constants.MigrationActorSub, or LegacyMigrationActorSub in a
// database migrated before the subject was namespaced). creatorSub is
// keyCreatedBy resolved to its IdP subject; callers resolve it first and do not
// publish when that fails (see GatewayEventsService.KeyEventActor), because a
// guess would stamp an identity the gateway rejects.
//
// A non-empty result is keyCreatedBy, the internal UUID: the broadcaster
// resolves it to the subject the gateway stored, which is also how a key
// migrated with a v1 creator reaches the gateway under that v1 creator.
func KeyEventActor(gatewayVersion, keyCreatedBy, creatorSub, callerUserID string) string {
	if !gwversion.RechecksKeyEventOwnership(gatewayVersion) {
		return callerUserID
	}
	if keyCreatedBy == "" || IsMigrationActorSub(creatorSub) {
		return ""
	}
	return keyCreatedBy
}

// IsMigrationActorSub reports whether sub is the migration client's synthetic
// actor, under the current or the legacy subject.
func IsMigrationActorSub(sub string) bool {
	return sub == constants.MigrationActorSub || sub == constants.LegacyMigrationActorSub
}
