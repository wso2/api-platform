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
)

// This file is the error catalog: every client-facing error condition is
// declared here exactly once, binding its code, HTTP status, and message
// template together (see resources/ERROR_CATALOG_PLAN.md). Codes reference
// the string constants in codes.go, which stay the single source for the
// code values while handlers not yet on the error-mapper pattern still
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
	Unauthorized        = def(CodeCommonUnauthorized, http.StatusUnauthorized, "Invalid or expired credentials.")
	Forbidden           = def(CodeCommonForbidden, http.StatusForbidden, "You do not have permission to perform this action.")
	ValidationFailed    = def(CodeCommonValidationFailed, http.StatusBadRequest, "%s")
	NotFound            = def(CodeCommonNotFound, http.StatusNotFound, "The requested resource could not be found.")
	Conflict            = def(CodeCommonConflict, http.StatusConflict, "The request conflicts with the current state of the resource.")
	NotAcceptable       = def(CodeCommonNotAcceptable, http.StatusNotAcceptable, "The requested media type is not supported.")
	UnprocessableEntity = def(CodeCommonUnprocessableEntity, http.StatusUnprocessableEntity, "The request could not be processed.")
	Internal            = def(CodeCommonInternalError, http.StatusInternalServerError, "An unexpected error occurred.")
	ServiceUnavailable  = def(CodeCommonServiceUnavailable, http.StatusServiceUnavailable, "The service is temporarily unavailable.")
	TooManyRequests     = def(CodeCommonTooManyRequests, http.StatusTooManyRequests, "%s")
)

// REST API entries.
var (
	RESTAPINotFound = def(CodeRESTAPINotFound, http.StatusNotFound, "The specified REST API could not be found.")
	// RESTAPIExists covers three distinct conflicts (handle, name+version,
	// project) — the call site supplies which one.
	RESTAPIExists                     = def(CodeRESTAPIExists, http.StatusConflict, "%s")
	RESTAPIDeploymentValidationFailed = def(CodeRESTAPIDeploymentValidationFailed, http.StatusBadRequest, "%s")
)

// LLM provider / proxy entries.
var (
	LLMProviderNotFound                   = def(CodeLLMProviderNotFound, http.StatusNotFound, "The specified LLM provider could not be found.")
	LLMProviderRefNotFound                = def(CodeLLMProviderRefNotFound, http.StatusBadRequest, "The referenced LLM provider could not be found.")
	LLMProviderExists                     = def(CodeLLMProviderExists, http.StatusConflict, "An LLM provider with this ID already exists.")
	LLMProviderLimitReached               = def(CodeLLMProviderLimitReached, http.StatusConflict, "LLM provider limit reached for the organization.")
	LLMProviderAPIKeyNotFound             = def(CodeLLMProviderAPIKeyNotFound, http.StatusNotFound, "The specified API key could not be found.")
	LLMProviderAPIKeyForbidden            = def(CodeLLMProviderAPIKeyForbidden, http.StatusForbidden, "You do not have permission to access this API key.")
	LLMProviderDeploymentValidationFailed = def(CodeLLMProviderDeploymentValidationFailed, http.StatusBadRequest, "%s")

	LLMProxyNotFound                   = def(CodeLLMProxyNotFound, http.StatusNotFound, "The specified LLM proxy could not be found.")
	LLMProxyExists                     = def(CodeLLMProxyExists, http.StatusConflict, "An LLM proxy with this ID already exists.")
	LLMProxyLimitReached               = def(CodeLLMProxyLimitReached, http.StatusConflict, "LLM proxy limit reached for the organization.")
	LLMProxyAPIKeyNotFound             = def(CodeLLMProxyAPIKeyNotFound, http.StatusNotFound, "The specified API key could not be found.")
	LLMProxyAPIKeyForbidden            = def(CodeLLMProxyAPIKeyForbidden, http.StatusForbidden, "You do not have permission to access this API key.")
	LLMProxyDeploymentValidationFailed = def(CodeLLMProxyDeploymentValidationFailed, http.StatusBadRequest, "%s")
)

// LLM provider template entries.
var (
	LLMProviderTemplateNotFound          = def(CodeLLMProviderTemplateNotFound, http.StatusNotFound, "The specified LLM provider template could not be found.")
	LLMProviderTemplateVersionNotFound   = def(CodeLLMProviderTemplateVersionNotFound, http.StatusNotFound, "The specified LLM provider template version could not be found.")
	LLMProviderTemplateRefNotFound       = def(CodeLLMProviderTemplateRefNotFound, http.StatusBadRequest, "The referenced LLM provider template could not be found.")
	LLMProviderTemplateExists            = def(CodeLLMProviderTemplateExists, http.StatusConflict, "An LLM provider template with this ID already exists.")
	LLMProviderTemplateVersionExists     = def(CodeLLMProviderTemplateVersionExists, http.StatusConflict, "This template version already exists.")
	LLMProviderTemplateManagedByReserved = def(CodeLLMProviderTemplateManagedByReserved, http.StatusBadRequest, "'wso2' is reserved and cannot be used as managedBy on custom templates.")
	LLMProviderTemplateInUse             = def(CodeLLMProviderTemplateInUse, http.StatusConflict, "This template version is in use by one or more providers.")
	LLMProviderTemplateReadOnly          = def(CodeLLMProviderTemplateReadOnly, http.StatusForbidden, "Built-in templates are read-only and cannot be modified.")
	LLMProviderTemplateNotToggleable     = def(CodeLLMProviderTemplateNotToggleable, http.StatusForbidden, "Only built-in templates can be enabled or disabled.")
)

// Gateway entries.
var (
	GatewayNotFound              = def(CodeGatewayNotFound, http.StatusNotFound, "The specified gateway could not be found.")
	GatewayTokenNotFound         = def(CodeGatewayTokenNotFound, http.StatusNotFound, "The specified gateway token could not be found.")
	GatewayConnectionUnavailable = def(CodeGatewayConnectionUnavailable, http.StatusServiceUnavailable, "No gateway connections are currently available.")
	GatewayHasActiveDeployments  = def(CodeGatewayHasActiveDeployments, http.StatusConflict, "The gateway has active deployments and cannot be deleted.")
	GatewayNameConflict          = def(CodeGatewayNameConflict, http.StatusConflict, "A gateway with this name already exists.")
	GatewayTokenLimitReached     = def(CodeGatewayTokenLimitReached, http.StatusConflict, "Gateway token limit reached.")
)

// Deployment entries, shared across REST API / LLM provider / LLM proxy /
// MCP proxy deployment operations. DeploymentNotActive's verb is the artifact
// kind, e.g. "API", "LLM provider".
var (
	DeploymentBaseNotFound    = def(CodeDeploymentBaseNotFound, http.StatusNotFound, "The specified base deployment could not be found.")
	DeploymentRestoreConflict = def(CodeDeploymentRestoreConflict, http.StatusConflict, "Cannot restore the currently deployed deployment, or the deployment is invalid.")
	DeploymentNotFound        = def(CodeDeploymentNotFound, http.StatusNotFound, "The specified deployment could not be found.")
	DeploymentNotActive       = def(CodeDeploymentNotActive, http.StatusConflict, "No active deployment found for this %s on the gateway.")
	DeploymentGatewayMismatch = def(CodeDeploymentGatewayMismatch, http.StatusBadRequest, "Deployment is bound to a different gateway.")
	DeploymentActive          = def(CodeDeploymentActive, http.StatusConflict, "Cannot delete an active deployment - undeploy it first.")
	DeploymentInvalidStatus   = def(CodeDeploymentInvalidStatus, http.StatusBadRequest, "The specified deployment status filter is invalid.")
)

// MCP proxy entries.
var (
	MCPProxyNotFound                   = def(CodeMCPProxyNotFound, http.StatusNotFound, "The specified MCP proxy could not be found.")
	MCPProxyExists                     = def(CodeMCPProxyExists, http.StatusConflict, "An MCP proxy with this ID already exists.")
	MCPProxyLimitReached               = def(CodeMCPProxyLimitReached, http.StatusConflict, "MCP proxy limit reached for the organization.")
	MCPProxyDeploymentValidationFailed = def(CodeMCPProxyDeploymentValidationFailed, http.StatusBadRequest, "%s")
)

// Organization / project / application entries.
var (
	OrganizationNotFound = def(CodeOrganizationNotFound, http.StatusNotFound, "The specified organization could not be found.")
	OrganizationExists   = def(CodeOrganizationExists, http.StatusConflict, "An organization with this name already exists.")
	ProjectNotFound      = def(CodeProjectNotFound, http.StatusNotFound, "The specified project could not be found.")
	ProjectRefNotFound   = def(CodeProjectRefNotFound, http.StatusBadRequest, "The referenced project could not be found.")
	ProjectExists        = def(CodeProjectExists, http.StatusConflict, "A project with this name already exists in the organization.")
	ApplicationNotFound  = def(CodeApplicationNotFound, http.StatusNotFound, "The specified application could not be found.")
	ApplicationExists    = def(CodeApplicationExists, http.StatusConflict, "An application with this name already exists.")
)

// Subscription entries.
var (
	SubscriptionNotFound     = def(CodeSubscriptionNotFound, http.StatusNotFound, "The specified subscription could not be found.")
	SubscriptionExists       = def(CodeSubscriptionExists, http.StatusConflict, "A subscription for this application and API already exists.")
	SubscriptionForbidden    = def(CodeSubscriptionForbidden, http.StatusForbidden, "You do not have permission to access this subscription.")
	SubscriptionPlanNotFound = def(CodeSubscriptionPlanNotFound, http.StatusNotFound, "The specified subscription plan could not be found.")
	SubscriptionPlanExists   = def(CodeSubscriptionPlanExists, http.StatusConflict, "A subscription plan with this name already exists.")
)

// Custom policy entries.
var (
	PolicyVersionConflict     = def(CodePolicyVersionConflict, http.StatusConflict, "The policy version conflicts with an existing version.")
	PolicyInvalidState        = def(CodePolicyInvalidState, http.StatusConflict, "The policy is not in a valid state for this operation.")
	PolicyInUse               = def(CodePolicyInUse, http.StatusConflict, "The policy is in use and cannot be deleted.")
	CustomPolicyNotFound      = def(CodeCustomPolicyNotFound, http.StatusNotFound, "The specified policy could not be found.")
	CustomPolicyVersionNotFnd = def(CodeCustomPolicyVersionNotFound, http.StatusNotFound, "The specified policy version could not be found.")
)

// Secret entries.
var (
	SecretNotFound = def(CodeSecretNotFound, http.StatusNotFound, "The specified secret could not be found.")
	SecretExists   = def(CodeSecretExists, http.StatusConflict, "A secret with this name already exists.")
	SecretInUse    = def(CodeSecretInUse, http.StatusConflict, "The secret is in use and cannot be deleted.")
)

// Artifact entries — generic artifact-reference flows and the
// data-plane-origin guard. The guard produces client-appropriate messages at
// the call site, so these are passthrough templates.
var (
	ArtifactNotFound         = def(CodeArtifactNotFound, http.StatusNotFound, "The specified artifact could not be found.")
	ArtifactExists           = def(CodeArtifactExists, http.StatusConflict, "The artifact already exists.")
	ArtifactReadOnly         = def(CodeArtifactReadOnly, http.StatusForbidden, "%s")
	ArtifactRuntimeImmutable = def(CodeArtifactRuntimeImmutable, http.StatusForbidden, "%s")
	ArtifactDeployed         = def(CodeArtifactDeployed, http.StatusConflict, "%s")
)

// Application API key entries.
var (
	ApplicationAPIKeyNotFound  = def(CodeApplicationAPIKeyNotFound, http.StatusNotFound, "The specified API key could not be found.")
	ApplicationAPIKeyForbidden = def(CodeApplicationAPIKeyForbidden, http.StatusForbidden, "You do not have permission to access this API key.")
)

// WebSub / WebBroker API entries.
var (
	WebSubAPINotFound        = def(CodeWebSubAPINotFound, http.StatusNotFound, "The specified WebSub API could not be found.")
	WebSubAPIExists          = def(CodeWebSubAPIExists, http.StatusConflict, "A WebSub API with this ID already exists.")
	WebSubAPILimitReached    = def(CodeWebSubAPILimitReached, http.StatusConflict, "WebSub API limit reached for the organization.")
	WebBrokerAPINotFound     = def(CodeWebBrokerAPINotFound, http.StatusNotFound, "The specified WebBroker API could not be found.")
	WebBrokerAPIExists       = def(CodeWebBrokerAPIExists, http.StatusConflict, "A WebBroker API with this ID already exists.")
	WebBrokerAPILimitReached = def(CodeWebBrokerAPILimitReached, http.StatusConflict, "WebBroker API limit reached for the organization.")
)

// HMAC secret entries. The 32-character minimum is a fixed, publicly
// documented rule, so stating it in the client message reveals nothing the
// API contract does not already.
var (
	HmacSecretNotFound      = def(CodeHmacSecretNotFound, http.StatusNotFound, "The specified HMAC secret could not be found.")
	HmacSecretExists        = def(CodeHmacSecretExists, http.StatusConflict, "An HMAC secret with this name already exists.")
	HmacSecretInvalidValue  = def(CodeHmacSecretInvalidValue, http.StatusBadRequest, "The secret value must be at least 32 characters.")
	HmacSecretNotConfigured = def(CodeHmacSecretNotConfigured, http.StatusServiceUnavailable, "HMAC secret management is not configured on this server.")
)
