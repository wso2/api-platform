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

import "regexp"

// SecretPlaceholderRe matches {{ secret "handle" }} (and the escaped-quote variant
// {{ secret \"handle\" }}) in artifact config blobs.  A single definition here ensures
// ref-extraction (repository) and ref-validation (service) always match the same set.
var SecretPlaceholderRe = regexp.MustCompile(`\{\{\s*secret\s+\\?"([^"\\]+)\\?"\s*\}\}`)

// ValidLifecycleStates Valid lifecycle states
var ValidLifecycleStates = map[string]bool{
	"STAGED":     true,
	"CREATED":    true,
	"PUBLISHED":  true,
	"DEPRECATED": true,
	"RETIRED":    true,
	"BLOCKED":    true,
}

// ValidAPITypes Valid API types
var ValidAPITypes = map[string]bool{
	"HTTP":       true,
	"WS":         true,
	"SOAPTOREST": true,
	"SOAP":       true,
	"GRAPHQL":    true,
	"WEBSUB":     true,
	"SSE":        true,
	"WEBHOOK":    true,
	"ASYNC":      true,
}

// ValidTransports Valid transport protocols
var ValidTransports = map[string]bool{
	"http":  true,
	"https": true,
	"ws":    true,
	"wss":   true,
}

// Gateway Functionality Type Constants
const (
	GatewayFunctionalityTypeRegular = "regular"
	GatewayFunctionalityTypeAI      = "ai"
	GatewayFunctionalityTypeEvent   = "event"
)

// ValidGatewayFunctionalityType Valid gateway functionality types
var ValidGatewayFunctionalityType = map[string]bool{
	GatewayFunctionalityTypeRegular: true,
	GatewayFunctionalityTypeAI:      true,
	GatewayFunctionalityTypeEvent:   true,
}

// DefaultGatewayFunctionalityType Default gateway functionality type for new gateways
const DefaultGatewayFunctionalityType = GatewayFunctionalityTypeRegular

// Kinds of artifacts
const (
	RestApi             = "RestApi"
	WebSubApi           = "WebSubApi"
	WebBrokerApi        = "WebBrokerApi"
	LLMProvider         = "LlmProvider"
	LLMProviderTemplate = "LlmProviderTemplate"
	LLMProxy            = "LlmProxy"
	MCPProxy            = "Mcp"
)

// Artifact origin values. Origin distinguishes control-plane created artifacts
// (control_plane) from artifacts pushed up by a data-plane gateway (gateway_api).
// gateway_api artifacts are read-only in the control plane. The values match the
// gateway's origin naming (see gateway-controller models.Origin).
const (
	OriginCP = "control_plane"
	OriginDP = "gateway_api"
)

// API Type Constants
const (
	APITypeHTTP       = "HTTP"
	APITypeWS         = "WS"
	APITypeSOAPToREST = "SOAPTOREST"
	APITypeSOAP       = "SOAP"
	APITypeGraphQL    = "GRAPHQL"
	APITypeWebSub     = "WEBSUB"
	APITypeSSE        = "SSE"
	APITypeWebhook    = "WEBHOOK"
	APITypeAsync      = "ASYNC"
)

// API SubType Constants
const (
	APISubTypeHTTP      = "REST"
	APISubTypeGraphQL   = "GQL"
	APISubTypeAsync     = "ASYNC"
	APISubTypeWebSocket = "WEBSOCKET"
	APISubTypeSOAP      = "SOAP"
)

// Constants for association types
const (
	AssociationTypeGateway = "gateway"
)

// API Key allowed targets constants
const APIKeyAllowedTargetsAll = "ALL"

// AdminRole is the role name that grants administrative privileges
const AdminRole = "admin"

// DeletedUser is returned for audit-identity fields (createdBy/updatedBy/
// revokedBy/performedBy) and external/data-plane events when the stored
// internal UUID has no entry in user_idp_references — an anonymous write, or
// a user whose mapping was removed.
const DeletedUser = "deleted_user"

// Deployment limit constants
const (
	// DeploymentLimitBuffer is the buffer added to MaxPerAPIGateway for hard limit enforcement
	DeploymentLimitBuffer = 100
)

// Per-organization artifact creation limits are no longer hardcoded here. They are
// configured via config.ArtifactLimits (config file keys artifact_limits.max_* or
// env vars ARTIFACT_LIMITS_MAX_*) and default to unlimited. Enforcement uses
// config.LimitReached, which treats a limit <= 0 as "no limit".

// Gateway artifact apiVersion (the `apiVersion:` field on deployment artifacts).
// GatewayApiVersionV1Alpha1 is the legacy value for gateways < 1.2.0 — use it only
// in down-convert paths (gatewaytranslator) that must produce artifacts
// consumable by old gateways. New code should use GatewayApiVersion.
const (
	GatewayApiVersionV1Alpha1 = "gateway.api-platform.wso2.com/v1alpha1"
	GatewayApiVersion         = "gateway.api-platform.wso2.com/v1"
)

// Platform-api resource URL version. APIBasePath is the single source of truth for
// the prefix every handler route group is mounted under. NOTE: this is a DIFFERENT
// axis from GatewayApiVersion* (the gateway artifact apiVersion) — the two are
// governed independently and currently hold different values ("v0.9" vs "v1").
const (
	APIVersion  = "v0.9"
	APIBasePath = "/api/" + APIVersion
)

// Custom Policy ManagedBy constants
const (
	PolicyManagedByOrganization = "organization"
	PolicyManagedByWSO2         = "wso2"
	PolicyManagedByLegacyCustomer = "customer"
)

const TemplateManagedByOrganization = "organization"

// ReservedTemplateGroupIDPrefix is the group_id namespace reserved for WSO2-shipped
// built-in templates. Custom templates created via the REST API must not use it; a
// generated group_id that falls in this namespace is rewritten with the "x" prefix
// (e.g. "wso2-openai" -> "xwso2-openai").
const ReservedTemplateGroupIDPrefix = "wso2-"

// ValidPolicyManagedBy holds accepted values for the managed_by field on gateway custom policies
var ValidPolicyManagedBy = map[string]bool{
	PolicyManagedByOrganization: true,
	PolicyManagedByWSO2:         true,
}

// API key status constants
const (
	APIKeyStatusActive  = "active"
	APIKeyStatusRevoked = "revoked"
)

// ValidAPIKeyStatuses holds accepted values for api_keys.status
var ValidAPIKeyStatuses = map[string]bool{
	APIKeyStatusActive:  true,
	APIKeyStatusRevoked: true,
}

// Gateway token status constants
const (
	GatewayTokenStatusActive  = "active"
	GatewayTokenStatusRevoked = "revoked"
)

// ValidGatewayTokenStatuses holds accepted values for gateway_tokens.status
var ValidGatewayTokenStatuses = map[string]bool{
	GatewayTokenStatusActive:  true,
	GatewayTokenStatusRevoked: true,
}

// ValidArtifactKinds holds accepted values for artifacts.type for the core (non-plugin)
// artifact kinds. Plugin-owned kinds (e.g. WebSubApi, WebBrokerApi) are registered
// into the ArtifactTableRegistry during plugin Init.
var ValidArtifactKinds = map[string]bool{
	RestApi:     true,
	LLMProvider: true,
	LLMProxy:    true,
	MCPProxy:    true,
}

// Throttle limit unit constants
const (
	ThrottleLimitUnitMinute = "MINUTE"
	ThrottleLimitUnitHour   = "HOUR"
	ThrottleLimitUnitDay    = "DAY"
	ThrottleLimitUnitMonth  = "MONTH"
)

// Subscription plan limit type constants (subscription_plan_limits.limit_type).
// NOTE: only LimitTypeRequestCount is currently produced/consumed; BANDWIDTH and
// token-based types exist in the schema but are not yet wired through the
// platform-api, gateway events or gateway-controller.
const (
	LimitTypeRequestCount = "REQUEST_COUNT"
	LimitTypeBandwidth    = "BANDWIDTH"
	LimitTypeTotalToken   = "TOTAL_TOKEN_COUNT"
)

// ValidThrottleLimitUnits holds accepted values for subscription_plan_limits.time_unit
var ValidThrottleLimitUnits = map[string]bool{
	ThrottleLimitUnitMinute: true,
	ThrottleLimitUnitHour:   true,
	ThrottleLimitUnitDay:    true,
	ThrottleLimitUnitMonth:  true,
}

// Metadata key constants for deployment metadata
const (
	// MetadataKeyEndpointUrl is the metadata key for the per-deployment endpoint URL override.
	MetadataKeyEndpointUrl = "endpointUrl"
	// MetadataKeyVhostMain is the metadata key for the per-deployment main vhost value.
	MetadataKeyVhostMain = "vhostMain"
	// MetadataKeyVhostSandbox is the metadata key for the per-deployment sandbox vhost value.
	MetadataKeyVhostSandbox = "vhostSandbox"
	// VhostGatewayDefault is the sentinel value that instructs the gateway-controller to resolve
	// and persist the current gateway default vhosts, ensuring deployments are immune to future
	// gateway config changes.
	VhostGatewayDefault = "_gateway_default_"
)
