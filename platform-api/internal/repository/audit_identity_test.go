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

// Coverage for the audit-fields/identity-mapping follow-up to #2371/#2370:
// user_idp_references get-or-create/reverse-lookup semantics,
// user_organization_mappings membership and cascade-on-delete, and the audit
// columns written by the api_keys, artifact_gateway_mappings and
// deployment_status paths.

package repository

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
)

// TestIdentity_GetOrCreateUUID_IsIdempotentAndReversible verifies the core
// sub<->UUID mapping contract: repeated resolution of the same identity
// returns the same UUID, and the reverse lookup finds it.
func TestIdentity_GetOrCreateUUID_IsIdempotentAndReversible(t *testing.T) {
	tdb := openSchemaDB(t)

	repo := NewUserIdentityMappingRepo(tdb.db)

	uuid1, err := repo.GetOrCreateUUID("it-sub-idempotent")
	if err != nil {
		t.Fatalf("GetOrCreateUUID failed: %v", err)
	}
	uuid2, err := repo.GetOrCreateUUID("it-sub-idempotent")
	if err != nil {
		t.Fatalf("GetOrCreateUUID (second call) failed: %v", err)
	}
	if uuid1 != uuid2 {
		t.Fatalf("expected the same UUID on repeated resolution, got %q then %q", uuid1, uuid2)
	}

	sub, found, err := repo.GetSubByUUID(uuid1)
	if err != nil {
		t.Fatalf("GetSubByUUID failed: %v", err)
	}
	if !found || sub != "it-sub-idempotent" {
		t.Fatalf("expected reverse lookup to find it-sub-idempotent, got found=%v sub=%q", found, sub)
	}
}

// TestIdentity_GetSubByUUID_NotFoundAfterDelete mimics forcefully removing a
// user: after the user_idp_references row is deleted, the reverse lookup must
// report not-found (the contract behind constants.DeletedUser at the service
// layer) rather than erroring.
func TestIdentity_GetSubByUUID_NotFoundAfterDelete(t *testing.T) {
	tdb := openSchemaDB(t)

	repo := NewUserIdentityMappingRepo(tdb.db)
	uuid, err := repo.GetOrCreateUUID("it-sub-to-delete")
	if err != nil {
		t.Fatalf("GetOrCreateUUID failed: %v", err)
	}

	tdb.exec(t, `DELETE FROM user_idp_references WHERE idp_id = ?`, "it-sub-to-delete")

	_, found, err := repo.GetSubByUUID(uuid)
	if err != nil {
		t.Fatalf("GetSubByUUID after delete failed: %v", err)
	}
	if found {
		t.Fatalf("expected found=false after the mapping row is deleted")
	}
}

// TestMembership_AddListCountAndOrgDeleteCascade exercises
// user_organization_mappings end to end: add, list, count, and that deleting
// the parent organization removes the membership row via ON DELETE CASCADE (or
// the repo's defense-in-depth manual delete on engines where that isn't
// guaranteed).
func TestMembership_AddListCountAndOrgDeleteCascade(t *testing.T) {
	tdb := openSchemaDB(t)

	identityRepo := NewUserIdentityMappingRepo(tdb.db)
	orgRepo := NewOrganizationRepo(tdb.db)
	membershipRepo := NewUserOrganizationMappingRepo(tdb.db)

	userUUID, err := identityRepo.GetOrCreateUUID("it-sub-member")
	if err != nil {
		t.Fatalf("GetOrCreateUUID failed: %v", err)
	}

	orgA := &model.Organization{ID: id(), Handle: "it-org-a-" + id()[:8], Name: "IT Org A", Region: "us"}
	orgB := &model.Organization{ID: id(), Handle: "it-org-b-" + id()[:8], Name: "IT Org B", Region: "us"}
	if err := orgRepo.CreateOrganization(orgA); err != nil {
		t.Fatalf("create orgA failed: %v", err)
	}
	if err := orgRepo.CreateOrganization(orgB); err != nil {
		t.Fatalf("create orgB failed: %v", err)
	}

	if err := membershipRepo.AddMembership(userUUID, orgA.ID); err != nil {
		t.Fatalf("AddMembership(orgA) failed: %v", err)
	}
	if err := membershipRepo.AddMembership(userUUID, orgB.ID); err != nil {
		t.Fatalf("AddMembership(orgB) failed: %v", err)
	}
	// Idempotent re-add must not error or duplicate.
	if err := membershipRepo.AddMembership(userUUID, orgA.ID); err != nil {
		t.Fatalf("AddMembership(orgA) duplicate should be a no-op, got: %v", err)
	}

	total, err := orgRepo.CountOrganizationsForUser(userUUID)
	if err != nil {
		t.Fatalf("CountOrganizationsForUser failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected 2 memberships, got %d", total)
	}

	orgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser failed: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("expected 2 orgs listed, got %d", len(orgs))
	}

	// Deleting orgA must remove its membership row (cascade or manual delete),
	// leaving orgB's membership untouched.
	if err := orgRepo.DeleteOrganization(orgA.ID); err != nil {
		t.Fatalf("DeleteOrganization(orgA) failed: %v", err)
	}

	remaining, err := orgRepo.CountOrganizationsForUser(userUUID)
	if err != nil {
		t.Fatalf("CountOrganizationsForUser after delete failed: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 membership remaining after orgA delete, got %d", remaining)
	}

	remainingOrgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("ListOrganizationsForUser after delete failed: %v", err)
	}
	if len(remainingOrgs) != 1 || remainingOrgs[0].ID != orgB.ID {
		t.Fatalf("expected only orgB to remain, got %+v", remainingOrgs)
	}
}

// TestAuditColumns_APIKeyCreateUpdateRevoke_SetUpdatedBy guards the
// previously-missing updated_by on api_keys creation, and confirms Update and
// Revoke each set updated_by (to a different actor) without touching
// created_by.
func TestAuditColumns_APIKeyCreateUpdateRevoke_SetUpdatedBy(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, projID := seedOrgProject(t, tdb, "audit-apikey")

	apiRepo := NewAPIRepo(tdb.db)
	api := &model.API{
		Handle:          "audit-apikey-api-" + id()[:8],
		Name:            "Audit APIKey API",
		Version:         "v1.0",
		CreatedBy:       "it-creator",
		UpdatedBy:       "it-creator",
		ProjectID:       projID,
		OrganizationID:  orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Audit APIKey API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		t.Fatalf("CreateAPI failed: %v", err)
	}

	keyRepo := NewAPIKeyRepo(tdb.db, nil)
	key := &model.APIKey{
		UUID:           id(),
		ArtifactUUID:   api.ID,
		Name:           "it-key",
		DisplayName:    "IT Key",
		MaskedAPIKey:   "ab12",
		APIKeyHashes:   `{"sha256":"` + id() + `"}`,
		Status:         "active",
		CreatedBy:      "it-creator",
		UpdatedBy:      "it-creator",
		AllowedTargets: "ALL",
	}
	if err := keyRepo.Create(key); err != nil {
		t.Fatalf("APIKey Create failed: %v", err)
	}

	afterCreate, err := keyRepo.GetByArtifactAndName(api.ID, "it-key")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after create failed: %v", err)
	}
	if afterCreate.UpdatedBy != "it-creator" {
		t.Fatalf("expected updated_by=it-creator on creation, got %q", afterCreate.UpdatedBy)
	}

	afterCreate.MaskedAPIKey = "cd34"
	afterCreate.UpdatedBy = "it-updater"
	if err := keyRepo.Update(afterCreate); err != nil {
		t.Fatalf("APIKey Update failed: %v", err)
	}

	afterUpdate, err := keyRepo.GetByArtifactAndName(api.ID, "it-key")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after update failed: %v", err)
	}
	if afterUpdate.UpdatedBy != "it-updater" {
		t.Fatalf("expected updated_by=it-updater after update, got %q", afterUpdate.UpdatedBy)
	}
	if afterUpdate.CreatedBy != "it-creator" {
		t.Fatalf("Update must not touch created_by, got %q", afterUpdate.CreatedBy)
	}

	if err := keyRepo.Revoke(api.ID, "it-key", "it-revoker"); err != nil {
		t.Fatalf("APIKey Revoke failed: %v", err)
	}

	afterRevoke, err := keyRepo.GetByArtifactAndName(api.ID, "it-key")
	if err != nil {
		t.Fatalf("GetByArtifactAndName after revoke failed: %v", err)
	}
	if afterRevoke.Status != "revoked" {
		t.Fatalf("expected status=revoked, got %q", afterRevoke.Status)
	}
	if afterRevoke.UpdatedBy != "it-revoker" {
		t.Fatalf("expected updated_by=it-revoker after revoke, got %q", afterRevoke.UpdatedBy)
	}
	if afterRevoke.CreatedBy != "it-creator" {
		t.Fatalf("Revoke must not touch created_by, got %q", afterRevoke.CreatedBy)
	}
}

// seedArtifactAndGateway inserts an artifacts row (the FK target for
// artifact_gateway_mappings / deployment_status) and a gateway, returning both
// UUIDs. Used by the association/deployment-status audit tests below.
func seedArtifactAndGateway(t *testing.T, tdb *schemaDB, orgID, prefix string) (artifactID, gatewayID string) {
	t.Helper()
	artifactID = "art-" + id()[:8]
	if _, err := tdb.db.Exec(tdb.db.Rebind(
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`),
		artifactID, "RestApi", orgID); err != nil {
		t.Fatalf("insert artifact failed: %v", err)
	}
	gwRepo := NewGatewayRepo(tdb.db)
	gw := &model.Gateway{
		ID:                id(),
		OrganizationID:    orgID,
		Name:              prefix + " gw",
		Handle:            prefix + "-gw-" + id()[:6],
		Endpoints:         []string{"https://localhost:8443"},
		FunctionalityType: "REGULAR",
		Version:           "1.0.0",
		CreatedBy:         "it-user",
	}
	if err := gwRepo.Create(gw); err != nil {
		t.Fatalf("gateway Create failed: %v", err)
	}
	return artifactID, gw.ID
}

// TestAuditColumns_ArtifactGatewayMapping_SetCreatedUpdatedBy guards the
// previously-NULL created_by/updated_by on artifact_gateway_mappings: create
// seeds both from the acting user, and update bumps updated_by while leaving
// created_by.
func TestAuditColumns_ArtifactGatewayMapping_SetCreatedUpdatedBy(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, _ := seedOrgProject(t, tdb, "audit-agm")
	artifactID, gatewayID := seedArtifactAndGateway(t, tdb, orgID, "agm")

	apiRepo := NewAPIRepo(tdb.db)
	if err := apiRepo.CreateAPIAssociation(&model.APIAssociation{
		ArtifactID:     artifactID,
		OrganizationID: orgID,
		GatewayID:      gatewayID,
		CreatedBy:      "it-creator",
	}); err != nil {
		t.Fatalf("CreateAPIAssociation failed: %v", err)
	}

	assocs, err := apiRepo.GetAPIAssociations(artifactID, "gateway", orgID)
	if err != nil {
		t.Fatalf("GetAPIAssociations failed: %v", err)
	}
	if len(assocs) != 1 {
		t.Fatalf("expected 1 association, got %d", len(assocs))
	}
	if assocs[0].CreatedBy != "it-creator" || assocs[0].UpdatedBy != "it-creator" {
		t.Fatalf("expected created_by=updated_by=it-creator on create, got created_by=%q updated_by=%q",
			assocs[0].CreatedBy, assocs[0].UpdatedBy)
	}

	if err := apiRepo.UpdateAPIAssociation(artifactID, gatewayID, "gateway", orgID, "it-updater"); err != nil {
		t.Fatalf("UpdateAPIAssociation failed: %v", err)
	}
	assocs, err = apiRepo.GetAPIAssociations(artifactID, "gateway", orgID)
	if err != nil {
		t.Fatalf("GetAPIAssociations (after update) failed: %v", err)
	}
	if assocs[0].CreatedBy != "it-creator" || assocs[0].UpdatedBy != "it-updater" {
		t.Fatalf("expected created_by=it-creator updated_by=it-updater after update, got created_by=%q updated_by=%q",
			assocs[0].CreatedBy, assocs[0].UpdatedBy)
	}
}

// TestAuditColumns_DeploymentStatus_SetPerformedBy guards the previously-NULL
// performed_by on deployment_status, seeded from the deploying actor
// (deployment.CreatedBy) at deploy time.
func TestAuditColumns_DeploymentStatus_SetPerformedBy(t *testing.T) {
	tdb := openSchemaDB(t)
	orgID, _ := seedOrgProject(t, tdb, "audit-ds")
	artifactID, gatewayID := seedArtifactAndGateway(t, tdb, orgID, "ds")

	depRepo := NewDeploymentRepo(tdb.db, NewArtifactTableRegistry())
	status := model.DeploymentStatusDeployed
	dep := &model.Deployment{
		Name:           "dep-" + id()[:8],
		ArtifactID:     artifactID,
		OrganizationID: orgID,
		GatewayID:      gatewayID,
		Content:        []byte("{}"),
		Status:         &status,
		CreatedBy:      "it-deployer",
	}
	if err := depRepo.CreateWithLimitEnforcement(dep, 10); err != nil {
		t.Fatalf("CreateWithLimitEnforcement failed: %v", err)
	}

	var performedBy string
	if err := tdb.db.QueryRow(tdb.db.Rebind(
		`SELECT performed_by FROM deployment_status WHERE artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?`),
		artifactID, orgID, gatewayID).Scan(&performedBy); err != nil {
		t.Fatalf("query performed_by failed: %v", err)
	}
	if performedBy != "it-deployer" {
		t.Fatalf("expected performed_by=it-deployer, got %q", performedBy)
	}
}

// seedOrgProject creates one organization and one project through the real
// repositories and returns their UUIDs. It is the lightweight parent graph that
// REST APIs, gateways and API keys hang off.
func seedOrgProject(t *testing.T, tdb *schemaDB, prefix string) (orgID, projID string) {
	t.Helper()
	orgRepo := NewOrganizationRepo(tdb.db)
	projectRepo := NewProjectRepo(tdb.db)

	org := &model.Organization{ID: id(), Handle: prefix + "-" + id()[:8], Name: prefix + " org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("create org failed: %v", err)
	}
	projID = id()
	pName := prefix + "-proj-" + projID[:6]
	proj := &model.Project{ID: projID, Handle: pName, Name: pName, OrganizationID: org.ID, Description: "p"}
	if err := projectRepo.CreateProject(proj); err != nil {
		t.Fatalf("create project failed: %v", err)
	}
	return org.ID, projID
}
