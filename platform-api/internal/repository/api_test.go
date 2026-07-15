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
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/database"
	"github.com/wso2/api-platform/platform-api/internal/model"

	_ "github.com/mattn/go-sqlite3"
)

func createTestOrganizationAndProject(t *testing.T, db *database.DB, orgUUID, projectUUID string) {
	t.Helper()

	orgQuery := `
		INSERT INTO organizations (uuid, handle, display_name, region, idp_organization_ref_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'idp-ref', datetime('now'), datetime('now'))
	`
	_, err := db.Exec(orgQuery, orgUUID, "test-org-"+orgUUID, "Test Org", "default")
	if err != nil {
		t.Fatalf("Failed to create test organization: %v", err)
	}

	projectQuery := `
		INSERT INTO projects (uuid, handle, display_name, organization_uuid, created_at, updated_at)
		VALUES (?, ?, ?, ?, datetime('now'), datetime('now'))
	`
	_, err = db.Exec(projectQuery, projectUUID, "test-project-"+projectUUID, "Test Project", orgUUID)
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
		Configuration: model.RestAPIConfig{
			Name:      "Test API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
	if !reflect.DeepEqual(created.Configuration.Transport, api.Configuration.Transport) {
		t.Fatalf("GetAPIByUUID returned unexpected transport: %+v", created.Configuration.Transport)
	}
	if created.Configuration.Name != api.Configuration.Name || created.Configuration.Version != api.Configuration.Version {
		t.Fatalf("GetAPIByUUID returned unexpected configuration: %+v", created.Configuration)
	}
}

// TestAPIRepo_CreateAPI_SetsUpdatedBy guards against a prior gap where the
// rest_apis INSERT omitted updated_by, leaving it NULL until the first update.
func TestAPIRepo_CreateAPI_SetsUpdatedBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	repo := NewAPIRepo(db)

	orgUUID := "org-crud-updatedby"
	projectUUID := "project-crud-updatedby"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	api := &model.API{
		Handle:          "updatedby-api",
		Name:            "UpdatedBy API",
		Version:         "1.0.0",
		CreatedBy:       "test-user",
		UpdatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:      "UpdatedBy API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
	if created.UpdatedBy == "" {
		t.Fatal("expected updated_by to be set on creation, got empty string")
	}
	if created.UpdatedBy != created.CreatedBy {
		t.Fatalf("expected updated_by == created_by on creation, got created_by=%q updated_by=%q", created.CreatedBy, created.UpdatedBy)
	}
}

// TestAPIRepo_CreateAPIAssociation_NormalizesTimestampsToUTC guards against a
// prior gap where CreateAPIAssociation persisted whatever CreatedAt/UpdatedAt
// the caller passed in (typically local-server-time time.Now()) instead of
// normalizing to UTC, unlike every other repository Create method.
func TestAPIRepo_CreateAPIAssociation_NormalizesTimestampsToUTC(t *testing.T) {
	db, cleanup := setupTestDB(t)
	t.Cleanup(cleanup)

	apiRepo := NewAPIRepo(db)
	gatewayRepo := NewGatewayRepo(db)

	orgUUID := "org-assoc-utc"
	projectUUID := "project-assoc-utc"
	createTestOrganizationAndProject(t, db, orgUUID, projectUUID)

	restAPI := &model.API{
		Handle:          "assoc-utc-api",
		Name:            "Assoc UTC API",
		Version:         "1.0.0",
		CreatedBy:       "test-user",
		UpdatedBy:       "test-user",
		ProjectID:       projectUUID,
		OrganizationID:  orgUUID,
		LifeCycleStatus: "CREATED",
		Configuration: model.RestAPIConfig{
			Name:      "Assoc UTC API",
			Version:   "1.0.0",
			Transport: []string{"https"},
		},
	}
	if err := apiRepo.CreateAPI(restAPI); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	gateway := &model.Gateway{
		ID:             "gw-assoc-utc",
		OrganizationID: orgUUID,
		Handle:         "gw-assoc-utc",
		Name:           "Assoc UTC Gateway",
	}
	if err := gatewayRepo.Create(gateway); err != nil {
		t.Fatalf("gateway Create failed: %v", err)
	}

	// Deliberately pass a non-UTC CreatedAt/UpdatedAt, mimicking a caller that
	// computed time.Now() (server-local time) instead of time.Now().UTC().
	localZone := time.FixedZone("Test/Local", 5*60*60)
	association := &model.APIAssociation{
		ArtifactID:     restAPI.ID,
		OrganizationID: orgUUID,
		GatewayID:      gateway.ID,
		CreatedAt:      time.Date(2020, 1, 1, 0, 0, 0, 0, localZone),
		UpdatedAt:      time.Date(2020, 1, 1, 0, 0, 0, 0, localZone),
	}
	if err := apiRepo.CreateAPIAssociation(association); err != nil {
		t.Fatalf("CreateAPIAssociation failed: %v", err)
	}

	if association.CreatedAt.Location() != time.UTC {
		t.Fatalf("expected CreateAPIAssociation to normalize CreatedAt to UTC, got location %v", association.CreatedAt.Location())
	}
	if !association.UpdatedAt.Equal(association.CreatedAt) {
		t.Fatalf("expected UpdatedAt == CreatedAt on creation, got created=%v updated=%v", association.CreatedAt, association.UpdatedAt)
	}
	if time.Since(association.CreatedAt) > time.Minute {
		t.Fatalf("expected CreatedAt to be normalized to roughly now, got %v", association.CreatedAt)
	}

	associations, err := apiRepo.GetAPIAssociations(restAPI.ID, constants.AssociationTypeGateway, orgUUID)
	if err != nil {
		t.Fatalf("GetAPIAssociations failed: %v", err)
	}
	if len(associations) != 1 {
		t.Fatalf("expected exactly 1 association, got %d", len(associations))
	}
	if associations[0].CreatedAt.Year() == 2020 {
		t.Fatalf("expected the persisted CreatedAt to be the normalized value, not the caller-supplied 2020 date: %v", associations[0].CreatedAt)
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
		Configuration: model.RestAPIConfig{
			Name:      "Update API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
	api.Configuration = model.RestAPIConfig{
		Name:      "Updated API",
		Version:   "1.1.0",
		Transport: []string{"http", "https"},
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
	if !reflect.DeepEqual(updated.Configuration.Transport, api.Configuration.Transport) {
		t.Fatalf("UpdateAPI did not update transport: %+v", updated.Configuration.Transport)
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
		Configuration: model.RestAPIConfig{
			Name:      "Delete API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
		Configuration: model.RestAPIConfig{
			Name:      "Exists API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
		Configuration: model.RestAPIConfig{
			Name:      "Handle API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
		Configuration: model.RestAPIConfig{
			Name:      "Kind API",
			Version:   "1.0.0",
			Transport: []string{"https"},
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
		Configuration: model.RestAPIConfig{
			Name:      "Full Config API",
			Version:   "2.0.0",
			Context:   strPtr("/full-config"),
			Transport: []string{"http", "https"},
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

