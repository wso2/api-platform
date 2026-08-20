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

package database

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/config"
)

// TestSplitSQLStatements_SemicolonInLineComment ensures a semicolon inside a
// "--" line comment is not treated as a statement terminator. This guards the
// Postgres/SQL Server schema loader against comment text leaking out as SQL.
func TestSplitSQLStatements_SemicolonInLineComment(t *testing.T) {
	sql := `
-- limit_type values (REQUEST_COUNT, BANDWIDTH, ...); the quota window is
-- (time_amount x time_unit); limit_count_unit (KB/MB/GB) is only set for BANDWIDTH.
CREATE TABLE foo (id INT);
CREATE TABLE bar (id INT);
`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %#v", len(stmts), stmts)
	}
	for _, s := range stmts {
		if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(s)), "CREATE TABLE") {
			t.Fatalf("comment leaked into SQL, got statement: %q", s)
		}
	}
}

// TestSplitSQLStatements_SemicolonInInlineComment covers a trailing "--" comment
// on the same line as real SQL.
func TestSplitSQLStatements_SemicolonInInlineComment(t *testing.T) {
	sql := `CREATE TABLE foo (
    a INT, -- inline note with a ; semicolon
    b INT
);`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 1 {
		t.Fatalf("want 1 statement, got %d: %#v", len(stmts), stmts)
	}
}

// TestSplitSQLStatements_SemicolonInStringLiteral ensures string-literal
// semicolons are still preserved (existing behaviour must not regress).
func TestSplitSQLStatements_SemicolonInStringLiteral(t *testing.T) {
	sql := `INSERT INTO t (v) VALUES ('a;b');
INSERT INTO t (v) VALUES ('c');`
	stmts := splitSQLStatements(sql)
	if len(stmts) != 2 {
		t.Fatalf("want 2 statements, got %d: %#v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "'a;b'") {
		t.Fatalf("string-literal semicolon was split: %q", stmts[0])
	}
}

// TestNewConnection_SQLiteForeignKeysSurviveConnectionRecycling verifies FK
// enforcement survives connection recycling; SetMaxIdleConns(0) forces a
// fresh connection to simulate a ConnMaxLifetime recycle deterministically.
func TestNewConnection_SQLiteForeignKeysSurviveConnectionRecycling(t *testing.T) {
	dir := t.TempDir()
	slogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db, err := NewConnection(&config.Database{
		Driver:          DriverSQLite,
		Path:            filepath.Join(dir, "test.db"),
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: 300,
	}, slogger)
	if err != nil {
		t.Fatalf("NewConnection() error = %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Force every subsequent query onto a freshly-opened connection.
	db.DB.SetMaxIdleConns(0)

	var enabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("PRAGMA foreign_keys query: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d on a freshly-opened connection, want 1", enabled)
	}

	schemaPath := filepath.Join("schema.sqlite.sql")
	if err := db.InitSchema(schemaPath, slogger); err != nil {
		t.Fatalf("InitSchema() error = %v", err)
	}

	const orgUUID, artifactUUID, policyUUID = "org-fk-recycle", "artifact-fk-recycle", "policy-fk-recycle"
	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'idp-ref', datetime('now'), datetime('now'))`, orgUUID, "fk-recycle-org", "FK Recycle Org", "default"); err != nil {
		t.Fatalf("insert organization: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "LlmProvider", orgUUID); err != nil {
		t.Fatalf("insert artifact: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO gateway_custom_policies (uuid, organization_uuid, name, version, policy_definition, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		policyUUID, orgUUID, "fk-recycle-policy", "v1.0.0", []byte("{}")); err != nil {
		t.Fatalf("insert custom policy: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO gateway_custom_policy_usages (policy_uuid, artifact_uuid) VALUES (?, ?)`,
		policyUUID, artifactUUID); err != nil {
		t.Fatalf("insert usage: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM artifacts WHERE uuid = ?`, artifactUUID); err != nil {
		t.Fatalf("delete artifact: %v", err)
	}

	var usageCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM gateway_custom_policy_usages WHERE artifact_uuid = ?`, artifactUUID).Scan(&usageCount); err != nil {
		t.Fatalf("count usages: %v", err)
	}
	if usageCount != 0 {
		t.Fatalf("usage rows after artifact delete = %d, want 0 (ON DELETE CASCADE did not fire)", usageCount)
	}
}

// TestSplitSQLStatements_RealSchemas parses the dialect schema files that go
// through the splitter (Postgres and SQL Server) and asserts every produced
// statement begins with a real SQL keyword — i.e. no comment fragment leaked.
func TestSplitSQLStatements_RealSchemas(t *testing.T) {
	validPrefixes := []string{"CREATE", "IF", "ALTER", "INSERT", "DROP", "UPDATE", "DELETE", "SELECT", "WITH", "BEGIN", "SET", "GO"}
	for _, f := range []string{"schema.postgres.sql", "schema.sqlserver.sql", "schema.sql", "schema.sqlite.sql"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		for _, stmt := range splitSQLStatements(string(data)) {
			up := strings.ToUpper(strings.TrimSpace(stmt))
			ok := false
			for _, p := range validPrefixes {
				if strings.HasPrefix(up, p) {
					ok = true
					break
				}
			}
			if !ok {
				preview := stmt
				if len(preview) > 120 {
					preview = preview[:120]
				}
				t.Fatalf("%s: statement does not start with a SQL keyword (comment leak?):\n%q", f, preview)
			}
		}
	}
}
