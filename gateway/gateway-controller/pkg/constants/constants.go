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

	// Transport Socket Configuration
	EnvoyTLSTransportSocket = "envoy.transport_sockets.tls"
	DefaultCertificateKey   = "default"
	DefaultMatchID          = "0"

	// Configuration Validation Constants
	MaxReasonableTimeoutSeconds  = uint32(3600)  // 1 hour in seconds
	MaxReasonablePolicyTimeoutMs = uint32(60000) // 60 seconds in milliseconds

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
	ExtProcRouteCacheActionDefault       = "DEFAULT"
	ExtProcRouteCacheActionRetain        = "RETAIN"
	ExtProcRouteCacheActionClear         = "CLEAR"
	ExtProcHeaderModeDefault             = "DEFAULT"
	ExtProcHeaderModeSend                = "SEND"
	ExtProcHeaderModeSkip                = "SKIP"
	ExtProcRequestAttributeRouteName     = "xds.route_name"
	ExtProcRequestAttributeRouteMetadata = "xds.route_metadata"

	// Policy Engine
	PolicyEngineClusterName = "api-platform/policy-engine"

	// MCP related constants
	MCP_RESOURCE_PATH          = "/mcp"
	MCP_PRM_RESOURCE_PATH      = "/.well-known/oauth-protected-resource"
	SPEC_VERSION_2025_JUNE     = "2025-06-18"
	SPEC_VERSION_2025_NOVEMBER = "2025-11-25"

	// Router constants
	BASE_PATH = "/"
	WILD_CARD = "*"

	// LLM Transformer constants
	UPSTREAM_AUTH_APIKEY_POLICY_NAME                = "ModifyHeaders"
	UPSTREAM_AUTH_APIKEY_POLICY_VERSION             = "v1.0.0"
	UPSTREAM_AUTH_APIKEY_POLICY_REQUEST_HEADERS_KEY = "requestHeaders"
	UPSTREAM_AUTH_APIKEY_POLICY_HEADER_ACTION_KEY   = "action"
	UPSTREAM_AUTH_APIKEY_POLICY_HEADER_ACTION       = "SET"
	UPSTREAM_AUTH_APIKEY_POLICY_HEADER_NAME         = "name"
	UPSTREAM_AUTH_APIKEY_POLICY_HEADER_VALUE        = "value"
	UPSTREAM_AUTH_APIKEY_POLICY_PARAMS              = "requestHeaders:\n" +
		"  - action: SET\n" +
		"    name: %s\n" +
		"    value: %s\n"

	ACCESS_CONTROL_DENY_POLICY_NAME    = "Respond"
	ACCESS_CONTROL_DENY_POLICY_VERSION = "v1.0.0"
	// YAML for default 404 Respond policy params
	ACCESS_CONTROL_DENY_POLICY_PARAMS = "statusCode: 404\n" +
		"body: \"{\\\"message\\\": \\\"Resource not found.\\\"}\"\n" +
		"headers:\n" +
		"  - name: Content-Type\n" +
		"    value: application/json\n"
)
