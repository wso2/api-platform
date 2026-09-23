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

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// llmProxyImporter imports LLM Proxy artifacts (project-scoped).
type llmProxyImporter struct {
	proxyRepo    repository.LLMProxyRepository
	providerRepo repository.LLMProviderRepository
	artifactRepo repository.ArtifactRepository
}

func newLLMProxyImporter(proxyRepo repository.LLMProxyRepository, providerRepo repository.LLMProviderRepository, artifactRepo repository.ArtifactRepository) *llmProxyImporter {
	return &llmProxyImporter{proxyRepo: proxyRepo, providerRepo: providerRepo, artifactRepo: artifactRepo}
}

func (i *llmProxyImporter) Kind() string          { return constants.LLMProxy }
func (i *llmProxyImporter) RequiresProject() bool { return true }

func (i *llmProxyImporter) Import(ctx *ImportContext) (*ImportResult, error) {
	version := utils.ImportVersion(ctx.Configuration)

	// The gateway pushes the artifact spec in the same shape the control plane emits
	// when generating a deployment (dto.LLMProxyDeploymentSpec). Decode into that shape
	// and reverse-map it into the stored model.LLMProxyConfig — the inverse of
	// generateLLMProxyDeploymentYAML in llm_deployment.go.
	var spec dto.LLMProxyDeploymentSpec
	if err := utils.DecodeSpec(ctx.Configuration.Spec, &spec); err != nil {
		return nil, err
	}
	cfg := mapLLMProxySpecToConfig(spec)

	if ctx.Existing == nil {
		// spec.provider is the provider's handle (artifacts carry no UUIDs in the
		// gateway). Resolve it to the provider's control-plane UUID (provider_uuid is a
		// FK); a missing provider surfaces as a clean error rather than a raw FK failure.
		providerUUID, err := i.resolveProviderUUID(model.PrimaryLLMProxyProviderID(cfg), ctx.OrgID)
		if err != nil {
			return nil, err
		}
		proxy := &model.LLMProxy{
			UUID:             ctx.ID,
			OrganizationUUID: ctx.OrgID,
			ID:               utils.ImportHandle(ctx.Configuration),
			Name:             utils.ImportDisplayName(ctx.Configuration),
			ProjectUUID:      ctx.ProjectID,
			Version:          version,
			ProviderUUID:     providerUUID,
			OpenAPISpec:      i.providerOpenAPISpec(model.PrimaryLLMProxyProviderID(cfg), ctx.OrgID),
			Origin:           constants.OriginDP,
			Configuration:    cfg,
		}
		if err := i.proxyRepo.Create(proxy); err != nil {
			return nil, fmt.Errorf("failed to create LLM proxy from gateway import: %w", err)
		}
		return &ImportResult{ID: proxy.UUID, DeployedVersion: version, Deployable: true}, nil
	}

	existing, err := i.proxyRepo.GetByID(ctx.Existing.Handle, ctx.OrgID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing LLM proxy: %w", err)
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
		existing.ProjectUUID = ctx.ProjectID
		// Resolve the (possibly changed) provider handle to its CP UUID before persisting.
		providerUUID, err := i.resolveProviderUUID(model.PrimaryLLMProxyProviderID(cfg), ctx.OrgID)
		if err != nil {
			return nil, err
		}
		existing.ProviderUUID = providerUUID
		existing.Configuration = cfg
		if strings.TrimSpace(existing.OpenAPISpec) == "" {
			existing.OpenAPISpec = i.providerOpenAPISpec(model.PrimaryLLMProxyProviderID(cfg), ctx.OrgID)
		}
	case utils.WriteGatewaySpecificOnly:
		// CP-owned: only update gateway-specific upstream auth. Every attachment
		// now carries its own, so each is matched by provider id and updated in
		// place — the control plane keeps ownership of everything else.
		if stored, err := model.NormaliseLLMProxyAttachments(existing.Configuration); err == nil {
			incoming := make(map[string]*model.UpstreamAuth, len(cfg.Providers))
			for _, attachment := range cfg.Providers {
				incoming[attachment.ID] = attachment.Auth
			}
			for i := range stored {
				if auth, ok := incoming[stored[i].ID]; ok {
					stored[i].Auth = auth
				}
			}
			existing.Configuration.Providers = stored
			existing.Configuration.Provider = ""
			existing.Configuration.UpstreamAuth = nil
			existing.Configuration.AdditionalProviders = nil
		}
	}
	if err := i.proxyRepo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update LLM proxy from gateway import: %w", err)
	}
	return &ImportResult{ID: ctx.ID, DeployedVersion: version, Deployable: true}, nil
}

// resolveProviderUUID resolves the LLM provider handle referenced by the proxy spec
// (spec.provider is the provider handle, not a UUID — gateway artifacts carry no UUIDs)
// to the provider's control-plane UUID. Returns a clean ErrInvalidInput if the provider
// does not exist, instead of letting a missing reference surface as a raw FK error.
func (i *llmProxyImporter) resolveProviderUUID(providerHandle, orgID string) (string, error) {
	if providerHandle == "" {
		return "", apperror.ValidationFailed.New("The LLM proxy import requires a provider reference.")
	}
	art, err := i.artifactRepo.GetByHandle(providerHandle, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to validate referenced LLM provider %q: %w", providerHandle, err)
	}
	if art == nil || art.Type != constants.LLMProvider {
		return "", apperror.ValidationFailed.New(fmt.Sprintf("The referenced LLM provider %q does not exist.", providerHandle))
	}
	return art.UUID, nil
}

// providerOpenAPISpec best-effort loads the fronted provider and returns its OpenAPI
// definition. A missing provider or load error yields an empty spec.
func (i *llmProxyImporter) providerOpenAPISpec(providerHandle, orgID string) string {
	if i.providerRepo == nil || providerHandle == "" {
		return ""
	}
	prov, err := i.providerRepo.GetByID(providerHandle, orgID)
	if err != nil || prov == nil {
		return ""
	}
	return prov.OpenAPISpec
}

// mapLLMProxySpecProviders reverse-maps whichever provider shape a pushed artifact
// used into the canonical attachment list, which is the only shape stored.
// A gateway running against the current control plane pushes
// `providers`; one written against the older shape pushes `provider` plus
// `additionalProviders`, and both describe the same attachments.
//
// The legacy branch also carries `additionalProviders` through, which the
// importer previously dropped entirely — a gateway-pushed multi-provider proxy
// silently lost every provider but its primary on import.
func mapLLMProxySpecProviders(spec dto.LLMProxyDeploymentSpec) []model.LLMProxyAttachment {
	if len(spec.Providers) > 0 {
		attachments := make([]model.LLMProxyAttachment, 0, len(spec.Providers))
		for _, entry := range spec.Providers {
			attachment := model.LLMProxyAttachment{
				ID:          entry.ID,
				Alias:       entry.Alias,
				IsPrimary:   entry.IsPrimary,
				Auth:        mapUpstreamAuthAPIToModel(entry.Auth),
				Transformer: mapTransformerAPIToModel(entry.Transformer),
			}
			if attachment.IsPrimary {
				attachment.Auth = defaultUpstreamAuthToNone(attachment.Auth)
			}
			attachments = append(attachments, attachment)
		}
		return attachments
	}

	if spec.Provider == nil {
		return nil
	}
	attachments := []model.LLMProxyAttachment{{
		ID:          spec.Provider.ID,
		Alias:       spec.Provider.As,
		IsPrimary:   true,
		Auth:        defaultUpstreamAuthToNone(mapUpstreamAuthAPIToModel(spec.Provider.Auth)),
		Transformer: mapTransformerAPIToModel(spec.Provider.Transformer),
	}}
	for _, additional := range spec.AdditionalProviders {
		attachments = append(attachments, model.LLMProxyAttachment{
			ID:          additional.ID,
			Alias:       additional.As,
			Auth:        mapUpstreamAuthAPIToModel(additional.Auth),
			Transformer: mapTransformerAPIToModel(additional.Transformer),
		})
	}
	return attachments
}

// mapLLMProxySpecToConfig reverse-maps a gateway-pushed LLM proxy deployment spec into
// the control plane's stored model.LLMProxyConfig. It is the inverse of
// generateLLMProxyDeploymentYAML: the provider shape is normalised into the canonical
// attachment list, and the policy list (which carries security on the wire) is lifted
// back into the first-class Security field.
// LLM proxies have no rate-limiting field, so rate-limit policies are NOT lifted
// (liftRateLimits=false) and are kept as ordinary policies so a gateway-pushed proxy
// that carries them (e.g. llm-cost-based-ratelimit) still surfaces them in the response.
func mapLLMProxySpecToConfig(spec dto.LLMProxyDeploymentSpec) model.LLMProxyConfig {
	cfg := model.LLMProxyConfig{
		Name:            spec.DisplayName,
		Version:         spec.Version,
		InboundTemplate: spec.InboundTemplate,
		Providers:       mapLLMProxySpecProviders(spec),
	}
	if spec.Context != "" {
		context := spec.Context
		cfg.Context = &context
	}
	if spec.VHost != "" {
		vhost := spec.VHost
		cfg.Vhost = &vhost
	}
	// Security/rate-limiting are pushed as global (api-key-auth, api-level limits) and
	// operation (resource-scoped limits) policies by the forward conversion; older gateways
	// may still push legacy policies, so lift from all three.
	liftInput := mapGlobalPoliciesAPIToLLMPolicies(&spec.GlobalPolicies)
	liftInput = append(liftInput, mapOperationPoliciesAPIToLLMPolicies(&spec.OperationPolicies)...)
	liftInput = append(liftInput, mapPoliciesAPIToModel(&spec.Policies)...)
	security, _, remaining := liftLLMPolicies(liftInput, false)
	cfg.Security, cfg.Policies = security, remaining
	return cfg
}
