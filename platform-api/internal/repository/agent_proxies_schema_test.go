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

package repository

import (
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
)

// seedAgentProxyOwners inserts an organization, a project and an artifact row so
// an agent_proxies insert has all three of its foreign keys satisfied.
func seedAgentProxyOwners(t *testing.T, db *database.DB, orgUUID, projectUUID, artifactUUID string) {
	t.Helper()

	if _, err := db.Exec(`INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, ?, 'default', 'idp-ref', datetime('now'), datetime('now'))`,
		orgUUID, "org-"+orgUUID, "Org "+orgUUID); err != nil {
		t.Fatalf("insert organization %s: %v", orgUUID, err)
	}
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		projectUUID, "proj-"+projectUUID, "Project "+projectUUID, orgUUID); err != nil {
		t.Fatalf("insert project %s: %v", projectUUID, err)
	}
	if _, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, 'AgentProxy', ?)`,
		artifactUUID, orgUUID); err != nil {
		t.Fatalf("insert artifact %s: %v", artifactUUID, err)
	}
}

// insertAgentProxy writes one agent_proxies row, naming every column.
func insertAgentProxy(db *database.DB, uuid, orgUUID, projectUUID, handle, protocol string) error {
	_, err := db.Exec(`INSERT INTO agent_proxies (
			uuid, organization_uuid, project_uuid, handle, display_name, version, protocol,
			description, configuration, origin, data_version, created_by, created_at, updated_by, updated_at
		) VALUES (?, ?, ?, ?, ?, 'v1.0', ?, ?, ?, 'control_plane', '1.0', 'tester', datetime('now'), 'tester', datetime('now'))`,
		uuid, orgUUID, projectUUID, handle, "Agent "+handle, protocol, "test agent proxy", []byte(`{}`))
	return err
}

// TestAgentProxiesTableExistsWithExpectedIndexes asserts a fresh database gets
// the table, the project index and the org-scoped unique index — and does not
// get a standalone organization_uuid index, which the unique index already
// covers with organization_uuid leftmost.
func TestAgentProxiesTableExistsWithExpectedIndexes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	var tableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'agent_proxies'`).Scan(&tableCount); err != nil {
		t.Fatalf("query sqlite_master for agent_proxies: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("agent_proxies table count = %d, want 1", tableCount)
	}

	// The test pool is capped at one connection, so every query below is fully
	// drained and closed before the next one is issued.
	var sawProjectIndex bool
	for _, name := range queryStrings(t, db,
		`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = 'agent_proxies'`) {
		switch name {
		case "idx_agent_proxies_project":
			sawProjectIndex = true
		case "idx_agent_proxies_org":
			t.Error("agent_proxies has a standalone organization_uuid index; " +
				"UNIQUE(organization_uuid, handle) already covers it")
		}
	}
	if !sawProjectIndex {
		t.Error("agent_proxies is missing idx_agent_proxies_project")
	}

	// PRAGMA index_list reports the implicit unique index backing
	// UNIQUE(organization_uuid, handle), which sqlite_master leaves with a NULL
	// sql column, so the unique constraint is asserted from here.
	uniqueIndexes := queryStrings(t, db, `SELECT name FROM pragma_index_list('agent_proxies') WHERE "unique" = 1`)

	var sawUniqueIndex bool
	for _, name := range uniqueIndexes {
		cols := queryStrings(t, db, `SELECT name FROM pragma_index_info(?) ORDER BY seqno`, name)
		if strings.Join(cols, ",") == "organization_uuid,handle" {
			sawUniqueIndex = true
		}
	}
	if !sawUniqueIndex {
		t.Errorf("agent_proxies has no unique index on (organization_uuid, handle); unique indexes found: %v", uniqueIndexes)
	}
}

// queryStrings runs a single-column query to completion and returns its values.
// Draining and closing before returning is what keeps the one-connection test
// pool from deadlocking when the caller queries again with the result in hand.
func queryStrings(t *testing.T, db *database.DB, query string, args ...any) []string {
	t.Helper()

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan %q: %v", query, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %q: %v", query, err)
	}
	return out
}

// TestAgentProxiesProtocolIsRequiredAtRuntime asserts the database refuses a row
// with no protocol rather than filling in a default. CP-D24 makes protocol the
// sole persisted discriminator, so an implicit value would silently decide which
// configuration variant a stored row claims to hold.
func TestAgentProxiesProtocolIsRequiredAtRuntime(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	const orgUUID, projectUUID = "org-proto", "proj-proto"
	seedAgentProxyOwners(t, db, orgUUID, projectUUID, "agent-proto-null")
	seedArtifactOnly(t, db, orgUUID, "agent-proto-missing")

	if err := insertAgentProxy(db, "agent-proto-null", orgUUID, projectUUID, "null-protocol", ""); err != nil {
		t.Fatalf("empty-string protocol should still insert (NOT NULL only rejects NULL): %v", err)
	}

	// NULL protocol.
	_, err := db.Exec(`INSERT INTO agent_proxies (
			uuid, organization_uuid, project_uuid, handle, display_name, version, protocol, configuration
		) VALUES (?, ?, ?, ?, ?, 'v1.0', NULL, ?)`,
		"agent-proto-missing", orgUUID, projectUUID, "null-protocol-2", "Agent", []byte(`{}`))
	if err == nil {
		t.Error("inserting agent_proxies with protocol = NULL succeeded, want a NOT NULL violation")
	}

	// Omitted protocol — no DEFAULT means this is the same violation, not a
	// silent fallback to a2a.
	_, err = db.Exec(`INSERT INTO agent_proxies (
			uuid, organization_uuid, project_uuid, handle, display_name, version, configuration
		) VALUES (?, ?, ?, ?, ?, 'v1.0', ?)`,
		"agent-proto-missing", orgUUID, projectUUID, "null-protocol-3", "Agent", []byte(`{}`))
	if err == nil {
		t.Error("inserting agent_proxies without a protocol succeeded, want a NOT NULL violation (no DEFAULT)")
	}
}

// seedArtifactOnly inserts an extra artifact row under an already-seeded org.
func seedArtifactOnly(t *testing.T, db *database.DB, orgUUID, artifactUUID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, 'AgentProxy', ?)`,
		artifactUUID, orgUUID); err != nil {
		t.Fatalf("insert artifact %s: %v", artifactUUID, err)
	}
}

// TestAgentProxiesHandleIsUniquePerOrganization asserts the handle collides
// within one organization and does not collide across two.
func TestAgentProxiesHandleIsUniquePerOrganization(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	seedAgentProxyOwners(t, db, "org-a", "proj-a", "agent-a1")
	seedArtifactOnly(t, db, "org-a", "agent-a2")
	seedAgentProxyOwners(t, db, "org-b", "proj-b", "agent-b1")

	if err := insertAgentProxy(db, "agent-a1", "org-a", "proj-a", "trip-planner", "a2a"); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	err := insertAgentProxy(db, "agent-a2", "org-a", "proj-a", "trip-planner", "a2a")
	if err == nil {
		t.Error("a duplicate handle within one organization was accepted, want a UNIQUE violation")
	} else if !IsUniqueViolation(err) {
		t.Errorf("duplicate handle produced %v, want an error IsUniqueViolation recognises (it maps to 409, not 500)", err)
	}

	if err := insertAgentProxy(db, "agent-b1", "org-b", "proj-b", "trip-planner", "a2a"); err != nil {
		t.Fatalf("the same handle in a different organization was rejected: %v", err)
	}
}

// TestAgentProxiesCascadeFromArtifactAndProject asserts both owning rows take
// the agent_proxies row with them, so deleting an Agent Proxy artifact — or the
// project it lives in — leaves nothing orphaned.
func TestAgentProxiesCascadeFromArtifactAndProject(t *testing.T) {
	countAgents := func(t *testing.T, db *database.DB) int {
		t.Helper()
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM agent_proxies`).Scan(&n); err != nil {
			t.Fatalf("count agent_proxies: %v", err)
		}
		return n
	}

	t.Run("artifact delete cascades", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		seedAgentProxyOwners(t, db, "org-c", "proj-c", "agent-c1")
		if err := insertAgentProxy(db, "agent-c1", "org-c", "proj-c", "cascade-artifact", "a2a"); err != nil {
			t.Fatalf("insert agent proxy: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM artifacts WHERE uuid = ?`, "agent-c1"); err != nil {
			t.Fatalf("delete artifact: %v", err)
		}
		if n := countAgents(t, db); n != 0 {
			t.Errorf("agent_proxies rows after artifact delete = %d, want 0", n)
		}
	})

	t.Run("project delete cascades", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()

		seedAgentProxyOwners(t, db, "org-d", "proj-d", "agent-d1")
		if err := insertAgentProxy(db, "agent-d1", "org-d", "proj-d", "cascade-project", "a2a"); err != nil {
			t.Fatalf("insert agent proxy: %v", err)
		}
		if _, err := db.Exec(`DELETE FROM projects WHERE uuid = ?`, "proj-d"); err != nil {
			t.Fatalf("delete project: %v", err)
		}
		if n := countAgents(t, db); n != 0 {
			t.Errorf("agent_proxies rows after project delete = %d, want 0", n)
		}
	})
}

// TestAgentProxiesOrgScopedListUsesUniqueIndex asserts the org-scoped list query
// — including the protocol filter Section 3 adds — is served by the
// (organization_uuid, handle) unique index rather than a table scan. This is why
// no separate organization_uuid index is created.
func TestAgentProxiesOrgScopedListUsesUniqueIndex(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	rows, err := db.Query(`EXPLAIN QUERY PLAN
		SELECT uuid, handle, display_name, version, protocol FROM agent_proxies
		WHERE organization_uuid = ? AND protocol = ?`, "org-e", "a2a")
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN: %v", err)
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan row: %v", err)
		}
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate plan rows: %v", err)
	}
	if !strings.Contains(plan.String(), "USING INDEX") {
		t.Errorf("org-scoped agent_proxies list is not index-served; plan was:\n%s", plan.String())
	}
}
