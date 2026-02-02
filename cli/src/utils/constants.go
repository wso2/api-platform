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

package utils

const CliName = "ap"

// WSO2AP Configuration
const (
	ConfigPath        = ".wso2ap/config.yaml"
	CachePath         = ".wso2ap/cache"
	PoliciesCachePath = ".wso2ap/cache/policies"
	TempPath          = ".wso2ap/.tmp"
)

// Gateway
const (
	// Image Build Defaults
	DefaultManifestFile      = "build.yaml"
	DefaultImageRepository   = "ghcr.io/wso2/api-platform"
	DefaultGatewayBuilder    = "ghcr.io/wso2/api-platform/gateway-builder:%s"    // %s = version
	DefaultGatewayController = "ghcr.io/wso2/api-platform/gateway-controller:%s" // %s = version
	DefaultGatewayRouter     = "ghcr.io/wso2/api-platform/gateway-router:%s"     // %s = version

	// REST API Endpoints
	GatewayHealthPath       = "/health"
	GatewayAPIsPath         = "/apis"
	GatewayAPIByIDPath      = "/apis/%s"
	GatewayMCPProxiesPath   = "/mcp-proxies"
	GatewayMCPProxyByIDPath = "/mcp-proxies/%s"

	// Auth Types
	AuthTypeNone   = "none"
	AuthTypeBasic  = "basic"
	AuthTypeBearer = "bearer"

	// Auth Environment Variables
	EnvGatewayUsername = "WSO2AP_GW_USERNAME" // For Basic Auth
	EnvGatewayPassword = "WSO2AP_GW_PASSWORD" // For Basic Auth
	EnvGatewayToken    = "WSO2AP_GW_TOKEN"    // For Bearer Auth

	// Image Build Configuration
	GatewayVerifyChecksumOnBuild = true

	// Allowed Policy Zip Sizes for Safety (in bytes)
	MaxZipFiles            = 1000              // Maximum number of files allowed in the zip (non-directory entries).
	MaxUncompressedPerFile = 20 * 1024 * 1024  // Maximum uncompressed size allowed per file (20 MB).
	MaxTotalUncompressed   = 100 * 1024 * 1024 // Maximum total uncompressed size allowed for the archive (100 MB).
)

// PolicyHub REST API defaults and paths
const (
	PolicyHubResolvePath    = "/policies/resolve"                                                                                                                   // Resolve path appended to PolicyHub base URL
	PolicyHubEnvVar         = "WSO2AP_POLICYHUB_BASE_URL"                                                                                                           // Environment variable name to override the PolicyHub base URL
	PolicyHubBaseURLDefault = "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0" // Default PolicyHub base URL (can be overridden via env)
)
