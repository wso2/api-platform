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
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// seedAgentProxyOrgProject inserts an organization and a project so an Agent
// proxy create has its owners in place. Unlike seedAgentProxyOwners it does not
// insert the artifact row, because AgentProxyRepo.Create writes that itself.
func seedAgentProxyOrgProject(t *testing.T, db *database.DB, orgUUID, projectUUID string) {
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
}

func agentStrPtr(s string) *string { return &s }
func agentBoolPtr(b bool) *bool    { return &b }

// fullAgentProxy builds an Agent proxy exercising every configuration branch:
// both transports, common and per-operation policies, a managed public card and
// a passthrough protected card with an explicit false rewriteUrls.
func fullAgentProxy(orgUUID, projectUUID, handle string) *model.AgentProxy {
	return &model.AgentProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      projectUUID,
		Handle:           handle,
		Name:             "Weather Agent",
		Description:      "Provides forecasts and severe-weather alerts",
		Version:          "v1.0",
		Protocol:         model.AgentProxyProtocolA2A,
		CreatedBy:        "tester",
		UpdatedBy:        "tester",
		Configuration: model.AgentProxyConfiguration{
			Context: agentStrPtr("/weather"),
			Vhost:   agentStrPtr("agents.gw.com"),
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "http://weather-agent:9000",
					Auth: &model.UpstreamAuth{
						Type:   "api-key",
						Header: "X-API-Key",
						Value:  `{{ secret "weather-upstream" }}`,
					},
				},
			},
			Resilience: &model.Resilience{IdleTimeout: "5s"},
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: model.A2AOperationConfigs{
					Transports: []model.A2ATransport{
						{ProtocolBinding: "JSONRPC", PathPrefix: agentStrPtr("/rpc")},
						{ProtocolBinding: "HTTP+JSON", PathPrefix: agentStrPtr("/rest")},
					},
					Policies: []model.Policy{{Name: "jwt-auth", Version: "v1"}},
					Operations: []model.A2AOperation{{
						Name:       "SendMessage",
						Policies:   []model.Policy{{Name: "advanced-ratelimit", Version: "v1"}},
						Resilience: &model.Resilience{Timeout: "30s"},
					}},
				},
				AgentCard: &model.AgentCardConfig{
					Public: &model.PublicAgentCard{
						Mode:     agentStrPtr(model.AgentCardModeManaged),
						Path:     agentStrPtr(model.DefaultAgentCardPath),
						Policies: []model.Policy{{Name: "cors", Version: "v1"}},
						Content: model.AgentCardDocument{
							"name":            "Weather Agent",
							"version":         "1.0.0",
							"x-vendor-custom": map[string]interface{}{"kept": true},
						},
					},
					Protected: &model.ProtectedAgentCard{
						Mode:        model.AgentCardModePassthrough,
						RewriteUrls: agentBoolPtr(false),
					},
				},
			},
		},
	}
}

// minimalAgentProxy omits the card block and every optional part of the
// operation configuration (it carries only the required transports), so their
// absence can be asserted to survive a round trip.
func minimalAgentProxy(orgUUID, projectUUID, handle string) *model.AgentProxy {
	return &model.AgentProxy{
		OrganizationUUID: orgUUID,
		ProjectUUID:      projectUUID,
		Handle:           handle,
		Name:             "Minimal Agent",
		Version:          "v1.0",
		Protocol:         model.AgentProxyProtocolA2A,
		CreatedBy:        "tester",
		UpdatedBy:        "tester",
		Configuration: model.AgentProxyConfiguration{
			Upstream: model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "http://agent:9000"}},
			A2A: &model.A2AProtocolConfig{
				ProtocolVersion: "1.0",
				OperationConfigs: model.A2AOperationConfigs{
					Transports: []model.A2ATransport{{ProtocolBinding: "JSONRPC"}},
				},
			},
		},
	}
}

func TestAgentProxyRepoRoundTripsProtocolAndTypedConfiguration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	in := fullAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Create(in); err != nil {
		t.Fatalf("create: %v", err)
	}
	if in.UUID == "" {
		t.Fatal("create did not assign a UUID")
	}

	got, err := repo.GetByHandle("weather-agent", "org-1")
	if err != nil {
		t.Fatalf("get by handle: %v", err)
	}
	if got == nil {
		t.Fatal("get by handle returned no row")
	}
	if got.Protocol != model.AgentProxyProtocolA2A {
		t.Fatalf("protocol = %q, want %q", got.Protocol, model.AgentProxyProtocolA2A)
	}
	if got.Configuration.A2A == nil {
		t.Fatal("a2a configuration was not round-tripped")
	}
	if got.Configuration.A2A.ProtocolVersion != "1.0" {
		t.Fatalf("a2a.protocolVersion = %q, want 1.0", got.Configuration.A2A.ProtocolVersion)
	}
	if n := len(got.Configuration.A2A.OperationConfigs.Transports); n != 2 {
		t.Fatalf("transports = %d, want 2", n)
	}
	if len(got.Configuration.A2A.OperationConfigs.Policies) != 1 || len(got.Configuration.A2A.OperationConfigs.Operations) != 1 {
		t.Fatal("operation policies/operations were not round-tripped")
	}
	if got.Configuration.Upstream.Main == nil || got.Configuration.Upstream.Main.Auth == nil {
		t.Fatal("upstream auth was not round-tripped")
	}
	if got.Configuration.Upstream.Main.Auth.Value != `{{ secret "weather-upstream" }}` {
		t.Fatalf("secret placeholder = %q", got.Configuration.Upstream.Main.Auth.Value)
	}

	card := got.Configuration.A2A.AgentCard
	if card == nil || card.Public == nil || card.Protected == nil {
		t.Fatal("agent card blocks were not round-tripped")
	}
	if card.Public.Content["x-vendor-custom"] == nil {
		t.Fatal("free-form card extension was dropped")
	}
	// An explicit false must survive; a *bool is what separates it from omission.
	if card.Protected.RewriteUrls == nil || *card.Protected.RewriteUrls {
		t.Fatalf("protected rewriteUrls = %v, want explicit false", card.Protected.RewriteUrls)
	}

	byUUID, err := repo.GetByUUID(in.UUID, "org-1")
	if err != nil || byUUID == nil {
		t.Fatalf("get by uuid: %v (row %v)", err, byUUID)
	}
	if byUUID.Protocol != model.AgentProxyProtocolA2A || byUUID.Configuration.A2A == nil {
		t.Fatal("get by uuid did not populate protocol/configuration")
	}
}

func TestAgentProxyRepoOmittedBlocksStayOmitted(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "minimal-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := repo.GetByHandle("minimal-agent", "org-1")
	if err != nil || got == nil {
		t.Fatalf("get: %v (row %v)", err, got)
	}
	if got.Configuration.A2A.AgentCard != nil {
		t.Fatal("omitted agentCard block was materialized on read")
	}
	if len(got.Configuration.A2A.OperationConfigs.Transports) != 1 {
		t.Fatal("operationConfigs.transports were not round-tripped")
	}
	if got.Configuration.A2A.OperationConfigs.Policies != nil || got.Configuration.A2A.OperationConfigs.Operations != nil {
		t.Fatal("omitted operationConfigs policies/operations were materialized on read")
	}
	if got.Configuration.Context != nil || got.Configuration.Vhost != nil {
		t.Fatal("omitted context/vhost were materialized on read")
	}
	// The effective-mode helper is what callers use instead of probing for content.
	if mode := got.Configuration.A2A.EffectivePublicCardMode(); mode != model.AgentCardModePassthrough {
		t.Fatalf("effective public card mode = %q, want passthrough", mode)
	}
	if path := got.Configuration.A2A.EffectivePublicCardPath(); path != model.DefaultAgentCardPath {
		t.Fatalf("effective public card path = %q, want %q", path, model.DefaultAgentCardPath)
	}
}

// TestAgentProxyPersistedJSONCarriesNoColumnBackedFields inspects the stored blob
// directly: the discriminator and every piece of column-backed metadata must be
// absent from it, so there is exactly one authoritative location for each.
func TestAgentProxyPersistedJSONCarriesNoColumnBackedFields(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(fullAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}

	var raw []byte
	if err := db.QueryRow(`SELECT configuration FROM agent_proxies WHERE handle = ?`, "weather-agent").Scan(&raw); err != nil {
		t.Fatalf("read configuration column: %v", err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("stored configuration is not a JSON object: %v", err)
	}
	if _, ok := document["a2a"]; !ok {
		t.Fatal("stored configuration is missing the a2a block")
	}
	// The stored a2a block has the gateway's spec.a2a layout: transports live
	// under operationConfigs.
	var a2a map[string]json.RawMessage
	if err := json.Unmarshal(document["a2a"], &a2a); err != nil {
		t.Fatalf("stored a2a block is not a JSON object: %v", err)
	}
	var operationConfigs map[string]json.RawMessage
	if err := json.Unmarshal(a2a["operationConfigs"], &operationConfigs); err != nil {
		t.Fatalf("stored a2a.operationConfigs is not a JSON object: %v", err)
	}
	if _, ok := operationConfigs["transports"]; !ok {
		t.Error("stored configuration is missing a2a.operationConfigs.transports")
	}
	for _, forbidden := range []string{
		"protocol", "specVersion", "spec", "kind", "id", "handle", "name", "displayName",
		"description", "version", "projectId", "organizationId", "origin", "dataVersion",
		"createdBy", "updatedBy", "createdAt", "updatedAt", "associatedGateways",
	} {
		if _, ok := document[forbidden]; ok {
			t.Errorf("stored configuration carries column-backed field %q", forbidden)
		}
	}
}

// TestAgentProxyReadRejectsProtocolConfigurationMismatch covers the two drift
// shapes the column/document split makes possible. Neither may be resolved by
// inferring the protocol from a JSON key.
func TestAgentProxyReadRejectsProtocolConfigurationMismatch(t *testing.T) {
	t.Run("configuration missing its matching block", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

		repo := NewAgentProxyRepo(db)
		if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "drift-agent")); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := db.Exec(`UPDATE agent_proxies SET configuration = ? WHERE handle = ?`,
			[]byte(`{"upstream":{"main":{"url":"http://agent:9000"}}}`), "drift-agent"); err != nil {
			t.Fatalf("force drift: %v", err)
		}

		if _, err := repo.GetByHandle("drift-agent", "org-1"); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
			t.Fatalf("get error = %v, want ErrAgentProxyProtocolMismatch", err)
		}
	})

	t.Run("configuration carrying a null discriminator", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

		repo := NewAgentProxyRepo(db)
		if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "null-disc-agent")); err != nil {
			t.Fatalf("create: %v", err)
		}
		// The rule is that the key must not exist in the document at all. A null
		// value is still the key existing, and decoding it to a nil pointer would
		// wave it through.
		if _, err := db.Exec(`UPDATE agent_proxies SET configuration = ? WHERE handle = ?`,
			[]byte(`{"protocol":null,"upstream":{"main":{"url":"http://agent:9000"}},"a2a":{"protocolVersion":"1.0","operationConfigs":{"transports":[{"protocolBinding":"JSONRPC"}]}}}`),
			"null-disc-agent"); err != nil {
			t.Fatalf("force drift: %v", err)
		}

		if _, err := repo.GetByHandle("null-disc-agent", "org-1"); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
			t.Fatalf("get error = %v, want ErrAgentProxyProtocolMismatch", err)
		}
	})

	t.Run("configuration carrying a second discriminator", func(t *testing.T) {
		db, cleanup := setupTestDB(t)
		defer cleanup()
		seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

		repo := NewAgentProxyRepo(db)
		if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "dup-agent")); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := db.Exec(`UPDATE agent_proxies SET configuration = ? WHERE handle = ?`,
			[]byte(`{"protocol":"a2a","upstream":{"main":{"url":"http://agent:9000"}},"a2a":{"protocolVersion":"1.0","operationConfigs":{"transports":[{"protocolBinding":"JSONRPC"}]}}}`),
			"dup-agent"); err != nil {
			t.Fatalf("force drift: %v", err)
		}

		if _, err := repo.GetByHandle("dup-agent", "org-1"); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
			t.Fatalf("get error = %v, want ErrAgentProxyProtocolMismatch", err)
		}
	})
}

func TestAgentProxyCreateRejectsUnsupportedProtocol(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)

	unsupported := minimalAgentProxy("org-1", "proj-1", "bad-protocol")
	unsupported.Protocol = "mcp"
	if err := repo.Create(unsupported); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
		t.Fatalf("create error = %v, want ErrAgentProxyProtocolMismatch", err)
	}

	missingBlock := minimalAgentProxy("org-1", "proj-1", "no-block")
	missingBlock.Configuration.A2A = nil
	if err := repo.Create(missingBlock); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
		t.Fatalf("create error = %v, want ErrAgentProxyProtocolMismatch", err)
	}

	// Neither attempt may leave an artifact row behind.
	var artifacts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE type = 'AgentProxy'`).Scan(&artifacts); err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if artifacts != 0 {
		t.Fatalf("artifact rows = %d, want 0", artifacts)
	}
}

func TestAgentProxyUpdateRefusesProtocolChange(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	created := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}

	changed := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	changed.Protocol = "mcp"
	if err := repo.Update(changed); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
		t.Fatalf("update error = %v, want the unsupported protocol to be rejected", err)
	}

	// A supported-but-different protocol must be refused as immutable rather than
	// written. There is only one registered protocol today, so this asserts the
	// comparison happens against the stored column value.
	stored, err := repo.GetByHandle("weather-agent", "org-1")
	if err != nil || stored == nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Protocol != model.AgentProxyProtocolA2A {
		t.Fatalf("protocol = %q after refused update", stored.Protocol)
	}
}

func TestAgentProxyUpdateReplacesMetadataAndConfiguration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(fullAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}

	updated := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	updated.Name = "Renamed Agent"
	updated.Version = "v2.0"
	updated.UpdatedBy = "editor"
	if err := repo.Update(updated); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByHandle("weather-agent", "org-1")
	if err != nil || got == nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Name != "Renamed Agent" || got.Version != "v2.0" {
		t.Fatalf("columns not updated: name=%q version=%q", got.Name, got.Version)
	}
	if got.UpdatedBy != "editor" {
		t.Fatalf("updated_by = %q, want editor", got.UpdatedBy)
	}
	// The card block was dropped by the replacement, so it must be gone.
	if got.Configuration.A2A.AgentCard != nil {
		t.Fatal("omitted agentCard survived a full replacement")
	}

	// The list projection reads the same columns.
	list, err := repo.List("org-1", AgentProxyListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Renamed Agent" || list[0].Version != "v2.0" {
		t.Fatalf("list projection did not reflect the update: %+v", list)
	}
	if list[0].Protocol != model.AgentProxyProtocolA2A {
		t.Fatalf("list protocol = %q, want a2a", list[0].Protocol)
	}
}

func TestAgentProxyUpdatePreservesStoredDataVersion(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`UPDATE agent_proxies SET data_version = '1.7' WHERE handle = ?`, "weather-agent"); err != nil {
		t.Fatalf("seed data version: %v", err)
	}

	update := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Update(update); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByHandle("weather-agent", "org-1")
	if err != nil || got == nil {
		t.Fatalf("reload: %v", err)
	}
	if got.DataVersion != "1.7" {
		t.Fatalf("data_version = %q, want the stored 1.7 to be preserved", got.DataVersion)
	}
}

func TestAgentProxyListAndCountFilterByProtocolWithinOrg(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	seedAgentProxyOrgProject(t, db, "org-2", "proj-2")

	repo := NewAgentProxyRepo(db)
	for _, handle := range []string{"agent-a", "agent-b", "agent-c"} {
		if err := repo.Create(minimalAgentProxy("org-1", "proj-1", handle)); err != nil {
			t.Fatalf("create %s: %v", handle, err)
		}
	}
	if err := repo.Create(minimalAgentProxy("org-2", "proj-2", "other-org-agent")); err != nil {
		t.Fatalf("create in second org: %v", err)
	}

	all, err := repo.List("org-1", AgentProxyListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("unfiltered list = %d rows, want 3 (org isolation)", len(all))
	}

	filtered, err := repo.List("org-1", AgentProxyListOptions{Limit: 10, Protocol: model.AgentProxyProtocolA2A})
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}
	if len(filtered) != 3 {
		t.Fatalf("a2a-filtered list = %d rows, want 3", len(filtered))
	}

	none, err := repo.List("org-1", AgentProxyListOptions{Limit: 10, Protocol: "does-not-exist"})
	if err != nil {
		t.Fatalf("list with unmatched protocol: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("unmatched protocol returned %d rows, want 0", len(none))
	}

	// The count applies the same predicate as the list, so a filtered page can
	// never be paginated against an unfiltered total.
	for _, tc := range []struct {
		name  string
		opts  AgentProxyListOptions
		count int
	}{
		{"unfiltered", AgentProxyListOptions{}, 3},
		{"a2a", AgentProxyListOptions{Protocol: model.AgentProxyProtocolA2A}, 3},
		{"unmatched", AgentProxyListOptions{Protocol: "does-not-exist"}, 0},
	} {
		got, err := repo.Count("org-1", tc.opts)
		if err != nil {
			t.Fatalf("count %s: %v", tc.name, err)
		}
		if got != tc.count {
			t.Errorf("count %s = %d, want %d", tc.name, got, tc.count)
		}
	}

	// Pagination stays inside the filter.
	page, err := repo.List("org-1", AgentProxyListOptions{Limit: 2, Offset: 0, Protocol: model.AgentProxyProtocolA2A})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("first page = %d rows, want 2", len(page))
	}
	rest, err := repo.List("org-1", AgentProxyListOptions{Limit: 2, Offset: 2, Protocol: model.AgentProxyProtocolA2A})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	if len(rest) != 1 {
		t.Fatalf("second page = %d rows, want 1", len(rest))
	}
}

func TestAgentProxyProjectScopedListAndCount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	if _, err := db.Exec(`INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))`,
		"proj-other", "proj-other", "Other", "org-1"); err != nil {
		t.Fatalf("insert second project: %v", err)
	}

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "agent-a")); err != nil {
		t.Fatalf("create: %v", err)
	}
	other := minimalAgentProxy("org-1", "proj-other", "agent-b")
	if err := repo.Create(other); err != nil {
		t.Fatalf("create in other project: %v", err)
	}

	rows, err := repo.ListByProject("org-1", "proj-1")
	if err != nil {
		t.Fatalf("list by project: %v", err)
	}
	if len(rows) != 1 || rows[0].Handle != "agent-a" {
		t.Fatalf("list by project = %+v", rows)
	}
	count, err := repo.CountByProject("org-1", "proj-1")
	if err != nil {
		t.Fatalf("count by project: %v", err)
	}
	if count != 1 {
		t.Fatalf("count by project = %d, want 1", count)
	}
}

func TestAgentProxyRejectsCrossOrganizationProject(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	seedAgentProxyOrgProject(t, db, "org-2", "proj-2")

	repo := NewAgentProxyRepo(db)

	// Both foreign keys are satisfied on their own here — only the explicit check
	// catches that the project belongs to a different tenant.
	crossTenant := minimalAgentProxy("org-1", "proj-2", "cross-tenant")
	if err := repo.Create(crossTenant); !errors.Is(err, ErrAgentProxyProjectOrgMismatch) {
		t.Fatalf("create error = %v, want ErrAgentProxyProjectOrgMismatch", err)
	}

	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}
	moved := minimalAgentProxy("org-1", "proj-2", "weather-agent")
	if err := repo.Update(moved); !errors.Is(err, ErrAgentProxyProjectOrgMismatch) {
		t.Fatalf("update error = %v, want ErrAgentProxyProjectOrgMismatch", err)
	}
}

func TestAgentProxyHandleUniquenessAndExistence(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	seedAgentProxyOrgProject(t, db, "org-2", "proj-2")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := repo.Create(minimalAgentProxy("org-1", "proj-1", "weather-agent"))
	if err == nil {
		t.Fatal("duplicate handle within an organization was accepted")
	}
	if !IsUniqueViolation(err) {
		t.Fatalf("duplicate handle error = %v, want a unique violation (mapped to 409)", err)
	}

	// The same handle in a different organization is fine.
	if err := repo.Create(minimalAgentProxy("org-2", "proj-2", "weather-agent")); err != nil {
		t.Fatalf("create in second org: %v", err)
	}

	exists, err := repo.Exists("weather-agent", "org-1")
	if err != nil || !exists {
		t.Fatalf("exists = %v, %v; want true, nil", exists, err)
	}
	missing, err := repo.Exists("nope", "org-1")
	if err != nil || missing {
		t.Fatalf("exists for unknown handle = %v, %v; want false, nil", missing, err)
	}
}

func TestAgentProxyDeleteRemovesArtifactAndIsNotFoundTwice(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	created := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Delete("weather-agent", "org-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var artifacts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE uuid = ?`, created.UUID).Scan(&artifacts); err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if artifacts != 0 {
		t.Fatalf("artifact rows = %d after delete, want 0", artifacts)
	}
	if err := repo.Delete("weather-agent", "org-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second delete = %v, want sql.ErrNoRows", err)
	}
	got, err := repo.GetByHandle("weather-agent", "org-1")
	if err != nil || got != nil {
		t.Fatalf("get after delete = %v, %v; want nil, nil", got, err)
	}
}

func TestAgentProxyUpdateOnMissingRowReturnsNoRows(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Update(minimalAgentProxy("org-1", "proj-1", "absent")); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("update = %v, want sql.ErrNoRows", err)
	}
}

// TestAgentProxySecretReferencesAreRecordedAndResolvable covers the drift this
// section makes live: an Agent proxy's secret references must be recorded at
// artifact level, and the secret-reference lookup must identify the referent by
// handle and display name rather than by a raw UUID with a blank name.
func TestAgentProxySecretReferencesAreRecordedAndResolvable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	created := fullAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Create(created); err != nil {
		t.Fatalf("create: %v", err)
	}

	var refs int
	if err := db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE artifact_uuid = ? AND secret_handle = ?`,
		created.UUID, "weather-upstream").Scan(&refs); err != nil {
		t.Fatalf("count refs: %v", err)
	}
	if refs != 1 {
		t.Fatalf("artifact_secret_refs rows = %d, want 1", refs)
	}

	secretRepo := NewSecretRepo(db)
	found, err := secretRepo.FindRefs("org-1", "weather-upstream")
	if err != nil {
		t.Fatalf("find refs: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("FindRefs returned %d references, want 1", len(found))
	}
	if found[0].Handle != "weather-agent" {
		t.Fatalf("reference handle = %q, want the Agent proxy handle (a raw UUID here means agent_proxies is missing from the lookup)", found[0].Handle)
	}
	if found[0].Name != "Weather Agent" {
		t.Fatalf("reference display name = %q, want %q", found[0].Name, "Weather Agent")
	}
	if found[0].Type != "AgentProxy" {
		t.Fatalf("reference type = %q, want AgentProxy", found[0].Type)
	}

	// Dropping the placeholder must drop the reference with it.
	updated := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Update(updated); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM artifact_secret_refs WHERE artifact_uuid = ?`, created.UUID).Scan(&refs); err != nil {
		t.Fatalf("count refs after update: %v", err)
	}
	if refs != 0 {
		t.Fatalf("artifact_secret_refs rows = %d after dropping the placeholder, want 0", refs)
	}
}
