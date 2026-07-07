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

package utils

import "net/http"

// Common, domain-agnostic catalog codes. These mirror the COMMON_* codes
// documented as shared response examples in resources/openapi.yaml, and are
// the fallback used by NewErrorResponse when a handler hasn't been migrated
// to a more specific domain code via NewErrorResponseWithCode.
const (
	CodeCommonValidationFailed    = "COMMON_VALIDATION_FAILED"
	CodeCommonUnauthorized        = "COMMON_UNAUTHORIZED"
	CodeCommonForbidden           = "COMMON_FORBIDDEN"
	CodeCommonNotFound            = "COMMON_NOT_FOUND"
	CodeCommonConflict            = "COMMON_CONFLICT"
	CodeCommonNotAcceptable       = "COMMON_NOT_ACCEPTABLE"
	CodeCommonUnprocessableEntity = "COMMON_UNPROCESSABLE_ENTITY"
	CodeCommonInternalError       = "COMMON_INTERNAL_ERROR"
	CodeCommonServiceUnavailable  = "COMMON_SERVICE_UNAVAILABLE"
)

// commonCodeByStatus maps an HTTP status code to its fallback catalog code.
// 400 maps to COMMON_VALIDATION_FAILED (not a separate COMMON_BAD_REQUEST) to
// match the single code documented for the shared BadRequest response in
// resources/openapi.yaml.
var commonCodeByStatus = map[int]string{
	http.StatusBadRequest:          CodeCommonValidationFailed,
	http.StatusUnauthorized:        CodeCommonUnauthorized,
	http.StatusForbidden:           CodeCommonForbidden,
	http.StatusNotFound:            CodeCommonNotFound,
	http.StatusConflict:            CodeCommonConflict,
	http.StatusNotAcceptable:       CodeCommonNotAcceptable,
	http.StatusUnprocessableEntity: CodeCommonUnprocessableEntity,
	http.StatusInternalServerError: CodeCommonInternalError,
	http.StatusServiceUnavailable:  CodeCommonServiceUnavailable,
}

// codeForStatus returns the fallback catalog code for an HTTP status,
// defaulting to COMMON_INTERNAL_ERROR for unmapped statuses.
func codeForStatus(httpStatus int) string {
	if code, ok := commonCodeByStatus[httpStatus]; ok {
		return code
	}
	return CodeCommonInternalError
}

// LLM provider/proxy domain codes, matching the examples documented in
// resources/openapi.yaml for the corresponding operations.
const (
	CodeLLMProviderNotFound                   = "LLM_PROVIDER_NOT_FOUND"
	CodeLLMProviderExists                     = "LLM_PROVIDER_EXISTS"
	CodeLLMProviderLimitReached               = "LLM_PROVIDER_LIMIT_REACHED"
	CodeLLMProviderAPIKeyNotFound             = "LLM_PROVIDER_API_KEY_NOT_FOUND"
	CodeLLMProviderDeploymentValidationFailed = "LLM_PROVIDER_DEPLOYMENT_VALIDATION_FAILED"
	CodeLLMProxyNotFound                      = "LLM_PROXY_NOT_FOUND"
	CodeLLMProxyExists                        = "LLM_PROXY_EXISTS"
	CodeLLMProxyLimitReached                  = "LLM_PROXY_LIMIT_REACHED"
	CodeLLMProxyAPIKeyNotFound                = "LLM_PROXY_API_KEY_NOT_FOUND"
	CodeLLMProxyDeploymentValidationFailed    = "LLM_PROXY_DEPLOYMENT_VALIDATION_FAILED"
)

// LLM provider template domain codes.
const (
	CodeLLMProviderTemplateNotFound          = "LLM_PROVIDER_TEMPLATE_NOT_FOUND"
	CodeLLMProviderTemplateExists            = "LLM_PROVIDER_TEMPLATE_EXISTS"
	CodeLLMProviderTemplateManagedByReserved = "LLM_PROVIDER_TEMPLATE_MANAGED_BY_RESERVED"
	CodeLLMProviderTemplateInUse             = "LLM_PROVIDER_TEMPLATE_IN_USE"
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
	CodeMCPProxyLimitReached               = "MCP_PROXY_LIMIT_REACHED"
	CodeMCPProxyDeploymentValidationFailed = "MCP_PROXY_DEPLOYMENT_VALIDATION_FAILED"
)

// Organization domain codes.
const (
	CodeOrganizationNotFound = "ORGANIZATION_NOT_FOUND"
	CodeOrganizationExists   = "ORGANIZATION_EXISTS"
)

// Project domain codes.
const (
	CodeProjectNotFound = "PROJECT_NOT_FOUND"
	CodeProjectExists   = "PROJECT_EXISTS"
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
	CodeSecretInUse = "SECRET_IN_USE"
)
