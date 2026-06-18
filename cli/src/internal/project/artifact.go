/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package project

import (
	"github.com/wso2/api-platform/cli/utils"
)

// apiVersions stamped into generated manifests, keyed by the platform plane the
// artifact belongs to.
const (
	ManagementAPIVersion  = "management.api-platform.wso2.com/v1"
	AIWorkspaceAPIVersion = "ai-workspace.api-platform.wso2.com/v1alpha"
	GatewayAPIVersion     = "gateway.api-platform.wso2.com/v1"
)

// DefaultGatewayType is used when no gateway type is supplied for an artifact.
const DefaultGatewayType = "wso2/api-platform"

// SupportedArtifactTypes returns the artifact types the CLI can scaffold and
// build, in display order.
func SupportedArtifactTypes() []string {
	return []string{
		utils.TypeREST,
		utils.TypeLLMProxy,
		utils.TypeLLMProvider,
		utils.TypeLLMProviderTemplate,
		utils.TypeMCPProxy,
	}
}

// IsValidArtifactType reports whether artifactType is one the CLI supports.
func IsValidArtifactType(artifactType string) bool {
	for _, t := range SupportedArtifactTypes() {
		if artifactType == t {
			return true
		}
	}
	return false
}

// IsAIWorkspaceType reports whether the artifact type belongs to the
// ai-workspace plane (LLM proxies/providers, MCP proxies) rather than the
// management plane (REST APIs).
func IsAIWorkspaceType(artifactType string) bool {
	switch artifactType {
	case utils.TypeLLMProxy, utils.TypeLLMProvider, utils.TypeLLMProviderTemplate, utils.TypeMCPProxy:
		return true
	default:
		return false
	}
}

// ArtifactKind maps an artifact type to its manifest kind.
func ArtifactKind(artifactType string) string {
	switch artifactType {
	case utils.TypeREST:
		return "RestApi"
	case utils.TypeLLMProxy:
		return "LlmProxy"
	case utils.TypeLLMProvider:
		return "LlmProvider"
	case utils.TypeLLMProviderTemplate:
		return "LlmProviderTemplate"
	case utils.TypeMCPProxy:
		return "Mcp"
	default:
		return "RestApi"
	}
}

// MetadataAPIVersion returns the apiVersion for an artifact's metadata.yaml,
// which differs between the management and ai-workspace planes.
func MetadataAPIVersion(artifactType string) string {
	if IsAIWorkspaceType(artifactType) {
		return AIWorkspaceAPIVersion
	}
	return ManagementAPIVersion
}
