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
	"fmt"
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
			Method:   api.OperationMethod(api.GET),
			Path:     constants.MCP_RESOURCE_PATH,
			Policies: nil,
		},
		{
			Method:   api.OperationMethod(api.POST),
			Path:     constants.MCP_RESOURCE_PATH,
			Policies: nil,
		},
		{
			Method:   api.OperationMethod(api.DELETE),
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
				Method:   api.OperationMethod(api.GET),
				Path:     constants.MCP_PRM_RESOURCE_PATH,
				Policies: nil,
			},
		)
	}

	return operations
}

// Transform converts an MCP proxy configuration (input) to an API configuration (output)
func (t *MCPTransformer) Transform(input any, output *api.APIConfiguration) (*api.APIConfiguration, error) {
	mcpConfig, ok := input.(*api.MCPProxyConfiguration)
	if !ok || mcpConfig == nil {
		return nil, fmt.Errorf("invalid input type: expected *api.MCPProxyConfiguration")
	}
	output.Version = api.ApiPlatformWso2Comv1
	output.Kind = api.Httprest

	if len(mcpConfig.Spec.Upstreams) == 0 {
		return nil, fmt.Errorf("at least one upstream is required")
	}

	// Build APIConfigData and set it into the APIConfiguration_Spec union
	apiData := api.APIConfigData{
		Name:    mcpConfig.Spec.Name,
		Version: mcpConfig.Spec.Version,
		Context: mcpConfig.Spec.Context,
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: api.Upstream{
				Url: &mcpConfig.Spec.Upstreams[0].Url,
			},
		},
		Operations: addMCPSpecificOperations(mcpConfig),
	}

	var specUnion api.APIConfiguration_Spec
	if err := specUnion.FromAPIConfigData(apiData); err != nil {
		return nil, err
	}
	output.Spec = specUnion

	if mcpConfig.Metadata != nil {
		output.Metadata = &api.Metadata{
			Name: mcpConfig.Metadata.Name,
		}
	}
	return output, nil
}
