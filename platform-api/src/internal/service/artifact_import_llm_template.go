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

// llmProviderTemplateImporter imports LLM Provider Template artifacts. Templates are
// organization-level configuration: they are not backed by the artifacts table and
// have no per-gateway deployment lifecycle.
type llmProviderTemplateImporter struct {
	templateRepo repository.LLMProviderTemplateRepository
}

func newLLMProviderTemplateImporter(templateRepo repository.LLMProviderTemplateRepository) *llmProviderTemplateImporter {
	return &llmProviderTemplateImporter{templateRepo: templateRepo}
}

func (i *llmProviderTemplateImporter) Kind() string          { return constants.LLMProviderTemplate }
func (i *llmProviderTemplateImporter) RequiresProject() bool { return false }

func (i *llmProviderTemplateImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	version := utils.ImportVersion(ctx.Configuration)

	// Decode the configuration-bearing fields without clobbering identity fields.
	var specTmpl model.LLMProviderTemplate
	if err := utils.DecodeSpec(ctx.Configuration.Spec, &specTmpl); err != nil {
		return nil, err
	}

	// Templates are not in the artifacts table, so the orchestrator cannot resolve them
	// by handle; resolve existence here by handle (metadata.name). When found, the
	// template keeps its own control-plane UUID; ctx.ID (a freshly generated UUID) is
	// used only when creating a new template.
	existing, err := i.templateRepo.GetByID(utils.ImportHandle(ctx.Configuration), ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up existing LLM provider template: %w", err)
	}

	if existing == nil {
		tmpl := &model.LLMProviderTemplate{
			UUID:             ctx.ID,
			OrganizationUUID: ctx.OrgID,
			ID:               utils.ImportHandle(ctx.Configuration),
			Name:             utils.ImportDisplayName(ctx.Configuration),
			Origin:           constants.OriginDP,
			Metadata:         specTmpl.Metadata,
			PromptTokens:     specTmpl.PromptTokens,
			CompletionTokens: specTmpl.CompletionTokens,
			TotalTokens:      specTmpl.TotalTokens,
			RemainingTokens:  specTmpl.RemainingTokens,
			RequestModel:     specTmpl.RequestModel,
			ResponseModel:    specTmpl.ResponseModel,
			ResourceMappings: specTmpl.ResourceMappings,
		}
		// Templates have no deployments table rows, so created_at/updated_at double as the
		// last-in-wins watermark: seed them with the gateway deployment time so a later push
		// can compare against it. The repo defaults them to now when DeployedAt is absent.
		if ctx.DeployedAt != nil {
			tmpl.CreatedAt = *ctx.DeployedAt
			tmpl.UpdatedAt = *ctx.DeployedAt
		}
		if err := i.templateRepo.Create(tmpl); err != nil {
			return nil, fmt.Errorf("failed to create LLM provider template from gateway import: %w", err)
		}
		return &ImportResult{ID: tmpl.UUID, DeployedVersion: version, Deployable: false}, nil
	}

	// Existing: last-in-wins by deployment time decides whether this push overwrites the
	// working copy. The template's updated_at holds the deployment time of the current working
	// copy (templates aren't artifact-backed, so there is no deployments row to derive it from).
	// A CP-owned template is never overwritten by a gateway push. Templates have no
	// gateway-specific data, so any non-winning push (CP-owned or stale) is a no-op.
	if utils.DecideMetadataWrite(false, existing.Origin, &existing.UpdatedAt, ctx.DeployedAt) == utils.WriteFullMetadata {
		existing.Name = utils.ImportDisplayName(ctx.Configuration)
		existing.Metadata = specTmpl.Metadata
		existing.PromptTokens = specTmpl.PromptTokens
		existing.CompletionTokens = specTmpl.CompletionTokens
		existing.TotalTokens = specTmpl.TotalTokens
		existing.RemainingTokens = specTmpl.RemainingTokens
		existing.RequestModel = specTmpl.RequestModel
		existing.ResponseModel = specTmpl.ResponseModel
		existing.ResourceMappings = specTmpl.ResourceMappings
		// Advance the watermark to this push's deployment time (this branch is only reached when
		// ctx.DeployedAt is newer than the current updated_at, so it is non-nil here).
		if ctx.DeployedAt != nil {
			existing.UpdatedAt = *ctx.DeployedAt
		}
		if err := i.templateRepo.Update(existing); err != nil {
			return nil, fmt.Errorf("failed to update LLM provider template from gateway import: %w", err)
		}
	}
	// Return the template's own control-plane UUID, not the orchestrator-generated one.
	return &ImportResult{ID: existing.UUID, DeployedVersion: version, Deployable: false}, nil
}
