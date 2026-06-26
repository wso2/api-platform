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
	"os"
	"strings"
	"testing"
)

// TestSplitSQLStatements_SemicolonInLineComment ensures a semicolon inside a
// "--" line comment is not treated as a statement terminator. This guards the
// Postgres/SQL Server schema loader against comment text leaking out as SQL.
func TestSplitSQLStatements_SemicolonInLineComment(t *testing.T) {
	sql := `
-- limit_type values (REQUEST_COUNT, BANDWIDTH, ...); the quota window is
-- (time_amount x time_unit); data_unit (KB/MB/GB) is only set for BANDWIDTH.
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
