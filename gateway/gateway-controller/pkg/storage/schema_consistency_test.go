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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// The schema exists as three dialect files plus a packaging copy, and only the
// SQLite one is embedded and executed by Go. The other two are applied by an
// operator by hand, so nothing at runtime would notice if a table or column
// were added to one and forgotten in another — a Postgres deployment would just
// fail at the first query against the missing column, in production, long after
// the change was reviewed.
//
// These tests compare the four files structurally, ignoring dialect-specific
// types and syntax, and pin every version header to currentSchemaVersion.

var schemaFiles = map[string]string{
	"sqlite":    "gateway-controller-db.sql",
	"postgres":  "gateway-controller-db.postgres.sql",
	"sqlserver": "gateway-controller-db.sqlserver.sql",
	// Shipped alongside the binary for operators. It is a symlink to the SQLite
	// file today; checked anyway so that replacing the symlink with a real copy
	// cannot silently start a drift.
	"resources": filepath.Join("..", "..", "resources", "gateway-controller-db.sql"),
}

// tablesOwnedElsewhere are defined by the event-gateway controller's own
// supplemental DDL (event-gateway/gateway-controller/pkg/dbschema), not here.
// They appear in no file in this package, so there is nothing to compare.
var tablesOwnedElsewhere = map[string]bool{
	"websub_apis":     true,
	"webbroker_apis":  true,
	"webhook_secrets": true,
}

var (
	createTableRe = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:dbo\.)?([a-z_][a-z0-9_]*)\s*\(`)
	versionRe     = regexp.MustCompile(`(?m)^--\s*Version:\s*(\d+)\s*$`)
	// A column definition line: a bare identifier at the start of a line inside
	// a CREATE TABLE body. Constraint lines (PRIMARY KEY, FOREIGN KEY, UNIQUE,
	// CHECK, CONSTRAINT) start with a reserved word and are filtered out.
	columnRe = regexp.MustCompile(`^([a-z_][a-z0-9_]*)\s+\S`)
)

var constraintKeywords = map[string]bool{
	"primary": true, "foreign": true, "unique": true, "check": true, "constraint": true,
}

func readSchemaFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	assert.NilError(t, err, "reading %s", path)
	return string(b)
}

// parseTables extracts table name -> column names from a DDL file. It walks the
// text rather than parsing SQL properly, which is enough for these files: every
// CREATE TABLE here is a plain column list terminated by a line starting with
// ")".
func parseTables(t *testing.T, ddl string) map[string][]string {
	t.Helper()

	tables := make(map[string][]string)
	for _, loc := range createTableRe.FindAllStringSubmatchIndex(ddl, -1) {
		name := strings.ToLower(ddl[loc[2]:loc[3]])
		body := ddl[loc[1]:]
		if end := strings.Index(body, "\n);"); end != -1 {
			body = body[:end]
		}

		var columns []string
		depth := 0
		for _, raw := range strings.Split(body, "\n") {
			line := strings.TrimSpace(raw)

			// Skip anything inside a parenthesised continuation (a multi-line
			// CHECK, for instance), so its contents are not read as columns.
			if depth > 0 {
				depth += strings.Count(line, "(") - strings.Count(line, ")")
				continue
			}

			if line == "" || strings.HasPrefix(line, "--") {
				continue
			}
			m := columnRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if constraintKeywords[strings.ToLower(m[1])] {
				continue
			}
			columns = append(columns, strings.ToLower(m[1]))
			depth += strings.Count(line, "(") - strings.Count(line, ")")
		}
		sort.Strings(columns)
		tables[name] = columns
	}
	return tables
}

// TestSchemaVersionHeadersMatchConstant pins every file's version header to the
// constant the running code checks against. A bump that updates the constant
// and forgets a file leaves an operator applying DDL that claims to be a
// version it is not.
func TestSchemaVersionHeadersMatchConstant(t *testing.T) {
	for dialect, path := range schemaFiles {
		ddl := readSchemaFile(t, path)

		m := versionRe.FindStringSubmatch(ddl)
		assert.Assert(t, m != nil, "%s (%s) has no `-- Version: N` header", dialect, path)
		assert.Equal(t, m[1], fmt.Sprintf("%d", currentSchemaVersion),
			"%s (%s) declares schema version %s, currentSchemaVersion is %d", dialect, path, m[1], currentSchemaVersion)
	}
}

// TestSQLitePragmaMatchesConstant checks the PRAGMA that actually stamps a new
// database. It is what initSchema later compares against, so a mismatch means
// every freshly created database is immediately rejected on the next startup.
func TestSQLitePragmaMatchesConstant(t *testing.T) {
	ddl := readSchemaFile(t, schemaFiles["sqlite"])

	m := regexp.MustCompile(`(?m)^PRAGMA\s+user_version\s*=\s*(\d+);`).FindStringSubmatch(ddl)
	assert.Assert(t, m != nil, "the SQLite schema does not stamp PRAGMA user_version")
	assert.Equal(t, m[1], fmt.Sprintf("%d", currentSchemaVersion),
		"the SQLite schema stamps user_version %s but currentSchemaVersion is %d; every new database would be rejected on the next startup", m[1], currentSchemaVersion)
}

// TestSchemaTablesMatchAcrossDialects compares table and column sets across the
// dialect files, ignoring types and syntax.
func TestSchemaTablesMatchAcrossDialects(t *testing.T) {
	reference := parseTables(t, readSchemaFile(t, schemaFiles["sqlite"]))
	assert.Assert(t, len(reference) > 0, "parsed no tables from the SQLite schema")

	for dialect, path := range schemaFiles {
		if dialect == "sqlite" {
			continue
		}
		t.Run(dialect, func(t *testing.T) {
			got := parseTables(t, readSchemaFile(t, path))

			for table, wantColumns := range reference {
				gotColumns, ok := got[table]
				if !ok {
					if tablesOwnedElsewhere[table] {
						continue
					}
					t.Errorf("table %q is missing from %s", table, path)
					continue
				}
				assert.DeepEqual(t, gotColumns, wantColumns)
			}
			for table := range got {
				if _, ok := reference[table]; !ok && !tablesOwnedElsewhere[table] {
					t.Errorf("table %q exists in %s but not in the SQLite schema", table, path)
				}
			}
		})
	}
}

// TestAgentsTableShape pins the agents table specifically, in every dialect.
// Most per-kind tables are a plain (uuid, gateway_id, configuration); the two
// that are not — this one and llm_proxies — are the easiest to get wrong in a
// hand-applied dialect file, because the extra columns are the part a copy of
// the neighbouring table would silently miss.
func TestAgentsTableShape(t *testing.T) {
	want := []string{"configuration", "gateway_id", "signed_protected_card", "signed_public_card", "uuid"}

	for dialect, path := range schemaFiles {
		tables := parseTables(t, readSchemaFile(t, path))
		columns, ok := tables[agentsResourceTable]
		assert.Assert(t, ok, "%s (%s) does not define the %s table", dialect, path, agentsResourceTable)
		assert.DeepEqual(t, columns, want)
	}
}

// TestEveryResourceTableIsDefined checks the tables the Go code maps kinds to
// actually exist in the schema. kindToResourceTable returning a table name that
// no DDL file creates is a runtime failure on first use, not a build error.
func TestEveryResourceTableIsDefined(t *testing.T) {
	tables := parseTables(t, readSchemaFile(t, schemaFiles["sqlite"]))

	for _, table := range builtinResourceTables {
		_, ok := tables[table]
		assert.Assert(t, ok, "builtinResourceTables lists %q, which the schema does not create", table)
	}

	for _, kind := range []string{"RestApi", "LlmProvider", "LlmProviderTemplate", "LlmProxy", "Mcp", "Agent"} {
		table, err := kindToResourceTable(kind)
		assert.NilError(t, err, "kind %q has no resource table", kind)
		_, ok := tables[table]
		assert.Assert(t, ok, "kind %q maps to table %q, which the schema does not create", kind, table)
	}
}
