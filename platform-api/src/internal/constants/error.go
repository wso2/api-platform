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

package constants

import "errors"

var (
	ErrHandleExists           = errors.New("handle already exists")
	ErrHandleDoesNotExist     = errors.New("handle does not exist")
	ErrHandleEmpty            = errors.New("handle cannot be empty")
	ErrHandleTooShort         = errors.New("handle must be at least 3 characters")
	ErrHandleTooLong          = errors.New("handle must be at most 63 characters")
	ErrInvalidHandle          = errors.New("handle must be lowercase alphanumeric with hyphens only (no consecutive hyphens, cannot start or end with hyphen)")
	ErrHandleGenerationFailed = errors.New("failed to generate unique handle after maximum retries")
	ErrHandleSourceEmpty      = errors.New("source string cannot be empty for handle generation")
	ErrOrganizationExists     = errors.New("organization already exists with the given UUID")
	ErrOrganizationNotFound   = errors.New("organization not found")
	ErrMultipleOrganizations  = errors.New("multiple organizations found")
	ErrInvalidInput           = errors.New("invalid input parameters")
)

var (
	ErrProjectExists                         = errors.New("project already exists in organization")
	ErrProjectNotFound                       = errors.New("project not found")
	ErrInvalidProjectName                    = errors.New("invalid project name")
	ErrOrganizationMustHAveAtLeastOneProject = errors.New("organization must have at least one project")
	ErrProjectHasAssociatedAPIs              = errors.New("project has associated APIs")
	ErrorInvalidProjectUUID                  = errors.New("invalid project UUID")
)

var (
	ErrAPINotFound                 = errors.New("api not found")
	ErrAPIAlreadyExists            = errors.New("api already exists in project")
	ErrAPINameVersionAlreadyExists = errors.New("api with same name and version already exists")
	ErrInvalidAPIContext           = errors.New("invalid api context format")
	ErrInvalidAPIVersion           = errors.New("invalid api version format")
	ErrInvalidAPIName              = errors.New("invalid api name format")
	ErrInvalidLifecycleState       = errors.New("invalid lifecycle state")
	ErrInvalidAPIType              = errors.New("invalid api type")
	ErrInvalidTransport            = errors.New("invalid transport protocol")
	ErrInvalidAPIDeployment        = errors.New("invalid api deployment")
	ErrGatewayNotAssociated        = errors.New("api is not associated with gateway")
	ErrAPIContextVersionConflict   = errors.New("api with same context and version already deployed in gateway")
)

var (
	ErrGatewayNotFound          = errors.New("gateway not found")
	ErrGatewayAlreadyAssociated = errors.New("gateway already associated with API")
	ErrGatewayHasAssociatedAPIs = errors.New("cannot delete gateway: it has associated APIs. Please remove all API associations before deleting the gateway")
)

var (
	ErrDeploymentNotFound          = errors.New("deployment not found")
	ErrDeploymentNotActive         = errors.New("no active deployment found for this API on the gateway")
	ErrDeploymentIsDeployed        = errors.New("cannot delete an active deployment - undeploy it first")
	ErrDeploymentAlreadyActive     = errors.New("deployment is already active")
	ErrBaseDeploymentNotFound      = errors.New("base deployment not found")
	ErrInvalidDeploymentStatus     = errors.New("invalid deployment status")
	ErrDeploymentNameRequired      = errors.New("deployment name is required")
	ErrDeploymentBaseRequired      = errors.New("base is required")
	ErrDeploymentGatewayIDRequired = errors.New("gatewayId is required")
	ErrAPINoBackendServices        = errors.New("API must have at least one backend service attached before deployment")
	ErrDeploymentAlreadyDeployed   = errors.New("cannot restore to the currently deployed deployment")
	ErrGatewayIDMismatch           = errors.New("gateway ID mismatch: deployment is bound to a different gateway")
)

var (
	ErrApiPortalSync = errors.New("failed to synchronize with dev portal")
)

var (
	ErrDevPortalNotFound                = errors.New("devportal not found")
	ErrDevPortalAlreadyExist            = errors.New("devportal already exists in organization")
	ErrDevPortalNameRequired            = errors.New("devportal name is required")
	ErrDevPortalIdentifierRequired      = errors.New("devportal identifier is required")
	ErrDevPortalAPIUrlRequired          = errors.New("devportal API URL is required")
	ErrDevPortalHostnameRequired        = errors.New("devportal hostname is required")
	ErrDevPortalAPIKeyRequired          = errors.New("devportal API key is required")
	ErrDevPortalHeaderKeyNameRequired   = errors.New("header key name is required for header transmission mode")
	ErrDevPortalIdentifierExists        = errors.New("devportal identifier already exists in organization")
	ErrDevPortalHostnameExists          = errors.New("devportal hostname already exists in organization")
	ErrDevPortalAPIUrlExists            = errors.New("devportal API URL already exists in organization")
	ErrDevPortalDefaultAlreadyExists    = errors.New("default devportal already exists for organization")
	ErrDevPortalCannotDeleteDefault     = errors.New("cannot delete default devportal")
	ErrDevPortalCannotDeactivateDefault = errors.New("cannot deactivate default devportal")
	ErrDevPortalBackendUnreachable      = errors.New("devportal backend is unreachable")
	ErrDevPortalSyncFailed              = errors.New("failed to sync organization to devportal")
	ErrDevPortalAuthenticationFailed    = errors.New("devportal authentication failed")
	ErrDevPortalForbidden               = errors.New("devportal access forbidden")
	ErrDevPortalConnectivityFailed      = errors.New("devportal connectivity check failed")
	ErrDevPortalInvalidVisibility       = errors.New("devportal visibility must be 'public' or 'private'")
	ErrDevPortalOrganizationConflict    = errors.New("organization conflict in devportal: an organization with the same organization ID exists but differs from the one being synced")

	// API Publication errors
	ErrAPIPublicationNotFound = errors.New("api publication not found")
	ErrAPIAlreadyPublished    = errors.New("api is already published to devportal")

	// API Publication Compensation errors
	ErrAPIPublicationSaveFailed = errors.New("api publication database save failed after devportal success")
	ErrAPIPublicationSplitBrain = errors.New("critical split-brain: api published to devportal but local operations failed and compensation failed")

	// API Project Import errors
	ErrAPIProjectNotFound   = errors.New("api project not found")
	ErrMalformedAPIProject  = errors.New("malformed api project")
	ErrInvalidAPIProject    = errors.New("invalid api project")
	ErrConfigFileNotFound   = errors.New("API Project config file not found")
	ErrOpenAPIFileNotFound  = errors.New("OpenAPI definition file not found")
	ErrWSO2ArtifactNotFound = errors.New("WSO2 API artifact not found")
)

var (
	ErrLLMProviderTemplateExists   = errors.New("llm provider template already exists")
	ErrLLMProviderTemplateNotFound = errors.New("llm provider template not found")
	ErrLLMProviderExists           = errors.New("llm provider already exists")
	ErrLLMProviderNotFound         = errors.New("llm provider not found")
	ErrLLMProxyExists              = errors.New("llm proxy already exists")
	ErrLLMProxyNotFound            = errors.New("llm proxy not found")
)

var (
	// API Key errors
	ErrAPIKeyNotFound      = errors.New("api key not found")
	ErrAPIKeyAlreadyExists = errors.New("api key already exists")
	ErrInvalidAPIKey       = errors.New("invalid api key")
	ErrGatewayUnavailable  = errors.New("gateway unavailable")
	ErrAPIKeyEventDelivery = errors.New("failed to deliver api key event to gateway")
	ErrAPIKeyHashingFailed = errors.New("failed to hash api key")
)
