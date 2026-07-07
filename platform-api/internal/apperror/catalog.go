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

package apperror

import (
	"net/http"

	"github.com/wso2/api-platform/platform-api/internal/utils"
)

// This file is the error catalog: every client-facing error condition is
// declared here exactly once, binding its code, HTTP status, and message
// template together (see resources/ERROR_CATALOG_PLAN.md). Codes reference
// the string constants in utils/codes.go, which stay the single source for
// the code values while handlers not yet on the error-mapper pattern still
// build responses from them directly.
//
// Message templates are client-sterile: no internal detail, no file paths,
// no raw error text. A template of "%s" marks a code whose human message is
// genuinely call-site-specific (validation-style codes); interpolated args
// must be user-supplied values (names, handles, IDs), never internal error
// strings. Fixed-message entries are called with no args.
//
// allDefs feeds the catalog integrity test (catalog_test.go): unique codes,
// valid statuses, and message/verb sanity are asserted there.

var allDefs []Def

func def(code string, status int, messageFmt string) Def {
	d := Def{Code: code, HTTPStatus: status, MessageFmt: messageFmt}
	allDefs = append(allDefs, d)
	return d
}

// Common, domain-agnostic entries. Unauthorized is intentionally the ONLY
// 401 entry, with a fixed message: every authentication failure (missing,
// expired, invalid, revoked) returns the identical payload per the unified
// auth-failure rule in error-handling.md; the specific reason travels in the
// wrapped cause / log message only.
var (
	Unauthorized        = def(utils.CodeCommonUnauthorized, http.StatusUnauthorized, "Invalid or expired credentials.")
	Forbidden           = def(utils.CodeCommonForbidden, http.StatusForbidden, "You do not have permission to perform this action.")
	ValidationFailed    = def(utils.CodeCommonValidationFailed, http.StatusBadRequest, "%s")
	NotFound            = def(utils.CodeCommonNotFound, http.StatusNotFound, "The requested resource could not be found.")
	Conflict            = def(utils.CodeCommonConflict, http.StatusConflict, "The request conflicts with the current state of the resource.")
	NotAcceptable       = def(utils.CodeCommonNotAcceptable, http.StatusNotAcceptable, "The requested media type is not supported.")
	UnprocessableEntity = def(utils.CodeCommonUnprocessableEntity, http.StatusUnprocessableEntity, "The request could not be processed.")
	Internal            = def(utils.CodeCommonInternalError, http.StatusInternalServerError, "An unexpected error occurred.")
	ServiceUnavailable  = def(utils.CodeCommonServiceUnavailable, http.StatusServiceUnavailable, "The service is temporarily unavailable.")
	TooManyRequests     = def(utils.CodeCommonTooManyRequests, http.StatusTooManyRequests, "%s")
)

// REST API entries.
var (
	RESTAPINotFound = def(utils.CodeRESTAPINotFound, http.StatusNotFound, "The specified REST API could not be found.")
	// RESTAPIExists covers three distinct conflicts (handle, name+version,
	// project) — the call site supplies which one.
	RESTAPIExists                     = def(utils.CodeRESTAPIExists, http.StatusConflict, "%s")
	RESTAPIDeploymentValidationFailed = def(utils.CodeRESTAPIDeploymentValidationFailed, http.StatusBadRequest, "%s")
)

// LLM provider / proxy entries.
var (
	LLMProviderNotFound                   = def(utils.CodeLLMProviderNotFound, http.StatusNotFound, "The specified LLM provider could not be found.")
	LLMProviderRefNotFound                = def(utils.CodeLLMProviderRefNotFound, http.StatusBadRequest, "The referenced LLM provider could not be found.")
	LLMProviderExists                     = def(utils.CodeLLMProviderExists, http.StatusConflict, "An LLM provider with this ID already exists.")
	LLMProviderLimitReached               = def(utils.CodeLLMProviderLimitReached, http.StatusConflict, "LLM provider limit reached for the organization.")
	LLMProviderAPIKeyNotFound             = def(utils.CodeLLMProviderAPIKeyNotFound, http.StatusNotFound, "The specified API key could not be found.")
	LLMProviderAPIKeyForbidden            = def(utils.CodeLLMProviderAPIKeyForbidden, http.StatusForbidden, "You do not have permission to access this API key.")
	LLMProviderDeploymentValidationFailed = def(utils.CodeLLMProviderDeploymentValidationFailed, http.StatusBadRequest, "%s")

	LLMProxyNotFound                   = def(utils.CodeLLMProxyNotFound, http.StatusNotFound, "The specified LLM proxy could not be found.")
	LLMProxyExists                     = def(utils.CodeLLMProxyExists, http.StatusConflict, "An LLM proxy with this ID already exists.")
	LLMProxyLimitReached               = def(utils.CodeLLMProxyLimitReached, http.StatusConflict, "LLM proxy limit reached for the organization.")
	LLMProxyAPIKeyNotFound             = def(utils.CodeLLMProxyAPIKeyNotFound, http.StatusNotFound, "The specified API key could not be found.")
	LLMProxyAPIKeyForbidden            = def(utils.CodeLLMProxyAPIKeyForbidden, http.StatusForbidden, "You do not have permission to access this API key.")
	LLMProxyDeploymentValidationFailed = def(utils.CodeLLMProxyDeploymentValidationFailed, http.StatusBadRequest, "%s")
)

// LLM provider template entries.
var (
	LLMProviderTemplateNotFound          = def(utils.CodeLLMProviderTemplateNotFound, http.StatusNotFound, "The specified LLM provider template could not be found.")
	LLMProviderTemplateVersionNotFound   = def(utils.CodeLLMProviderTemplateVersionNotFound, http.StatusNotFound, "The specified LLM provider template version could not be found.")
	LLMProviderTemplateRefNotFound       = def(utils.CodeLLMProviderTemplateRefNotFound, http.StatusBadRequest, "The referenced LLM provider template could not be found.")
	LLMProviderTemplateExists            = def(utils.CodeLLMProviderTemplateExists, http.StatusConflict, "An LLM provider template with this ID already exists.")
	LLMProviderTemplateVersionExists     = def(utils.CodeLLMProviderTemplateVersionExists, http.StatusConflict, "This template version already exists.")
	LLMProviderTemplateManagedByReserved = def(utils.CodeLLMProviderTemplateManagedByReserved, http.StatusBadRequest, "'wso2' is reserved and cannot be used as managedBy on custom templates.")
	LLMProviderTemplateInUse             = def(utils.CodeLLMProviderTemplateInUse, http.StatusConflict, "This template version is in use by one or more providers.")
	LLMProviderTemplateReadOnly          = def(utils.CodeLLMProviderTemplateReadOnly, http.StatusForbidden, "Built-in templates are read-only and cannot be modified.")
	LLMProviderTemplateNotToggleable     = def(utils.CodeLLMProviderTemplateNotToggleable, http.StatusForbidden, "Only built-in templates can be enabled or disabled.")
)

// Gateway entries.
var (
	GatewayNotFound              = def(utils.CodeGatewayNotFound, http.StatusNotFound, "The specified gateway could not be found.")
	GatewayTokenNotFound         = def(utils.CodeGatewayTokenNotFound, http.StatusNotFound, "The specified gateway token could not be found.")
	GatewayConnectionUnavailable = def(utils.CodeGatewayConnectionUnavailable, http.StatusServiceUnavailable, "No gateway connections are currently available.")
	GatewayHasActiveDeployments  = def(utils.CodeGatewayHasActiveDeployments, http.StatusConflict, "The gateway has active deployments and cannot be deleted.")
	GatewayNameConflict          = def(utils.CodeGatewayNameConflict, http.StatusConflict, "A gateway with this name already exists.")
	GatewayTokenLimitReached     = def(utils.CodeGatewayTokenLimitReached, http.StatusConflict, "Gateway token limit reached.")
)

// Deployment entries, shared across REST API / LLM provider / LLM proxy /
// MCP proxy deployment operations. DeploymentNotActive's verb is the artifact
// kind, e.g. "API", "LLM provider".
var (
	DeploymentBaseNotFound    = def(utils.CodeDeploymentBaseNotFound, http.StatusNotFound, "The specified base deployment could not be found.")
	DeploymentRestoreConflict = def(utils.CodeDeploymentRestoreConflict, http.StatusConflict, "Cannot restore the currently deployed deployment, or the deployment is invalid.")
	DeploymentNotFound        = def(utils.CodeDeploymentNotFound, http.StatusNotFound, "The specified deployment could not be found.")
	DeploymentNotActive       = def(utils.CodeDeploymentNotActive, http.StatusConflict, "No active deployment found for this %s on the gateway.")
	DeploymentGatewayMismatch = def(utils.CodeDeploymentGatewayMismatch, http.StatusBadRequest, "Deployment is bound to a different gateway.")
	DeploymentActive          = def(utils.CodeDeploymentActive, http.StatusConflict, "Cannot delete an active deployment - undeploy it first.")
	DeploymentInvalidStatus   = def(utils.CodeDeploymentInvalidStatus, http.StatusBadRequest, "The specified deployment status filter is invalid.")
)

// MCP proxy entries.
var (
	MCPProxyNotFound                   = def(utils.CodeMCPProxyNotFound, http.StatusNotFound, "The specified MCP proxy could not be found.")
	MCPProxyExists                     = def(utils.CodeMCPProxyExists, http.StatusConflict, "An MCP proxy with this ID already exists.")
	MCPProxyLimitReached               = def(utils.CodeMCPProxyLimitReached, http.StatusConflict, "MCP proxy limit reached for the organization.")
	MCPProxyDeploymentValidationFailed = def(utils.CodeMCPProxyDeploymentValidationFailed, http.StatusBadRequest, "%s")
)

// Organization / project / application entries.
var (
	OrganizationNotFound = def(utils.CodeOrganizationNotFound, http.StatusNotFound, "The specified organization could not be found.")
	OrganizationExists   = def(utils.CodeOrganizationExists, http.StatusConflict, "An organization with this name already exists.")
	ProjectNotFound      = def(utils.CodeProjectNotFound, http.StatusNotFound, "The specified project could not be found.")
	ProjectRefNotFound   = def(utils.CodeProjectRefNotFound, http.StatusBadRequest, "The referenced project could not be found.")
	ProjectExists        = def(utils.CodeProjectExists, http.StatusConflict, "A project with this name already exists in the organization.")
	ApplicationNotFound  = def(utils.CodeApplicationNotFound, http.StatusNotFound, "The specified application could not be found.")
	ApplicationExists    = def(utils.CodeApplicationExists, http.StatusConflict, "An application with this name already exists.")
)

// Subscription entries.
var (
	SubscriptionNotFound     = def(utils.CodeSubscriptionNotFound, http.StatusNotFound, "The specified subscription could not be found.")
	SubscriptionExists       = def(utils.CodeSubscriptionExists, http.StatusConflict, "A subscription for this application and API already exists.")
	SubscriptionForbidden    = def(utils.CodeSubscriptionForbidden, http.StatusForbidden, "You do not have permission to access this subscription.")
	SubscriptionPlanNotFound = def(utils.CodeSubscriptionPlanNotFound, http.StatusNotFound, "The specified subscription plan could not be found.")
	SubscriptionPlanExists   = def(utils.CodeSubscriptionPlanExists, http.StatusConflict, "A subscription plan with this name already exists.")
)

// Custom policy entries.
var (
	PolicyVersionConflict     = def(utils.CodePolicyVersionConflict, http.StatusConflict, "The policy version conflicts with an existing version.")
	PolicyInvalidState        = def(utils.CodePolicyInvalidState, http.StatusConflict, "The policy is not in a valid state for this operation.")
	PolicyInUse               = def(utils.CodePolicyInUse, http.StatusConflict, "The policy is in use and cannot be deleted.")
	CustomPolicyNotFound      = def(utils.CodeCustomPolicyNotFound, http.StatusNotFound, "The specified policy could not be found.")
	CustomPolicyVersionNotFnd = def(utils.CodeCustomPolicyVersionNotFound, http.StatusNotFound, "The specified policy version could not be found.")
)

// Secret entries.
var (
	SecretNotFound = def(utils.CodeSecretNotFound, http.StatusNotFound, "The specified secret could not be found.")
	SecretExists   = def(utils.CodeSecretExists, http.StatusConflict, "A secret with this name already exists.")
	SecretInUse    = def(utils.CodeSecretInUse, http.StatusConflict, "The secret is in use and cannot be deleted.")
)

// Artifact entries — generic artifact-reference flows and the
// data-plane-origin guard. The guard produces client-appropriate messages at
// the call site, so these are passthrough templates.
var (
	ArtifactNotFound         = def(utils.CodeArtifactNotFound, http.StatusNotFound, "The specified artifact could not be found.")
	ArtifactExists           = def(utils.CodeArtifactExists, http.StatusConflict, "The artifact already exists.")
	ArtifactReadOnly         = def(utils.CodeArtifactReadOnly, http.StatusForbidden, "%s")
	ArtifactRuntimeImmutable = def(utils.CodeArtifactRuntimeImmutable, http.StatusForbidden, "%s")
	ArtifactDeployed         = def(utils.CodeArtifactDeployed, http.StatusConflict, "%s")
)

// Application API key entries.
var (
	ApplicationAPIKeyNotFound  = def(utils.CodeApplicationAPIKeyNotFound, http.StatusNotFound, "The specified API key could not be found.")
	ApplicationAPIKeyForbidden = def(utils.CodeApplicationAPIKeyForbidden, http.StatusForbidden, "You do not have permission to access this API key.")
)
