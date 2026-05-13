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

package gatewayclient

import "fmt"

// Resource path fragments under ManagementAPIBasePath. Each constant matches
// the corresponding path in gateway-controller's management-openapi.yaml.
const (
	llmProvidersPath         = ManagementAPIBasePath + "/llm-providers"
	llmProviderTemplatesPath = ManagementAPIBasePath + "/llm-provider-templates"
	llmProxiesPath           = ManagementAPIBasePath + "/llm-proxies"
	mcpProxiesPath           = ManagementAPIBasePath + "/mcp-proxies"
	secretsPath              = ManagementAPIBasePath + "/secrets"
	certificatesPath         = ManagementAPIBasePath + "/certificates"
	subscriptionPlansPath    = ManagementAPIBasePath + "/subscription-plans"
	subscriptionsPath        = ManagementAPIBasePath + "/subscriptions"
)

// ApiKeyParentKind values map to the parent path segment used when nesting
// /api-keys/{name}.
const (
	ApiKeyParentKindRestApi     = "RestApi"
	ApiKeyParentKindLlmProvider = "LlmProvider"
	ApiKeyParentKindLlmProxy    = "LlmProxy"
)

// apiKeyParentPath returns the management-API path fragment for the parent
// of an API key resource. Use BuildAPIKeysPath to compose the full URL with
// the parent name and (optional) key handle.
func apiKeyParentPath(kind string) (string, error) {
	switch kind {
	case ApiKeyParentKindRestApi:
		return restAPIsResourcePath, nil
	case ApiKeyParentKindLlmProvider:
		return llmProvidersPath, nil
	case ApiKeyParentKindLlmProxy:
		return llmProxiesPath, nil
	default:
		return "", fmt.Errorf("unsupported ApiKey parent kind %q", kind)
	}
}

// LLMProvidersPath returns the configured /llm-providers base path.
func LLMProvidersPath() string { return llmProvidersPath }

// LLMProviderTemplatesPath returns the configured /llm-provider-templates base path.
func LLMProviderTemplatesPath() string { return llmProviderTemplatesPath }

// LLMProxiesPath returns the configured /llm-proxies base path.
func LLMProxiesPath() string { return llmProxiesPath }

// MCPProxiesPath returns the configured /mcp-proxies base path.
func MCPProxiesPath() string { return mcpProxiesPath }

// SecretsPath returns the configured /secrets base path.
func SecretsPath() string { return secretsPath }

// CertificatesPath returns the configured /certificates base path.
func CertificatesPath() string { return certificatesPath }

// SubscriptionPlansPath returns the configured /subscription-plans base path.
func SubscriptionPlansPath() string { return subscriptionPlansPath }

// SubscriptionsPath returns the configured /subscriptions base path.
func SubscriptionsPath() string { return subscriptionsPath }
