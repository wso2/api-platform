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
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

// Gateways below 1.2.0 apply an apikey.revoked / apikey.updated event only when
// its user id equals the created_by they stored from the create event; newer
// releases trust the control plane. The event must therefore carry the
// creator's identity (or the empty id Platform API v1 wrote for migrated keys)
// for old gateways, and the authorized caller otherwise.
func TestKeyEventActor(t *testing.T) {
	cases := []struct {
		name, version, createdBy, creatorSub, caller, want string
	}{
		{"1.2.0 trusts the platform: caller as is", "1.2.0", "creator-1", "alice", "admin-1", "admin-1"},
		{"current build (no version yet): caller as is", "", "creator-1", "alice", "admin-1", "admin-1"},
		{"STS build is a current build", "2026.09.24", "creator-1", "alice", "admin-1", "admin-1"},
		{"1.1.0: the key's creator, whoever revokes", "1.1.0", "creator-1", "alice", "admin-1", "creator-1"},
		{"1.0.0: the creator revoking their own key", "1.0.0", "creator-1", "alice", "creator-1", "creator-1"},
		{"1.1.0: migrated key (creator is the migration actor) -> empty id", "1.1.0", "actor-uuid", constants.MigrationActorSub, "admin-1", ""},
		{"1.1.0: key with no creator -> empty id", "1.1.0", "", "", "admin-1", ""},
		{"1.1.0: creator could not be resolved -> still the creator", "1.1.0", "creator-1", "", "admin-1", "creator-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := KeyEventActor(tc.version, tc.createdBy, tc.creatorSub, tc.caller); got != tc.want {
				t.Fatalf("KeyEventActor(%q, %q, %q, %q) = %q, want %q", tc.version, tc.createdBy, tc.creatorSub, tc.caller, got, tc.want)
			}
		})
	}
}
