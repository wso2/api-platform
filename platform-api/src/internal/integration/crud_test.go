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

package integration

import (
	"fmt"
	"testing"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// seedOrgProject creates one organization and one project through the real
// repositories and returns their UUIDs. It is the lightweight parent graph for
// the CRUD tests below (REST APIs, gateways and API keys all hang off these).
func seedOrgProject(t *testing.T, it *itDB, prefix string) (orgID, projID string) {
	t.Helper()
	orgRepo := repository.NewOrganizationRepo(it.db)
	projectRepo := repository.NewProjectRepo(it.db)

	org := &model.Organization{ID: id(), Handle: prefix + "-" + id()[:8], Name: prefix + " org", Region: "us"}
	if err := orgRepo.CreateOrganization(org); err != nil {
		t.Fatalf("[%s] create org failed: %v", it.driver, err)
	}
	projID = id()
	pName := prefix + "-proj-" + projID[:6]
	proj := &model.Project{ID: projID, Handle: pName, Name: pName, OrganizationID: org.ID, Description: "p"}
	if err := projectRepo.CreateProject(proj); err != nil {
		t.Fatalf("[%s] create project failed: %v", it.driver, err)
	}
	return org.ID, projID
}

// TestLifecycle_RestAPICreateUpdateAndList drives the real APIRepo through the
// full create → read → update → list path (the rest_apis two-table write of
// artifacts + rest_apis, the unique handle/name+version constraints, and the
// org/project listing queries) across the active engine. This covers the API
// creation, update and listing flows that the cascade suite only sets up by
// raw INSERT.
func TestLifecycle_RestAPICreateUpdateAndList(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "api")
	apiRepo := repository.NewAPIRepo(it.db)

	// --- Create ---
	const n = 3
	createdIDs := make([]string, 0, n)
	for i := range n {
		api := &model.API{
			Handle:          fmt.Sprintf("rest-api-%d-%s", i, id()[:6]),
			Name:            fmt.Sprintf("Rest API %d %s", i, id()[:6]),
			Version:         "v1.0",
			Description:     "created by integration test",
			ProjectID:       projID,
			OrganizationID:  orgID,
			LifeCycleStatus: "CREATED",
			Configuration:   model.RestAPIConfig{Name: fmt.Sprintf("Rest API %d", i), Version: "v1.0"},
		}
		if err := apiRepo.CreateAPI(api); err != nil {
			t.Fatalf("[%s] CreateAPI %d failed: %v", it.driver, i, err)
		}
		// CreateAPI generates the UUID itself and writes it back onto the model.
		if api.ID == "" {
			t.Fatalf("[%s] CreateAPI %d did not populate the generated ID", it.driver, i)
		}
		createdIDs = append(createdIDs, api.ID)
	}

	// --- Read back a single API and confirm the round-trip ---
	first, err := apiRepo.GetAPIByUUID(createdIDs[0], orgID)
	if err != nil {
		t.Fatalf("[%s] GetAPIByUUID failed: %v", it.driver, err)
	}
	if first == nil {
		t.Fatalf("[%s] GetAPIByUUID: want the created API, got nil", it.driver)
	}
	if first.LifeCycleStatus != "CREATED" || first.ProjectID != projID {
		t.Fatalf("[%s] GetAPIByUUID round-trip mismatch: status=%q project=%q", it.driver, first.LifeCycleStatus, first.ProjectID)
	}

	// Handle existence check (true for a created handle, false for a random one).
	exists, err := apiRepo.CheckAPIExistsByHandleInOrganization(first.Handle, orgID)
	if err != nil {
		t.Fatalf("[%s] CheckAPIExistsByHandleInOrganization failed: %v", it.driver, err)
	}
	if !exists {
		t.Fatalf("[%s] CheckAPIExistsByHandleInOrganization: want true for %q", it.driver, first.Handle)
	}
	missing, err := apiRepo.CheckAPIExistsByHandleInOrganization("no-such-handle-"+id(), orgID)
	if err != nil {
		t.Fatalf("[%s] CheckAPIExistsByHandleInOrganization(missing) failed: %v", it.driver, err)
	}
	if missing {
		t.Fatalf("[%s] CheckAPIExistsByHandleInOrganization: want false for a random handle", it.driver)
	}

	// --- Update ---
	first.Name = first.Name + " (updated)"
	first.Description = "updated by integration test"
	first.LifeCycleStatus = "PUBLISHED"
	first.UpdatedBy = "it-user"
	if err := apiRepo.UpdateAPI(first); err != nil {
		t.Fatalf("[%s] UpdateAPI failed: %v", it.driver, err)
	}
	updated, err := apiRepo.GetAPIByUUID(first.ID, orgID)
	if err != nil {
		t.Fatalf("[%s] GetAPIByUUID after update failed: %v", it.driver, err)
	}
	if updated.Name != first.Name || updated.LifeCycleStatus != "PUBLISHED" || updated.Description != "updated by integration test" {
		t.Fatalf("[%s] UpdateAPI did not persist: name=%q status=%q desc=%q",
			it.driver, updated.Name, updated.LifeCycleStatus, updated.Description)
	}
	if updated.UpdatedBy != "it-user" {
		t.Fatalf("[%s] UpdateAPI did not persist updated_by: got %q", it.driver, updated.UpdatedBy)
	}

	// --- List (by organization and by project) ---
	byOrg, err := apiRepo.GetAPIsByOrganizationUUID(orgID, "")
	if err != nil {
		t.Fatalf("[%s] GetAPIsByOrganizationUUID failed: %v", it.driver, err)
	}
	if len(byOrg) != n {
		t.Fatalf("[%s] GetAPIsByOrganizationUUID: want %d, got %d", it.driver, n, len(byOrg))
	}
	byProject, err := apiRepo.GetAPIsByProjectUUID(projID, orgID)
	if err != nil {
		t.Fatalf("[%s] GetAPIsByProjectUUID failed: %v", it.driver, err)
	}
	if len(byProject) != n {
		t.Fatalf("[%s] GetAPIsByProjectUUID: want %d, got %d", it.driver, n, len(byProject))
	}
	// Org-scoped filter on a different project returns nothing.
	byOther, err := apiRepo.GetAPIsByOrganizationUUID(orgID, id())
	if err != nil {
		t.Fatalf("[%s] GetAPIsByOrganizationUUID(other project) failed: %v", it.driver, err)
	}
	if len(byOther) != 0 {
		t.Fatalf("[%s] GetAPIsByOrganizationUUID(other project): want 0, got %d", it.driver, len(byOther))
	}
}

// TestLifecycle_GatewayCreateListAndTokenGenerate drives the real GatewayRepo
// through gateway creation, lookup and listing, then through registration-token
// generation, lookup and revocation (the gateway_tokens table). This covers the
// gateway creation and gateway token generation flows across the active engine.
func TestLifecycle_GatewayCreateListAndTokenGenerate(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "gw")
	gwRepo := repository.NewGatewayRepo(it.db)

	// --- Create gateways ---
	const n = 3
	gatewayIDs := make([]string, 0, n)
	for i := range n {
		gw := &model.Gateway{
			ID:                id(),
			OrganizationID:    orgID,
			Name:              fmt.Sprintf("gateway %d", i),
			Handle:            fmt.Sprintf("gw-%d-%s", i, id()[:6]),
			Description:       "created by integration test",
			Endpoints:         []string{"https://localhost:443"},
			FunctionalityType: "REGULAR",
			Version:           "1.0.0",
			Properties:        map[string]interface{}{"region": "us"},
			CreatedBy:         "it-user",
		}
		if err := gwRepo.Create(gw); err != nil {
			t.Fatalf("[%s] gateway Create %d failed: %v", it.driver, i, err)
		}
		gatewayIDs = append(gatewayIDs, gw.ID)
	}

	// --- Read back a single gateway and confirm the round-trip ---
	first := gatewayIDs[0]
	got, err := gwRepo.GetByUUID(first)
	if err != nil {
		t.Fatalf("[%s] GetByUUID failed: %v", it.driver, err)
	}
	if got == nil || got.OrganizationID != orgID {
		t.Fatalf("[%s] GetByUUID round-trip mismatch: %+v", it.driver, got)
	}
	if got.IsActive {
		t.Fatalf("[%s] new gateway should default to inactive, got is_active=true", it.driver)
	}
	if got.Properties["region"] != "us" {
		t.Fatalf("[%s] gateway properties did not round-trip: %+v", it.driver, got.Properties)
	}
	byHandle, err := gwRepo.GetByHandleAndOrgID(got.Handle, orgID)
	if err != nil || byHandle == nil || byHandle.ID != first {
		t.Fatalf("[%s] GetByHandleAndOrgID mismatch: err=%v got=%+v", it.driver, err, byHandle)
	}

	// --- List by organization ---
	list, err := gwRepo.GetByOrganizationID(orgID)
	if err != nil {
		t.Fatalf("[%s] GetByOrganizationID failed: %v", it.driver, err)
	}
	if len(list) != n {
		t.Fatalf("[%s] GetByOrganizationID: want %d, got %d", it.driver, n, len(list))
	}

	// --- Generate a registration token for the gateway ---
	tokenHash := "hash-" + id()
	token := &model.GatewayToken{
		ID:        id(),
		GatewayID: first,
		TokenHash: tokenHash,
		Salt:      "salt-" + id()[:8],
		Status:    constants.GatewayTokenStatusActive,
		CreatedBy: "it-user",
	}
	if err := gwRepo.CreateToken(token); err != nil {
		t.Fatalf("[%s] CreateToken failed: %v", it.driver, err)
	}

	if count, err := gwRepo.CountActiveTokens(first); err != nil || count != 1 {
		t.Fatalf("[%s] CountActiveTokens: want 1, got %d (err=%v)", it.driver, count, err)
	}
	active, err := gwRepo.GetActiveTokensByGatewayUUID(first)
	if err != nil || len(active) != 1 {
		t.Fatalf("[%s] GetActiveTokensByGatewayUUID: want 1, got %d (err=%v)", it.driver, len(active), err)
	}
	byHash, err := gwRepo.GetActiveTokenByHash(tokenHash)
	if err != nil {
		t.Fatalf("[%s] GetActiveTokenByHash failed: %v", it.driver, err)
	}
	if byHash == nil || byHash.ID != token.ID || byHash.GatewayID != first {
		t.Fatalf("[%s] GetActiveTokenByHash mismatch: %+v", it.driver, byHash)
	}

	// --- Revoke the token; it should no longer count as active ---
	if err := gwRepo.RevokeToken(token.ID, "it-user"); err != nil {
		t.Fatalf("[%s] RevokeToken failed: %v", it.driver, err)
	}
	if count, err := gwRepo.CountActiveTokens(first); err != nil || count != 0 {
		t.Fatalf("[%s] CountActiveTokens after revoke: want 0, got %d (err=%v)", it.driver, count, err)
	}
	if revoked, err := gwRepo.GetActiveTokenByHash(tokenHash); err != nil || revoked != nil {
		t.Fatalf("[%s] GetActiveTokenByHash after revoke: want nil, got %+v (err=%v)", it.driver, revoked, err)
	}
	gone, err := gwRepo.GetTokenByUUID(token.ID)
	if err != nil {
		t.Fatalf("[%s] GetTokenByUUID after revoke failed: %v", it.driver, err)
	}
	if gone == nil || gone.Status != constants.GatewayTokenStatusRevoked || gone.RevokedBy == nil || *gone.RevokedBy != "it-user" {
		t.Fatalf("[%s] revoked token state mismatch: %+v", it.driver, gone)
	}
}

// TestLifecycle_GatewayEndpointRoundTrip verifies that GetByOrganizationID and
// List correctly return endpoints via the JOIN query introduced to replace the
// nested-query pattern that deadlocked under SQLite's single-connection pool.
// It covers: multiple endpoints per gateway, zero endpoints, cross-gateway
// endpoint isolation, insertion-order preservation, and post-update re-listing.
func TestLifecycle_GatewayEndpointRoundTrip(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, _ := seedOrgProject(t, it, "ep")
	gwRepo := repository.NewGatewayRepo(it.db)

	twoEndpoints := []string{
		"https://primary.example.com:443/api",
		"http://secondary.example.com:8080/api",
	}
	oneEndpoint := []string{
		"https://single.example.com:443/",
	}

	gwMulti := &model.Gateway{
		ID: id(), OrganizationID: orgID,
		Name: "multi-ep", Handle: "multi-ep-" + id()[:6],
		FunctionalityType: "REGULAR", Version: "1.0.0",
		Endpoints: twoEndpoints,
	}
	gwSingle := &model.Gateway{
		ID: id(), OrganizationID: orgID,
		Name: "single-ep", Handle: "single-ep-" + id()[:6],
		FunctionalityType: "REGULAR", Version: "1.0.0",
		Endpoints: oneEndpoint,
	}

	for _, gw := range []*model.Gateway{gwMulti, gwSingle} {
		if err := gwRepo.Create(gw); err != nil {
			t.Fatalf("[%s] Create(%s) failed: %v", it.driver, gw.Handle, err)
		}
	}

	checkEndpoints := func(t *testing.T, label string, gateways []*model.Gateway) {
		t.Helper()
		byID := make(map[string]*model.Gateway, len(gateways))
		for _, g := range gateways {
			byID[g.ID] = g
		}

		got := byID[gwMulti.ID]
		if got == nil {
			t.Fatalf("[%s] %s: multi-endpoint gateway missing from results", it.driver, label)
		}
		if len(got.Endpoints) != len(twoEndpoints) {
			t.Fatalf("[%s] %s: want %d endpoints for multi-ep gateway, got %d", it.driver, label, len(twoEndpoints), len(got.Endpoints))
		}
		for i, want := range twoEndpoints {
			if got.Endpoints[i] != want {
				t.Fatalf("[%s] %s: endpoint[%d] mismatch: want %q, got %q", it.driver, label, i, want, got.Endpoints[i])
			}
		}

		gotSingle := byID[gwSingle.ID]
		if gotSingle == nil {
			t.Fatalf("[%s] %s: single-endpoint gateway missing from results", it.driver, label)
		}
		if len(gotSingle.Endpoints) != 1 || gotSingle.Endpoints[0] != "https://single.example.com:443/" {
			t.Fatalf("[%s] %s: single-ep gateway endpoints mismatch: %+v", it.driver, label, gotSingle.Endpoints)
		}
	}

	// GetByOrganizationID uses the JOIN path; this was the deadlock site.
	list, err := gwRepo.GetByOrganizationID(orgID)
	if err != nil {
		t.Fatalf("[%s] GetByOrganizationID failed: %v", it.driver, err)
	}
	if len(list) != 2 {
		t.Fatalf("[%s] GetByOrganizationID: want 2 gateways, got %d", it.driver, len(list))
	}
	checkEndpoints(t, "GetByOrganizationID", list)

	// List also uses the JOIN path.
	all, err := gwRepo.List()
	if err != nil {
		t.Fatalf("[%s] List failed: %v", it.driver, err)
	}
	checkEndpoints(t, "List", all)

	// Replace the endpoints on the multi-endpoint gateway and verify the list reflects it.
	updatedEndpoints := []string{"https://new.example.com:9443/v2"}
	gwMulti.Endpoints = updatedEndpoints
	gwMulti.UpdatedBy = "it-user"
	if err := gwRepo.UpdateGateway(gwMulti); err != nil {
		t.Fatalf("[%s] UpdateGateway failed: %v", it.driver, err)
	}
	afterUpdate, err := gwRepo.GetByOrganizationID(orgID)
	if err != nil {
		t.Fatalf("[%s] GetByOrganizationID after update failed: %v", it.driver, err)
	}
	byID := make(map[string]*model.Gateway, len(afterUpdate))
	for _, g := range afterUpdate {
		byID[g.ID] = g
	}
	updated := byID[gwMulti.ID]
	if updated == nil {
		t.Fatalf("[%s] updated gateway missing from list", it.driver)
	}
	if len(updated.Endpoints) != 1 || updated.Endpoints[0] != "https://new.example.com:9443/v2" {
		t.Fatalf("[%s] post-update endpoints mismatch: %+v", it.driver, updated.Endpoints)
	}
}

// TestLifecycle_APIKeyCreateListRevoke drives the real APIKeyRepo through key
// creation, lookup, listing, update and revocation. The key hangs off a REST
// API artifact created via APIRepo, so this exercises the api_keys ->
// artifacts foreign key alongside the API key generation flow.
func TestLifecycle_APIKeyCreateListRevoke(t *testing.T) {
	it := openITDB(t)
	defer it.db.Close()
	orgID, projID := seedOrgProject(t, it, "key")

	// An API key references an artifact; create a REST API to back it.
	apiRepo := repository.NewAPIRepo(it.db)
	api := &model.API{
		Handle:          "key-api-" + id()[:8],
		Name:            "Key API " + id()[:6],
		Version:         "v1.0",
		ProjectID:       projID,
		OrganizationID:  orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Key API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		t.Fatalf("[%s] CreateAPI (for key) failed: %v", it.driver, err)
	}
	artifactUUID := api.ID

	keyRepo := repository.NewAPIKeyRepo(it.db, repository.NewArtifactTableRegistry())

	// --- Create two API keys on the same artifact ---
	const n = 2
	for i := range n {
		key := &model.APIKey{
			UUID:           id(),
			ArtifactUUID:   artifactUUID,
			Name:           fmt.Sprintf("key-%d", i),
			MaskedAPIKey:   "ab12",
			APIKeyHashes:   `{"sha256":"` + id() + `"}`,
			Status:         "active",
			CreatedBy:      "it-user",
			AllowedTargets: "ALL",
		}
		if err := keyRepo.Create(key); err != nil {
			t.Fatalf("[%s] APIKey Create %d failed: %v", it.driver, i, err)
		}
	}

	// --- Read back a single key and confirm the round-trip ---
	got, err := keyRepo.GetByArtifactAndName(artifactUUID, "key-0")
	if err != nil {
		t.Fatalf("[%s] GetByArtifactAndName failed: %v", it.driver, err)
	}
	if got == nil || got.ArtifactUUID != artifactUUID || got.Status != "active" {
		t.Fatalf("[%s] GetByArtifactAndName round-trip mismatch: %+v", it.driver, got)
	}
	if got.AllowedTargets != "ALL" {
		t.Fatalf("[%s] APIKey allowed_targets did not round-trip: %q", it.driver, got.AllowedTargets)
	}

	// --- List by artifact ---
	keys, err := keyRepo.ListByArtifact(artifactUUID)
	if err != nil {
		t.Fatalf("[%s] ListByArtifact failed: %v", it.driver, err)
	}
	if len(keys) != n {
		t.Fatalf("[%s] ListByArtifact: want %d, got %d", it.driver, n, len(keys))
	}

	// --- Update the key material ---
	got.MaskedAPIKey = "cd34"
	got.APIKeyHashes = `{"sha256":"` + id() + `"}`
	if err := keyRepo.Update(got); err != nil {
		t.Fatalf("[%s] APIKey Update failed: %v", it.driver, err)
	}
	afterUpdate, err := keyRepo.GetByArtifactAndName(artifactUUID, "key-0")
	if err != nil {
		t.Fatalf("[%s] GetByArtifactAndName after update failed: %v", it.driver, err)
	}
	if afterUpdate.MaskedAPIKey != "cd34" {
		t.Fatalf("[%s] APIKey Update did not persist masked key: %q", it.driver, afterUpdate.MaskedAPIKey)
	}

	// --- Revoke the key ---
	if err := keyRepo.Revoke(artifactUUID, "key-0"); err != nil {
		t.Fatalf("[%s] APIKey Revoke failed: %v", it.driver, err)
	}
	revoked, err := keyRepo.GetByArtifactAndName(artifactUUID, "key-0")
	if err != nil {
		t.Fatalf("[%s] GetByArtifactAndName after revoke failed: %v", it.driver, err)
	}
	if revoked.Status != "revoked" {
		t.Fatalf("[%s] APIKey Revoke did not persist status: %q", it.driver, revoked.Status)
	}
}
