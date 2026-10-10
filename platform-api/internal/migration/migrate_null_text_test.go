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

package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// v1SourceSchema holds only the v1 columns the migrators below read.
const v1SourceSchema = `
CREATE TABLE artifacts (uuid TEXT, handle TEXT, name TEXT, version TEXT, organization_uuid TEXT,
	created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE rest_apis (uuid TEXT, description TEXT, created_by TEXT, project_uuid TEXT,
	lifecycle_status TEXT, transport TEXT, configuration BLOB);
CREATE TABLE llm_provider_templates (uuid TEXT, organization_uuid TEXT, handle TEXT, name TEXT,
	description TEXT, created_by TEXT, configuration BLOB, created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE llm_providers (uuid TEXT, description TEXT, created_by TEXT, template_uuid TEXT,
	openapi_spec TEXT, model_list TEXT, configuration BLOB);
CREATE TABLE llm_proxies (uuid TEXT, project_uuid TEXT, description TEXT, created_by TEXT,
	provider_uuid TEXT, openapi_spec TEXT, configuration BLOB);
CREATE TABLE mcp_proxies (uuid TEXT, project_uuid TEXT, description TEXT, created_by TEXT, configuration BLOB);
CREATE TABLE websub_apis (uuid TEXT, project_uuid TEXT, description TEXT, created_by TEXT,
	lifecycle_status TEXT, transport TEXT, configuration BLOB);
CREATE TABLE projects (uuid TEXT, name TEXT, organization_uuid TEXT, description TEXT,
	created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE applications (uuid TEXT, handle TEXT, project_uuid TEXT, organization_uuid TEXT, created_by TEXT,
	name TEXT, description TEXT, type TEXT, created_at TIMESTAMP, updated_at TIMESTAMP);
CREATE TABLE gateways (uuid TEXT, organization_uuid TEXT, name TEXT, display_name TEXT, description TEXT,
	properties BLOB, manifest BLOB, vhost TEXT, is_critical BOOLEAN, gateway_functionality_type TEXT,
	version TEXT, is_active BOOLEAN, created_at TIMESTAMP, updated_at TIMESTAMP);

-- One row per table, every description NULL as v1 allows.
INSERT INTO artifacts VALUES
	('art-rest', 'rest-h', 'Rest', 'v1', 'org-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
	('art-prov', 'prov-h', 'Prov', 'v1', 'org-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
	('art-prox', 'prox-h', 'Prox', 'v1', 'org-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
	('art-mcp',  'mcp-h',  'Mcp',  'v1', 'org-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00'),
	('art-ws',   'ws-h',   'Ws',   'v1', 'org-1', '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO rest_apis VALUES ('art-rest', NULL, NULL, 'proj-1', 'CREATED', '["http"]', '{}');
INSERT INTO llm_provider_templates VALUES ('tpl-1', 'org-1', 'tpl-h', 'Tpl', NULL, NULL, '{}', '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO llm_providers VALUES ('art-prov', NULL, NULL, 'tpl-1', '', '', '{}');
INSERT INTO llm_proxies VALUES ('art-prox', 'proj-1', NULL, NULL, 'art-prov', '', '{}');
INSERT INTO mcp_proxies VALUES ('art-mcp', 'proj-1', NULL, NULL, '{}');
INSERT INTO websub_apis VALUES ('art-ws', 'proj-1', NULL, NULL, 'CREATED', '["http"]', '{}');
INSERT INTO projects VALUES ('proj-1', 'default', 'org-1', NULL, '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO applications VALUES ('app-1', 'app-h', 'proj-1', 'org-1', NULL, 'App', NULL, 'genai', '2026-01-01 00:00:00', '2026-01-01 00:00:00');
INSERT INTO gateways VALUES ('gw-1', 'org-1', 'gw', 'GW', NULL, '{}', NULL, NULL, 0, 'regular', '1.1.0', 1, '2026-01-01 00:00:00', '2026-01-01 00:00:00');
`

func openSQLite(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func execFile(t *testing.T, db *sql.DB, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("apply %s: %v", path, err)
	}
}

// Every migrator that carries a v1 description writes "" for a v1 NULL into
// the real v2 schema, because v2 scans those columns into plain strings.
func TestMigrators_NullDescriptionBecomesEmpty(t *testing.T) {
	src := openSQLite(t, "v1.db")
	if _, err := src.Exec(v1SourceSchema); err != nil {
		t.Fatalf("v1 schema: %v", err)
	}
	tgt := openSQLite(t, "v2.db")
	execFile(t, tgt, filepath.Join("..", "database", "schema.sqlite.sql"))
	execFile(t, tgt, filepath.Join("..", "..", "plugins", "eventgateway", "schema", "schema.sqlite.sql"))

	k := testKernels(t)
	rc := &RunContext{Cfg: k.cfg, Log: k.log, Kernels: k, CP: k.cp, Src: src, Tgt: tgt,
		Direction: DirectionForward, BatchSize: 10, Handles: &HandleReport{}}

	cases := []struct {
		m     ResourceMigrator
		table string
		id    string
	}{
		{&projectsMigrator{baseMigrator{name: "projects"}}, "projects", "proj-1"},
		{&gatewaysMigrator{baseMigrator{name: "gateways"}}, "gateways", "gw-1"},
		{&applicationsMigrator{baseMigrator{name: "applications"}}, "applications", "app-1"},
		{&restApisMigrator{baseMigrator{name: "rest_apis"}}, "rest_apis", "art-rest"},
		{&llmTemplatesMigrator{baseMigrator{name: "llm_provider_templates"}}, "llm_provider_templates", "tpl-1"},
		{&llmProvidersMigrator{baseMigrator{name: "llm_providers"}}, "llm_providers", "art-prov"},
		{&llmProxiesMigrator{baseMigrator{name: "llm_proxies"}}, "llm_proxies", "art-prox"},
		{&mcpProxiesMigrator{baseMigrator{name: "mcp_proxies"}}, "mcp_proxies", "art-mcp"},
		{&websubApisMigrator{baseMigrator{name: "websub_apis"}}, "websub_apis", "art-ws"},
	}
	for _, tc := range cases {
		t.Run(tc.table, func(t *testing.T) {
			if _, err := tc.m.Migrate(context.Background(), rc); err != nil {
				t.Fatalf("migrate: %v", err)
			}
			var desc sql.NullString
			if err := tgt.QueryRow(`SELECT description FROM `+tc.table+` WHERE uuid = ?`, tc.id).Scan(&desc); err != nil {
				t.Fatalf("read back: %v", err)
			}
			if !desc.Valid || desc.String != "" {
				t.Errorf("description = %+v, want \"\" (not NULL)", desc)
			}
		})
	}
}
