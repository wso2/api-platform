/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com) All Rights Reserved.
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package service

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/repository"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// ArtifactSnapshot is one artifact's definition as it currently stands, rendered
// but deliberately NOT translated for any gateway.
//
// A build stores this shape: the target gateway is not known when a build is
// prepared, and the same build is deployable to gateways on different data
// versions, so translation belongs to the deployment rather than the snapshot.
// DataVersion records the platform version the definition was written at, which
// is what a later deploy translates FROM.
type ArtifactSnapshot struct {
	// Definition is the kind's own deployment struct, ready to be marshalled.
	Definition any
	// DataVersion is the artifact's platform data version.
	DataVersion string
	// Origin says where the artifact came from; a DP-originated artifact is
	// read-only in the control plane and cannot be built or deployed from it.
	Origin string
}

// ArtifactDefinition renders one artifact kind's current definition into the
// snapshot a build stores, and reconstitutes a stored snapshot so a deploy can
// translate it for its target gateway.
//
// It exists because builds and deployments are shared across every artifact kind
// — they hang off artifact_uuid, not a kind-specific table — while rendering is
// not: each kind has its own definition, its own repository and its own
// deployment YAML. This is the one seam where that difference lives.
type ArtifactDefinition interface {
	// Kind is the artifact kind, matching both the artifact registry's alias and
	// the key gatewaytranslator dispatches on (they share one key space).
	Kind() string

	// Current loads the artifact and renders its definition as it stands. It
	// takes the resolved artifact row rather than an identifier because the
	// kinds' repositories do not agree on one: REST APIs are fetched by UUID,
	// LLM providers and proxies by handle. It returns the kind's own not-found
	// error when the artifact has gone, so callers keep per-kind error
	// semantics.
	Current(artifact *model.Artifact) (*ArtifactSnapshot, error)

	// Decode unmarshals stored build content back into this kind's deployment
	// struct, ready for gatewaytranslator.Translate.
	Decode(content []byte) (any, error)
}

// ArtifactDefinitions resolves the ArtifactDefinition for an artifact kind. It is
// assembled at wiring time, where every kind's repository is in scope.
type ArtifactDefinitions map[string]ArtifactDefinition

// NewArtifactDefinitions indexes the given definitions by their kind.
func NewArtifactDefinitions(definitions ...ArtifactDefinition) ArtifactDefinitions {
	indexed := make(ArtifactDefinitions, len(definitions))
	for _, definition := range definitions {
		indexed[definition.Kind()] = definition
	}
	return indexed
}

// For returns the definition for an artifact kind, or an error naming the kind
// when none is registered — a kind that reaches a build without a definition is
// a wiring mistake, not a user error.
func (d ArtifactDefinitions) For(kind string) (ArtifactDefinition, error) {
	definition, ok := d[kind]
	if !ok {
		return nil, apperror.Internal.New().
			WithLogMessage(fmt.Sprintf("no artifact definition registered for kind %q", kind))
	}
	return definition, nil
}

// restAPIDefinition renders REST APIs.
type restAPIDefinition struct {
	apiRepo repository.APIRepository
	apiUtil *utils.APIUtil
}

// NewRestAPIDefinition returns the ArtifactDefinition for REST APIs.
func NewRestAPIDefinition(apiRepo repository.APIRepository, apiUtil *utils.APIUtil) ArtifactDefinition {
	return &restAPIDefinition{apiRepo: apiRepo, apiUtil: apiUtil}
}

func (d *restAPIDefinition) Kind() string { return constants.RestApi }

func (d *restAPIDefinition) Current(artifact *model.Artifact) (*ArtifactSnapshot, error) {
	apiModel, err := d.apiRepo.GetAPIByUUID(artifact.UUID, artifact.OrganizationUUID)
	if err != nil {
		return nil, err
	}
	if apiModel == nil {
		return nil, apperror.RESTAPINotFound.New()
	}
	definition, err := d.apiUtil.BuildAPIDeploymentYAML(apiModel)
	if err != nil {
		return nil, fmt.Errorf("failed to build API deployment YAML: %w", err)
	}
	return &ArtifactSnapshot{
		Definition:  definition,
		DataVersion: apiModel.DataVersion,
		Origin:      apiModel.Origin,
	}, nil
}

func (d *restAPIDefinition) Decode(content []byte) (any, error) {
	definition := &dto.APIDeploymentYAML{}
	if err := yaml.Unmarshal(content, definition); err != nil {
		return nil, fmt.Errorf("failed to parse stored API deployment YAML: %w", err)
	}
	return definition, nil
}

// llmProxyDefinition renders LLM proxies.
type llmProxyDefinition struct {
	proxyRepo repository.LLMProxyRepository
}

// NewLLMProxyDefinition returns the ArtifactDefinition for LLM proxies.
func NewLLMProxyDefinition(proxyRepo repository.LLMProxyRepository) ArtifactDefinition {
	return &llmProxyDefinition{proxyRepo: proxyRepo}
}

func (d *llmProxyDefinition) Kind() string { return constants.LLMProxy }

func (d *llmProxyDefinition) Current(artifact *model.Artifact) (*ArtifactSnapshot, error) {
	proxy, err := d.proxyRepo.GetByID(artifact.Handle, artifact.OrganizationUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.LLMProxyNotFound.New()
	}
	definition, err := generateLLMProxyDeploymentYAML(proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to generate LLM proxy deployment YAML: %w", err)
	}
	return &ArtifactSnapshot{
		Definition:  &definition,
		DataVersion: proxy.DataVersion,
		Origin:      proxy.Origin,
	}, nil
}

func (d *llmProxyDefinition) Decode(content []byte) (any, error) {
	definition := &dto.LLMProxyDeploymentYAML{}
	if err := yaml.Unmarshal(content, definition); err != nil {
		return nil, fmt.Errorf("failed to parse stored LLM proxy deployment YAML: %w", err)
	}
	return definition, nil
}

// llmProviderDefinition renders LLM providers. A provider's definition names the
// template it was created from, which is resolved here rather than supplied by a
// caller, so a provider build stays a pure function of the provider.
type llmProviderDefinition struct {
	providerRepo repository.LLMProviderRepository
	templateRepo repository.LLMProviderTemplateRepository
}

// NewLLMProviderDefinition returns the ArtifactDefinition for LLM providers.
func NewLLMProviderDefinition(
	providerRepo repository.LLMProviderRepository,
	templateRepo repository.LLMProviderTemplateRepository,
) ArtifactDefinition {
	return &llmProviderDefinition{providerRepo: providerRepo, templateRepo: templateRepo}
}

func (d *llmProviderDefinition) Kind() string { return constants.LLMProvider }

func (d *llmProviderDefinition) Current(artifact *model.Artifact) (*ArtifactSnapshot, error) {
	provider, err := d.providerRepo.GetByID(artifact.Handle, artifact.OrganizationUUID)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, apperror.LLMProviderNotFound.New()
	}
	templateHandle, err := d.templateHandle(provider.TemplateUUID, artifact.OrganizationUUID)
	if err != nil {
		return nil, err
	}
	definition, err := generateLLMProviderDeploymentYAML(provider, templateHandle)
	if err != nil {
		return nil, fmt.Errorf("failed to generate LLM provider deployment YAML: %w", err)
	}
	return &ArtifactSnapshot{
		Definition:  &definition,
		DataVersion: provider.DataVersion,
		Origin:      provider.Origin,
	}, nil
}

func (d *llmProviderDefinition) Decode(content []byte) (any, error) {
	definition := &dto.LLMProviderDeploymentYAML{}
	if err := yaml.Unmarshal(content, definition); err != nil {
		return nil, fmt.Errorf("failed to parse stored LLM provider deployment YAML: %w", err)
	}
	return definition, nil
}

// templateHandle mirrors LLMProviderDeploymentService.getTemplateHandle: a
// provider whose template has gone is not renderable, and says so as a
// template-not-found rather than a bare nil dereference.
func (d *llmProviderDefinition) templateHandle(templateUUID, orgUUID string) (string, error) {
	if templateUUID == "" {
		return "", apperror.LLMProviderTemplateNotFound.New()
	}
	template, err := d.templateRepo.GetByUUID(templateUUID, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve template: %w", err)
	}
	if template == nil {
		return "", apperror.LLMProviderTemplateNotFound.New()
	}
	return template.ID, nil
}

// mcpProxyDefinition renders MCP proxies.
type mcpProxyDefinition struct {
	proxyRepo repository.MCPProxyRepository
	mcpUtils  *utils.MCPUtils
}

// NewMCPProxyDefinition returns the ArtifactDefinition for MCP proxies.
func NewMCPProxyDefinition(proxyRepo repository.MCPProxyRepository, mcpUtils *utils.MCPUtils) ArtifactDefinition {
	return &mcpProxyDefinition{proxyRepo: proxyRepo, mcpUtils: mcpUtils}
}

func (d *mcpProxyDefinition) Kind() string { return constants.MCPProxy }

func (d *mcpProxyDefinition) Current(artifact *model.Artifact) (*ArtifactSnapshot, error) {
	proxy, err := d.proxyRepo.GetByUUID(artifact.UUID, artifact.OrganizationUUID)
	if err != nil {
		return nil, err
	}
	if proxy == nil {
		return nil, apperror.MCPProxyNotFound.New()
	}
	definition, err := d.mcpUtils.BuildMCPDeploymentYAML(proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to build MCP proxy deployment YAML: %w", err)
	}
	return &ArtifactSnapshot{
		Definition:  definition,
		DataVersion: proxy.DataVersion,
		Origin:      proxy.Origin,
	}, nil
}

func (d *mcpProxyDefinition) Decode(content []byte) (any, error) {
	definition := &model.MCPProxyDeploymentYAML{}
	if err := yaml.Unmarshal(content, definition); err != nil {
		return nil, fmt.Errorf("failed to parse stored MCP proxy deployment YAML: %w", err)
	}
	return definition, nil
}
