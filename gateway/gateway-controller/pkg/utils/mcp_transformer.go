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

import (
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/generated"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

type MCPTransformer struct {
}

// protocolVersionComparator compares two MCP protocol version strings in YYYY-MM-DD format
// Returns true if current is equal to or newer than base
func protocolVersionComparator(base, current string) bool {
	return strings.Compare(base, current) <= 0
}

// addMCPSpecificOperations adds MCP-specific HTTP operations based on the MCP specification version defined
func addMCPSpecificOperations(mcpConfig *api.MCPProxyConfiguration) []api.Operation {
	operations := []api.Operation{
		{
			Method:   api.GET,
			Path:     constants.MCP_RESOURCE_PATH,
			Policies: nil,
		},
		{
			Method:   api.POST,
			Path:     constants.MCP_RESOURCE_PATH,
			Policies: nil,
		},
		{
			Method:   api.DELETE,
			Path:     constants.MCP_RESOURCE_PATH,
			Policies: nil,
		},
	}
	var mcpSpecVersion string
	if mcpConfig.Spec.SpecVersion != nil && *mcpConfig.Spec.SpecVersion != "" {
		mcpSpecVersion = *mcpConfig.Spec.SpecVersion
	} else {
		mcpSpecVersion = LATEST_SUPPORTED_MCP_SPEC_VERSION
	}

	if protocolVersionComparator(constants.SPEC_VERSION_2025_JUNE, mcpSpecVersion) {
		operations = append(operations,
			api.Operation{
				Method:   api.GET,
				Path:     constants.MCP_PRM_RESOURCE_PATH,
				Policies: nil,
			},
		)
	}

	return operations
}

// Transform converts an MCP proxy configuration (input) to an API configuration (output)
func (t *MCPTransformer) Transform(input any, output *api.APIConfiguration) *api.APIConfiguration {
	mcpConfig, ok := input.(*api.MCPProxyConfiguration)
	if !ok || mcpConfig == nil {
		return output
	}
	output.Version = api.ApiPlatformWso2Comv1
	output.Kind = api.APIConfigurationKindHttprest
	// Build APIConfigData and set it into the APIConfiguration_Spec union
	apiData := api.APIConfigData{
		Name:       mcpConfig.Spec.Name,
		Version:    mcpConfig.Spec.Version,
		Context:    mcpConfig.Spec.Context,
		Upstreams:  mcpConfig.Spec.Upstreams,
		Operations: addMCPSpecificOperations(mcpConfig),
	}

	var specUnion api.APIConfiguration_Spec
	_ = specUnion.FromAPIConfigData(apiData)
	output.Spec = specUnion
	return output
}
