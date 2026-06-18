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

package service

import (
	"fmt"

	"platform-api/src/internal/constants"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// restAPIImporter imports REST API artifacts (project-scoped).
type restAPIImporter struct {
	apiRepo      repository.APIRepository
	artifactRepo repository.ArtifactRepository
}

func newRestAPIImporter(apiRepo repository.APIRepository, artifactRepo repository.ArtifactRepository) *restAPIImporter {
	return &restAPIImporter{apiRepo: apiRepo, artifactRepo: artifactRepo}
}

func (i *restAPIImporter) Kind() string          { return constants.RestApi }
func (i *restAPIImporter) RequiresProject() bool { return true }

func (i *restAPIImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	version := utils.ImportVersion(ctx.Configuration)

	var cfg model.RestAPIConfig
	if err := utils.DecodeSpec(ctx.Configuration.Spec, &cfg); err != nil {
		return nil, err
	}

	if ctx.Existing == nil {
		// Create a new DP-originated REST API preserving the CP UUID.
		api := &model.API{
			ID:              ctx.ID,
			Handle:          utils.ImportHandle(ctx.Configuration),
			Name:            utils.ImportDisplayName(ctx.Configuration),
			Version:         version,
			Kind:            constants.RestApi,
			ProjectID:       ctx.ProjectID,
			OrganizationID:  ctx.OrgID,
			Origin:          constants.OriginDP,
			LifeCycleStatus: "CREATED",
			Transport:       []string{"http", "https"},
			Configuration:   cfg,
		}
		if err := i.apiRepo.CreateAPI(api); err != nil {
			return nil, fmt.Errorf("failed to create REST API from gateway import: %w", err)
		}
		return &ImportResult{ID: api.ID, DeployedVersion: version, Deployable: true}, nil
	}

	// Existing artifact: load it once and mutate only the fields owned by the import, so
	// unrelated fields (description, createdBy, lifecycle status, transport) are preserved
	// rather than reset to zero values by a partial UpdateAPI. Mirrors the MCP/LLM importers.
	existing, err := i.apiRepo.GetAPIByUUID(ctx.ID, ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing REST API: %w", err)
	}
	if existing == nil {
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	}

	switch ctx.MetadataMode {
	case utils.SkipWorkingCopy:
		// Stale, out-of-order push: a newer deployment already defines the working copy.
		// Leave it untouched (the per-gateway deployment status is still recorded upstream).
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	case utils.WriteFullMetadata:
		existing.Name = utils.ImportDisplayName(ctx.Configuration)
		existing.Version = version
		existing.ProjectID = ctx.ProjectID
		existing.Configuration = cfg
	case utils.WriteGatewaySpecificOnly:
		// CP-owned: update only gateway-specific data (upstream).
		existing.Configuration.Upstream = cfg.Upstream
	}
	if err := i.apiRepo.UpdateAPI(existing); err != nil {
		return nil, fmt.Errorf("failed to update REST API from gateway import: %w", err)
	}
	return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
}
