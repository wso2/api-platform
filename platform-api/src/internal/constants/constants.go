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

// Artifact Type Constants
const (
	ArtifactTypeAPI        = "API"
	ArtifactTypeMCP        = "MCP"
	ArtifactTypeAPIProduct = "API_PRODUCT"
)

// ValidArtifactTypes Valid artifact types deployed to gateways
var ValidArtifactTypes = map[string]bool{
	ArtifactTypeAPI:        true,
	ArtifactTypeMCP:        true,
	ArtifactTypeAPIProduct: true,
}

// Constants for association types
const (
	AssociationTypeGateway = "gateway"
)

// API Key allowed targets constants
const APIKeyAllowedTargetsAll = "ALL"

// AdminRole is the role name that grants administrative privileges
const AdminRole = "admin"

// Deployment limit constants
const (
	// DeploymentLimitBuffer is the buffer added to MaxPerAPIGateway for hard limit enforcement
	DeploymentLimitBuffer = 5

	// MaxLLMProvidersPerOrganization is the maximum number of LLM providers allowed per organization.
	MaxLLMProvidersPerOrganization = 5
	// MaxLLMProxiesPerOrganization is the maximum number of LLM proxies allowed per organization.
	MaxLLMProxiesPerOrganization = 5
	// MaxMCPProxiesPerOrganization is the maximum number of MCP proxies allowed per organization.
	MaxMCPProxiesPerOrganization = 5
	// MaxWebSubAPIsPerOrganization is the maximum number of WebSub APIs allowed per organization.
	MaxWebSubAPIsPerOrganization = 5
	// MaxWebBrokerAPIsPerOrganization is the maximum number of WebBroker APIs allowed per organization.
	MaxWebBrokerAPIsPerOrganization = 5
)

// Gateway artifact apiVersion (the `apiVersion:` field on deployment artifacts).
// GatewayApiVersionV1Alpha1 is the legacy value for gateways < 1.2.0 — use it only
// in down-convert paths (deploymenttransform) that must produce artifacts
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
	PolicyManagedByCustomer = "customer"
	PolicyManagedByWSO2     = "wso2"
)

// ValidPolicyManagedBy holds accepted values for the managed_by field on gateway custom policies
var ValidPolicyManagedBy = map[string]bool{
	PolicyManagedByCustomer: true,
	PolicyManagedByWSO2:     true,
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

// ValidArtifactKinds holds accepted values for artifacts.type
var ValidArtifactKinds = map[string]bool{
	RestApi:      true,
	WebSubApi:    true,
	WebBrokerApi: true,
	LLMProvider:  true,
	LLMProxy:     true,
	MCPProxy:     true,
}

// Throttle limit unit constants
const (
	ThrottleLimitUnitSecond = "SECOND"
	ThrottleLimitUnitMinute = "MINUTE"
	ThrottleLimitUnitHour   = "HOUR"
	ThrottleLimitUnitDay    = "DAY"
	ThrottleLimitUnitMonth  = "MONTH"
	ThrottleLimitUnitYear   = "YEAR"
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
	ThrottleLimitUnitSecond: true,
	ThrottleLimitUnitMinute: true,
	ThrottleLimitUnitHour:   true,
	ThrottleLimitUnitDay:    true,
	ThrottleLimitUnitMonth:  true,
	ThrottleLimitUnitYear:   true,
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
