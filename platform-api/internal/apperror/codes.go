/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package apperror

// Common, domain-agnostic catalog codes. These mirror the codes documented
// as shared response examples in resources/openapi.yaml, and are
// the fallback used by NewErrorResponse when a handler hasn't been
// migrated to a more specific domain code via NewErrorResponseWithCode.
const (
	CodeCommonValidationFailed    = "VALIDATION_FAILED"
	CodeCommonUnauthorized        = "UNAUTHORIZED"
	CodeCommonForbidden           = "FORBIDDEN"
	CodeCommonNotFound            = "NOT_FOUND"
	CodeCommonConflict            = "CONFLICT"
	CodeCommonNotAcceptable       = "NOT_ACCEPTABLE"
	CodeCommonUnprocessableEntity = "UNPROCESSABLE_ENTITY"
	CodeCommonInternalError       = "INTERNAL_ERROR"
	CodeCommonServiceUnavailable  = "SERVICE_UNAVAILABLE"
	CodeCommonTooManyRequests     = "TOO_MANY_REQUESTS"
)

// LLM provider/proxy domain codes, matching the examples documented in
// resources/openapi.yaml for the corresponding operations.
const (
	CodeLLMProviderNotFound                   = "LLM_PROVIDER_NOT_FOUND"
	CodeLLMProviderExists                     = "LLM_PROVIDER_EXISTS"
	CodeLLMProviderAPIKeyNotFound             = "LLM_PROVIDER_API_KEY_NOT_FOUND"
	CodeLLMProviderDeploymentValidationFailed = "LLM_PROVIDER_DEPLOYMENT_VALIDATION_FAILED"
	CodeLLMProxyNotFound                      = "LLM_PROXY_NOT_FOUND"
	CodeLLMProxyExists                        = "LLM_PROXY_EXISTS"
	CodeLLMProxyAPIKeyNotFound                = "LLM_PROXY_API_KEY_NOT_FOUND"
	CodeLLMProviderRefNotFound                = "LLM_PROVIDER_REF_NOT_FOUND"
	CodeLLMProxyDeploymentValidationFailed    = "LLM_PROXY_DEPLOYMENT_VALIDATION_FAILED"
	CodeLLMProviderAPIKeyForbidden            = "LLM_PROVIDER_API_KEY_FORBIDDEN"
	CodeLLMProxyAPIKeyForbidden               = "LLM_PROXY_API_KEY_FORBIDDEN"
)

// LLM provider template domain codes. The *_VERSION_* and *_REF_* codes keep
// one HTTP status per code in the apperror catalog: NOT_FOUND codes are 404s
// for the resource targeted by the URL, REF_NOT_FOUND codes are 400s for a
// resource referenced in the request body, and VERSION codes distinguish a
// missing/duplicate version from a missing/duplicate template.
const (
	CodeLLMProviderTemplateNotFound          = "LLM_PROVIDER_TEMPLATE_NOT_FOUND"
	CodeLLMProviderTemplateVersionNotFound   = "LLM_PROVIDER_TEMPLATE_VERSION_NOT_FOUND"
	CodeLLMProviderTemplateRefNotFound       = "LLM_PROVIDER_TEMPLATE_REF_NOT_FOUND"
	CodeLLMProviderTemplateExists            = "LLM_PROVIDER_TEMPLATE_EXISTS"
	CodeLLMProviderTemplateVersionExists     = "LLM_PROVIDER_TEMPLATE_VERSION_EXISTS"
	CodeLLMProviderTemplateManagedByReserved = "LLM_PROVIDER_TEMPLATE_MANAGED_BY_RESERVED"
	CodeLLMProviderTemplateInUse             = "LLM_PROVIDER_TEMPLATE_IN_USE"
	CodeLLMProviderTemplateDisabled          = "LLM_PROVIDER_TEMPLATE_DISABLED"
	CodeLLMProviderTemplateReadOnly          = "LLM_PROVIDER_TEMPLATE_READ_ONLY"
	CodeLLMProviderTemplateNotToggleable     = "LLM_PROVIDER_TEMPLATE_NOT_TOGGLEABLE"
)

// Gateway domain codes, matching the examples documented in resources/openapi.yaml.
const (
	CodeGatewayNotFound              = "GATEWAY_NOT_FOUND"
	CodeGatewayTokenNotFound         = "GATEWAY_TOKEN_NOT_FOUND"
	CodeGatewayConnectionUnavailable = "GATEWAY_CONNECTION_UNAVAILABLE"
	CodeGatewayHasActiveDeployments  = "GATEWAY_HAS_ACTIVE_DEPLOYMENTS"
	CodeGatewayNameConflict          = "GATEWAY_NAME_CONFLICT"
	CodeGatewayTokenLimitReached     = "GATEWAY_TOKEN_LIMIT_REACHED"
)

// Deployment domain codes, shared across REST API / LLM provider / LLM proxy /
// MCP proxy deployment operations (identical conditions across all four).
const (
	CodeDeploymentBaseNotFound    = "DEPLOYMENT_BASE_NOT_FOUND"
	CodeDeploymentRestoreConflict = "DEPLOYMENT_RESTORE_CONFLICT"
	CodeDeploymentNotFound        = "DEPLOYMENT_NOT_FOUND"
	CodeDeploymentNotActive       = "DEPLOYMENT_NOT_ACTIVE"
	CodeDeploymentGatewayMismatch = "DEPLOYMENT_GATEWAY_MISMATCH"
	CodeDeploymentActive          = "DEPLOYMENT_ACTIVE"
	CodeDeploymentInvalidStatus   = "DEPLOYMENT_INVALID_STATUS"
)

// REST API domain codes.
const (
	CodeRESTAPINotFound                   = "REST_API_NOT_FOUND"
	CodeRESTAPIExists                     = "REST_API_EXISTS"
	CodeRESTAPIDeploymentValidationFailed = "REST_API_DEPLOYMENT_VALIDATION_FAILED"
)

// MCP proxy domain codes.
const (
	CodeMCPProxyNotFound                   = "MCP_PROXY_NOT_FOUND"
	CodeMCPProxyExists                     = "MCP_PROXY_EXISTS"
	CodeMCPProxyDeploymentValidationFailed = "MCP_PROXY_DEPLOYMENT_VALIDATION_FAILED"
)

// Organization domain codes.
const (
	CodeOrganizationNotFound = "ORGANIZATION_NOT_FOUND"
	CodeOrganizationExists   = "ORGANIZATION_EXISTS"
)

// Project domain codes. PROJECT_REF_NOT_FOUND is the 400 counterpart of
// PROJECT_NOT_FOUND for a project referenced in a request body (e.g. the
// projectId in a create request) rather than targeted by the URL.
const (
	CodeProjectNotFound    = "PROJECT_NOT_FOUND"
	CodeProjectRefNotFound = "PROJECT_REF_NOT_FOUND"
	CodeProjectExists      = "PROJECT_EXISTS"
)

// Application domain codes.
const (
	CodeApplicationNotFound = "APPLICATION_NOT_FOUND"
	CodeApplicationExists   = "APPLICATION_EXISTS"
)

// Subscription domain codes.
const (
	CodeSubscriptionNotFound     = "SUBSCRIPTION_NOT_FOUND"
	CodeSubscriptionExists       = "SUBSCRIPTION_EXISTS"
	CodeSubscriptionForbidden    = "SUBSCRIPTION_FORBIDDEN"
	CodeSubscriptionPlanNotFound = "SUBSCRIPTION_PLAN_NOT_FOUND"
	CodeSubscriptionPlanExists   = "SUBSCRIPTION_PLAN_EXISTS"
)

// Custom policy domain codes.
const (
	CodePolicyVersionConflict = "POLICY_VERSION_CONFLICT"
	CodePolicyInvalidState    = "POLICY_INVALID_STATE"
	CodePolicyInUse           = "POLICY_IN_USE"
)

// Secret domain codes.
const (
	CodeSecretNotFound = "SECRET_NOT_FOUND"
	CodeSecretExists   = "SECRET_EXISTS"
	CodeSecretInUse    = "SECRET_IN_USE"
)

// Artifact domain codes. Used by flows that operate on a generic artifact
// reference (REST API / LLM provider / LLM proxy / MCP proxy) — API keys,
// subscriptions, and application associations — where the caller shouldn't
// need to know which concrete artifact kind was targeted, and by the
// data-plane-origin guard that protects DP-originated artifacts from
// control-plane mutation.
const (
	CodeArtifactNotFound         = "ARTIFACT_NOT_FOUND"
	CodeArtifactExists           = "ARTIFACT_EXISTS"
	CodeArtifactReadOnly         = "ARTIFACT_READ_ONLY"
	CodeArtifactRuntimeImmutable = "ARTIFACT_RUNTIME_IMMUTABLE"
	CodeArtifactDeployed         = "ARTIFACT_DEPLOYED"
)

// Custom policy domain codes.
const (
	CodeCustomPolicyNotFound        = "CUSTOM_POLICY_NOT_FOUND"
	CodeCustomPolicyVersionNotFound = "CUSTOM_POLICY_VERSION_NOT_FOUND"
)

// Application API key domain codes (application-scoped API key mappings,
// distinct from the LLM provider/proxy API key codes above).
const (
	CodeApplicationAPIKeyNotFound  = "APPLICATION_API_KEY_NOT_FOUND"
	CodeApplicationAPIKeyForbidden = "APPLICATION_API_KEY_FORBIDDEN"
)

// WebSub and WebBroker API domain codes, raised by the event-gateway plugin
// and by the two gateway-internal artifact lookups.
const (
	CodeWebSubAPINotFound    = "WEBSUB_API_NOT_FOUND"
	CodeWebSubAPIExists      = "WEBSUB_API_EXISTS"
	CodeWebBrokerAPINotFound = "WEBBROKER_API_NOT_FOUND"
	CodeWebBrokerAPIExists   = "WEBBROKER_API_EXISTS"
)

// HMAC secret domain codes (WebSub subscriber callback signing secrets).
// HMAC_SECRET_NOT_CONFIGURED is a 503 rather than a 500: the encryption key is
// a deployment-time setting, so the condition is transient from the client's
// point of view and resolvable without a code change.
const (
	CodeHmacSecretNotFound      = "HMAC_SECRET_NOT_FOUND"
	CodeHmacSecretExists        = "HMAC_SECRET_EXISTS"
	CodeHmacSecretInvalidValue  = "HMAC_SECRET_INVALID_VALUE"
	CodeHmacSecretNotConfigured = "HMAC_SECRET_NOT_CONFIGURED"
)
