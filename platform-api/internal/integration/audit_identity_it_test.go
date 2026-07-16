/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

//go:build integration

// Cross-database coverage for the audit-fields/identity-mapping follow-up to
// #2371/#2370: user_idp_references get-or-create/reverse-lookup semantics,
// user_organization_mappings membership + cascade-on-delete parity across
// sqlite/postgres/sqlserver, and updated_by persistence on the four create
// paths that previously left it NULL (rest_apis, llm_providers, llm_proxies,
// api_keys). Run via `make it` (sqlite), `make it-postgres`, `make it-sqlserver`.

package integration

import (
	"testing"

	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
)

// TestIdentity_GetOrCreateUUID_IsIdempotentAndReversible verifies the core
// sub<->UUID mapping contract: repeated resolution of the same identity
// returns the same UUID, and the reverse lookup finds it.
func TestIdentity_GetOrCreateUUID_IsIdempotentAndReversible(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()

	repo := repository.NewUserIdentityMappingRepo(it.db)

	uuid1, err := repo.GetOrCreateUUID("it-sub-idempotent")
	if err != nil {
		t.Fatalf("[%s] GetOrCreateUUID failed: %v", it.driver, err)
	}
	uuid2, err := repo.GetOrCreateUUID("it-sub-idempotent")
	if err != nil {
		t.Fatalf("[%s] GetOrCreateUUID (second call) failed: %v", it.driver, err)
	}
	if uuid1 != uuid2 {
		t.Fatalf("[%s] expected the same UUID on repeated resolution, got %q then %q", it.driver, uuid1, uuid2)
	}

	sub, found, err := repo.GetSubByUUID(uuid1)
	if err != nil {
		t.Fatalf("[%s] GetSubByUUID failed: %v", it.driver, err)
	}
	if !found || sub != "it-sub-idempotent" {
		t.Fatalf("[%s] expected reverse lookup to find it-sub-idempotent, got found=%v sub=%q", it.driver, found, sub)
	}
}

// TestIdentity_GetSubByUUID_NotFoundAfterDelete mimics forcefully removing a
// user: after the user_idp_references row is deleted, the reverse lookup must
// report not-found (the contract behind constants.DeletedUser at the service
// layer) rather than erroring.
func TestIdentity_GetSubByUUID_NotFoundAfterDelete(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()

	repo := repository.NewUserIdentityMappingRepo(it.db)
	uuid, err := repo.GetOrCreateUUID("it-sub-to-delete")
	if err != nil {
		t.Fatalf("[%s] GetOrCreateUUID failed: %v", it.driver, err)
	}

	it.exec(t, `DELETE FROM user_idp_references WHERE idp_id = ?`, "it-sub-to-delete")

	_, found, err := repo.GetSubByUUID(uuid)
	if err != nil {
		t.Fatalf("[%s] GetSubByUUID after delete failed: %v", it.driver, err)
	}
	if found {
		t.Fatalf("[%s] expected found=false after the mapping row is deleted", it.driver)
	}
}

// TestMembership_AddListCountAndOrgDeleteCascade exercises
// user_organization_mappings end to end: add, list, count, and — the point of
// running this across every engine — that deleting the parent organization
// removes the membership row via ON DELETE CASCADE (or the repo's
// defense-in-depth manual delete on engines where that isn't guaranteed).
func TestMembership_AddListCountAndOrgDeleteCascade(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()

	identityRepo := repository.NewUserIdentityMappingRepo(it.db)
	orgRepo := repository.NewOrganizationRepo(it.db)
	membershipRepo := repository.NewUserOrganizationMappingRepo(it.db)

	userUUID, err := identityRepo.GetOrCreateUUID("it-sub-member")
	if err != nil {
		t.Fatalf("[%s] GetOrCreateUUID failed: %v", it.driver, err)
	}

	orgA := &model.Organization{ID: id(), Handle: "it-org-a-" + id()[:8], Name: "IT Org A", Region: "us"}
	orgB := &model.Organization{ID: id(), Handle: "it-org-b-" + id()[:8], Name: "IT Org B", Region: "us"}
	if err := orgRepo.CreateOrganization(orgA); err != nil {
		t.Fatalf("[%s] create orgA failed: %v", it.driver, err)
	}
	if err := orgRepo.CreateOrganization(orgB); err != nil {
		t.Fatalf("[%s] create orgB failed: %v", it.driver, err)
	}

	if err := membershipRepo.AddMembership(userUUID, orgA.ID); err != nil {
		t.Fatalf("[%s] AddMembership(orgA) failed: %v", it.driver, err)
	}
	if err := membershipRepo.AddMembership(userUUID, orgB.ID); err != nil {
		t.Fatalf("[%s] AddMembership(orgB) failed: %v", it.driver, err)
	}
	// Idempotent re-add must not error or duplicate.
	if err := membershipRepo.AddMembership(userUUID, orgA.ID); err != nil {
		t.Fatalf("[%s] AddMembership(orgA) duplicate should be a no-op, got: %v", it.driver, err)
	}

	total, err := orgRepo.CountOrganizationsForUser(userUUID)
	if err != nil {
		t.Fatalf("[%s] CountOrganizationsForUser failed: %v", it.driver, err)
	}
	if total != 2 {
		t.Fatalf("[%s] expected 2 memberships, got %d", it.driver, total)
	}

	orgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("[%s] ListOrganizationsForUser failed: %v", it.driver, err)
	}
	if len(orgs) != 2 {
		t.Fatalf("[%s] expected 2 orgs listed, got %d", it.driver, len(orgs))
	}

	// Deleting orgA must remove its membership row (cascade or manual delete),
	// leaving orgB's membership untouched.
	if err := orgRepo.DeleteOrganization(orgA.ID); err != nil {
		t.Fatalf("[%s] DeleteOrganization(orgA) failed: %v", it.driver, err)
	}

	remaining, err := orgRepo.CountOrganizationsForUser(userUUID)
	if err != nil {
		t.Fatalf("[%s] CountOrganizationsForUser after delete failed: %v", it.driver, err)
	}
	if remaining != 1 {
		t.Fatalf("[%s] expected 1 membership remaining after orgA delete, got %d", it.driver, remaining)
	}

	remainingOrgs, err := orgRepo.ListOrganizationsForUser(userUUID, 20, 0)
	if err != nil {
		t.Fatalf("[%s] ListOrganizationsForUser after delete failed: %v", it.driver, err)
	}
	if len(remainingOrgs) != 1 || remainingOrgs[0].ID != orgB.ID {
		t.Fatalf("[%s] expected only orgB to remain, got %+v", it.driver, remainingOrgs)
	}
}

// TestAuditColumns_UpdatedBySetOnCreate_RestAPI guards the previously-missing
// updated_by column on rest_apis creation across every engine's INSERT
// placeholder/rebind path.
func TestAuditColumns_UpdatedBySetOnCreate_RestAPI(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "audit-api")

	apiRepo := repository.NewAPIRepo(it.db)
	api := &model.API{
		Handle:          "audit-api-" + id()[:8],
		Name:            "Audit API",
		Version:         "v1.0",
		CreatedBy:       "it-creator",
		UpdatedBy:       "it-creator",
		ProjectID:       projID,
		OrganizationID:  orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Audit API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		t.Fatalf("[%s] CreateAPI failed: %v", it.driver, err)
	}

	got, err := apiRepo.GetAPIByUUID(api.ID, orgID)
	if err != nil {
		t.Fatalf("[%s] GetAPIByUUID failed: %v", it.driver, err)
	}
	if got.UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected updated_by=it-creator on creation, got %q", it.driver, got.UpdatedBy)
	}
}

// TestAuditColumns_UpdatedBySetOnCreate_LLMProviderAndProxy guards the
// previously-missing updated_by column (and the previously-missing SELECT of
// it) on llm_providers/llm_proxies creation and read-back.
func TestAuditColumns_UpdatedBySetOnCreate_LLMProviderAndProxy(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "audit-llm")

	templateRepo := repository.NewLLMProviderTemplateRepo(it.db)
	template := &model.LLMProviderTemplate{
		OrganizationUUID: orgID,
		ID:               "audit-template-" + id()[:8],
		GroupID:          "audit-template-" + id()[:8],
		Name:             "Audit Template",
		ManagedBy:        "wso2",
		Version:          "v1.0",
	}
	if err := templateRepo.Create(template); err != nil {
		t.Fatalf("[%s] template create failed: %v", it.driver, err)
	}

	providerRepo := repository.NewLLMProviderRepo(it.db)
	provider := &model.LLMProvider{
		OrganizationUUID: orgID,
		ID:               "audit-provider-" + id()[:8],
		Name:             "Audit Provider",
		Version:          "v1.0",
		TemplateUUID:     template.UUID,
		CreatedBy:        "it-creator",
		UpdatedBy:        "it-creator",
	}
	if err := providerRepo.Create(provider); err != nil {
		t.Fatalf("[%s] provider create failed: %v", it.driver, err)
	}

	gotProvider, err := providerRepo.GetByID(provider.ID, orgID)
	if err != nil {
		t.Fatalf("[%s] provider GetByID failed: %v", it.driver, err)
	}
	if gotProvider.UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected provider updated_by=it-creator, got %q", it.driver, gotProvider.UpdatedBy)
	}

	proxyRepo := repository.NewLLMProxyRepo(it.db)
	proxy := &model.LLMProxy{
		OrganizationUUID: orgID,
		ProjectUUID:      projID,
		ID:               "audit-proxy-" + id()[:8],
		Name:             "Audit Proxy",
		Version:          "v1.0",
		ProviderUUID:     provider.UUID,
		CreatedBy:        "it-creator",
		UpdatedBy:        "it-creator",
	}
	if err := proxyRepo.Create(proxy); err != nil {
		t.Fatalf("[%s] proxy create failed: %v", it.driver, err)
	}

	gotProxy, err := proxyRepo.GetByID(proxy.ID, orgID)
	if err != nil {
		t.Fatalf("[%s] proxy GetByID failed: %v", it.driver, err)
	}
	if gotProxy.UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected proxy updated_by=it-creator, got %q", it.driver, gotProxy.UpdatedBy)
	}
}

// TestAuditColumns_APIKeyCreateUpdateRevoke_SetUpdatedBy guards the
// previously-missing updated_by on api_keys creation, and confirms Update and
// Revoke each set updated_by (to a different actor) without touching
// created_by, across every engine's placeholder/rebind path.
func TestAuditColumns_APIKeyCreateUpdateRevoke_SetUpdatedBy(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "audit-apikey")

	apiRepo := repository.NewAPIRepo(it.db)
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
		t.Fatalf("[%s] CreateAPI failed: %v", it.driver, err)
	}

	keyRepo := repository.NewAPIKeyRepo(it.db, nil)
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
		t.Fatalf("[%s] APIKey Create failed: %v", it.driver, err)
	}

	afterCreate, err := keyRepo.GetByArtifactAndName(api.ID, "it-key")
	if err != nil {
		t.Fatalf("[%s] GetByArtifactAndName after create failed: %v", it.driver, err)
	}
	if afterCreate.UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected updated_by=it-creator on creation, got %q", it.driver, afterCreate.UpdatedBy)
	}

	afterCreate.MaskedAPIKey = "cd34"
	afterCreate.UpdatedBy = "it-updater"
	if err := keyRepo.Update(afterCreate); err != nil {
		t.Fatalf("[%s] APIKey Update failed: %v", it.driver, err)
	}

	afterUpdate, err := keyRepo.GetByArtifactAndName(api.ID, "it-key")
	if err != nil {
		t.Fatalf("[%s] GetByArtifactAndName after update failed: %v", it.driver, err)
	}
	if afterUpdate.UpdatedBy != "it-updater" {
		t.Fatalf("[%s] expected updated_by=it-updater after update, got %q", it.driver, afterUpdate.UpdatedBy)
	}
	if afterUpdate.CreatedBy != "it-creator" {
		t.Fatalf("[%s] Update must not touch created_by, got %q", it.driver, afterUpdate.CreatedBy)
	}

	if err := keyRepo.Revoke(api.ID, "it-key", "it-revoker"); err != nil {
		t.Fatalf("[%s] APIKey Revoke failed: %v", it.driver, err)
	}

	afterRevoke, err := keyRepo.GetByArtifactAndName(api.ID, "it-key")
	if err != nil {
		t.Fatalf("[%s] GetByArtifactAndName after revoke failed: %v", it.driver, err)
	}
	if afterRevoke.Status != "revoked" {
		t.Fatalf("[%s] expected status=revoked, got %q", it.driver, afterRevoke.Status)
	}
	if afterRevoke.UpdatedBy != "it-revoker" {
		t.Fatalf("[%s] expected updated_by=it-revoker after revoke, got %q", it.driver, afterRevoke.UpdatedBy)
	}
	if afterRevoke.CreatedBy != "it-creator" {
		t.Fatalf("[%s] Revoke must not touch created_by, got %q", it.driver, afterRevoke.CreatedBy)
	}
}

// seedArtifactAndGateway inserts an artifacts row (the FK target for
// artifact_gateway_mappings / deployment_status) and a gateway, returning both
// UUIDs. Used by the association/deployment-status audit tests below.
func seedArtifactAndGateway(t *testing.T, it *itDB, orgID, prefix string) (artifactID, gatewayID string) {
	t.Helper()
	artifactID = "art-" + id()[:8]
	if _, err := it.db.Exec(it.db.Rebind(
		`INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`),
		artifactID, "RestApi", orgID); err != nil {
		t.Fatalf("[%s] insert artifact failed: %v", it.driver, err)
	}
	gwRepo := repository.NewGatewayRepo(it.db)
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
		t.Fatalf("[%s] gateway Create failed: %v", it.driver, err)
	}
	return artifactID, gw.ID
}

// TestAuditColumns_ArtifactGatewayMapping_SetCreatedUpdatedBy guards the
// previously-NULL created_by/updated_by on artifact_gateway_mappings: create
// seeds both from the acting user, and update bumps updated_by while leaving
// created_by. Runs against every engine.
func TestAuditColumns_ArtifactGatewayMapping_SetCreatedUpdatedBy(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "audit-agm")
	artifactID, gatewayID := seedArtifactAndGateway(t, it, orgID, "agm")

	apiRepo := repository.NewAPIRepo(it.db)
	if err := apiRepo.CreateAPIAssociation(&model.APIAssociation{
		ArtifactID:     artifactID,
		OrganizationID: orgID,
		GatewayID:      gatewayID,
		CreatedBy:      "it-creator",
	}); err != nil {
		t.Fatalf("[%s] CreateAPIAssociation failed: %v", it.driver, err)
	}

	assocs, err := apiRepo.GetAPIAssociations(artifactID, "gateway", orgID)
	if err != nil {
		t.Fatalf("[%s] GetAPIAssociations failed: %v", it.driver, err)
	}
	if len(assocs) != 1 {
		t.Fatalf("[%s] expected 1 association, got %d", it.driver, len(assocs))
	}
	if assocs[0].CreatedBy != "it-creator" || assocs[0].UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected created_by=updated_by=it-creator on create, got created_by=%q updated_by=%q",
			it.driver, assocs[0].CreatedBy, assocs[0].UpdatedBy)
	}

	if err := apiRepo.UpdateAPIAssociation(artifactID, gatewayID, "gateway", orgID, "it-updater"); err != nil {
		t.Fatalf("[%s] UpdateAPIAssociation failed: %v", it.driver, err)
	}
	assocs, err = apiRepo.GetAPIAssociations(artifactID, "gateway", orgID)
	if err != nil {
		t.Fatalf("[%s] GetAPIAssociations (after update) failed: %v", it.driver, err)
	}
	if assocs[0].CreatedBy != "it-creator" || assocs[0].UpdatedBy != "it-updater" {
		t.Fatalf("[%s] expected created_by=it-creator updated_by=it-updater after update, got created_by=%q updated_by=%q",
			it.driver, assocs[0].CreatedBy, assocs[0].UpdatedBy)
	}
}

// TestAuditColumns_DeploymentStatus_SetPerformedBy guards the previously-NULL
// performed_by on deployment_status, seeded from the deploying actor
// (deployment.CreatedBy) at deploy time, across every engine.
func TestAuditColumns_DeploymentStatus_SetPerformedBy(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "audit-ds")
	artifactID, gatewayID := seedArtifactAndGateway(t, it, orgID, "ds")

	depRepo := repository.NewDeploymentRepo(it.db, repository.NewArtifactTableRegistry())
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
		t.Fatalf("[%s] CreateWithLimitEnforcement failed: %v", it.driver, err)
	}

	var performedBy string
	if err := it.db.QueryRow(it.db.Rebind(
		`SELECT performed_by FROM deployment_status WHERE artifact_uuid = ? AND organization_uuid = ? AND gateway_uuid = ?`),
		artifactID, orgID, gatewayID).Scan(&performedBy); err != nil {
		t.Fatalf("[%s] query performed_by failed: %v", it.driver, err)
	}
	if performedBy != "it-deployer" {
		t.Fatalf("[%s] expected performed_by=it-deployer, got %q", it.driver, performedBy)
	}
}

// TestAuditColumns_SubscriptionPlanCreateUpdate_SetCreatedUpdatedBy guards
// subscription_plans.created_by/updated_by: populated on create (updated_by
// mirrors created_by), and Update sets updated_by to a new actor while
// leaving created_by untouched — the same convention already guarded for
// rest_apis/llm_providers/llm_proxies/api_keys, closing a coverage gap this
// table never had (existing tests either raw-SQL-seed it for cascade checks,
// or construct a bare model for unrelated list/hydrate assertions, so
// neither exercises the audit columns).
func TestAuditColumns_SubscriptionPlanCreateUpdate_SetCreatedUpdatedBy(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "audit-plan")

	planRepo := repository.NewSubscriptionPlanRepo(it.db)
	plan := &model.SubscriptionPlan{
		Handle:           "audit-plan-" + id()[:8],
		Name:             "Audit Plan",
		OrganizationUUID: orgID,
		Status:           model.SubscriptionPlanStatusActive,
		CreatedBy:        "it-creator",
		UpdatedBy:        "it-creator",
	}
	if err := planRepo.Create(plan); err != nil {
		t.Fatalf("[%s] SubscriptionPlan Create failed: %v", it.driver, err)
	}

	got, err := planRepo.GetByHandleAndOrg(plan.Handle, orgID)
	if err != nil {
		t.Fatalf("[%s] GetByHandleAndOrg failed: %v", it.driver, err)
	}
	if got.CreatedBy != "it-creator" || got.UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected created_by=updated_by=it-creator on creation, got created_by=%q updated_by=%q",
			it.driver, got.CreatedBy, got.UpdatedBy)
	}

	got.UpdatedBy = "it-updater"
	if err := planRepo.Update(got); err != nil {
		t.Fatalf("[%s] SubscriptionPlan Update failed: %v", it.driver, err)
	}

	after, err := planRepo.GetByHandleAndOrg(plan.Handle, orgID)
	if err != nil {
		t.Fatalf("[%s] GetByHandleAndOrg (after update) failed: %v", it.driver, err)
	}
	if after.UpdatedBy != "it-updater" {
		t.Fatalf("[%s] expected updated_by=it-updater after update, got %q", it.driver, after.UpdatedBy)
	}
	if after.CreatedBy != "it-creator" {
		t.Fatalf("[%s] Update must not touch created_by, got %q", it.driver, after.CreatedBy)
	}
}

// TestAuditColumns_SubscriptionCreateUpdate_SetCreatedUpdatedBy guards
// subscriptions.created_by/updated_by with the same create+update contract.
// Closes the same kind of gap as the subscription_plans test above: the only
// existing coverage of this table is cascade_test.go's raw-SQL seed helper,
// which deliberately omits every audit column since it exists purely to
// exercise FK cascade-delete behavior.
func TestAuditColumns_SubscriptionCreateUpdate_SetCreatedUpdatedBy(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "audit-sub")

	artifactUUID := id()
	it.exec(t, `INSERT INTO artifacts (uuid, type, organization_uuid) VALUES (?, ?, ?)`,
		artifactUUID, "rest_api", orgID)

	subRepo := repository.NewSubscriptionRepo(it.db)
	sub := &model.Subscription{
		ArtifactUUID:     artifactUUID,
		SubscriberID:     "subscriber-" + id()[:8],
		OrganizationUUID: orgID,
		Status:           model.SubscriptionStatusActive,
		CreatedBy:        "it-creator",
		UpdatedBy:        "it-creator",
	}
	if err := subRepo.Create(sub); err != nil {
		t.Fatalf("[%s] Subscription Create failed: %v", it.driver, err)
	}

	got, err := subRepo.GetByID(sub.UUID, orgID)
	if err != nil {
		t.Fatalf("[%s] GetByID failed: %v", it.driver, err)
	}
	if got.CreatedBy != "it-creator" || got.UpdatedBy != "it-creator" {
		t.Fatalf("[%s] expected created_by=updated_by=it-creator on creation, got created_by=%q updated_by=%q",
			it.driver, got.CreatedBy, got.UpdatedBy)
	}

	got.UpdatedBy = "it-updater"
	if err := subRepo.Update(got); err != nil {
		t.Fatalf("[%s] Subscription Update failed: %v", it.driver, err)
	}

	after, err := subRepo.GetByID(sub.UUID, orgID)
	if err != nil {
		t.Fatalf("[%s] GetByID (after update) failed: %v", it.driver, err)
	}
	if after.UpdatedBy != "it-updater" {
		t.Fatalf("[%s] expected updated_by=it-updater after update, got %q", it.driver, after.UpdatedBy)
	}
	if after.CreatedBy != "it-creator" {
		t.Fatalf("[%s] Update must not touch created_by, got %q", it.driver, after.CreatedBy)
	}
}
