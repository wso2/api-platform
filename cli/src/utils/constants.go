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
	DefaultManifestFile         = "policy-manifest.yaml"
	DefaultManifestLockFile     = "policy-manifest-lock.yaml"
	DefaultGatewayVersion       = "0.2.0"
	DefaultImageRepository      = "ghcr.io/wso2/api-platform"
	DefaultGatewayBuilderRepo   = "ghcr.io/wso2/api-platform/gateway-builder"
	DefaultGatewayControllerImg = "" // Uses default from gateway-builder
	DefaultRouterImg            = "" // Uses default from gateway-builder

	// REST API Endpoints
	GatewayHealthPath       = "/health"
	GatewayAPIsPath         = "/apis"
	GatewayAPIByIDPath      = "/apis/%s"
	GatewayMCPProxiesPath   = "/mcp-proxies"
	GatewayMCPProxyByIDPath = "/mcp-proxies/%s"

	// BasicAuth Environment Variables
	EnvGatewayUsername = "WSO2AP_GW_USERNAME"
	EnvGatewayPassword = "WSO2AP_GW_PASSWORD"

	// Image Build Configuration
	GatewayVerifyChecksumOnBuild = false
)

// PolicyHub REST API
const (
	PolicyHubBaseURL     = "https://db720294-98fd-40f4-85a1-cc6a3b65bc9a-dev.e1-us-east-azure.choreoapis.dev/api-platform/policy-hub-api/policy-hub-public/v1.0"
	PolicyHubResolvePath = "/policies/resolve"
)
