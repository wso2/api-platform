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
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// gatewayRestOperation mirrors the gateway RestApi CR's flat operation shape
// ({method, path, policies}), which differs from model.Operation's nested Request.
type gatewayRestOperation struct {
	Method   string         `json:"method"`
	Path     string         `json:"path"`
	Policies []model.Policy `json:"policies,omitempty"`
}

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
	if err := normalizeGatewayRestOperations(ctx.Configuration.Spec, &cfg); err != nil {
		return nil, err
	}
	// Transport now lives on the configuration; default it when the pushed spec omits it.
	if len(cfg.Transport) == 0 {
		cfg.Transport = []string{"http", "https"}
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
		// The artifact was matched by handle but its REST API row is gone (e.g. a concurrent
		// delete).
		return nil, fmt.Errorf("REST API %q not found for update", ctx.ID)
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

// normalizeGatewayRestOperations converts the gateway's flat operations into the control
// plane's nested shape. Plain decoding into model.Operation silently drops the flat method,
// path and policies, leaving each operation with a nil Request. Operations that already
// carry a nested request are kept as-is.
func normalizeGatewayRestOperations(spec map[string]interface{}, cfg *model.RestAPIConfig) error {
	var gwSpec struct {
		Operations []gatewayRestOperation `json:"operations"`
	}
	if err := utils.DecodeSpec(spec, &gwSpec); err != nil {
		return err
	}
	if len(gwSpec.Operations) != len(cfg.Operations) {
		return nil
	}
	for idx, op := range gwSpec.Operations {
		if cfg.Operations[idx].Request != nil {
			continue
		}
		cfg.Operations[idx].Request = &model.OperationRequest{
			Method:   strings.ToUpper(op.Method), // normalize case before storing (GO-AUTH-006)
			Path:     op.Path,
			Policies: op.Policies,
		}
	}
	return nil
}
