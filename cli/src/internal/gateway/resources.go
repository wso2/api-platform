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

package gateway

import (
	"fmt"

	"github.com/wso2/api-platform/cli/utils"
)

// ResourceKind represents the type of gateway resource
const (
	ResourceKindRestAPI = "RestApi"
	ResourceKindMCP     = "mcp"
)

// Resource represents a parsed gateway resource
type Resource struct {
	Kind    string
	Handle  string // metadata.name
	RawYAML []byte
}

// ResourceHandler defines the interface for handling different resource kinds
type ResourceHandler interface {
	// GetEndpoint returns the GET endpoint to check if resource exists
	GetEndpoint(handle string) string

	// CreateEndpoint returns the POST endpoint to create a new resource
	CreateEndpoint() string

	// UpdateEndpoint returns the PUT endpoint to update an existing resource
	UpdateEndpoint(handle string) string
}

// RestAPIHandler handles RestApi kind resources
type RestAPIHandler struct{}

func (h *RestAPIHandler) GetEndpoint(handle string) string {
	return fmt.Sprintf(utils.GatewayAPIByIDPath, handle)
}

func (h *RestAPIHandler) CreateEndpoint() string {
	return utils.GatewayAPIsPath
}

func (h *RestAPIHandler) UpdateEndpoint(handle string) string {
	return fmt.Sprintf(utils.GatewayAPIByIDPath, handle)
}

// MCPHandler handles mcp kind resources
type MCPHandler struct{}

func (h *MCPHandler) GetEndpoint(handle string) string {
	return fmt.Sprintf(utils.GatewayMCPProxyByIDPath, handle)
}

func (h *MCPHandler) CreateEndpoint() string {
	return utils.GatewayMCPProxiesPath
}

func (h *MCPHandler) UpdateEndpoint(handle string) string {
	return fmt.Sprintf(utils.GatewayMCPProxyByIDPath, handle)
}

// GetResourceHandler returns the appropriate handler for a resource kind
func GetResourceHandler(kind string) ResourceHandler {
	switch kind {
	case ResourceKindRestAPI:
		return &RestAPIHandler{}
	case ResourceKindMCP:
		return &MCPHandler{}
	default:
		return nil
	}
}
