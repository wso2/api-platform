/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

package utils

import (
	"fmt"
	"reflect"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// expandGlobalModelRoutingOperations materializes only provider-template
// resource paths whose requestModel differs from the operation that currently
// covers them. Policies remain attached at API scope.
func expandGlobalModelRoutingOperations(ops []api.Operation, globals *[]api.Policy, template *models.StoredLLMProviderTemplate) []api.Operation {
	if globals == nil || template == nil || !containsGlobalModelRoutingPolicy(*globals) {
		return ops
	}

	result := append([]api.Operation(nil), ops...)
	seen := make(map[pathMethodKey]bool, len(ops))
	for _, op := range ops {
		seen[pathMethodKey{path: op.EffectivePath(), method: op.EffectiveMethod()}] = true
	}

	expanded := false
	for _, op := range sortOperationsBySpecificity(ops) {
		for _, path := range expandPolicyTargetPaths(op.EffectivePath(), &template.Configuration.Spec) {
			key := pathMethodKey{path: path, method: op.EffectiveMethod()}
			if seen[key] || !requestModelMappingDiffers(template, op.EffectivePath(), path) {
				continue
			}

			clone := op
			clone.Path = api.Ptr(path)
			if op.Policies != nil {
				policies := append([]api.Policy(nil), (*op.Policies)...)
				clone.Policies = &policies
			}
			result = append(result, clone)
			seen[key] = true
			expanded = true
		}
	}

	if !expanded {
		return ops
	}
	return sortOperationsBySpecificity(result)
}

func containsGlobalModelRoutingPolicy(policies []api.Policy) bool {
	for _, attached := range policies {
		if isGlobalModelRoutingPolicy(attached.Name) {
			return true
		}
	}
	return false
}

func isGlobalModelRoutingPolicy(name string) bool {
	switch name {
	case "cost-based-model-routing", "semantic-model-routing", "time-based-model-routing":
		return true
	default:
		return false
	}
}

func requestModelMappingDiffers(template *models.StoredLLMProviderTemplate, basePath, targetPath string) bool {
	if basePath == targetPath {
		return false
	}
	baseParams, err := buildTemplateParams(template, basePath)
	if err != nil {
		return false
	}
	targetParams, err := buildTemplateParams(template, targetPath)
	if err != nil {
		return false
	}
	baseModel, baseExists := baseParams["requestModel"]
	targetModel, targetExists := targetParams["requestModel"]
	return targetExists && (!baseExists || !reflect.DeepEqual(baseModel, targetModel))
}

// ResolveGlobalRequestModels replaces the shared API-level requestModel map
// with the provider-template mapping selected for each concrete route.
func (t *LLMProviderTransformer) ResolveGlobalRequestModels(source interface{}, rdc *models.RuntimeDeployConfig) error {
	if !hasGlobalModelRoutingPolicy(rdc) {
		return nil
	}

	templateHandle, err := t.templateHandleForLLMSource(source)
	if err != nil {
		return err
	}
	template, err := t.getTemplateByHandle(templateHandle)
	if err != nil {
		return fmt.Errorf("lookup template %q: %w", templateHandle, err)
	}
	return resolveGlobalRequestModels(rdc, template)
}

func (t *LLMProviderTransformer) templateHandleForLLMSource(source interface{}) (string, error) {
	switch sc := source.(type) {
	case api.LLMProviderConfiguration:
		return sc.Spec.Template, nil
	case api.LLMProxyConfiguration:
		provider, err := t.db.GetConfigByKindAndHandle(string(api.LLMProviderConfigurationKindLlmProvider), sc.Spec.Provider.Id)
		if err != nil {
			return "", fmt.Errorf("lookup provider %q: %w", sc.Spec.Provider.Id, err)
		}
		if provider == nil {
			return "", fmt.Errorf("provider %q not found", sc.Spec.Provider.Id)
		}
		config, ok := provider.SourceConfiguration.(api.LLMProviderConfiguration)
		if !ok {
			return "", fmt.Errorf("provider %q has invalid source configuration", sc.Spec.Provider.Id)
		}
		return config.Spec.Template, nil
	default:
		return "", fmt.Errorf("unsupported LLM source configuration: %T", source)
	}
}

func hasGlobalModelRoutingPolicy(rdc *models.RuntimeDeployConfig) bool {
	if rdc == nil {
		return false
	}
	for _, chain := range rdc.PolicyChains {
		if chain == nil {
			continue
		}
		for _, attached := range chain.Policies {
			if isGlobalModelRoutingPolicy(attached.Name) && attached.Params["attachedTo"] == string(policy.LevelAPI) {
				return true
			}
		}
	}
	return false
}

func resolveGlobalRequestModels(rdc *models.RuntimeDeployConfig, template *models.StoredLLMProviderTemplate) error {
	if template == nil {
		return fmt.Errorf("provider template is nil")
	}
	for key, chain := range rdc.PolicyChains {
		if chain == nil {
			continue
		}
		route := rdc.Routes[key]
		if route == nil {
			return fmt.Errorf("policy chain %q has no route", key)
		}

		indices := globalModelRoutingPolicyIndices(chain)
		if len(indices) == 0 {
			continue
		}
		params, err := buildTemplateParams(template, route.OperationPath)
		if err != nil {
			return fmt.Errorf("resolve provider-template parameters for route %q: %w", route.OperationPath, err)
		}
		model, exists := params["requestModel"]
		if !exists {
			return fmt.Errorf("provider template %q has no requestModel mapping for route %q", template.GetHandle(), route.OperationPath)
		}

		for _, i := range indices {
			attached := &chain.Policies[i]
			if attached.Name == "semantic-model-routing" {
				mapping, ok := model.(map[string]interface{})
				if !ok || mapping["location"] != api.ExtractionIdentifierLocation("payload") {
					return fmt.Errorf("semantic-model-routing supports only payload requestModel mappings on route %q", route.OperationPath)
				}
			}
			// API-level parameter maps are shared across route chains.
			attached.Params = *mergeParams(attached.Params, map[string]interface{}{"requestModel": model})
		}
	}
	return nil
}

func globalModelRoutingPolicyIndices(chain *models.PolicyChain) []int {
	indices := make([]int, 0)
	for i := range chain.Policies {
		attached := &chain.Policies[i]
		if isGlobalModelRoutingPolicy(attached.Name) && attached.Params["attachedTo"] == string(policy.LevelAPI) {
			indices = append(indices, i)
		}
	}
	return indices
}
