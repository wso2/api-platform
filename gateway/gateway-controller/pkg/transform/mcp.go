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

package transform

import (
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// MCPKindTransformer transforms an MCP StoredConfig into a RuntimeDeployConfig.
// It first uses utils.MCPTransformer to produce a RestAPI, then runs RestAPITransformer
// on the result, and finally restores the original kind in the metadata.
type MCPKindTransformer struct {
	mcpTransformer  *utils.MCPTransformer
	restTransformer *RestAPITransformer
}

// NewMCPKindTransformer creates a new MCPKindTransformer.
func NewMCPKindTransformer(
	routerConfig *config.RouterConfig,
	systemConfig *config.Config,
	policyDefinitions map[string]models.PolicyDefinition,
) *MCPKindTransformer {
	return &MCPKindTransformer{
		mcpTransformer:  utils.NewMCPTransformer(),
		restTransformer: NewRestAPITransformer(routerConfig, systemConfig, policyDefinitions),
	}
}

// Transform converts a StoredConfig (Mcp kind) into a RuntimeDeployConfig.
func (t *MCPKindTransformer) Transform(cfg *models.StoredConfig) (*models.RuntimeDeployConfig, error) {
	mcpConfig, ok := cfg.SourceConfiguration.(api.MCPProxyConfiguration)
	if !ok {
		return nil, fmt.Errorf("unsupported MCP source configuration type: %T", cfg.SourceConfiguration)
	}

	// Step 1: Transform MCP config → RestAPI using existing transformer
	var restAPI api.RestAPI
	if _, err := t.mcpTransformer.Transform(&mcpConfig, &restAPI); err != nil {
		return nil, fmt.Errorf("MCP transformation failed: %w", err)
	}

	// Step 2: Build a temporary StoredConfig with the RestAPI result
	tempCfg := &models.StoredConfig{
		UUID:                cfg.UUID,
		Kind:                cfg.Kind,
		Handle:              cfg.Handle,
		DisplayName:         cfg.DisplayName,
		Version:             cfg.Version,
		Configuration:       restAPI,
		SourceConfiguration: cfg.SourceConfiguration,
		DesiredState:        cfg.DesiredState,
		CreatedAt:           cfg.CreatedAt,
		UpdatedAt:           cfg.UpdatedAt,
	}

	// Step 3: Use RestAPITransformer to build RuntimeDeployConfig
	rdc, err := t.restTransformer.Transform(tempCfg)
	if err != nil {
		return nil, fmt.Errorf("RestAPI transformation for MCP failed: %w", err)
	}

	// Step 4: Restore original kind
	rdc.Metadata.Kind = cfg.Kind
	return rdc, nil
}
