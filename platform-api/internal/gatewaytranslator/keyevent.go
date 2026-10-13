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
// the empty id for a key the migration brought over — Platform API v1 created
// it on the gateway without a creator, and in v2 its creator resolves to the
// migration actor (constants.MigrationActorSub). creatorSub is keyCreatedBy
// resolved to its IdP subject, or "" when it could not be resolved.
func KeyEventActor(gatewayVersion, keyCreatedBy, creatorSub, callerUserID string) string {
	if !gwversion.RechecksKeyEventOwnership(gatewayVersion) {
		return callerUserID
	}
	if keyCreatedBy == "" || creatorSub == constants.MigrationActorSub {
		return ""
	}
	return keyCreatedBy
}
