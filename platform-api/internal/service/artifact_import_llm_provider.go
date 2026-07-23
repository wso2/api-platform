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
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2/api-platform/platform-api/config"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/dto"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// llmProviderImporter imports LLM Provider artifacts (organization-level).
type llmProviderImporter struct {
	providerRepo repository.LLMProviderRepository
	templateRepo repository.LLMProviderTemplateRepository
	artifactRepo repository.ArtifactRepository
	cfg          *config.Server
	slogger      *slog.Logger
}

func newLLMProviderImporter(providerRepo repository.LLMProviderRepository,
	templateRepo repository.LLMProviderTemplateRepository, artifactRepo repository.ArtifactRepository,
	cfg *config.Server, slogger *slog.Logger) *llmProviderImporter {
	return &llmProviderImporter{providerRepo: providerRepo, templateRepo: templateRepo, artifactRepo: artifactRepo,
		cfg: cfg, slogger: slogger}
}

func (i *llmProviderImporter) Kind() string          { return constants.LLMProvider }
func (i *llmProviderImporter) RequiresProject() bool { return false }

func (i *llmProviderImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	version := utils.ImportVersion(ctx.Configuration)

	// The gateway pushes the artifact spec in the same shape the control plane emits
	// when generating a deployment (dto.LLMProviderDeploymentSpec). Decode into that
	// shape and reverse-map it into the stored model.LLMProviderConfig — the inverse of
	// generateLLMProviderDeploymentYAML in llm_deployment.go.
	var spec dto.LLMProviderDeploymentSpec
	if err := utils.DecodeSpec(ctx.Configuration.Spec, &spec); err != nil {
		return nil, err
	}
	cfg := mapLLMProviderSpecToConfig(spec)

	if ctx.Existing == nil {
		tmpl, err := i.resolveTemplate(cfg.Template, ctx.OrgID)
		if err != nil {
			return nil, err
		}
		provider := &model.LLMProvider{
			UUID:             ctx.ID,
			OrganizationUUID: ctx.OrgID,
			ID:               utils.ImportHandle(ctx.Configuration),
			Name:             utils.ImportDisplayName(ctx.Configuration),
			Version:          version,
			TemplateUUID:     tmpl.UUID,
			OpenAPISpec:      resolveTemplateOpenAPISpec(context.Background(), tmpl, openAPISpecFetchLimit(i.cfg), i.slogger),
			Origin:           constants.OriginDP,
			Configuration:    cfg,
		}
		if err := i.providerRepo.Create(provider); err != nil {
			return nil, fmt.Errorf("failed to create LLM provider from gateway import: %w", err)
		}
		return &ImportResult{ID: provider.UUID, DeployedVersion: version, Deployable: true}, nil
	}

	existing, err := i.providerRepo.GetByID(ctx.Existing.Handle, ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing LLM provider: %w", err)
	}
	if existing == nil {
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	}

	switch ctx.MetadataMode {
	case utils.SkipWorkingCopy:
		// Stale, out-of-order push: a newer deployment already defines the working copy.
		return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
	case utils.WriteFullMetadata:
		existing.Name = utils.ImportDisplayName(ctx.Configuration)
		existing.Version = version
		existing.Configuration = cfg
		if cfg.Template != "" {
			tmpl, err := i.resolveTemplate(cfg.Template, ctx.OrgID)
			if err != nil {
				return nil, err
			}
			existing.TemplateUUID = tmpl.UUID
			if strings.TrimSpace(existing.OpenAPISpec) == "" {
				existing.OpenAPISpec = resolveTemplateOpenAPISpec(context.Background(), tmpl, openAPISpecFetchLimit(i.cfg), i.slogger)
			}
		}
	case utils.WriteGatewaySpecificOnly:
		// CP-owned: only update gateway-specific upstream.
		existing.Configuration.Upstream = cfg.Upstream
	}
	if err := i.providerRepo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update LLM provider from gateway import: %w", err)
	}
	return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
}

// resolveTemplate resolves the template handle referenced by the provider spec to the
// full template record. The template must already exist (FK requirement).
func (i *llmProviderImporter) resolveTemplate(templateHandle, orgID string) (*model.LLMProviderTemplate, error) {
	if templateHandle == "" {
		return nil, apperror.ValidationFailed.New("The LLM provider import requires a template reference.")
	}
	tmpl, err := i.templateRepo.GetByID(templateHandle, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve LLM provider template %q: %w", templateHandle, err)
	}
	if tmpl == nil {
		return nil, apperror.ValidationFailed.New(fmt.Sprintf("The referenced LLM provider template %q does not exist.", templateHandle))
	}
	return tmpl, nil
}

// mapLLMProviderSpecToConfig reverse-maps a gateway-pushed LLM provider deployment
// spec into the control plane's stored model.LLMProviderConfig. It is the inverse of
// generateLLMProviderDeploymentYAML: the upstream is un-flattened into a main endpoint,
// and the policy list (which carries security and rate-limiting on the wire) is lifted
// back into the first-class Security/RateLimiting fields the AI Workspace renders.
func mapLLMProviderSpecToConfig(spec dto.LLMProviderDeploymentSpec) model.LLMProviderConfig {
	cfg := model.LLMProviderConfig{
		Name:          spec.DisplayName,
		Version:       spec.Version,
		Template:      spec.Template,
		Upstream:      mapLLMUpstreamYAMLToModel(spec.Upstream),
		AccessControl: mapAccessControlAPI(&spec.AccessControl),
	}
	if spec.Context != "" {
		context := spec.Context
		cfg.Context = &context
	}
	if spec.VHost != "" {
		vhost := spec.VHost
		cfg.VHost = &vhost
	}
	// Security/rate-limiting are pushed as global (api-key-auth, api-level limits) and
	// operation (resource-scoped limits) policies by the forward conversion; older gateways
	// may still push legacy policies, so lift from all three.
	liftInput := mapGlobalPoliciesAPIToLLMPolicies(&spec.GlobalPolicies)
	liftInput = append(liftInput, mapOperationPoliciesAPIToLLMPolicies(&spec.OperationPolicies)...)
	liftInput = append(liftInput, mapPoliciesAPIToModel(&spec.Policies)...)
	cfg.Security, cfg.RateLimiting, cfg.Policies = liftLLMPolicies(liftInput, true)
	return cfg
}

// mapLLMUpstreamYAMLToModel converts the gateway's single flat upstream endpoint
// ({url, ref, auth}) into the control plane's main/sandbox UpstreamConfig, mapping the
// single endpoint to the main endpoint. Returns nil when no upstream is present.
func mapLLMUpstreamYAMLToModel(in dto.LLMUpstreamYAML) *model.UpstreamConfig {
	endpoint := &model.UpstreamEndpoint{URL: in.URL, Ref: in.Ref}
	endpoint.Auth = defaultUpstreamAuthToNone(mapUpstreamAuthAPIToModel(in.Auth))
	return &model.UpstreamConfig{Main: endpoint}
}
