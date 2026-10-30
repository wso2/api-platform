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

package storage

import (
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestSaveAudit(t *testing.T) {
	store := setupTestStorage(t)
	at := time.Date(2026, 10, 30, 6, 7, 1, 112825000, time.UTC)

	assert.NilError(t, store.SaveAudit("audit-1", at, "CREATE", "res-1", "rest_api", "admin", "corr-1"))

	var gatewayID, correlationID, action, resourceUUID, resourceType, performedBy string
	var performedAt time.Time
	err := store.db.QueryRow(`SELECT gateway_id, correlation_id, action, resource_uuid, resource_type, performed_by, performed_at
		FROM audit WHERE uuid = ?`, "audit-1").Scan(&gatewayID, &correlationID, &action, &resourceUUID, &resourceType, &performedBy, &performedAt)
	assert.NilError(t, err)
	assert.Equal(t, gatewayID, "platform-gateway-id")
	assert.Equal(t, correlationID, "corr-1")
	assert.Equal(t, action, "CREATE")
	assert.Equal(t, resourceUUID, "res-1")
	assert.Equal(t, resourceType, "rest_api")
	assert.Equal(t, performedBy, "admin")
	assert.Assert(t, performedAt.Equal(at), "performed_at = %v, want %v", performedAt, at)
}
