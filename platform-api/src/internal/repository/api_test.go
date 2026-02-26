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
	"reflect"
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/database"
	"platform-api/src/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

func createTestOrganizationAndProject(t *testing.T, db *database.DB, orgUUID, projectUUID string) {
	t.Helper()

	orgQuery := `
		INSERT INTO organizations (uuid, handle, name, region, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
	`
	_, err := db.Exec(orgQuery, orgUUID, "test-org-"+orgUUID, "Test Org", "default")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	projectQuery := `
		INSERT INTO projects (uuid, name, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, datetime('now'), datetime('now'))
	`
	_, err = db.Exec(projectQuery, projectUUID, "Test Project", orgUUID)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}
}

func strPtr(value string) *string {
	return &value
}

func TestAPIRepo_CreateAndRead(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-001"
	projectUUID := "project-crud-001"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "test-api",
		Name:            "Test API",
		Version:         "1.0.0",
		Description:     "Test API Description",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Transport:       []string{"https"},
		Configuration: model.RestAPIConfig{
			Name:    "Test API",
			Version: "1.0.0",
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	defer func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	}()

	if api.ID == "" {
		t.Fatal("CreateAPI should set api.ID")
	}

	created, err := repo.GetAPIByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if created == nil {
		t.Fatal("GetAPIByUUID returned nil")
	}

	if created.Handle != api.Handle || created.Name != api.Name || created.Version != api.Version {
		t.Fatalf("GetAPIByUUID returned unexpected API metadata: %+v", created)
	}
	if created.Description != api.Description || created.CreatedBy != api.CreatedBy || created.ProjectID != api.ProjectID {
		t.Fatalf("GetAPIByUUID returned unexpected API details: %+v", created)
	}
	if created.OrganizationID != api.OrganizationID || created.LifeCycleStatus != api.LifeCycleStatus {
		t.Fatalf("GetAPIByUUID returned unexpected lifecycle details: %+v", created)
	}
	if !reflect.DeepEqual(created.Transport, api.Transport) {
		t.Fatalf("GetAPIByUUID returned unexpected transport: %+v", created.Transport)
	}
	if created.Configuration.Name != api.Configuration.Name || created.Configuration.Version != api.Configuration.Version {
		t.Fatalf("GetAPIByUUID returned unexpected configuration: %+v", created.Configuration)
	}
}

func TestAPIRepo_Update(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-002"
	projectUUID := "project-crud-002"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "update-api",
		Name:            "Update API",
		Version:         "1.0.0",
		Description:     "Original Description",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Transport:       []string{"https"},
		Configuration: model.RestAPIConfig{
			Name:    "Update API",
			Version: "1.0.0",
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	defer func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	}()

	api.Name = "Updated API"
	api.Version = "1.1.0"
	api.Description = "Updated Description"
	api.LifeCycleStatus = "PUBLISHED"
	api.Transport = []string{"http", "https"}
	api.Configuration = model.RestAPIConfig{
		Name:    "Updated API",
		Version: "1.1.0",
	}

	if err := repo.UpdateAPI(api); err != nil {
		t.Fatalf("UpdateAPI failed: %v", err)
	}

	updated, err := repo.GetAPIByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if updated == nil {
		t.Fatal("GetAPIByUUID returned nil")
	}

	if updated.Name != api.Name || updated.Version != api.Version || updated.Description != api.Description {
		t.Fatalf("UpdateAPI changes not persisted: %+v", updated)
	}
	if updated.LifeCycleStatus != api.LifeCycleStatus {
		t.Fatalf("UpdateAPI did not update lifecycle status: %s", updated.LifeCycleStatus)
	}
	if !reflect.DeepEqual(updated.Transport, api.Transport) {
		t.Fatalf("UpdateAPI did not update transport: %+v", updated.Transport)
	}
	if updated.Configuration.Name != api.Configuration.Name || updated.Configuration.Version != api.Configuration.Version {
		t.Fatalf("UpdateAPI did not update configuration: %+v", updated.Configuration)
	}
}

func TestAPIRepo_Delete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-003"
	projectUUID := "project-crud-003"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "delete-api",
		Name:            "Delete API",
		Version:         "1.0.0",
		Description:     "Delete Description",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Transport:       []string{"https"},
		Configuration: model.RestAPIConfig{
			Name:    "Delete API",
			Version: "1.0.0",
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
		t.Fatalf("DeleteAPI failed: %v", err)
	}

	deleted, err := repo.GetAPIByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("Expected API to be deleted, got: %+v", deleted)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM artifacts WHERE uuid = ?", api.ID).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Failed to verify artifact cleanup: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected artifact to be removed, found %d", count)
	}

	exists, err := repo.CheckAPIExistsByHandleInOrganization(api.Handle, orgUUID)
	if err != nil {
		t.Fatalf("CheckAPIExistsByHandleInOrganization failed: %v", err)
	}
	if exists {
		t.Fatal("Expected API handle to be removed after delete")
	}
}

func TestAPIRepo_CheckAPIExistsByNameAndVersionInOrganization(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-004"
	projectUUID := "project-crud-004"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "exists-api",
		Name:            "Exists API",
		Version:         "1.0.0",
		Description:     "Exists Description",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Transport:       []string{"https"},
		Configuration: model.RestAPIConfig{
			Name:    "Exists API",
			Version: "1.0.0",
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	defer func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	}()

	exists, err := repo.CheckAPIExistsByNameAndVersionInOrganization(api.Name, api.Version, orgUUID, "")
	if err != nil {
		t.Fatalf("CheckAPIExistsByNameAndVersionInOrganization failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected API to exist")
	}

	exists, err = repo.CheckAPIExistsByNameAndVersionInOrganization(api.Name, api.Version, orgUUID, api.Handle)
	if err != nil {
		t.Fatalf("CheckAPIExistsByNameAndVersionInOrganization with exclude failed: %v", err)
	}
	if exists {
		t.Fatal("Expected API to be excluded by handle")
	}

	exists, err = repo.CheckAPIExistsByNameAndVersionInOrganization("unknown", "1.0.0", orgUUID, "")
	if err != nil {
		t.Fatalf("CheckAPIExistsByNameAndVersionInOrganization failed: %v", err)
	}
	if exists {
		t.Fatal("Expected non-existing API to return false")
	}
}

func TestAPIRepo_CheckAPIExistsByHandleInOrganization(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-005"
	projectUUID := "project-crud-005"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "handle-api",
		Name:            "Handle API",
		Version:         "1.0.0",
		Description:     "Handle Description",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Transport:       []string{"https"},
		Configuration: model.RestAPIConfig{
			Name:    "Handle API",
			Version: "1.0.0",
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	defer func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	}()

	exists, err := repo.CheckAPIExistsByHandleInOrganization(api.Handle, orgUUID)
	if err != nil {
		t.Fatalf("CheckAPIExistsByHandleInOrganization failed: %v", err)
	}
	if !exists {
		t.Fatal("Expected API handle to exist")
	}

	exists, err = repo.CheckAPIExistsByHandleInOrganization("unknown", orgUUID)
	if err != nil {
		t.Fatalf("CheckAPIExistsByHandleInOrganization failed: %v", err)
	}
	if exists {
		t.Fatal("Expected unknown handle to return false")
	}

	exists, err = repo.CheckAPIExistsByHandleInOrganization(api.Handle, "unknown-org")
	if err != nil {
		t.Fatalf("CheckAPIExistsByHandleInOrganization failed: %v", err)
	}
	if exists {
		t.Fatal("Expected unknown org to return false")
	}
}

func TestAPIRepo_CreateSetsArtifactKind(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-006"
	projectUUID := "project-crud-006"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "kind-api",
		Name:            "Kind API",
		Version:         "1.0.0",
		Description:     "Kind Description",
		CreatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Transport:       []string{"https"},
		Configuration: model.RestAPIConfig{
			Name:    "Kind API",
			Version: "1.0.0",
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	t.Cleanup(func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	})

	metadata, err := repo.GetAPIMetadataByHandle(api.Handle, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIMetadataByHandle failed: %v", err)
	}
	if metadata == nil {
		t.Fatal("GetAPIMetadataByHandle returned nil")
	}
	if metadata.Kind != constants.RestApi {
		t.Fatalf("Expected artifact kind %s, got %s", constants.RestApi, metadata.Kind)
	}
}

func TestAPIRepo_CreateAndRead_FullConfiguration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-007"
	projectUUID := "project-crud-007"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	execCond := "request.header['x-user'] == 'admin'"
	policyParams := map[string]interface{}{
		"issuer":    "https://issuer.example.com",
		"audiences": []interface{}{"aud1", "aud2"},
		"scopes":    []interface{}{"read", "write"},
	}

	api := &model.API{
		Handle:          "full-config-api",
		Name:            "Full Config API",
		Version:         "2.0.0",
		Description:     "Full config description",
		CreatedBy:       "config-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "PUBLISHED",
		Transport:       []string{"http", "https"},
		Configuration: model.RestAPIConfig{
			Name:    "Full Config API",
			Version: "2.0.0",
			Context: strPtr("/full-config"),
			Vhosts:  &model.VhostsConfig{Main: "api.example.com"},
			Upstream: model.UpstreamConfig{
				Main: &model.UpstreamEndpoint{
					URL: "https://backend.example.com",
					Auth: &model.UpstreamAuth{
						Type:   "header",
						Header: "Authorization",
						Value:  "Bearer token",
					},
				},
				Sandbox: &model.UpstreamEndpoint{
					Ref: "sandbox-ref",
				},
			},
			Policies: []model.Policy{
				{
					Name:               "jwt-auth",
					Version:            "v0.1.0",
					ExecutionCondition: &execCond,
					Params:             &policyParams,
				},
			},
			Operations: []model.Operation{
				{
					Name:        "GetResource",
					Description: "Get resource by id",
					Request: &model.OperationRequest{
						Method: "GET",
						Path:   "/resources/{id}",
						Policies: []model.Policy{
							{
								Name:    "rate-limit",
								Version: "v1",
								Params: &map[string]interface{}{
									"limit":  "100",
									"period": "1m",
								},
							},
						},
					},
				},
			},
		},
	}

	if err := repo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}
	defer func() {
		if err := repo.DeleteAPI(api.ID, orgUUID); err != nil {
			t.Errorf("DeleteAPI cleanup failed: %v", err)
		}
	}()

	created, err := repo.GetAPIByUUID(api.ID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if created == nil {
		t.Fatal("GetAPIByUUID returned nil")
	}

	if !reflect.DeepEqual(created.Configuration, api.Configuration) {
		t.Fatalf("Full configuration mismatch. expected=%+v actual=%+v", api.Configuration, created.Configuration)
	}
}

// TestAPIRepo_LegacyVhostDeserialization verifies that existing JSON rows containing the legacy
// "vhost" field (and no "vhosts" field) are deserialized correctly: the value is promoted to Vhosts.Main.
func TestAPIRepo_LegacyVhostDeserialization(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-legacy-vhost"
	projectUUID := "project-legacy-vhost"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	// Insert a row with a legacy "vhost" JSON field directly in the configuration column.
	// The schema stores API data across two tables: artifacts (metadata) and rest_apis (config).
	legacyConfig := `{"name":"Legacy API","version":"1.0.0","context":"/legacy","vhost":"legacy.example.com"}`
	apiID := "legacy-vhost-api-id"

	_, err := db.Exec(`
		INSERT INTO artifacts (uuid, handle, name, version, kind, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
		apiID, "legacy-vhost-api", "Legacy API", "1.0.0", "RestApi", orgUUID)
	if err != nil {
		t.Fatalf("failed to insert artifact row: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO rest_apis (uuid, description, created_by, project_uuid, lifecycle_status, transport, configuration)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		apiID, "", "test-user", projectUUID, "CREATED", `["https"]`, legacyConfig)
	if err != nil {
		t.Fatalf("failed to insert rest_api row: %v", err)
	}

	result, err := repo.GetAPIByUUID(apiID, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIByUUID failed: %v", err)
	}
	if result == nil {
		t.Fatal("GetAPIByUUID returned nil")
	}

	if result.Configuration.Vhosts == nil || result.Configuration.Vhosts.Main != "legacy.example.com" {
		t.Errorf("Vhosts.Main = %v, want %q", result.Configuration.Vhosts, "legacy.example.com")
	}
	if result.Configuration.Vhosts.Sandbox != nil {
		t.Errorf("Vhosts.Sandbox should be nil, got %v", result.Configuration.Vhosts.Sandbox)
	}
}
