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
 */

package repository

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

func TestGetAPIKeyByNameAndArtifactHandleResolvesKeyHandle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const (
		orgUUID      = "org-application-api-key"
		projectUUID  = "project-application-api-key"
		artifactUUID = "artifact-application-api-key"
		artifactID   = "new-openai-provider1"
		keyUUID      = "api-key-application-api-key"
		keyID        = "provider-new-key"
		keyName      = "Provider New Key"
	)

	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	if _, err := db.Exec(
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "RestApi", orgUUID,
	); err != nil {
		t.Fatalf("create artifact: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO rest_apis (
			uuid, organization_uuid, handle, display_name, version, project_uuid, configuration
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		artifactUUID, orgUUID, artifactID, "New OpenAI Provider", "v1.0",
		projectUUID, []byte("{}"),
	); err != nil {
		t.Fatalf("create artifact details: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO api_keys (
			uuid, artifact_uuid, handle, display_name, masked_api_key, api_key_hashes
		) VALUES (?, ?, ?, ?, ?, ?)`,
		keyUUID, artifactUUID, keyID, keyName, "****key", []byte("{}"),
	); err != nil {
		t.Fatalf("create API key: %v", err)
	}

	repo := NewApplicationRepo(db, NewArtifactTableRegistry())
	key, err := repo.GetAPIKeyByNameAndArtifactHandle(keyID, artifactID, orgUUID)
	if err != nil {
		t.Fatalf("resolve API key: %v", err)
	}
	if key == nil {
		t.Fatal("expected API key to resolve by its handle")
	}
	if key.APIKeyUUID != keyUUID {
		t.Fatalf("resolved API key UUID = %q, want %q", key.APIKeyUUID, keyUUID)
	}
	if key.Name != keyID {
		t.Fatalf("resolved API key handle = %q, want %q", key.Name, keyID)
	}
}

func TestListMappedAPIKeysReturnsKeyHandle(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	const (
		orgUUID      = "org-mapped-api-key"
		projectUUID  = "project-mapped-api-key"
		artifactUUID = "artifact-mapped-api-key"
		application  = "application-mapped-api-key"
		keyUUID      = "api-key-mapped-api-key"
		keyID        = "provider-new-key"
	)

	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	statements := []struct {
		query string
		args  []any
	}{
		{
			`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
			[]any{artifactUUID, "RestApi", orgUUID},
		},
		{
			`INSERT INTO rest_apis (
				uuid, organization_uuid, handle, display_name, version, project_uuid, configuration
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{artifactUUID, orgUUID, "new-openai-provider1", "New OpenAI Provider", "v1.0", projectUUID, []byte("{}")},
		},
		{
			`INSERT INTO applications (
				uuid, handle, project_uuid, organization_uuid, display_name, type
			) VALUES (?, ?, ?, ?, ?, ?)`,
			[]any{application, "new-application", projectUUID, orgUUID, "New Application", "GenAI"},
		},
		{
			`INSERT INTO api_keys (
				uuid, artifact_uuid, handle, display_name, masked_api_key, api_key_hashes
			) VALUES (?, ?, ?, ?, ?, ?)`,
			[]any{keyUUID, artifactUUID, keyID, "Provider New Key", "****key", []byte("{}")},
		},
		{
			`INSERT INTO application_api_key_mappings (application_uuid, api_key_id) VALUES (?, ?)`,
			[]any{application, keyUUID},
		},
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement.query, statement.args...); err != nil {
			t.Fatalf("prepare mapped API key: %v", err)
		}
	}

	repo := NewApplicationRepo(db, NewArtifactTableRegistry())
	keys, err := repo.ListMappedAPIKeys(application)
	if err != nil {
		t.Fatalf("list mapped API keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("mapped API key count = %d, want 1", len(keys))
	}
	if keys[0].Name != keyID {
		t.Fatalf("mapped API key handle = %q, want %q", keys[0].Name, keyID)
	}
}

// TestLifecycle_ApplicationByIDOrHandle exercises GetApplicationByIDOrHandle,
// whose `ORDER BY CASE … FetchFirstClause(1)` query was part of the LIMIT-1 fix
// (a single-row lookup that resolves by UUID or handle).
func TestLifecycle_ApplicationByIDOrHandle(t *testing.T) {
	tdb := openSchemaDB(t)
	orgRepo := NewOrganizationRepo(tdb.db)
	projectRepo := NewProjectRepo(tdb.db)
	appRepo := NewApplicationRepo(tdb.db, NewArtifactTableRegistry())

	org := &model.Organization{ID: id(), Handle: "ap-" + id()[:8], Name: "app org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("create org failed: %v", err)
	}
	projID := id()
	projName := "p-" + projID[:6]
	proj := &model.Project{ID: projID, Handle: projName, Name: projName, OrganizationID: org.ID}
	if err := projectRepo.CreateProject(proj); err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	app := &model.Application{
		UUID: id(), Handle: "app-" + id()[:8], ProjectUUID: proj.ID,
		OrganizationUUID: org.ID, Name: "app", Type: "standard",
	}
	if err := appRepo.CreateApplication(app); err != nil {
		t.Fatalf("create application failed: %v", err)
	}

	byUUID, err := appRepo.GetApplicationByIDOrHandle(app.UUID, org.ID)
	if err != nil {
		t.Fatalf("GetApplicationByIDOrHandle(uuid) failed: %v", err)
	}
	if byUUID == nil || byUUID.UUID != app.UUID {
		t.Fatalf("lookup by uuid: want %s, got %+v", app.UUID, byUUID)
	}

	byHandle, err := appRepo.GetApplicationByIDOrHandle(app.Handle, org.ID)
	if err != nil {
		t.Fatalf("GetApplicationByIDOrHandle(handle) failed: %v", err)
	}
	if byHandle == nil || byHandle.UUID != app.UUID {
		t.Fatalf("lookup by handle: want %s, got %+v", app.UUID, byHandle)
	}

	missing, err := appRepo.GetApplicationByIDOrHandle("does-not-exist-"+id(), org.ID)
	if err != nil {
		t.Fatalf("GetApplicationByIDOrHandle(missing) failed: %v", err)
	}
	if missing != nil {
		t.Fatalf("lookup of missing app: want nil, got %+v", missing)
	}
}
