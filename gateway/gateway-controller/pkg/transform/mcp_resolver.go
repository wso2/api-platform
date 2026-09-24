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
	"net/http"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// MCPResolverName is the resolver the policy engine registers for MCP proxies. A wire value: a
// route naming a resolver the runtime does not advertise is skipped at ingest.
const MCPResolverName = "mcp"

// mcpResolverOperation is the operation component of every MCP chain key. Constant because an
// MCP proxy has one chain whatever tool is invoked; per-tool behaviour lives in policy params.
const mcpResolverOperation = "mcp"

// mcpResolutionApplies reports whether this config is an MCP proxy, and so whether its
// multiplexed route carries the resolver. It reads SourceConfiguration because a transformer
// runs after an MCP proxy is desugared into a RestAPI, which only the source still identifies.
func mcpResolutionApplies(cfg *models.StoredConfig) (bool, error) {
	if cfg == nil || cfg.Kind != string(models.KindMcp) {
		return false, nil
	}

	if _, ok := cfg.SourceConfiguration.(api.MCPProxyConfiguration); !ok {
		// Refusing is deliberate: falling back to no resolver would deploy an MCP proxy
		// whose policies never learn which tool was invoked, and it would look healthy.
		return false, fmt.Errorf("kind %q carries source configuration of type %T, expected api.MCPProxyConfiguration",
			cfg.Kind, cfg.SourceConfiguration)
	}

	return true, nil
}

// isMCPMultiplexedRoute reports whether an operation is the one MCP route carrying every logical
// operation. GET, DELETE, OPTIONS and the OAuth protected-resource route each mean one thing, so they stay route-keyed.
func isMCPMultiplexedRoute(method, opPath string) bool {
	return method == http.MethodPost && opPath == constants.MCP_RESOURCE_PATH
}
