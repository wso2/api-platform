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

package eventhub

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/microsoft/go-mssqldb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSQLServerHub stands up an EventHub against a live SQL Server using the
// PRODUCTION schema (composite PRIMARY KEY (gateway_id, event_id)). The SQLite
// unit-test schema uses a single-column event_id PK, which globally enforces
// event_id uniqueness and therefore hides the gateway-scoping behaviour of the
// duplicate check. This test exercises the real shape.
func setupSQLServerHub(t *testing.T) (*sql.DB, EventHub) {
	t.Helper()

	dsn := os.Getenv("SQLSERVER_TEST_DSN")
	if dsn == "" {
		t.Skip("SQLSERVER_TEST_DSN is not set; skipping sqlserver eventhub tests")
	}

	db, err := sql.Open("sqlserver", dsn)
	require.NoError(t, err)

	// Production-shaped schema: composite PK so a given event_id is unique only
	// within a gateway, and an FK so an event for an unregistered gateway fails.
	_, err = db.Exec(`
		IF OBJECT_ID(N'dbo.gateway_states', N'U') IS NULL
		CREATE TABLE dbo.gateway_states (
			gateway_id NVARCHAR(64) NOT NULL PRIMARY KEY,
			version_id NVARCHAR(64) NOT NULL DEFAULT '',
			updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME()
		);`)
	require.NoError(t, err)
	_, err = db.Exec(`
		IF OBJECT_ID(N'dbo.events', N'U') IS NULL
		CREATE TABLE dbo.events (
			gateway_id NVARCHAR(64) NOT NULL,
			processed_timestamp DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
			originated_timestamp DATETIME2(7) NOT NULL,
			entity_type NVARCHAR(64) NOT NULL,
			action NVARCHAR(20) NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
			entity_id NVARCHAR(255) NOT NULL,
			event_id NVARCHAR(64) NOT NULL,
			event_data NVARCHAR(MAX) NOT NULL,
			PRIMARY KEY (gateway_id, event_id),
			FOREIGN KEY (gateway_id) REFERENCES dbo.gateway_states(gateway_id) ON DELETE CASCADE
		);`)
	require.NoError(t, err)

	hub := New(db, testLogger(), Config{
		PollInterval:    time.Hour, // we drive Publish directly; no polling needed
		CleanupInterval: time.Hour,
		RetentionPeriod: time.Hour,
	})
	require.NoError(t, hub.Initialize())

	t.Cleanup(func() {
		hub.Close()
		db.Close()
	})
	return db, hub
}

// TestSQLServerPublish_DuplicateCheckIsGatewayScoped verifies the fix: an
// INSERT failure for one gateway must not be masked as a "duplicate" just
// because a DIFFERENT gateway already used the same event_id.
//
// Repro of the original bug:
//  1. Register gateway A, publish event "shared-id" to A — succeeds.
//  2. Do NOT register gateway B.
//  3. Publish event "shared-id" to B — the INSERT fails the gateway_states FK.
//     The post-failure existence check then runs. With a non-gateway-scoped
//     check it finds A's row and returns nil (silent suppression). With the
//     gateway-scoped check it finds no B row and surfaces the real error.
func TestSQLServerPublish_DuplicateCheckIsGatewayScoped(t *testing.T) {
	db, hub := setupSQLServerHub(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	gwA := "gw-a-" + suffix
	gwB := "gw-b-" + suffix // intentionally never registered
	sharedEventID := "shared-evt-" + suffix

	require.NoError(t, hub.RegisterGateway(gwA))
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM gateway_states WHERE gateway_id = @p1", gwA) })

	evt := Event{
		EventType:           EventTypeAPI,
		Action:              "CREATE",
		EntityID:            "entity-1",
		EventID:             sharedEventID,
		OriginatedTimestamp: time.Now(),
		EventData:           EmptyEventData,
	}

	// (1) Publish to the registered gateway A — must succeed.
	require.NoError(t, hub.PublishEvent(gwA, evt))

	// (3) Publish the SAME event_id to the UNREGISTERED gateway B. The INSERT
	// hits the FK violation; the fixed gateway-scoped existence check must NOT
	// treat A's row as B's duplicate, so a real error is returned.
	err := hub.PublishEvent(gwB, evt)
	assert.Error(t, err, "publish to unregistered gateway B must surface an error, not be silently suppressed as a duplicate")

	// Sanity: A's row exists, B's does not.
	var countA, countB int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM events WHERE gateway_id = @p1 AND event_id = @p2", gwA, sharedEventID).Scan(&countA))
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM events WHERE gateway_id = @p1 AND event_id = @p2", gwB, sharedEventID).Scan(&countB))
	assert.Equal(t, 1, countA, "gateway A event should be persisted")
	assert.Equal(t, 0, countB, "gateway B event must not exist")
}

// TestSQLServerPublish_TrueDuplicateStillSuppressed verifies the fix does not
// regress legitimate same-gateway de-duplication: re-publishing the same
// (gateway_id, event_id) is still treated as a duplicate and returns nil.
func TestSQLServerPublish_TrueDuplicateStillSuppressed(t *testing.T) {
	db, hub := setupSQLServerHub(t)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	gw := "gw-dup-" + suffix
	eventID := "dup-evt-" + suffix

	require.NoError(t, hub.RegisterGateway(gw))
	t.Cleanup(func() { _, _ = db.Exec("DELETE FROM gateway_states WHERE gateway_id = @p1", gw) })

	evt := Event{
		EventType:           EventTypeAPI,
		Action:              "CREATE",
		EntityID:            "entity-1",
		EventID:             eventID,
		OriginatedTimestamp: time.Now(),
		EventData:           EmptyEventData,
	}

	require.NoError(t, hub.PublishEvent(gw, evt))
	// Same gateway + same event_id again → genuine duplicate → suppressed (nil).
	assert.NoError(t, hub.PublishEvent(gw, evt))

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM events WHERE gateway_id = @p1 AND event_id = @p2", gw, eventID).Scan(&count))
	assert.Equal(t, 1, count, "duplicate publish must not create a second row")
}
