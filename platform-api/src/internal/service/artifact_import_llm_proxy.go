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
	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
	"platform-api/src/internal/utils"
)

// llmProxyImporter imports LLM Proxy artifacts (project-scoped).
type llmProxyImporter struct {
	proxyRepo    repository.LLMProxyRepository
	artifactRepo repository.ArtifactRepository
}

func newLLMProxyImporter(proxyRepo repository.LLMProxyRepository, artifactRepo repository.ArtifactRepository) *llmProxyImporter {
	return &llmProxyImporter{proxyRepo: proxyRepo, artifactRepo: artifactRepo}
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
		providerUUID, err := i.resolveProviderUUID(cfg.Provider, ctx.OrgID)
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
		providerUUID, err := i.resolveProviderUUID(cfg.Provider, ctx.OrgID)
		if err != nil {
			return nil, err
		}
		existing.ProviderUUID = providerUUID
		existing.Configuration = cfg
	case utils.WriteGatewaySpecificOnly:
		// CP-owned: only update gateway-specific upstream auth.
		existing.Configuration.UpstreamAuth = cfg.UpstreamAuth
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
		return "", fmt.Errorf("%w: LLM proxy import requires a provider reference", constants.ErrInvalidInput)
	}
	art, err := i.artifactRepo.GetByHandle(providerHandle, orgID)
	if err != nil {
		return "", fmt.Errorf("failed to validate referenced LLM provider %q: %w", providerHandle, err)
	}
	if art == nil || art.Type != constants.LLMProvider {
		return "", fmt.Errorf("%w: referenced LLM provider %q does not exist", constants.ErrInvalidInput, providerHandle)
	}
	return art.UUID, nil
}

// mapLLMProxySpecToConfig reverse-maps a gateway-pushed LLM proxy deployment spec into
// the control plane's stored model.LLMProxyConfig. It is the inverse of
// generateLLMProxyDeploymentYAML: the provider object ({id, auth}) is flattened into the
// provider handle plus the gateway-specific upstream auth, and the policy list (which
// carries security on the wire) is lifted back into the first-class Security field.
// (LLM proxies have no rate-limiting field, so the lifted rate-limiting — which the proxy
// flow never emits — is discarded.)
func mapLLMProxySpecToConfig(spec dto.LLMProxyDeploymentSpec) model.LLMProxyConfig {
	cfg := model.LLMProxyConfig{
		Name:     spec.DisplayName,
		Version:  spec.Version,
		Provider: spec.Provider.ID,
	}
	if spec.Context != "" {
		context := spec.Context
		cfg.Context = &context
	}
	if spec.VHost != "" {
		vhost := spec.VHost
		cfg.Vhost = &vhost
	}
	if spec.Provider.Auth != nil {
		cfg.UpstreamAuth = mapUpstreamAuthAPIToModel(spec.Provider.Auth)
	}
	// Security/rate-limiting are pushed as global (api-key-auth, api-level limits) and
	// operation (resource-scoped limits) policies by the forward conversion; older gateways
	// may still push legacy policies, so lift from all three.
	liftInput := mapGlobalPoliciesAPIToLLMPolicies(&spec.GlobalPolicies)
	liftInput = append(liftInput, mapOperationPoliciesAPIToLLMPolicies(&spec.OperationPolicies)...)
	liftInput = append(liftInput, mapPoliciesAPIToModel(&spec.Policies)...)
	security, _, remaining := liftLLMPolicies(liftInput)
	cfg.Security, cfg.Policies = security, remaining
	return cfg
}
