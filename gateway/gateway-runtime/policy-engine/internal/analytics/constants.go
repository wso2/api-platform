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

package analytics

const (
	// UpstreamSuccessResponseDetail is used to indicate a successful response from the upstream server.
	UpstreamSuccessResponseDetail = "via_upstream"
	// ExtAuthDeniedResponseDetail indicates that the external authorization request was denied.
	ExtAuthDeniedResponseDetail = "ext_authz_denied"
	// ExtAuthErrorResponseDetail indicates an error occurred during external authorization.
	ExtAuthErrorResponseDetail = "ext_authz_error"
	// RouteNotFoundResponseDetail indicates that no route was found for the request.
	RouteNotFoundResponseDetail = "route_not_found"
	// GatewayLabel represents the label used to identify the Envoy Gateway.
	GatewayLabel = "ENVOY"

	// TokenEndpointPath is the path for the token endpoint.
	TokenEndpointPath = "/testkey"
	// HealthEndpointPath is the path for the health check endpoint.
	HealthEndpointPath = "/health"
	// JwksEndpointPath is the path for the JWKS (JSON Web Key Set) endpoint.
	JwksEndpointPath = "/.wellknown/jwks"
	// DefaultForUnassigned is the default value used for unassigned properties.
	DefaultForUnassigned = "UNKNOWN"
	// DataProviderClassProperty specifies the property name for the custom data provider class.
	DataProviderClassProperty = "publisher.custom.data.provider.class"

	// APIThrottleOutErrorCode is the error code for API-level throttling.
	APIThrottleOutErrorCode = 900800
	// HardLimitExceededErrorCode is the error code for exceeding a hard limit.
	HardLimitExceededErrorCode = 900801
	// ResourceThrottleOutErrorCode is the error code for resource-level throttling.
	ResourceThrottleOutErrorCode = 900802
	// ApplicationThrottleOutErrorCode is the error code for application-level throttling.
	ApplicationThrottleOutErrorCode = 900803
	// SubscriptionThrottleOutErrorCode is the error code for subscription-level throttling.
	SubscriptionThrottleOutErrorCode = 900804
	// BlockedErrorCode is the error code for blocked requests.
	BlockedErrorCode = 900805
	// CustomPolicyThrottleOutErrorCode is the error code for custom policy throttling.
	CustomPolicyThrottleOutErrorCode = 900806

	// NhttpReceiverInputOutputErrorSending indicates an error while sending data via the NHTTP receiver.
	NhttpReceiverInputOutputErrorSending = 101000
	// NhttpReceiverInputOutputErrorReceiving indicates an error while receiving data via the NHTTP receiver.
	NhttpReceiverInputOutputErrorReceiving = 101001
	// NhttpSenderInputOutputErrorSending indicates an error while sending data via the NHTTP sender.
	NhttpSenderInputOutputErrorSending = 101500
	// NhttpConnectionFailed indicates that the NHTTP connection failed.
	NhttpConnectionFailed = 101503
	// NhttpConnectionTimeout indicates that the NHTTP connection timed out.
	NhttpConnectionTimeout = 101504
	// NhttpConnectionClosed indicates that the NHTTP connection was closed.
	NhttpConnectionClosed = 101505
	// NhttpProtocolViolation indicates a protocol violation in the NHTTP connection.
	NhttpProtocolViolation = 101506
	// NhttpConnectTimeout indicates a timeout occurred while attempting to connect via NHTTP.
	NhttpConnectTimeout = 101508

	// WebsocketHandshakeResourcePrefix is the prefix used for WebSocket handshake resources.
	WebsocketHandshakeResourcePrefix = "init-request:"
	// GatewayURL represents the original Gateway URL header key.
	GatewayURL = "x-original-gw-url"
	// XForwardProtoHeader represents the header for the forwarded protocol.
	XForwardProtoHeader = "x-forwarded-proto"
	// XForwardPortHeader represents the header for the forwarded port.
	XForwardPortHeader = "x-forwarded-port"
)

const (
	// Wso2MetadataPrefix is the prefix for WSO2 metadata.
	Wso2MetadataPrefix = "x-wso2-"
	// APIIDKey is the key for the API ID.
	APIIDKey = Wso2MetadataPrefix + "api-id"
	// APICreatorKey is the key for the API creator.
	APICreatorKey = Wso2MetadataPrefix + "api-creator"
	// APINameKey is the key for the API name.
	APINameKey = Wso2MetadataPrefix + "api-name"
	// APIVersionKey is the key for the API version.
	APIVersionKey = Wso2MetadataPrefix + "api-version"
	// APITypeKey is the key for the API type.
	APITypeKey = Wso2MetadataPrefix + "api-kind"
	// APIUserNameKey is the key for the API user name.
	APIUserNameKey = Wso2MetadataPrefix + "username"
	// APIContextKey is the key for the API context.
	APIContextKey = Wso2MetadataPrefix + "api-context"
	// IsMockAPI is the key indicating if the API is a mock API.
	IsMockAPI = Wso2MetadataPrefix + "is-mock-api"
	// APICreatorTenantDomainKey is the key for the API creator tenant domain.
	APICreatorTenantDomainKey = Wso2MetadataPrefix + "api-creator-tenant-domain"
	// APIOrganizationIDKey is the key for the API organization ID.
	APIOrganizationIDKey = Wso2MetadataPrefix + "api-organization-id"
	// ProjectIDKey is the key for the project ID.
	ProjectIDKey = Wso2MetadataPrefix + "project-id"

	// AppIDKey is the key for the application ID.
	AppIDKey = Wso2MetadataPrefix + "application-id"
	// AppUUIDKey is the key for the application UUID.
	AppUUIDKey = Wso2MetadataPrefix + "application-uuid"
	// AppKeyTypeKey is the key for the application key type.
	AppKeyTypeKey = Wso2MetadataPrefix + "application-key-type"
	// AppNameKey is the key for the application name.
	AppNameKey = Wso2MetadataPrefix + "application-name"
	// AppOwnerKey is the key for the application owner.
	AppOwnerKey = Wso2MetadataPrefix + "application-owner"

	// CorrelationIDKey is the key for the correlation ID.
	CorrelationIDKey = Wso2MetadataPrefix + "correlation-id"
	// RegionKey is the key for the region.
	RegionKey = Wso2MetadataPrefix + "region"

	// APIResourceTemplateKey is the key for the API resource template.
	APIResourceTemplateKey = Wso2MetadataPrefix + "api-resource-template"

	// DestinationKey is the key for the destination.
	DestinationKey = Wso2MetadataPrefix + "destination"
	// DefaultForUnknown is the default value used for unassigned properties.
	DefaultForUnknown = "UNKNOWN"

	// UserAgentKey is the key for the user agent.
	UserAgentKey = Wso2MetadataPrefix + "user-agent"
	// ClientIPKey is the key for the client IP.
	ClientIPKey = Wso2MetadataPrefix + "client-ip"

	// ErrorCodeKey is the key for the error code.
	ErrorCodeKey = "ErrorCode"
	// RatelimitWso2OrgPrefix is the prefix for WSO2 organization rate limit.
	RatelimitWso2OrgPrefix = "customorg"
	// APIEnvironmentKey is the key for the API environment.
	APIEnvironmentKey = Wso2MetadataPrefix + "api-environment"
	// OrganizationAndAirlPolicy is the key for the organization and rate limit policy.
	OrganizationAndAirlPolicy = "ratelimit:organization-and-rlpolicy"
	// Subscription is the key for the subscription.
	Subscription = "ratelimit:subscription"
	// ExtractTokenFrom is the key for extracting the token from.
	ExtractTokenFrom = "aitoken:extracttokenfrom"
	// PromptTokenID is the key for the prompt token ID.
	PromptTokenID = "aitoken:prompttokenid"
	// CompletionTokenID is the key for the completion token ID.
	CompletionTokenID = "aitoken:completiontokenid"
	// TotalTokenID is the key for the total token ID.
	TotalTokenID = "aitoken:totaltokenid"
	// PromptTokenCount is the key for the prompt token count.
	PromptTokenCount = "aitoken:prompttokencount"
	// CompletionTokenCount is the key for the completion token count.
	CompletionTokenCount = "aitoken:completiontokencount"
	// TotalTokenCount is the key for the total token count.
	TotalTokenCount = "aitoken:totaltokencount"
	// ModelID is the key for the model ID.
	ModelID = "aitoken:modelid"
	// Model is the key for the model.
	Model = "aitoken:model"
	// AiProviderName is the key for the AI provider name.
	AiProviderName = "ai:providername"
	// AiProviderAPIVersion is the key for the AI provider API version.
	AiProviderAPIVersion = "ai:providerversion"
	//anonymousValue is the value for anonymous
	anonymousValue = "anonymous"
	// Unknown is the default value used for unassigned properties.
	Unknown = "UNKNOWN"

	// BillingCustomerIDKey is the key for the billing customer ID.
	BillingCustomerIDKey = Wso2MetadataPrefix + "billing-customer-id"
	// BillingSubscriptionIDKey is the key for the billing subscription ID.
	BillingSubscriptionIDKey = Wso2MetadataPrefix + "billing-subscription-id"
	// SubscriptionStatusKey is the key for the subscription status.
	SubscriptionStatusKey = Wso2MetadataPrefix + "subscription-status"
	// SubscriptionPlanNameKey is the key for the subscription plan name.
	SubscriptionPlanNameKey = Wso2MetadataPrefix + "subscription-plan-name"

	// ResolvedOperationKey is the canonical protocol operation the request resolved
	// to. Stamped by the kernel from SharedContext.ResolvedOperation, so it is
	// present whether or not the analytics system policy is in the chain. Mirrors
	// kernel.ResolvedOperationKey; the two are separate constants because the
	// packages must not import each other.
	ResolvedOperationKey = Wso2MetadataPrefix + "resolved-operation"

	// TerminalReasonKey names why the engine ended the request, when it — rather
	// than the upstream — decided the outcome. Mirrors kernel.TerminalReasonKey,
	// and its values are the constants.TerminalReason* set. Absent on a
	// pass-through.
	TerminalReasonKey = Wso2MetadataPrefix + "terminal-reason"

	// A2ARequestPropertiesKey and A2AResponsePropertiesKey carry the JSON-encoded
	// A2A analytics properties the analytics system policy assembles for an Agent
	// request. Key names are mirrored as string literals in that policy's own Go
	// module (gateway/system-policies/analytics), which cannot share a constant
	// with this one; each side has a test asserting the spelling.
	A2ARequestPropertiesKey  = "a2a_request_properties"
	A2AResponsePropertiesKey = "a2a_response_properties"

	// A2ATransportAttributeKey and A2AProtocolVersionAttributeKey are the resolver's
	// own attribute names, as the kernel stamps them onto a request it refused
	// before any chain was bound.
	//
	// They are read only as a fallback: on a request that resolved, the analytics
	// system policy assembles the same two facts into A2ARequestPropertiesKey, and
	// that block wins. A refused request never reaches that policy — it lives in the
	// chain the request failed to bind — so without these a version rejection would
	// reach a dashboard identifying the Agent but not which binding or protocol
	// version it was aimed at, which is exactly what an operator needs to see when
	// a fleet of clients is on the wrong version.
	//
	// Mirrored string literals for the module-boundary reason above; a test pins
	// them against the resolver's constants.
	A2ATransportAttributeKey       = "a2a.transport"
	A2AProtocolVersionAttributeKey = "a2a.protocol.version"
)

// A2A outcome vocabulary. Bounded on purpose: these are the values a downstream
// success-rate or volume dashboard groups by, so every one of them has to be a
// member of a closed set rather than anything derived from a request.
const (
	// A2AOutcomeSuccess and A2AOutcomeFailure are the outcomes of an Agent invocation.
	// Derived from the A2A result rather than from the HTTP status: a JSON-RPC error
	// rides a 200, so status alone reports a failed invocation as a success.
	A2AOutcomeSuccess = "SUCCESS"
	A2AOutcomeFailure = "FAILURE"

	// A2AOutcomeUnknown is a third outcome, not an absent one: the request completed
	// with a success status but nothing that could say whether the *invocation*
	// succeeded was readable.
	//
	// It exists because the alternative is worse in both directions. Reporting SUCCESS
	// would state as fact something nobody determined — on the JSON-RPC transport,
	// where the status carries no outcome information at all, that is precisely the
	// false-success this whole derivation exists to prevent. Reporting FAILURE would
	// invent one. A downstream success rate should exclude these from both numerator
	// and denominator and surface the count: a rising UNKNOWN share means the gateway
	// is losing visibility, which is itself the signal.
	A2AOutcomeUnknown = "UNKNOWN"

	// Failure origins — which component is answerable for a failed invocation.
	// Without this a dashboard cannot tell an agent that is erroring from a
	// gateway that is rejecting, and both look like the agent's fault.
	//
	// Spelled in upper case, like the outcome vocabulary above and for the same
	// reason: both are closed sets a dashboard groups by, and a reader looking at
	// one event should not have to remember which of the two dimensions happens to
	// be lower case. Case is part of the published value — a consumer matching
	// `origin == "upstream"` will not match `UPSTREAM` — so this set and the
	// outcome set are the only spellings, and neither is normalized on read.
	//
	// A2AFailureOriginClient covers a request the gateway refused before it
	// reached the agent for a reason the caller controls (unparseable envelope,
	// unknown operation, oversized body).
	A2AFailureOriginClient = "CLIENT"
	// A2AFailureOriginPolicy is a policy short-circuit: authentication,
	// authorization, rate limiting, a guardrail.
	A2AFailureOriginPolicy = "POLICY"
	// A2AFailureOriginGateway is a fault inside the gateway itself, with no
	// upstream involved.
	A2AFailureOriginGateway = "GATEWAY"
	// A2AFailureOriginUpstream is the agent's own failure — a transport error it
	// returned, or a JSON-RPC error object in an otherwise successful response.
	A2AFailureOriginUpstream = "UPSTREAM"

	// a2aTransportHTTPJSON is the REST-shaped A2A binding, as common/agentproto spells
	// it into a route's resolver config and the resolver carries it forward. Compared
	// here only to decide whether a 2xx is itself an outcome (see
	// a2aSuccessStatusOutcome); this package reports the value through, it does not
	// interpret it otherwise. A literal rather than an import because the value
	// arrives as an opaque string out of Envoy dynamic metadata; a test pins it.
	a2aTransportHTTPJSON = "HTTP+JSON"

	// A2AOperationUnknown groups invocations whose operation could not be determined
	// — the request named one this protocol version does not define, or its envelope
	// did not parse. Grouped rather than reported verbatim: the value that failed to
	// resolve is caller-supplied, so echoing it into the operation dimension would
	// make that dimension unbounded.
	A2AOperationUnknown = "unknown"

	// Request types. Only A2ARequestTypeOperation counts as an Agent invocation;
	// the other two share the Agent's routes but are not calls to the agent, and
	// aggregating them together would inflate the invocation volume of every
	// deployed Agent by however often its clients fetch its card.
	A2ARequestTypeOperation = "operation"
	// A2ARequestTypeAgentCard is a fetch of the public Agent Card, which is a
	// discovery document served at a known path and not an A2A operation.
	A2ARequestTypeAgentCard = "agentCard"
	// A2ARequestTypePreflight is a CORS preflight, answered by the gateway.
	A2ARequestTypePreflight = "preflight"
)
