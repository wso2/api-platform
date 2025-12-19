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

func NewMCPTransformer() *MCPTransformer {
	return &MCPTransformer{}
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
	output.ApiVersion = api.APIConfigurationApiVersionGatewayApiPlatformWso2Comv1alpha1
	output.Kind = api.RestApi

	// Build APIConfigData and set it into the APIConfiguration_Spec union
	apiData := api.APIConfigData{
		DisplayName: mcpConfig.Spec.DisplayName,
		Version:     mcpConfig.Spec.Version,
		Operations:  addMCPSpecificOperations(mcpConfig),
	}
	if mcpConfig.Spec.Context != nil {
		apiData.Context = *mcpConfig.Spec.Context
	}

	apiData.Upstream.Main = api.Upstream{
		Url: mcpConfig.Spec.Upstream.Url,
	}

	// Process policies
	var policies []api.Policy
	// Normal policies need to be set before the upstream auth because if the upstream auth
	// policy sets the authorization header, it may clash with the authentication policies set.
	// Setting upstream auth last ensures that the authorization header is set only after the JWT
	// validation has been performed.
	if mcpConfig.Spec.Policies != nil && len(*mcpConfig.Spec.Policies) > 0 {
		policies = append(policies, *mcpConfig.Spec.Policies...)
	}

	// Set upstream auth if present
	upstream := mcpConfig.Spec.Upstream
	if upstream.Auth != nil {
		params, err := GetParamsOfPolicy(constants.MODIFY_HEADERS_POLICY_PARAMS, *upstream.Auth.Header, *upstream.Auth.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to build upstream auth params: %w", err)
		}
		pol := api.Policy{
			Name:    constants.MODIFY_HEADERS_POLICY_NAME,
			Version: constants.MODIFY_HEADERS_POLICY_VERSION, Params: &params}
		policies = append(policies, pol)
	}

	apiData.Policies = &policies

	// Set vhost if present
	if mcpConfig.Spec.Vhost != nil {
		v := struct {
			Main    string  `json:"main" yaml:"main"`
			Sandbox *string `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main: *mcpConfig.Spec.Vhost,
		}
		apiData.Vhosts = &v
	}

	var specUnion api.APIConfiguration_Spec
	if err := specUnion.FromAPIConfigData(apiData); err != nil {
		return nil, err
	}
	output.Spec = specUnion

	output.Metadata = api.Metadata{
		Name: mcpConfig.Metadata.Name,
	}

	return output, nil
}
