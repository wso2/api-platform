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
	"regexp"
	"strings"
	"testing"
)

// agentProxiesDialectFiles are every schema file that must define agent_proxies.
// A dialect missing the table means the Agent Proxy kind is absent on that
// engine, and nothing else in the build would notice: there is no ALTER TABLE
// path, so the DDL has to be right in all four files the first time.
var agentProxiesDialectFiles = []string{
	"schema.sql",
	"schema.sqlite.sql",
	"schema.postgres.sql",
	"schema.sqlserver.sql",
}

// wantAgentProxyColumns is the documented column order: identity and ownership,
// then resource identity, then payload and provenance, then the audit block.
// Order is asserted, not just membership — column order is fixed at CREATE TABLE
// and a dialect file that silently reorders still functions, so it would
// otherwise pass unnoticed.
var wantAgentProxyColumns = []string{
	// identity and ownership
	"uuid",
	"organization_uuid",
	"project_uuid",
	// resource identity
	"handle",
	"display_name",
	"version",
	"protocol",
	"description",
	// authoring payload and provenance
	"configuration",
	"origin",
	// audit
	"data_version",
	"created_by",
	"created_at",
	"updated_by",
	"updated_at",
}

// createTableRe matches both the Postgres/SQLite and the T-SQL spelling of the
// table's opening line.
var createTableRe = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:dbo\.)?agent_proxies\s*\(`)

// orgIndexRe matches a standalone organization_uuid index on agent_proxies in
// any dialect's spelling. It deliberately matches the CREATE statement rather
// than the bare name, so the comment recording why the index is absent does not
// itself trip the assertion.
var orgIndexRe = regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?idx_agent_proxies_org\b`)

// readAgentProxiesTable returns the body of the agent_proxies CREATE TABLE
// statement in the named schema file, without the enclosing parentheses.
func readAgentProxiesTable(t *testing.T, file string) string {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	body, ok := extractParenBody(string(data), createTableRe)
	if !ok {
		t.Fatalf("%s: no CREATE TABLE agent_proxies statement found", file)
	}
	return body
}

// extractParenBody finds the first match of open and returns everything between
// its trailing "(" and the matching ")".
func extractParenBody(src string, open *regexp.Regexp) (string, bool) {
	loc := open.FindStringIndex(src)
	if loc == nil {
		return "", false
	}
	depth := 1
	for i := loc[1]; i < len(src); i++ {
		switch src[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return src[loc[1]:i], true
			}
		}
	}
	return "", false
}

// columnNamesInOrder returns the leading identifier of every line in a
// CREATE TABLE body that declares a column — i.e. skipping comments and the
// table-level UNIQUE/FOREIGN KEY/PRIMARY KEY/CHECK constraint clauses.
func columnNamesInOrder(body string) []string {
	var cols []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "UNIQUE") || strings.HasPrefix(upper, "FOREIGN KEY") ||
			strings.HasPrefix(upper, "PRIMARY KEY") || strings.HasPrefix(upper, "CHECK") ||
			strings.HasPrefix(upper, "CONSTRAINT") {
			continue
		}
		name, _, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		cols = append(cols, name)
	}
	return cols
}

// columnDefinition returns the single line declaring the named column.
func columnDefinition(t *testing.T, file, body, column string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if name, _, ok := strings.Cut(line, " "); ok && name == column {
			return line
		}
	}
	t.Fatalf("%s: agent_proxies has no %q column", file, column)
	return ""
}

// TestAgentProxiesColumnOrderMatchesAcrossDialects asserts every dialect file
// declares agent_proxies with the same columns in the same order (R8-SYNC-STRUCTURE).
func TestAgentProxiesColumnOrderMatchesAcrossDialects(t *testing.T) {
	for _, file := range agentProxiesDialectFiles {
		body := readAgentProxiesTable(t, file)
		got := columnNamesInOrder(body)
		if len(got) != len(wantAgentProxyColumns) {
			t.Fatalf("%s: agent_proxies has %d columns %v, want %d %v",
				file, len(got), got, len(wantAgentProxyColumns), wantAgentProxyColumns)
		}
		for i, want := range wantAgentProxyColumns {
			if got[i] != want {
				t.Errorf("%s: agent_proxies column %d = %q, want %q (full order: %v)",
					file, i, got[i], want, got)
			}
		}
	}
}

// TestAgentProxiesColumnBlockPlacement pins the two orderings a reordering
// dialect file would otherwise break silently: both foreign-key columns sit
// directly after uuid, and data_version immediately precedes created_by with
// origin ahead of it (R5-DATA-VERSION).
func TestAgentProxiesColumnBlockPlacement(t *testing.T) {
	indexOf := func(cols []string, name string) int {
		for i, c := range cols {
			if c == name {
				return i
			}
		}
		return -1
	}

	for _, file := range agentProxiesDialectFiles {
		cols := columnNamesInOrder(readAgentProxiesTable(t, file))

		if got := indexOf(cols, "uuid"); got != 0 {
			t.Errorf("%s: uuid is at index %d, want 0", file, got)
		}
		if got := indexOf(cols, "organization_uuid"); got != 1 {
			t.Errorf("%s: organization_uuid is at index %d, want 1 (directly after uuid)", file, got)
		}
		if got := indexOf(cols, "project_uuid"); got != 2 {
			t.Errorf("%s: project_uuid is at index %d, want 2 (directly after organization_uuid)", file, got)
		}

		dataVersion, createdBy := indexOf(cols, "data_version"), indexOf(cols, "created_by")
		if dataVersion < 0 || createdBy < 0 || createdBy != dataVersion+1 {
			t.Errorf("%s: data_version (index %d) must immediately precede created_by (index %d)",
				file, dataVersion, createdBy)
		}
		if origin := indexOf(cols, "origin"); origin < 0 || origin >= dataVersion {
			t.Errorf("%s: origin (index %d) must sit ahead of data_version (index %d)",
				file, origin, dataVersion)
		}
	}
}

// TestAgentProxiesProtocolIsRequiredWithNoDefault asserts protocol is the
// persisted discriminator every dialect demands on insert. A DEFAULT would make
// an A2A Agent Proxy the implicit outcome of a write that never named a
// protocol, which is exactly what CP-D24 forbids.
func TestAgentProxiesProtocolIsRequiredWithNoDefault(t *testing.T) {
	for _, file := range agentProxiesDialectFiles {
		body := readAgentProxiesTable(t, file)
		def := columnDefinition(t, file, body, "protocol")

		if !strings.Contains(strings.ToUpper(def), "VARCHAR(20)") {
			t.Errorf("%s: protocol is declared %q, want VARCHAR(20)", file, def)
		}
		if !strings.Contains(strings.ToUpper(def), "NOT NULL") {
			t.Errorf("%s: protocol is declared %q, want NOT NULL", file, def)
		}
		if strings.Contains(strings.ToUpper(def), "DEFAULT") {
			t.Errorf("%s: protocol is declared %q, want no DEFAULT — the protocol is always written explicitly", file, def)
		}
		// R4-NO-ENUM-CHECK: supported protocols are validated in the service
		// layer, so adding one must never need a DDL migration.
		if strings.Contains(strings.ToUpper(body), "CHECK") {
			t.Errorf("%s: agent_proxies declares a CHECK constraint; enum validation belongs in the service layer", file)
		}
	}
}

// TestAgentProxiesConstraints asserts the org-scoped uniqueness and the three
// foreign keys, allowing only the documented SQL Server cascade-path divergence.
func TestAgentProxiesConstraints(t *testing.T) {
	for _, file := range agentProxiesDialectFiles {
		body := readAgentProxiesTable(t, file)
		normalized := strings.Join(strings.Fields(body), " ")

		if !strings.Contains(normalized, "UNIQUE(organization_uuid, handle)") {
			t.Errorf("%s: agent_proxies is missing UNIQUE(organization_uuid, handle)", file)
		}
		if !strings.Contains(normalized, "FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE") {
			t.Errorf("%s: agent_proxies is missing the artifacts FK with ON DELETE CASCADE", file)
		}
		if !strings.Contains(normalized, "FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE") {
			t.Errorf("%s: agent_proxies is missing the projects FK with ON DELETE CASCADE", file)
		}

		// SQL Server rejects two cascade paths converging on organizations
		// (error 1785), and organizations -> projects -> agent_proxies already
		// cascades. Every other dialect keeps CASCADE.
		wantOrgFK := "FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE"
		if file == "schema.sqlserver.sql" {
			wantOrgFK = "FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION"
		}
		if !strings.Contains(normalized, wantOrgFK) {
			t.Errorf("%s: agent_proxies is missing %q", file, wantOrgFK)
		}
	}
}

// TestAgentProxiesIndexes asserts the project index exists in every dialect and
// that no standalone organization_uuid index does: UNIQUE(organization_uuid,
// handle) already indexes organization_uuid leftmost, so one would be a pure
// write-cost duplicate (R6-NO-REDUNDANT-INDEX).
func TestAgentProxiesIndexes(t *testing.T) {
	for _, file := range agentProxiesDialectFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(data)

		wantIndex := "CREATE INDEX IF NOT EXISTS idx_agent_proxies_project ON agent_proxies(project_uuid);"
		wantGuard := ""
		if file == "schema.sqlserver.sql" {
			wantIndex = "CREATE INDEX idx_agent_proxies_project ON dbo.agent_proxies(project_uuid);"
			wantGuard = "IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_agent_proxies_project' AND object_id = OBJECT_ID(N'dbo.agent_proxies'))"
		}
		if !strings.Contains(content, wantIndex) {
			t.Errorf("%s: missing %q", file, wantIndex)
		}
		if wantGuard != "" && !strings.Contains(content, wantGuard) {
			t.Errorf("%s: idx_agent_proxies_project is not guarded by a sys.indexes check (R9-INDEX)", file)
		}

		// Match a CREATE INDEX statement specifically, so the explanatory comment
		// naming the index this table deliberately does not have is not a hit.
		if orgIndexRe.MatchString(content) {
			t.Errorf("%s: declares idx_agent_proxies_org, which duplicates the leftmost column of "+
				"UNIQUE(organization_uuid, handle) (R6-NO-REDUNDANT-INDEX)", file)
		}
	}
}

// TestAgentProxiesDDLIsIdempotent asserts each dialect's CREATE TABLE carries
// the existence guard its engine actually accepts (R9-TABLE) — IF NOT EXISTS is
// not valid T-SQL, and an OBJECT_ID guard is not valid anywhere else.
func TestAgentProxiesDDLIsIdempotent(t *testing.T) {
	for _, file := range agentProxiesDialectFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(data)

		if file == "schema.sqlserver.sql" {
			want := "IF OBJECT_ID(N'dbo.agent_proxies', N'U') IS NULL\nCREATE TABLE dbo.agent_proxies ("
			if !strings.Contains(content, want) {
				t.Errorf("%s: agent_proxies is not guarded by an OBJECT_ID existence check", file)
			}
			continue
		}
		if !strings.Contains(content, "CREATE TABLE IF NOT EXISTS agent_proxies (") {
			t.Errorf("%s: agent_proxies is not created with IF NOT EXISTS", file)
		}
	}
}
