/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package handlers

import (
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// buildResourceStatus derives the server-managed status block from a stored
// configuration row. All timestamps are reported as UTC RFC3339 times.
//
// The Id is always set to the Handle (a.k.a. metadata.name). The State mirrors
// the StoredConfig's DesiredState. DeployedAt is only populated when the
// resource is currently deployed.
func buildResourceStatus(cfg *models.StoredConfig) api.ResourceStatus {
	id := cfg.Handle
	state := api.ResourceStatusState(cfg.DesiredState)
	createdAt := cfg.CreatedAt
	updatedAt := cfg.UpdatedAt

	status := api.ResourceStatus{
		Id:        &id,
		State:     &state,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	if cfg.DeployedAt != nil {
		deployedAt := *cfg.DeployedAt
		status.DeployedAt = &deployedAt
	}
	return status
}

// buildResourceResponse merges a resource configuration value with the server-
// managed status block. It accepts any of the k8s-shaped resource types and
// returns a value with the Status field populated. Any user-provided Status in
// the input is replaced with the authoritative server value.
//
// The input is assumed to be the StoredConfig's Configuration or
// SourceConfiguration (not a pointer). Unknown types are returned unchanged so
// callers can fall through to a generic JSON response if required.
func buildResourceResponse(cfg any, status api.ResourceStatus) any {
	switch v := cfg.(type) {
	case api.RestAPI:
		v.Status = &status
		return v
	case *api.RestAPI:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	case api.WebSubAPI:
		v.Status = &status
		return v
	case *api.WebSubAPI:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	case api.MCPProxyConfiguration:
		v.Status = &status
		return v
	case *api.MCPProxyConfiguration:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	case api.LLMProviderTemplate:
		v.Status = &status
		return v
	case *api.LLMProviderTemplate:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	case api.LLMProviderConfiguration:
		v.Status = &status
		return v
	case *api.LLMProviderConfiguration:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	case api.LLMProxyConfiguration:
		v.Status = &status
		return v
	case *api.LLMProxyConfiguration:
		if v == nil {
			return nil
		}
		cp := *v
		cp.Status = &status
		return cp
	}
	return cfg
}

// buildResourceResponseFromStored is a convenience wrapper that extracts the
// canonical status from the StoredConfig and merges it with the supplied
// configuration payload. `cfg` should typically be StoredConfig.Configuration
// (the templated/resolved view) but callers may pass SourceConfiguration when
// they want to echo back the user-supplied body instead.
func buildResourceResponseFromStored(cfg any, stored *models.StoredConfig) any {
	return buildResourceResponse(cfg, buildResourceStatus(stored))
}

// buildTemplateResourceResponse builds a k8s-style response body for resources
// that are not deployable (LLMProviderTemplate) and therefore lack the
// DesiredState/DeployedAt fields carried by StoredConfig. Only the handle and
// created/updated timestamps are surfaced.
func buildTemplateResourceResponse(template *models.StoredLLMProviderTemplate) any {
	if template == nil {
		return nil
	}
	id := template.GetHandle()
	createdAt := template.CreatedAt
	updatedAt := template.UpdatedAt
	status := api.ResourceStatus{
		Id:        &id,
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}
	cfg := template.Configuration
	return buildResourceResponse(cfg, status)
}

// buildSecretResourceResponse assembles a k8s-style SecretConfiguration body
// from a stored secret. When includeValue is false the plaintext value is
// omitted from the response entirely (used by list and create/update responses
// so we don't surface secret material). Timestamps that are the zero value are
// also omitted — they're only populated once the secret has been round-tripped
// through storage.
//
// We emit a map rather than the generated SecretConfiguration struct because
// the OpenAPI spec marks spec.value as required (for requests) which means the
// generated struct has no omitempty tag on Value and would leak empty strings
// into the wire response.
func buildSecretResourceResponse(secret *models.Secret, includeValue bool) any {
	if secret == nil {
		return nil
	}
	spec := map[string]any{
		"displayName": secret.DisplayName,
	}
	if secret.Description != nil {
		spec["description"] = *secret.Description
	}
	if includeValue {
		spec["value"] = secret.Value
	}
	status := map[string]any{
		"id": secret.Handle,
	}
	if !secret.CreatedAt.IsZero() {
		status["createdAt"] = secret.CreatedAt
	}
	if !secret.UpdatedAt.IsZero() {
		status["updatedAt"] = secret.UpdatedAt
	}
	return map[string]any{
		"apiVersion": string(api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1),
		"kind":       string(api.SecretConfigurationRequestKindSecret),
		"metadata":   map[string]any{"name": secret.Handle},
		"spec":       spec,
		"status":     status,
	}
}

// buildSecretMetaResourceResponse assembles a k8s-style SecretConfiguration
// body from a SecretMeta row (list views). The plaintext value is never
// populated. See buildSecretResourceResponse for why we emit a map.
func buildSecretMetaResourceResponse(meta models.SecretMeta) any {
	status := map[string]any{
		"id": meta.Handle,
	}
	if !meta.CreatedAt.IsZero() {
		status["createdAt"] = meta.CreatedAt
	}
	if !meta.UpdatedAt.IsZero() {
		status["updatedAt"] = meta.UpdatedAt
	}
	return map[string]any{
		"apiVersion": string(api.SecretConfigurationRequestApiVersionGatewayApiPlatformWso2Comv1alpha1),
		"kind":       string(api.SecretConfigurationRequestKindSecret),
		"metadata":   map[string]any{"name": meta.Handle},
		"spec":       map[string]any{"displayName": meta.DisplayName},
		"status":     status,
	}
}
