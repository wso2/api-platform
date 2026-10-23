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
	"errors"
	"testing"
)

// An artifact's project is read from its own kind's table, so an artifact of
// another kind sharing the handle, or one in another organization, never answers.
func TestArtifactRepo_GetProjectHandle(t *testing.T) {
	db, cleanup := setupTestDBWithoutForeignKeys(t)
	defer cleanup()

	const (
		orgUUID     = "org-project-001"
		otherOrg    = "org-project-002"
		projectUUID = "proj-project-001"
	)
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)
	projectHandle := "test-project-" + projectUUID

	seed := []struct {
		query string
		args  []interface{}
	}{
		{`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('rest-1', 'RestApi', ?)`, []interface{}{orgUUID}},
		{`INSERT INTO rest_apis (uuid, organization_uuid, handle, display_name, project_uuid, configuration)
			VALUES ('rest-1', ?, 'shared', 'Shared', ?, '{}')`, []interface{}{orgUUID, projectUUID}},
		{`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('mcp-1', 'Mcp', ?)`, []interface{}{orgUUID}},
		{`INSERT INTO mcp_proxies (uuid, organization_uuid, handle, display_name, configuration)
			VALUES ('mcp-1', ?, 'shared', 'Shared', '{}')`, []interface{}{orgUUID}},
		{`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES ('prov-1', 'LlmProvider', ?)`, []interface{}{orgUUID}},
		{`INSERT INTO llm_providers (uuid, organization_uuid, handle, display_name, template_uuid, configuration)
			VALUES ('prov-1', ?, 'provider', 'Provider', 'template-1', '{}')`, []interface{}{orgUUID}},
	}
	for _, s := range seed {
		if _, err := db.Exec(s.query, s.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	repo := NewArtifactRepo(db, NewArtifactTableRegistry())
	for _, tc := range []struct {
		name, kind, handle, org string
		wantProject             string
		wantFound               bool
	}{
		{"REST API in a project", "RestApi", "shared", orgUUID, projectHandle, true},
		{"MCP proxy sharing the handle, in no project", "Mcp", "shared", orgUUID, "", true},
		{"kind that has no project", "LlmProvider", "provider", orgUUID, "", true},
		{"unknown handle", "RestApi", "missing", orgUUID, "", false},
		{"another organization", "RestApi", "shared", otherOrg, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, found, err := repo.GetProjectHandle(tc.kind, tc.handle, tc.org)
			if err != nil {
				t.Fatalf("GetProjectHandle: %v", err)
			}
			if project != tc.wantProject || found != tc.wantFound {
				t.Fatalf("GetProjectHandle = (%q, %v), want (%q, %v)", project, found, tc.wantProject, tc.wantFound)
			}
		})
	}

	if _, _, err := repo.GetProjectHandle("NotAKind", "shared", orgUUID); !errors.Is(err, ErrUnknownArtifactKind) {
		t.Fatalf("unknown kind error = %v, want ErrUnknownArtifactKind", err)
	}
}
