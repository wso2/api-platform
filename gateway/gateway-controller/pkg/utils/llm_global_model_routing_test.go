/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

func globalRoutingTemplate() *models.StoredLLMProviderTemplate {
	return &models.StoredLLMProviderTemplate{Configuration: api.LLMProviderTemplate{
		Metadata: api.Metadata{Name: "test-template"},
		Spec: api.LLMProviderTemplateData{
			RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.model"},
			ResourceMappings: &api.LLMProviderTemplateResourceMappings{Resources: &[]api.LLMProviderTemplateResourceMapping{
				{Resource: "/special/*", RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.nested.model"}},
				{Resource: "/special/exact", RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.exact.model"}},
			}},
		},
	}}
}

func globalRoutingTemplateWithUnchangedRequestModel() *models.StoredLLMProviderTemplate {
	return &models.StoredLLMProviderTemplate{Configuration: api.LLMProviderTemplate{Spec: api.LLMProviderTemplateData{
		RequestModel: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.model"},
		ResourceMappings: &api.LLMProviderTemplateResourceMappings{Resources: &[]api.LLMProviderTemplateResourceMapping{{
			Resource:         "/responses",
			PromptTokens:     &api.ExtractionIdentifier{Location: "payload", Identifier: "$.usage.input_tokens"},
			CompletionTokens: &api.ExtractionIdentifier{Location: "payload", Identifier: "$.usage.output_tokens"},
		}}},
	}}}
}

func TestGlobalModelRoutingMappingsPreserveScopeOrderAndIsolation(t *testing.T) {
	for _, name := range []string{"time-based-model-routing", "semantic-model-routing", "cost-based-model-routing"} {
		t.Run(name, func(t *testing.T) {
			shared := map[string]interface{}{"attachedTo": "api", "setting": "keep", "requestModel": map[string]interface{}{"identifier": "stale"}}
			condition := "true"
			rdc := &models.RuntimeDeployConfig{Routes: map[string]*models.Route{}, PolicyChains: map[string]*models.PolicyChain{}}
			for _, path := range []string{"/messages", "/special/*", "/special/exact"} {
				rdc.Routes[path] = &models.Route{OperationPath: path}
				rdc.PolicyChains[path] = &models.PolicyChain{Policies: []models.Policy{
					{Name: "log-message", Params: map[string]interface{}{"attachedTo": "api"}},
					{Name: name, Version: "v0", Params: shared, ExecutionCondition: &condition},
					{Name: name, Params: map[string]interface{}{"attachedTo": "route", "requestModel": "operation-mapping"}},
				}}
			}

			require.NoError(t, resolveGlobalRequestModels(rdc, globalRoutingTemplate()))
			for path, expected := range map[string]string{"/messages": "$.model", "/special/*": "$.nested.model", "/special/exact": "$.exact.model"} {
				chain := rdc.PolicyChains[path].Policies
				require.Len(t, chain, 3)
				require.Equal(t, "log-message", chain[0].Name)
				require.NotContains(t, chain[0].Params, "requestModel")
				require.Equal(t, "api", chain[1].Params["attachedTo"])
				require.Equal(t, "keep", chain[1].Params["setting"])
				require.Equal(t, &condition, chain[1].ExecutionCondition)
				require.Equal(t, expected, chain[1].Params["requestModel"].(map[string]interface{})["identifier"])
				require.Equal(t, "operation-mapping", chain[2].Params["requestModel"])
			}
			require.Equal(t, "stale", shared["requestModel"].(map[string]interface{})["identifier"])
		})
	}
}

func TestGlobalModelRoutingExpandsOnlyChangedRequestModelMappings(t *testing.T) {
	deny := []api.Policy{{Name: "respond", Version: "v1"}}
	ops := []api.Operation{
		{Path: api.Ptr("/*"), Method: api.Ptr(api.OperationMethod("POST"))},
		{Path: api.Ptr("/special/*"), Method: api.Ptr(api.OperationMethod("POST")), Policies: &deny},
	}
	globals := []api.Policy{{Name: "time-based-model-routing", Version: "v0"}}

	expanded := expandGlobalModelRoutingOperations(ops, &globals, globalRoutingTemplate())
	require.Len(t, expanded, 3)
	require.Equal(t, "/special/exact", expanded[0].EffectivePath())
	require.Equal(t, deny, *expanded[0].Policies)
	require.Equal(t, ops, expandGlobalModelRoutingOperations(ops, nil, globalRoutingTemplate()))

	methods := []api.OperationMethod{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	catchAll := make([]api.Operation, 0, len(methods))
	for _, method := range methods {
		catchAll = append(catchAll, api.Operation{Path: api.Ptr("/*"), Method: api.Ptr(method)})
	}
	require.Equal(t, catchAll, expandGlobalModelRoutingOperations(catchAll, &globals, globalRoutingTemplateWithUnchangedRequestModel()))
}

func TestGlobalModelRoutingExpansionRunsForProvidersAndProxies(t *testing.T) {
	db := newTestMockDB()
	store := storage.NewConfigStore()
	template := globalRoutingTemplate()
	template.UUID = "global-routing-template"
	require.NoError(t, db.SaveLLMProviderTemplate(template))

	upstreamURL := "https://example.com"
	providerSource := api.LLMProviderConfiguration{
		Metadata: api.Metadata{Name: "global-routing-provider"},
		Spec: api.LLMProviderConfigData{
			DisplayName: "global-routing-provider",
			Version:     "v1.0",
			Context:     api.Ptr("/global-routing-provider"),
			Template:    template.GetHandle(),
			Upstream: api.LLMProviderConfigData_Upstream{
				Url: &upstreamURL,
			},
			AccessControl: api.LLMAccessControl{Mode: api.AllowAll},
		},
	}
	require.NoError(t, db.SaveConfig(&models.StoredConfig{
		UUID:   "global-routing-provider",
		Kind:   string(api.LLMProviderConfigurationKindLlmProvider),
		Handle: "global-routing-provider",
		Configuration: api.RestAPI{Spec: api.APIConfigData{
			Context: "/global-routing-provider",
		}},
		SourceConfiguration: providerSource,
	}))

	globals := []api.Policy{{Name: "time-based-model-routing", Version: "v0"}}
	providerSource.Spec.GlobalPolicies = &globals
	proxy := api.LLMProxyConfiguration{
		Metadata: api.Metadata{Name: "global-routing-proxy"},
		Spec: api.LLMProxyConfigData{
			DisplayName:    "global-routing-proxy",
			Version:        "v1.0",
			Provider:       api.LLMProxyProvider{Id: "global-routing-provider"},
			GlobalPolicies: &globals,
		},
	}

	transformer := NewLLMProviderTransformer(store, db, &config.RouterConfig{ListenerPort: 8080}, newTestPolicyVersionResolver())
	for name, input := range map[string]interface{}{
		"provider": &providerSource,
		"proxy":    &proxy,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := transformer.Transform(input, &api.RestAPI{})
			require.NoError(t, err)
			require.True(t, hasOperation(result.Spec.Operations, "/special/exact", "POST"))
			require.True(t, hasOperation(result.Spec.Operations, "/special/*", "POST"))
			require.True(t, hasOperation(result.Spec.Operations, "/*", "POST"))
		})
	}
}

func hasOperation(operations []api.Operation, path, method string) bool {
	for _, operation := range operations {
		if operation.EffectivePath() == path && operation.EffectiveMethod() == method {
			return true
		}
	}
	return false
}

func TestGlobalModelRoutingRejectsMissingAndUnsupportedMappings(t *testing.T) {
	t.Run("missing requestModel", func(t *testing.T) {
		rdc := runtimeConfigWithGlobalRoutingPolicy("cost-based-model-routing", "/messages")
		err := resolveGlobalRequestModels(rdc, &models.StoredLLMProviderTemplate{Configuration: api.LLMProviderTemplate{Metadata: api.Metadata{Name: "missing-model"}}})
		require.ErrorContains(t, err, "has no requestModel mapping")
	})

	t.Run("semantic routing requires payload", func(t *testing.T) {
		rdc := runtimeConfigWithGlobalRoutingPolicy("semantic-model-routing", "/messages")
		template := &models.StoredLLMProviderTemplate{Configuration: api.LLMProviderTemplate{Spec: api.LLMProviderTemplateData{
			RequestModel: &api.ExtractionIdentifier{Location: "header", Identifier: "x-model"},
		}}}
		err := resolveGlobalRequestModels(rdc, template)
		require.ErrorContains(t, err, "supports only payload")
	})
}

func runtimeConfigWithGlobalRoutingPolicy(name, path string) *models.RuntimeDeployConfig {
	return &models.RuntimeDeployConfig{
		Routes: map[string]*models.Route{"route": {OperationPath: path}},
		PolicyChains: map[string]*models.PolicyChain{"route": {Policies: []models.Policy{{
			Name: name, Params: map[string]interface{}{"attachedTo": "api"},
		}}}},
	}
}
