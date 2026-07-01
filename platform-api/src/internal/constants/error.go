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
	ErrHandleImmutable 		  = errors.New("id is immutable and cannot be changed")
)

var (
	ErrProjectExists                         = errors.New("project already exists in organization")
	ErrProjectNotFound                       = errors.New("project not found")
	ErrInvalidProjectName                    = errors.New("invalid project name")
	ErrOrganizationMustHAveAtLeastOneProject = errors.New("organization must have at least one project")
	ErrProjectHasAssociatedAPIs              = errors.New("project has associated APIs")
	ErrProjectHasAssociatedMCPProxies        = errors.New("project has associated MCP proxies")
	ErrorInvalidProjectUUID                  = errors.New("invalid project UUID")
)

var (
	ErrApplicationExists          = errors.New("application already exists in project")
	ErrApplicationNotFound        = errors.New("application not found")
	ErrInvalidApplicationName     = errors.New("invalid application name")
	ErrInvalidApplicationType     = errors.New("invalid application type")
	ErrUnsupportedApplicationType = errors.New("unsupported application type")
	ErrInvalidApplicationID       = errors.New("invalid application ID")
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
	ErrInvalidDeployment           = errors.New("invalid api deployment")
	ErrGatewayNotAssociated        = errors.New("api is not associated with gateway")
	ErrAPIContextVersionConflict   = errors.New("api with same context and version already deployed in gateway")
	ErrUpstreamRequired            = errors.New("upstream configuration is required")
)

var (
	ErrGatewayNotFound                  = errors.New("gateway not found")
	ErrGatewayAlreadyAssociated         = errors.New("gateway already associated with API")
	ErrGatewayHasAssociatedAPIs         = errors.New("cannot delete gateway: it has associated APIs. Please remove all API associations before deleting the gateway")
	ErrGatewayHasDeployments            = errors.New("cannot delete gateway: it has active API deployments. Please undeploy all APIs before deleting the gateway")
	ErrGatewayVersionMismatch           = errors.New("gateway version mismatch")
	ErrGatewayFunctionalityTypeMismatch = errors.New("gateway functionality type mismatch")
)

var (
	ErrCustomPolicyNotFound        = errors.New("custom policy not found")
	ErrCustomPolicyInUse           = errors.New("custom policy is in use by one or more APIs")
	ErrCustomPolicyVersionMismatch = errors.New("custom policy version does not match")
)

var (
	ErrDeploymentNotFound            = errors.New("deployment not found")
	ErrDeploymentNotActive           = errors.New("no active deployment found for this API on the gateway")
	ErrDeploymentIsDeployed          = errors.New("cannot delete an active deployment - undeploy it first")
	ErrDeploymentAlreadyActive       = errors.New("deployment is already active")
	ErrBaseDeploymentNotFound        = errors.New("base deployment not found")
	ErrInvalidDeploymentStatus       = errors.New("invalid deployment status")
	ErrDeploymentNameRequired        = errors.New("deployment name is required")
	ErrDeploymentBaseRequired        = errors.New("base is required")
	ErrDeploymentGatewayIDRequired   = errors.New("gatewayId is required")
	ErrAPINoBackendServices          = errors.New("API must have at least one backend service attached before deployment")
	ErrDeploymentAlreadyDeployed     = errors.New("cannot restore to the currently deployed deployment")
	ErrInvalidDeploymentRestoreState = errors.New("deployment cannot be restored: only ARCHIVED or UNDEPLOYED deployments are eligible")
	ErrGatewayIDMismatch             = errors.New("gateway ID mismatch: deployment is bound to a different gateway")
	ErrAssociationGatewayDeployed    = errors.New("cannot remove gateway association: the provider is actively deployed on the gateway - undeploy it first")
)

var (
	ErrArtifactNotFound    = errors.New("artifact not found")
	ErrArtifactExists      = errors.New("artifact already exists")
	ErrArtifactInvalidKind = errors.New("invalid artifact kind")
	// ErrArtifactReadOnly is returned when a mutating control-plane operation is
	// attempted on a data-plane-originated (origin=DP) artifact. Such artifacts are
	// read-only in the control plane; only documentation/OpenAPI updates are allowed.
	ErrArtifactReadOnly = errors.New("artifact is read-only: it originated from a data-plane gateway")
	// ErrArtifactDeployed is returned when a DP-originated artifact is deleted from the
	// control plane while still deployed on one or more gateways. It can only be deleted
	// once it is undeployed on all gateways it was deployed to.
	ErrArtifactDeployed = errors.New("artifact is still deployed on a gateway and cannot be deleted")
	// ErrArtifactRuntimeImmutable is returned when an update to a data-plane-originated
	// (origin=DP) artifact would change its gateway runtime artifact. The gateway owns
	// the runtime artifact, so a change to it cannot be applied from the control plane;
	// edits that leave the runtime artifact unchanged (description, lifecycle status,
	// display metadata, docs) are allowed. It is a distinct error from ErrArtifactReadOnly
	// (which blocks all edits) and is mapped to 403 separately by the guard-response handler.
	ErrArtifactRuntimeImmutable = errors.New(
		"the update changes the gateway runtime configuration, which is owned by the data-plane gateway and cannot be modified from the control plane")
)

var (
	// API Project Import errors
	ErrAPIProjectNotFound   = errors.New("api project not found")
	ErrMalformedAPIProject  = errors.New("malformed api project")
	ErrInvalidAPIProject    = errors.New("invalid api project")
	ErrConfigFileNotFound   = errors.New("API Project config file not found")
	ErrOpenAPIFileNotFound  = errors.New("OpenAPI definition file not found")
	ErrWSO2ArtifactNotFound = errors.New("WSO2 API artifact not found")
)

var (
	ErrLLMProviderTemplateExists            = errors.New("llm provider template already exists")
	ErrLLMProviderTemplateNotFound          = errors.New("llm provider template not found")
	ErrLLMProviderTemplateVersionExists     = errors.New("llm provider template version already exists")
	ErrLLMProviderTemplateInUse             = errors.New("llm provider template is in use by one or more providers")
	ErrLLMProviderTemplateReadOnly          = errors.New("built-in llm provider template is read-only")
	ErrLLMProviderTemplateManagedByReserved = errors.New("'wso2' is reserved and cannot be used as managedBy on custom templates")
	ErrLLMProviderExists                    = errors.New("llm provider already exists")
	ErrLLMProviderNotFound                  = errors.New("llm provider not found")
	ErrLLMProviderLimitReached              = errors.New("llm provider limit reached for organization")
	ErrLLMProxyExists                       = errors.New("llm proxy already exists")
	ErrLLMProxyNotFound                     = errors.New("llm proxy not found")
	ErrLLMProxyLimitReached                 = errors.New("llm proxy limit reached for organization")
)

var (
	ErrMCPProxyExists       = errors.New("mcp proxy already exists")
	ErrMCPProxyNotFound     = errors.New("mcp proxy not found")
	ErrMCPProxyLimitReached = errors.New("mcp proxy limit reached for organization")
)

var (
	ErrWebSubAPIExists                = errors.New("websub api already exists")
	ErrWebSubAPINotFound              = errors.New("websub api not found")
	ErrWebSubAPILimitReached          = errors.New("websub api limit reached for organization")
	ErrProjectHasAssociatedWebSubAPIs = errors.New("project has associated WebSub APIs")
)

var (
	ErrHmacSecretNotFound             = errors.New("hmac secret not found")
	ErrHmacSecretAlreadyExists        = errors.New("hmac secret with this name already exists")
	ErrHmacSecretEncryptionKeyMissing = errors.New("hmac secret encryption key is not configured")
	ErrHmacSecretInvalidValue         = errors.New("secret value must be at least 32 characters")
)

var (
	ErrWebBrokerAPIExists                = errors.New("webbroker api already exists")
	ErrWebBrokerAPINotFound              = errors.New("webbroker api not found")
	ErrWebBrokerAPILimitReached          = errors.New("webbroker api limit reached for organization")
	ErrProjectHasAssociatedWebBrokerAPIs = errors.New("project has associated WebBroker APIs")
	ErrDevPortalNotFound                 = errors.New("dev portal not found")
)

var (
	// API Key errors
	ErrAPIKeyNotFound      = errors.New("api key not found")
	ErrAPIKeyAlreadyExists = errors.New("api key already exists")
	ErrInvalidAPIKey       = errors.New("invalid api key")
	ErrAPIKeyForbidden     = errors.New("forbidden: only the key creator can perform this action")
	ErrGatewayUnavailable  = errors.New("gateway unavailable")
	ErrAPIKeyEventDelivery = errors.New("failed to deliver api key event to gateway")
	ErrAPIKeyHashingFailed = errors.New("failed to hash api key")
)

var (
	ErrInvalidURL            = errors.New("invalid URL")
	ErrURLUnreachable        = errors.New("URL is unreachable")
	ErrMCPServerUnauthorized = errors.New("MCP server returned 401 Unauthorized")
)

var (
	// Subscription errors (application-level subscriptions for REST APIs)
	ErrSubscriptionNotFound           = errors.New("subscription not found")
	ErrSubscriptionAlreadyExists      = errors.New("application is already subscribed to this API")
	ErrSubscriptionSubscriberMismatch = errors.New("subscriber does not match this subscription")
)

var (
	// Subscription plan errors
	ErrSubscriptionPlanNotFound           = errors.New("subscription plan not found")
	ErrSubscriptionPlanNotFoundOrInactive = errors.New("subscription plan not found or not active")
	ErrSubscriptionPlanAlreadyExists      = errors.New("subscription plan with this name already exists for the organization")
	ErrInvalidThrottleLimitUnit           = errors.New("invalid throttle limit unit: must be one of SECOND, MINUTE, HOUR, DAY, MONTH, YEAR")
)

var (
	// Gateway Internal API errors
	ErrMissingAPIKey   = errors.New("API key is required")
	ErrInvalidAPIToken = errors.New("invalid API token")
)

var (
	ErrSecretAlreadyExists = errors.New("secret already exists for this organization and handle")
	ErrSecretNotFound      = errors.New("secret not found")
	ErrSecretInUse         = errors.New("secret is referenced by one or more resources")
	ErrSecretRefMissing    = errors.New("one or more referenced secrets do not exist")
	ErrInvalidSecretType   = errors.New("invalid secret type: must be GENERIC or CERTIFICATE")
)
