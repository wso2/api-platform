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

	// Cipher Suite Validation
	CipherInvalidChars1 = ";"
	CipherInvalidChars2 = "|"

	// TLS Version Ordering
	TLSVersionOrderTLS10 = 0
	TLSVersionOrderTLS11 = 1
	TLSVersionOrderTLS12 = 2
	TLSVersionOrderTLS13 = 3

	// External Processor (ext_proc) Filter
	ExtProcFilterName                    = "api_platform.policy_engine.envoy.filters.http.ext_proc"
	ExtProcConfigType                    = "type.googleapis.com/envoy.extensions.filters.http.ext_proc.v3.ExternalProcessor"
	ExtProcMetadataNamespace             = ExtProcFilterName
	ExtProcRouteCacheActionDefault       = "DEFAULT"
	ExtProcRouteCacheActionRetain        = "RETAIN"
	ExtProcRouteCacheActionClear         = "CLEAR"
	ExtProcHeaderModeDefault             = "DEFAULT"
	ExtProcHeaderModeSend                = "SEND"
	ExtProcHeaderModeSkip                = "SKIP"
	ExtProcRequestAttributeRouteName     = "xds.route_name"
	ExtProcRequestAttributeRouteMetadata = "xds.route_metadata"

	// Policy Engine
	PolicyEngineClusterName       = "api-platform/policy-engine"
	DefaultPolicyEngineSocketPath = "/var/run/api-platform/policy-engine.sock"

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

	// UpstreamDefinitionClusterPrefix is the prefix used for clusters created from upstreamDefinitions
	// Cluster names follow the format: upstream_<definition_name>
	UpstreamDefinitionClusterPrefix = "upstream_"

	WEBSUB_PATH                    = "/hub"
	WEBSUB_HUB_INTERNAL_HTTP_PORT  = 8083
	WEBSUB_HUB_INTERNAL_HTTPS_PORT = 8446
	WEBSUB_HUB_DYNAMIC_HTTP_PORT   = 8082
	WEBSUB_HUB_DYNAMIC_HTTPS_PORT  = 8445

	// LLM Transformer constants
	UPSTREAM_AUTH_APIKEY_POLICY_NAME   = "set-headers"
	UPSTREAM_AUTH_APIKEY_POLICY_PARAMS = "request:\n" +
		"  headers:\n" +
		"    - name: '%s'\n" +
		"      value: '%s'\n"
	PROXY_HOST__HEADER_POLICY_NAME   = "set-headers"
	PROXY_HOST__HEADER_POLICY_PARAMS = "request:\n" +
		"  headers:\n" +
		"    - name: Host\n" +
		"      value: '%s'\n"

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
)

var WILDCARD_HTTP_METHODS = []string{
	"GET",
	"POST",
	"PUT",
	"PATCH",
	"DELETE",
	"OPTIONS",
}
