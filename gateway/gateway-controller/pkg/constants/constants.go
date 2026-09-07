/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package constants

import (
	"regexp"
	"time"
)

// SecretPlaceholderRe matches {{ secret "handle" }} placeholders embedded in YAML
// or JSON content. The quotes may be JSON-escaped (\") so both forms are matched.
// The handle is in capture group 1.
//
// This is the single canonical definition — use it everywhere secret handles must
// be extracted (on-demand sync, template pre-scan, etc.) to guarantee that
// extraction and rendering can never silently diverge.
var SecretPlaceholderRe = regexp.MustCompile(`\{\{\s*secret\s+\\?"([^"\\]+)\\?"\s*\}\}`)

const (
	PlatformGatewayId = "platform-gateway-id"
	// XDS/Envoy Constants
	TransportSocketPrefix   = "ts"
	LoadBalancerIDKey       = "lb_id"
	TransportSocketMatchKey = "envoy.transport_socket_match"

	// TLS Protocol Versions
	TLSVersion10 = "TLS1_0"
	TLSVersion11 = "TLS1_1"
	TLSVersion12 = "TLS1_2"
	TLSVersion13 = "TLS1_3"

	// ALPN Protocol Names
	ALPNProtocolHTTP2  = "h2"
	ALPNProtocolHTTP11 = "http/1.1"

	// TLS Cipher Configuration
	CipherSuiteSeparator = ","

	// Network Configuration
	HTTPDefaultPort  = uint32(80)
	HTTPSDefaultPort = uint32(443)

	// URL Schemes
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"

	// Localhost
	LocalhostIP = "127.0.0.1"

	// Transport Socket Configuration
	EnvoyTLSTransportSocket = "envoy.transport_sockets.tls"
	DefaultCertificateKey   = "default"
	DefaultMatchID          = "0"

	// Configuration Validation Constants
	MaxReasonableTimeoutMs       = uint32(3600000) // 1 hour in milliseconds
	MaxReasonablePolicyTimeoutMs = uint32(60000)   // 60 seconds in milliseconds

	// MaxReasonableBufferLimitBytes caps the downstream per-connection buffer limit in bytes,
	// preventing unreasonably large values that could lead to resource exhaustion or performance degradation.
	MaxReasonableBufferLimitBytes = uint32(104857600) // 100 MiB

	// MaxReasonableConnectionTimeoutMs caps connection-level timeouts (request, request-headers,etc.),
	// allowing higher values than MaxReasonableTimeoutMs to support long-lived idle connections.
	MaxReasonableConnectionTimeoutMs = uint32(86400000) // 24 hours in milliseconds

	// Cipher Suite Validation
	CipherInvalidChars1 = ";"
	CipherInvalidChars2 = "|"

	// TLS Version Ordering
	TLSVersionOrderTLS10 = 0
	TLSVersionOrderTLS11 = 1
	TLSVersionOrderTLS12 = 2
	TLSVersionOrderTLS13 = 3

	// External Processor (ext_proc) Filter
	ExtProcFilterName                = "api_platform.policy_engine.envoy.filters.http.ext_proc"
	ExtProcConfigType                = "type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor"
	ExtProcMetadataNamespace         = ExtProcFilterName
	ExtProcRouteCacheActionDefault   = "DEFAULT"
	ExtProcRouteCacheActionRetain    = "RETAIN"
	ExtProcRouteCacheActionClear     = "CLEAR"
	ExtProcHeaderModeDefault         = "DEFAULT"
	ExtProcHeaderModeSend            = "SEND"
	ExtProcHeaderModeSkip            = "SKIP"
	ExtProcRequestAttributeRouteName = "xds.route_name"

	// Policy Engine
	PolicyEngineClusterName       = "api-platform/policy-engine"
	DefaultPolicyEngineSocketPath = "/var/run/api-platform/policy-engine.sock"

	// GatewayHealthPathPrefix is reserved for the gateway's own readiness/liveness
	// direct-response routes (see GatewayReadyPath/GatewayHealthyPath). No API,
	// LLMProvider, or LLMProxy resource may register a path under this prefix —
	// path validation for those resource kinds must reject it.
	GatewayHealthPathPrefix = "/_gateway-health"
	GatewayReadyPath        = GatewayHealthPathPrefix + "/ready"
	GatewayHealthyPath      = GatewayHealthPathPrefix + "/healthy"

	// gRPC Access Log Service
	GRPCAccessLogClusterName = "apip_als_cluster"
	DefaultALSSocketPath     = "/var/run/api-platform/gateway-analytics.sock"
	DefaultALSLogName        = "envoy_access_log"

	// MCP related constants
	MCP_RESOURCE_PATH          = "/mcp"
	MCP_PRM_RESOURCE_PATH      = "/.well-known/oauth-protected-resource"
	SPEC_VERSION_2025_JUNE     = "2025-06-18"
	SPEC_VERSION_2025_NOVEMBER = "2025-11-25"

	// Router constants
	BASE_PATH = "/"
	WILD_CARD = "*"
	// VHostGatewayDefault is the sentinel value written by platform-api to indicate that the
	// gateway-controller should resolve and persist its current configured default vhost values.
	VHostGatewayDefault = "_gateway_default_"

	WEBSUBHUB_INTERNAL_CLUSTER_NAME = "WEBSUBHUB_INTERNAL_CLUSTER"

	// Target Upstream Header for dynamic cluster selection
	// This header is set by the policy engine when UpstreamName is used
	// Routes can be configured with cluster_header to read this header and select the target cluster
	TargetUpstreamHeader = "x-target-upstream"

	// InternalLoopbackHeader marks the LLM proxy's internal loopback forward to its provider.
	// The proxy stamps it via a set-headers policy; the analytics system policy reads it on the
	// provider hop so the duplicate provider analytics event can be dropped from Moesif.
	InternalLoopbackHeader = "x-wso2-internal-loopback"

	// UpstreamDefinitionClusterPrefix is the prefix used for clusters created from upstreamDefinitions
	// Cluster names follow the format: upstream_<definition_name>
	UpstreamDefinitionClusterPrefix = "upstream_"

	WEBSUB_PATH                    = "/hub"
	WEBSUB_HUB_INTERNAL_HTTP_PORT  = 8083
	WEBSUB_HUB_INTERNAL_HTTPS_PORT = 8446
	WEBSUB_HUB_DYNAMIC_HTTP_PORT   = 8082
	WEBSUB_HUB_DYNAMIC_HTTPS_PORT  = 8445

	API_KEY_AUTH_POLICY_NAME           = "api-key-auth"
	UPSTREAM_AUTH_APIKEY_POLICY_NAME   = "set-headers"
	UPSTREAM_AUTH_APIKEY_POLICY_PARAMS = "request:\n" +
		"  headers:\n" +
		"    - name: '%s'\n" +
		"      value: '%s'\n"
	UPSTREAM_AUTH_OAUTH2_POLICY_NAME = "oauth2-generator"
	PROXY_HOST__HEADER_POLICY_NAME   = "host-rewrite"
	PROXY_HOST__HEADER_POLICY_PARAMS = "host: '%s'\n"

	ACCESS_CONTROL_DENY_POLICY_NAME = "respond"
	// YAML for default 404 respond policy params
	ACCESS_CONTROL_DENY_POLICY_PARAMS = "statusCode: 404\n" +
		"body: \"{\\\"message\\\": \\\"Resource not found.\\\"}\"\n" +
		"headers:\n" +
		"  - name: Content-Type\n" +
		"    value: application/json\n"

	SET_HEADERS_POLICY_NAME   = "set-headers"
	SET_HEADERS_POLICY_PARAMS = "request:\n" +
		"  headers:\n" +
		"    - name: '%s'\n" +
		"      value: '%s'\n"

	// API Key constants
	APIKeyPrefix = "apip_"
	APIKeyLen    = 32 // Length of the random part of the API key in bytes

	// API Key length constants
	DefaultMinAPIKeyLength = 36
	DefaultMaxAPIKeyLength = 128

	// API Key name and display name length constants
	APIKeyNameMinLength  = 3
	APIKeyNameMaxLength  = 63
	DisplayNameMaxLength = 100

	// HashingAlgorithm constants
	HashingAlgorithmSHA256 = "sha256"

	// System policy constants
	ANALYTICS_SYSTEM_POLICY_NAME    = "wso2_apip_sys_analytics"
	ANALYTICS_SYSTEM_POLICY_VERSION = "v1"

	// A2A_SYSTEM_POLICY_NAME is the in-repo policy that answers the A2A requests
	// the gateway serves itself rather than proxying to the agent behind it
	// (gateway/system-policies/a2a). It serves a managed public Agent Card, and
	// answers the protected (extended) Agent Card operation — requiring an
	// authenticated request, then serving the configured card or forwarding to
	// the upstream. It is named for the protocol so each such concern joins it
	// instead of becoming another policy, another module, and another build-lock
	// line.
	//
	// Unlike the analytics policy it is not a defaultSystemPolicies entry: those
	// inject into every chain and have no route predicate, whereas this is
	// attached per route by the Agent transformer. The shared "wso2_apip_sys_"
	// prefix is what keeps it out of the control-plane gateway manifest
	// (pkg/controlplane/client.go), which is the only enforcement point for
	// "internal, not user-attachable" — a name without it would be published to
	// the control plane as an author-attachable policy.
	A2A_SYSTEM_POLICY_NAME = "wso2_apip_sys_a2a"

	// A2A_POLICY_PARAM_* are the parameter names the policy above reads. Each
	// gateway-answered A2A concern takes a nested block of its own rather than
	// top-level fields, so the two cannot be confused for one another: a chain
	// carries at most one instance per block, and an instance carrying the wrong
	// block would serve the wrong thing on the wrong route.
	//
	// The policy's module cannot import this one, so the names are spelled once
	// on each side (a2a.ParamAgentCard / ParamProtectedAgentCard / ParamContent /
	// ParamETag) and a test on each side asserts the literal.
	A2A_POLICY_PARAM_AGENT_CARD = "agentCard"
	A2A_POLICY_PARAM_CONTENT    = "content"
	A2A_POLICY_PARAM_ETAG       = "etag"

	// A2A_POLICY_PARAM_PROTECTED_AGENT_CARD is the block configuring the
	// protected (extended) Agent Card. It is attached to the canonical
	// GetExtendedAgentCard chain of an Agent with an explicit
	// agentCard.protected block, in either mode, and to nothing else.
	//
	// It sits at the tail of that chain, after everything the author attached, so
	// their own policies decide the request first — the order they configure is
	// theirs, and the gateway adds itself at the end rather than in the middle of
	// it. Whichever of those policies authenticated the caller, and in whichever
	// scope, the instance sees the result.
	//
	// The block is present in both modes and carries content only in managed
	// mode. An empty block means passthrough: require an authenticated request,
	// then forward the operation to the upstream unchanged.
	A2A_POLICY_PARAM_PROTECTED_AGENT_CARD = "protectedAgentCard"

	// ResilienceDurationPattern is the single source of truth for the format of resilience
	// timeout strings.
	//   - accepted:  "30s", "500ms", "1m", "2h", "1.5s", and "0s" (zero disables the timeout)
	//   - rejected:  compound durations ("1h30m"), negatives ("-30s"), and unitless values ("0", "30")
	ResilienceDurationPattern = `^\d+(\.\d+)?(ms|s|m|h)$`
)

// DP->CP artifact push timing. The bottom-up (DP->CP) push waits for the local deployment
// to be reflected in the in-memory store before pushing to the control plane.
const (
	// CPPushDeploymentTimeout is the maximum time to wait for a gateway-originated artifact
	// to finish deploying before abandoning the DP->CP push.
	CPPushDeploymentTimeout = 60 * time.Second
	// CPPushPollInterval is how often the in-memory store is re-checked while waiting.
	CPPushPollInterval = 500 * time.Millisecond
)

var WILDCARD_HTTP_METHODS = []string{
	"GET",
	"POST",
	"PUT",
	"PATCH",
	"DELETE",
	"OPTIONS",
}

// ResilienceDurationRegex is the compiled ResilienceDurationPattern, shared by the management-API
// validator and the xDS downstream parser so both enforce exactly what the
// CRD admission controller enforces.
var ResilienceDurationRegex = regexp.MustCompile(ResilienceDurationPattern)
