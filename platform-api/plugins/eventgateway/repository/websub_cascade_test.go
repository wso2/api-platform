/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package repository

import (
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/database"
)

// openEventGatewayDB opens a SQLite database through database.NewConnection
// with the core schema and then this plugin's schema applied, so the
// websub_apis, websub_api_hmac_secrets and webbroker_apis tables exist.
func openEventGatewayDB(t *testing.T) *database.DB {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Database{Driver: "sqlite3", Path: filepath.Join(t.TempDir(), "test.db")}
	db, err := database.NewConnection(cfg, logger)
	if err != nil {
		t.Fatalf("open SQLite database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.InitSchema("../../../internal/database/schema.sql", logger); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}
	if err := db.InitSchema("../schema/schema.sql", logger); err != nil {
		t.Fatalf("InitSchema eventgateway failed: %v", err)
	}
	return db
}

// mustExec runs a `?`-placeholder statement.
func mustExec(t *testing.T, db *database.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(db.Rebind(query), args...); err != nil {
		t.Fatalf("exec failed: %v\nquery: %s", err, query)
	}
}

// countRows returns the number of rows in table whose col equals val.
func countRows(t *testing.T, db *database.DB, table, col, val string) int {
	t.Helper()
	var n int
	q := db.Rebind(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", table, col))
	if err := db.QueryRow(q, val).Scan(&n); err != nil {
		t.Fatalf("count(%s) failed: %v", table, err)
	}
	return n
}

// TestCascade_DeleteWebSubAPIRemovesHmacSecrets verifies that deleting the
// artifacts row that backs a WebSub API cascade-removes both the websub_apis row
// and all associated websub_api_hmac_secrets rows (two independent CASCADE edges
// from artifacts). This catches regressions if either FK is changed to NO ACTION.
func TestCascade_DeleteWebSubAPIRemovesHmacSecrets(t *testing.T) {
	db := openEventGatewayDB(t)

	orgUUID := uuid.NewString()
	projectUUID := uuid.NewString()
	artifactUUID := uuid.NewString()

	mustExec(t, db, `INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid) VALUES (?, ?, ?, ?, ?)`,
		orgUUID, "wsc-"+orgUUID[:8], "cascade org", "us", "idp-ref")
	mustExec(t, db, `INSERT INTO projects (uuid, handle, display_name, organization_uuid) VALUES (?, ?, ?, ?)`,
		projectUUID, "cascade-proj", "cascade-proj", orgUUID)
	mustExec(t, db, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "WebSubApi", orgUUID)
	mustExec(t, db, `INSERT INTO websub_apis (uuid, organization_uuid, handle, display_name, version, project_uuid, lifecycle_status, configuration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		artifactUUID, orgUUID, "ws-api-"+artifactUUID[:8], "ws-api", "v1.0", projectUUID, "CREATED", []byte("{}"))

	secret1 := uuid.NewString()
	secret2 := uuid.NewString()
	mustExec(t, db, `INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
		secret1, artifactUUID, "github-secret", []byte("enc1"), "active")
	mustExec(t, db, `INSERT INTO websub_api_hmac_secrets (uuid, artifact_uuid, handle, encrypted_secret, status) VALUES (?, ?, ?, ?, ?)`,
		secret2, artifactUUID, "gitlab-secret", []byte("enc2"), "active")

	if got := countRows(t, db, "websub_api_hmac_secrets", "artifact_uuid", artifactUUID); got != 2 {
		t.Fatalf("precondition: want 2 hmac secrets, got %d", got)
	}
	if got := countRows(t, db, "websub_apis", "uuid", artifactUUID); got != 1 {
		t.Fatalf("precondition: want 1 websub_api row, got %d", got)
	}

	mustExec(t, db, `DELETE FROM artifacts WHERE uuid = ?`, artifactUUID)

	if got := countRows(t, db, "websub_api_hmac_secrets", "artifact_uuid", artifactUUID); got != 0 {
		t.Fatalf("hmac secrets not cascade-deleted after artifact delete: %d remain", got)
	}
	if got := countRows(t, db, "websub_apis", "uuid", artifactUUID); got != 0 {
		t.Fatalf("websub_api not cascade-deleted after artifact delete: %d remain", got)
	}
}
