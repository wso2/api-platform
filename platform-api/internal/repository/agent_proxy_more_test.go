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
	"errors"
	"strings"
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestAgentProxyCreateRejectsUnknownProject(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	err := repo.Create(minimalAgentProxy("org-1", "proj-missing", "orphan"))
	if !errors.Is(err, ErrAgentProxyProjectOrgMismatch) {
		t.Fatalf("create error = %v, want ErrAgentProxyProjectOrgMismatch", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("create error = %q, want the not-found variant", err)
	}

	var artifacts int
	if err := db.QueryRow(`SELECT COUNT(*) FROM artifacts WHERE type = 'AgentProxy'`).Scan(&artifacts); err != nil {
		t.Fatalf("count artifacts: %v", err)
	}
	if artifacts != 0 {
		t.Fatalf("artifact rows = %d after a refused create, want 0", artifacts)
	}
}

func TestAgentProxyCreateDefaultsOriginAndDataVersion(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	defaulted := minimalAgentProxy("org-1", "proj-1", "defaulted")
	if err := repo.Create(defaulted); err != nil {
		t.Fatalf("create: %v", err)
	}
	if defaulted.Origin != constants.OriginCP || defaulted.DataVersion == "" {
		t.Fatalf("origin/dataVersion = %q/%q, want CP and a computed version", defaulted.Origin, defaulted.DataVersion)
	}

	explicit := minimalAgentProxy("org-1", "proj-1", "imported")
	explicit.Origin = constants.OriginDP
	explicit.DataVersion = "9.9"
	if err := repo.Create(explicit); err != nil {
		t.Fatalf("create explicit: %v", err)
	}
	got, err := repo.GetByHandle("imported", "org-1")
	if err != nil || got == nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Origin != constants.OriginDP || got.DataVersion != "9.9" {
		t.Fatalf("stored origin/dataVersion = %q/%q, want the caller's values", got.Origin, got.DataVersion)
	}
}

func TestAgentProxyGatewayAssociations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	createTestGateway(t, db, "gw-1", "org-1")
	createTestGateway(t, db, "gw-2", "org-1")

	repo := NewAgentProxyRepo(db)

	t.Run("create records associations and get resolves handles", func(t *testing.T) {
		p := minimalAgentProxy("org-1", "proj-1", "assoc-agent")
		p.AssociatedGateways = []model.AssociatedGatewayMapping{{GatewayUUID: "gw-1", Metadata: `{"k":"v"}`}}
		if err := repo.Create(p); err != nil {
			t.Fatalf("create: %v", err)
		}
		got, err := repo.GetByHandle("assoc-agent", "org-1")
		if err != nil || got == nil {
			t.Fatalf("reload: %v", err)
		}
		if len(got.AssociatedGateways) != 1 {
			t.Fatalf("associations = %+v", got.AssociatedGateways)
		}
		a := got.AssociatedGateways[0]
		if a.GatewayUUID != "gw-1" || a.GatewayHandle != "test-gateway-gw-1" || a.Metadata != `{"k":"v"}` {
			t.Fatalf("association = %+v", a)
		}
	})

	t.Run("create with an unknown gateway rolls back", func(t *testing.T) {
		p := minimalAgentProxy("org-1", "proj-1", "bad-gw-agent")
		p.AssociatedGateways = []model.AssociatedGatewayMapping{{GatewayUUID: "gw-missing"}}
		if err := repo.Create(p); err == nil {
			t.Fatal("create with an unknown gateway succeeded")
		}
		if exists, err := repo.Exists("bad-gw-agent", "org-1"); err != nil || exists {
			t.Fatalf("exists after rollback = %v, %v; want false, nil", exists, err)
		}
	})

	t.Run("update replaces associations only when asked", func(t *testing.T) {
		keep := minimalAgentProxy("org-1", "proj-1", "assoc-agent")
		if err := repo.Update(keep); err != nil {
			t.Fatalf("update without replace: %v", err)
		}
		got, _ := repo.GetByHandle("assoc-agent", "org-1")
		if len(got.AssociatedGateways) != 1 || got.AssociatedGateways[0].GatewayUUID != "gw-1" {
			t.Fatalf("associations after non-replacing update = %+v", got.AssociatedGateways)
		}

		replace := minimalAgentProxy("org-1", "proj-1", "assoc-agent")
		replace.ReplaceAssociatedGateways = true
		replace.AssociatedGateways = []model.AssociatedGatewayMapping{{GatewayUUID: "gw-2"}}
		if err := repo.Update(replace); err != nil {
			t.Fatalf("replacing update: %v", err)
		}
		got, _ = repo.GetByHandle("assoc-agent", "org-1")
		if len(got.AssociatedGateways) != 1 || got.AssociatedGateways[0].GatewayUUID != "gw-2" {
			t.Fatalf("associations after replacement = %+v", got.AssociatedGateways)
		}

		bad := minimalAgentProxy("org-1", "proj-1", "assoc-agent")
		bad.ReplaceAssociatedGateways = true
		bad.AssociatedGateways = []model.AssociatedGatewayMapping{{GatewayUUID: "gw-missing"}}
		if err := repo.Update(bad); err == nil {
			t.Fatal("replacing with an unknown gateway succeeded")
		}
		got, _ = repo.GetByHandle("assoc-agent", "org-1")
		if len(got.AssociatedGateways) != 1 || got.AssociatedGateways[0].GatewayUUID != "gw-2" {
			t.Fatalf("a failed replacement was not rolled back: %+v", got.AssociatedGateways)
		}
	})

	t.Run("ensure gateway association", func(t *testing.T) {
		p := minimalAgentProxy("org-1", "proj-1", "ensure-agent")
		if err := repo.Create(p); err != nil {
			t.Fatalf("create: %v", err)
		}

		// No association yet: one is created, seeded from the deploy metadata.
		meta, err := repo.EnsureGatewayAssociation(p.UUID, "gw-1", "org-1", "tester", `{"seed":1}`, true)
		if err != nil || meta != `{"seed":1}` {
			t.Fatalf("first ensure = %q, %v", meta, err)
		}
		// Existing association, metadata omitted: the stored metadata is used.
		meta, err = repo.EnsureGatewayAssociation(p.UUID, "gw-1", "org-1", "tester", "", false)
		if err != nil || meta != `{"seed":1}` {
			t.Fatalf("ensure with omitted metadata = %q, %v", meta, err)
		}
		// Existing association, explicit empty metadata: the explicit value wins.
		meta, err = repo.EnsureGatewayAssociation(p.UUID, "gw-1", "org-1", "tester", "", true)
		if err != nil || meta != "" {
			t.Fatalf("ensure with explicit empty metadata = %q, %v", meta, err)
		}
		// Unknown gateway is a genuine failure.
		if _, err := repo.EnsureGatewayAssociation(p.UUID, "gw-missing", "org-1", "tester", "", false); err == nil {
			t.Fatal("ensure against an unknown gateway succeeded")
		}
	})
}

func TestAgentProxyUpdateCrossOrganizationAndDataVersion(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	seedAgentProxyOrgProject(t, db, "org-2", "proj-2")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}

	// The handle exists, but in another organization: that is not-found, not an update.
	foreign := minimalAgentProxy("org-2", "proj-2", "weather-agent")
	if err := repo.Update(foreign); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-org update = %v, want sql.ErrNoRows", err)
	}
	if err := repo.Delete("weather-agent", "org-2"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cross-org delete = %v, want sql.ErrNoRows", err)
	}
	if got, err := repo.GetByHandle("weather-agent", "org-1"); err != nil || got == nil {
		t.Fatalf("owner row disappeared after cross-org attempts: %v, %v", got, err)
	}

	// An omitted project is accepted — the project column is never rewritten.
	noProject := minimalAgentProxy("org-1", "", "weather-agent")
	if err := repo.Update(noProject); err != nil {
		t.Fatalf("update without project: %v", err)
	}

	// An empty stored data version is recomputed rather than written back empty.
	if _, err := db.Exec(`UPDATE agent_proxies SET data_version = '' WHERE handle = ?`, "weather-agent"); err != nil {
		t.Fatalf("clear data version: %v", err)
	}
	update := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	if err := repo.Update(update); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := repo.GetByHandle("weather-agent", "org-1")
	if err != nil || got == nil {
		t.Fatalf("reload: %v", err)
	}
	if got.DataVersion == "" {
		t.Fatal("empty stored data version was not recomputed")
	}

	// An explicit caller data version is written as given.
	explicit := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	explicit.DataVersion = "4.2"
	if err := repo.Update(explicit); err != nil {
		t.Fatalf("update with explicit version: %v", err)
	}
	got, _ = repo.GetByHandle("weather-agent", "org-1")
	if got.DataVersion != "4.2" {
		t.Fatalf("data_version = %q, want 4.2", got.DataVersion)
	}
}

func TestAgentProxyUpdateRejectsInvalidProtocolConfiguration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "weather-agent")); err != nil {
		t.Fatalf("create: %v", err)
	}
	missingBlock := minimalAgentProxy("org-1", "proj-1", "weather-agent")
	missingBlock.Configuration.A2A = nil
	if err := repo.Update(missingBlock); !errors.Is(err, ErrAgentProxyProtocolMismatch) {
		t.Fatalf("update = %v, want ErrAgentProxyProtocolMismatch", err)
	}
	got, _ := repo.GetByHandle("weather-agent", "org-1")
	if got == nil || got.Configuration.A2A == nil {
		t.Fatal("refused update overwrote the stored configuration")
	}
}

func TestAgentProxyCountByProjectIsOrgScoped(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	seedAgentProxyOrgProject(t, db, "org-2", "proj-2")

	repo := NewAgentProxyRepo(db)
	for _, h := range []string{"a", "b"} {
		if err := repo.Create(minimalAgentProxy("org-1", "proj-1", h)); err != nil {
			t.Fatalf("create %s: %v", h, err)
		}
	}

	for _, tc := range []struct {
		name, org, project string
		want               int
	}{
		{"owning org", "org-1", "proj-1", 2},
		{"project of another org", "org-2", "proj-1", 0},
		{"empty project", "org-2", "proj-2", 0},
		{"unknown project", "org-1", "nope", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.CountByProject(tc.org, tc.project)
			if err != nil {
				t.Fatalf("count: %v", err)
			}
			if got != tc.want {
				t.Fatalf("count = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestAgentProxyListSurfacesCorruptRow(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")

	repo := NewAgentProxyRepo(db)
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "good")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.Create(minimalAgentProxy("org-1", "proj-1", "corrupt")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Exec(`UPDATE agent_proxies SET configuration = ? WHERE handle = ?`, []byte(`{not json`), "corrupt"); err != nil {
		t.Fatalf("corrupt row: %v", err)
	}

	if _, err := repo.List("org-1", AgentProxyListOptions{Limit: 10}); err == nil {
		t.Fatal("list over a corrupt row succeeded")
	}
	if _, err := repo.ListByProject("org-1", "proj-1"); err == nil {
		t.Fatal("list by project over a corrupt row succeeded")
	}
	if _, err := repo.GetByHandle("corrupt", "org-1"); err == nil {
		t.Fatal("get of a corrupt row succeeded")
	}
}

func TestAgentProxyRepoSurfacesDatabaseErrors(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	seedAgentProxyOrgProject(t, db, "org-1", "proj-1")
	repo := NewAgentProxyRepo(db)
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	checks := map[string]func() error{
		"create": func() error { return repo.Create(minimalAgentProxy("org-1", "proj-1", "x")) },
		"update": func() error { return repo.Update(minimalAgentProxy("org-1", "proj-1", "x")) },
		"delete": func() error { return repo.Delete("x", "org-1") },
		"list": func() error {
			_, err := repo.List("org-1", AgentProxyListOptions{Limit: 10})
			return err
		},
		"count": func() error {
			_, err := repo.Count("org-1", AgentProxyListOptions{})
			return err
		},
		"count by project": func() error {
			_, err := repo.CountByProject("org-1", "proj-1")
			return err
		},
		"get by handle": func() error {
			_, err := repo.GetByHandle("x", "org-1")
			return err
		},
	}
	for name, fn := range checks {
		t.Run(name, func(t *testing.T) {
			err := fn()
			if err == nil {
				t.Fatal("call on a closed database succeeded")
			}
			if errors.Is(err, sql.ErrNoRows) {
				t.Fatalf("closed database reported as not-found: %v", err)
			}
		})
	}
}

func TestValidateAgentProxyProtocolConfiguration(t *testing.T) {
	valid := minimalAgentProxy("o", "p", "h")
	for _, tc := range []struct {
		name    string
		in      *model.AgentProxy
		wantErr error
		wantMsg string
	}{
		{name: "nil", in: nil, wantMsg: "nil"},
		{name: "empty protocol", in: &model.AgentProxy{}, wantErr: ErrAgentProxyProtocolMismatch, wantMsg: "empty"},
		{name: "unsupported", in: &model.AgentProxy{Protocol: "mcp"}, wantErr: ErrAgentProxyProtocolMismatch, wantMsg: "unsupported"},
		{name: "a2a without block", in: &model.AgentProxy{Protocol: model.AgentProxyProtocolA2A}, wantErr: ErrAgentProxyProtocolMismatch, wantMsg: "without an a2a"},
		{name: "valid", in: valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAgentProxyProtocolConfiguration(tc.in)
			if tc.wantMsg == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.wantMsg)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestDeserializeAgentProxyConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name     string
		in       []byte
		wantErr  bool
		mismatch bool
	}{
		{name: "nil", in: nil, wantErr: true},
		{name: "empty", in: []byte{}, wantErr: true},
		{name: "malformed", in: []byte(`{"upstream":`), wantErr: true},
		{name: "wrong shape", in: []byte(`[1,2,3]`), wantErr: true},
		{name: "protocol key", in: []byte(`{"protocol":"a2a"}`), wantErr: true, mismatch: true},
		{name: "empty object", in: []byte(`{}`)},
		{name: "valid", in: []byte(`{"upstream":{"main":{"url":"http://a"}},"a2a":{"protocolVersion":"1.0"}}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := deserializeAgentProxyConfiguration(tc.in)
			if !tc.wantErr {
				if err != nil || got == nil {
					t.Fatalf("got %v, %v; want a configuration", got, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("got %+v, want an error", got)
			}
			if errors.Is(err, ErrAgentProxyProtocolMismatch) != tc.mismatch {
				t.Fatalf("err = %v, mismatch sentinel expected = %v", err, tc.mismatch)
			}
		})
	}
}
