//go:build integration

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

package integration

import (
	"fmt"

	"github.com/cucumber/godog"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerCRUDSteps wires the repository CRUD lifecycle scenarios.
func registerCRUDSteps(ctx *godog.ScenarioContext, w *world) {
	// REST API lifecycle.
	ctx.Step(`^I create (\d+) REST APIs in the project$`, w.createRESTAPIs)
	ctx.Step(`^reading the first REST API back returns lifecycle status "([^"]*)"$`, w.firstRESTAPIHasStatus)
	ctx.Step(`^the first REST API handle is reported as existing$`, w.firstRESTAPIHandleExists)
	ctx.Step(`^a random handle is reported as not existing$`, w.randomHandleMissing)
	ctx.Step(`^I update the first REST API to status "([^"]*)"$`, w.updateFirstRESTAPI)
	ctx.Step(`^reading the first REST API back shows the updated name, status "([^"]*)" and updater "([^"]*)"$`, w.firstRESTAPIUpdated)
	ctx.Step(`^listing REST APIs by organization returns (\d+)$`, w.listRESTAPIsByOrg)
	ctx.Step(`^listing REST APIs by project returns (\d+)$`, w.listRESTAPIsByProject)
	ctx.Step(`^listing REST APIs by organization filtered to another project returns (\d+)$`, w.listRESTAPIsByOtherProject)

	// Gateway + token lifecycle.
	ctx.Step(`^I create (\d+) gateways$`, w.createGateways)
	ctx.Step(`^reading the first gateway back returns it as inactive with property region "([^"]*)"$`, w.firstGatewayInactive)
	ctx.Step(`^the first gateway is found by its handle$`, w.firstGatewayFoundByHandle)
	ctx.Step(`^listing gateways by organization returns (\d+)$`, w.listGatewaysByOrg)
	ctx.Step(`^I generate a registration token for the first gateway$`, w.generateGatewayToken)
	ctx.Step(`^the first gateway has (\d+) active tokens?$`, w.gatewayActiveTokenCount)
	ctx.Step(`^the token is found by its hash$`, w.tokenFoundByHash)
	ctx.Step(`^I revoke the token$`, w.revokeGatewayToken)
	ctx.Step(`^the token is no longer found by its hash$`, w.tokenNotFoundByHash)
	ctx.Step(`^the revoked token records status "([^"]*)" by "([^"]*)"$`, w.revokedTokenState)

	// API key lifecycle.
	ctx.Step(`^a REST API exists to back the API keys$`, w.seedAPIForKeys)
	ctx.Step(`^I create (\d+) API keys on the REST API$`, w.createAPIKeys)
	ctx.Step(`^reading the first API key back returns status "([^"]*)" and target "([^"]*)"$`, w.firstAPIKeyStatusAndTarget)
	ctx.Step(`^listing API keys by artifact returns (\d+)$`, w.listAPIKeysByArtifact)
	ctx.Step(`^I update the first API key material$`, w.updateFirstAPIKey)
	ctx.Step(`^reading the first API key back shows the updated masked key$`, w.firstAPIKeyMaskedUpdated)
	ctx.Step(`^I revoke the first API key$`, w.revokeFirstAPIKey)
	ctx.Step(`^reading the first API key back returns status "([^"]*)"$`, w.firstAPIKeyStatus)
}

// --- REST API ---------------------------------------------------------------

func (w *world) createRESTAPIs(n int) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	w.createdAPIIDs = w.createdAPIIDs[:0]
	for i := range n {
		api := &model.API{
			Handle:          fmt.Sprintf("rest-api-%d-%s", i, id()[:6]),
			Name:            fmt.Sprintf("Rest API %d %s", i, id()[:6]),
			Version:         "v1.0",
			Description:     "created by integration test",
			ProjectID:       w.projID,
			OrganizationID:  w.orgID,
			LifeCycleStatus: "CREATED",
			Configuration:   model.RestAPIConfig{Name: fmt.Sprintf("Rest API %d", i), Version: "v1.0"},
		}
		if err := apiRepo.CreateAPI(api); err != nil {
			return fmt.Errorf("[%s] CreateAPI %d failed: %w", w.it.driver, i, err)
		}
		if api.ID == "" {
			return fmt.Errorf("[%s] CreateAPI %d did not populate the generated ID", w.it.driver, i)
		}
		w.createdAPIIDs = append(w.createdAPIIDs, api.ID)
	}
	return nil
}

func (w *world) firstRESTAPIHasStatus(status string) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	first, err := apiRepo.GetAPIByUUID(w.createdAPIIDs[0], w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetAPIByUUID failed: %w", w.it.driver, err)
	}
	if first == nil {
		return fmt.Errorf("[%s] GetAPIByUUID: want the created API, got nil", w.it.driver)
	}
	if first.LifeCycleStatus != status || first.ProjectID != w.projID {
		return fmt.Errorf("[%s] GetAPIByUUID round-trip mismatch: status=%q project=%q", w.it.driver, first.LifeCycleStatus, first.ProjectID)
	}
	w.firstAPI = first
	return nil
}

func (w *world) firstRESTAPIHandleExists() error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	exists, err := apiRepo.CheckAPIExistsByHandleInOrganization(w.firstAPI.Handle, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] CheckAPIExistsByHandleInOrganization failed: %w", w.it.driver, err)
	}
	if !exists {
		return fmt.Errorf("[%s] CheckAPIExistsByHandleInOrganization: want true for %q", w.it.driver, w.firstAPI.Handle)
	}
	return nil
}

func (w *world) randomHandleMissing() error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	missing, err := apiRepo.CheckAPIExistsByHandleInOrganization("no-such-handle-"+id(), w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] CheckAPIExistsByHandleInOrganization(missing) failed: %w", w.it.driver, err)
	}
	if missing {
		return fmt.Errorf("[%s] CheckAPIExistsByHandleInOrganization: want false for a random handle", w.it.driver)
	}
	return nil
}

func (w *world) updateFirstRESTAPI(status string) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	w.firstAPI.Name = w.firstAPI.Name + " (updated)"
	w.firstAPI.Description = "updated by integration test"
	w.firstAPI.LifeCycleStatus = status
	w.firstAPI.UpdatedBy = "it-user"
	if err := apiRepo.UpdateAPI(w.firstAPI); err != nil {
		return fmt.Errorf("[%s] UpdateAPI failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) firstRESTAPIUpdated(status, updater string) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	updated, err := apiRepo.GetAPIByUUID(w.firstAPI.ID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetAPIByUUID after update failed: %w", w.it.driver, err)
	}
	if updated.Name != w.firstAPI.Name || updated.LifeCycleStatus != status || updated.Description != "updated by integration test" {
		return fmt.Errorf("[%s] UpdateAPI did not persist: name=%q status=%q desc=%q",
			w.it.driver, updated.Name, updated.LifeCycleStatus, updated.Description)
	}
	if updated.UpdatedBy != updater {
		return fmt.Errorf("[%s] UpdateAPI did not persist updated_by: got %q", w.it.driver, updated.UpdatedBy)
	}
	return nil
}

func (w *world) listRESTAPIsByOrg(want int) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	byOrg, err := apiRepo.GetAPIsByOrganizationUUID(w.orgID, "")
	if err != nil {
		return fmt.Errorf("[%s] GetAPIsByOrganizationUUID failed: %w", w.it.driver, err)
	}
	if len(byOrg) != want {
		return fmt.Errorf("[%s] GetAPIsByOrganizationUUID: want %d, got %d", w.it.driver, want, len(byOrg))
	}
	return nil
}

func (w *world) listRESTAPIsByProject(want int) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	byProject, err := apiRepo.GetAPIsByProjectUUID(w.projID, w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetAPIsByProjectUUID failed: %w", w.it.driver, err)
	}
	if len(byProject) != want {
		return fmt.Errorf("[%s] GetAPIsByProjectUUID: want %d, got %d", w.it.driver, want, len(byProject))
	}
	return nil
}

func (w *world) listRESTAPIsByOtherProject(want int) error {
	apiRepo := repository.NewAPIRepo(w.it.db)
	byOther, err := apiRepo.GetAPIsByOrganizationUUID(w.orgID, id())
	if err != nil {
		return fmt.Errorf("[%s] GetAPIsByOrganizationUUID(other project) failed: %w", w.it.driver, err)
	}
	if len(byOther) != want {
		return fmt.Errorf("[%s] GetAPIsByOrganizationUUID(other project): want %d, got %d", w.it.driver, want, len(byOther))
	}
	return nil
}

// --- Gateway + token --------------------------------------------------------

func (w *world) createGateways(n int) error {
	// The org+project are seeded by the feature Background; gateways hang off the org.
	gwRepo := repository.NewGatewayRepo(w.it.db)
	w.gatewayIDs = w.gatewayIDs[:0]
	for i := range n {
		gw := &model.Gateway{
			ID:                id(),
			OrganizationID:    w.orgID,
			Name:              fmt.Sprintf("gateway %d", i),
			Handle:            fmt.Sprintf("gw-%d-%s", i, id()[:6]),
			Description:       "created by integration test",
			Vhost:             "localhost",
			FunctionalityType: "REGULAR",
			Version:           "1.0.0",
			Properties:        map[string]interface{}{"region": "us"},
			CreatedBy:         "it-user",
		}
		if err := gwRepo.Create(gw); err != nil {
			return fmt.Errorf("[%s] gateway Create %d failed: %w", w.it.driver, i, err)
		}
		w.gatewayIDs = append(w.gatewayIDs, gw.ID)
	}
	w.firstGatewayID = w.gatewayIDs[0]
	return nil
}

func (w *world) firstGatewayInactive(region string) error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	got, err := gwRepo.GetByUUID(w.firstGatewayID)
	if err != nil {
		return fmt.Errorf("[%s] GetByUUID failed: %w", w.it.driver, err)
	}
	if got == nil || got.OrganizationID != w.orgID {
		return fmt.Errorf("[%s] GetByUUID round-trip mismatch: %+v", w.it.driver, got)
	}
	if got.IsActive {
		return fmt.Errorf("[%s] new gateway should default to inactive, got is_active=true", w.it.driver)
	}
	if got.Properties["region"] != region {
		return fmt.Errorf("[%s] gateway properties did not round-trip: %+v", w.it.driver, got.Properties)
	}
	return nil
}

func (w *world) firstGatewayFoundByHandle() error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	got, err := gwRepo.GetByUUID(w.firstGatewayID)
	if err != nil {
		return fmt.Errorf("[%s] GetByUUID failed: %w", w.it.driver, err)
	}
	byHandle, err := gwRepo.GetByHandleAndOrgID(got.Handle, w.orgID)
	if err != nil || byHandle == nil || byHandle.ID != w.firstGatewayID {
		return fmt.Errorf("[%s] GetByHandleAndOrgID mismatch: err=%v got=%+v", w.it.driver, err, byHandle)
	}
	return nil
}

func (w *world) listGatewaysByOrg(want int) error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	list, err := gwRepo.GetByOrganizationID(w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByOrganizationID failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] GetByOrganizationID: want %d, got %d", w.it.driver, want, len(list))
	}
	return nil
}

func (w *world) generateGatewayToken() error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	w.tokenHash = "hash-" + id()
	w.gwToken = &model.GatewayToken{
		ID:        id(),
		GatewayID: w.firstGatewayID,
		TokenHash: w.tokenHash,
		Salt:      "salt-" + id()[:8],
		Status:    constants.GatewayTokenStatusActive,
		CreatedBy: "it-user",
	}
	if err := gwRepo.CreateToken(w.gwToken); err != nil {
		return fmt.Errorf("[%s] CreateToken failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) gatewayActiveTokenCount(want int) error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	count, err := gwRepo.CountActiveTokens(w.firstGatewayID)
	if err != nil {
		return fmt.Errorf("[%s] CountActiveTokens failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] CountActiveTokens: want %d, got %d", w.it.driver, want, count)
	}
	if want > 0 {
		active, err := gwRepo.GetActiveTokensByGatewayUUID(w.firstGatewayID)
		if err != nil || len(active) != want {
			return fmt.Errorf("[%s] GetActiveTokensByGatewayUUID: want %d, got %d (err=%v)", w.it.driver, want, len(active), err)
		}
	}
	return nil
}

func (w *world) tokenFoundByHash() error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	byHash, err := gwRepo.GetActiveTokenByHash(w.tokenHash)
	if err != nil {
		return fmt.Errorf("[%s] GetActiveTokenByHash failed: %w", w.it.driver, err)
	}
	if byHash == nil || byHash.ID != w.gwToken.ID || byHash.GatewayID != w.firstGatewayID {
		return fmt.Errorf("[%s] GetActiveTokenByHash mismatch: %+v", w.it.driver, byHash)
	}
	return nil
}

func (w *world) revokeGatewayToken() error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	if err := gwRepo.RevokeToken(w.gwToken.ID, "it-user"); err != nil {
		return fmt.Errorf("[%s] RevokeToken failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) tokenNotFoundByHash() error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	revoked, err := gwRepo.GetActiveTokenByHash(w.tokenHash)
	if err != nil || revoked != nil {
		return fmt.Errorf("[%s] GetActiveTokenByHash after revoke: want nil, got %+v (err=%v)", w.it.driver, revoked, err)
	}
	return nil
}

func (w *world) revokedTokenState(status, by string) error {
	gwRepo := repository.NewGatewayRepo(w.it.db)
	gone, err := gwRepo.GetTokenByUUID(w.gwToken.ID)
	if err != nil {
		return fmt.Errorf("[%s] GetTokenByUUID after revoke failed: %w", w.it.driver, err)
	}
	if gone == nil || string(gone.Status) != status || gone.RevokedBy == nil || *gone.RevokedBy != by {
		return fmt.Errorf("[%s] revoked token state mismatch (want status %q by %q): %+v", w.it.driver, status, by, gone)
	}
	return nil
}

// --- API key ----------------------------------------------------------------

func (w *world) seedAPIForKeys() error {
	// The org+project are seeded by the feature Background; the API hangs off them.
	apiRepo := repository.NewAPIRepo(w.it.db)
	api := &model.API{
		Handle:          "key-api-" + id()[:8],
		Name:            "Key API " + id()[:6],
		Version:         "v1.0",
		ProjectID:       w.projID,
		OrganizationID:  w.orgID,
		LifeCycleStatus: "CREATED",
		Configuration:   model.RestAPIConfig{Name: "Key API", Version: "v1.0"},
	}
	if err := apiRepo.CreateAPI(api); err != nil {
		return fmt.Errorf("[%s] CreateAPI (for key) failed: %w", w.it.driver, err)
	}
	w.artifactUUID = api.ID
	return nil
}

func (w *world) createAPIKeys(n int) error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	for i := range n {
		key := &model.APIKey{
			UUID:           id(),
			ArtifactUUID:   w.artifactUUID,
			Name:           fmt.Sprintf("key-%d", i),
			MaskedAPIKey:   "ab12",
			APIKeyHashes:   `{"sha256":"` + id() + `"}`,
			Status:         "active",
			CreatedBy:      "it-user",
			AllowedTargets: "ALL",
		}
		if err := keyRepo.Create(key); err != nil {
			return fmt.Errorf("[%s] APIKey Create %d failed: %w", w.it.driver, i, err)
		}
	}
	return nil
}

func (w *world) firstAPIKeyStatusAndTarget(status, target string) error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	got, err := keyRepo.GetByArtifactAndName(w.artifactUUID, "key-0")
	if err != nil {
		return fmt.Errorf("[%s] GetByArtifactAndName failed: %w", w.it.driver, err)
	}
	if got == nil || got.ArtifactUUID != w.artifactUUID || got.Status != status {
		return fmt.Errorf("[%s] GetByArtifactAndName round-trip mismatch: %+v", w.it.driver, got)
	}
	if got.AllowedTargets != target {
		return fmt.Errorf("[%s] APIKey allowed_targets did not round-trip: %q", w.it.driver, got.AllowedTargets)
	}
	return nil
}

func (w *world) listAPIKeysByArtifact(want int) error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	keys, err := keyRepo.ListByArtifact(w.artifactUUID)
	if err != nil {
		return fmt.Errorf("[%s] ListByArtifact failed: %w", w.it.driver, err)
	}
	if len(keys) != want {
		return fmt.Errorf("[%s] ListByArtifact: want %d, got %d", w.it.driver, want, len(keys))
	}
	return nil
}

func (w *world) updateFirstAPIKey() error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	got, err := keyRepo.GetByArtifactAndName(w.artifactUUID, "key-0")
	if err != nil {
		return fmt.Errorf("[%s] GetByArtifactAndName failed: %w", w.it.driver, err)
	}
	got.MaskedAPIKey = "cd34"
	got.APIKeyHashes = `{"sha256":"` + id() + `"}`
	if err := keyRepo.Update(got); err != nil {
		return fmt.Errorf("[%s] APIKey Update failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) firstAPIKeyMaskedUpdated() error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	afterUpdate, err := keyRepo.GetByArtifactAndName(w.artifactUUID, "key-0")
	if err != nil {
		return fmt.Errorf("[%s] GetByArtifactAndName after update failed: %w", w.it.driver, err)
	}
	if afterUpdate.MaskedAPIKey != "cd34" {
		return fmt.Errorf("[%s] APIKey Update did not persist masked key: %q", w.it.driver, afterUpdate.MaskedAPIKey)
	}
	return nil
}

func (w *world) revokeFirstAPIKey() error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	if err := keyRepo.Revoke(w.artifactUUID, "key-0"); err != nil {
		return fmt.Errorf("[%s] APIKey Revoke failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) firstAPIKeyStatus(status string) error {
	keyRepo := repository.NewAPIKeyRepo(w.it.db)
	got, err := keyRepo.GetByArtifactAndName(w.artifactUUID, "key-0")
	if err != nil {
		return fmt.Errorf("[%s] GetByArtifactAndName after revoke failed: %w", w.it.driver, err)
	}
	if got.Status != status {
		return fmt.Errorf("[%s] APIKey status: want %q, got %q", w.it.driver, status, got.Status)
	}
	return nil
}
