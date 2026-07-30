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

package handler

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// seedHandlerTestActors inserts every literal actor string used as a
// created_by/updated_by fixture value across the handler package's
// real-DB-backed tests that call a service method directly (bypassing the
// HTTP identity-resolution middleware, which would otherwise create the
// mapping row itself), so those fixtures satisfy the created_by/updated_by
// foreign key to user_idp_references(uuid).
func seedHandlerTestActors(t *testing.T, db *database.DB) {
	t.Helper()
	for _, actor := range []string{"alice"} {
		if _, err := db.Exec(db.Rebind(`INSERT INTO user_idp_references (uuid, idp_id) VALUES (?, ?)`), actor, actor); err != nil {
			t.Fatalf("failed to seed test actor %q: %v", actor, err)
		}
	}
}
