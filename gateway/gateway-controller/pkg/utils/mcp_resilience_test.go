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

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
)

// mcpOpResilience returns the resilience block attached to the operation matching method+path,
// or nil if the operation is absent or carries no resilience.
func mcpOpResilience(ops []api.Operation, method api.OperationMethod, path string) *api.Resilience {
	for _, op := range ops {
		if op.Method == method && op.Path == path {
			return op.Resilience
		}
	}
	return nil
}

// When no resilience is set, the MCP forwarding routes (GET/POST/DELETE /mcp) default the route
// timeout to "0s" (disabled) — MCP is streaming, so a finite total cap would sever a live stream.
// The local-response routes (OPTIONS, PRM) must carry no resilience. See mcp-timeout-divergence.md.
func TestMCPTransform_Resilience_DefaultsRouteTimeoutDisabled(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			// A cors policy makes the transformer emit the OPTIONS routes so we can assert they are skipped.
			Policies: &[]api.Policy{{Name: "cors", Version: "v1"}},
		},
	}

	res, err := (&MCPTransformer{}).Transform(in, &api.RestAPI{})
	require.NoError(t, err)
	ops := res.Spec.Operations

	for _, m := range []api.OperationMethod{api.OperationMethodGET, api.OperationMethodPOST, api.OperationMethodDELETE} {
		r := mcpOpResilience(ops, m, constants.MCP_RESOURCE_PATH)
		require.NotNil(t, r, "forwarding %s %s should carry resilience", m, constants.MCP_RESOURCE_PATH)
		require.NotNil(t, r.Timeout)
		assert.Equal(t, "0s", *r.Timeout, "route timeout should default to disabled for MCP")
		assert.Nil(t, r.IdleTimeout, "idle timeout should be left to the global default when unset")
	}

	// Local-response routes must not carry a route timeout.
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodOPTIONS, constants.MCP_RESOURCE_PATH), "OPTIONS /mcp is a local reply")
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodGET, constants.MCP_PRM_RESOURCE_PATH), "PRM route is a local reply")
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodOPTIONS, constants.MCP_PRM_RESOURCE_PATH), "OPTIONS PRM is a local reply")
}

// An explicit resilience block overrides the disabled default and its idleTimeout is carried through,
// on the forwarding routes only.
func TestMCPTransform_Resilience_UserOverride(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			Resilience:  &api.Resilience{Timeout: stringPtr("30s"), IdleTimeout: stringPtr("120s")},
		},
	}

	res, err := (&MCPTransformer{}).Transform(in, &api.RestAPI{})
	require.NoError(t, err)
	ops := res.Spec.Operations

	r := mcpOpResilience(ops, api.OperationMethodGET, constants.MCP_RESOURCE_PATH)
	require.NotNil(t, r)
	require.NotNil(t, r.Timeout)
	assert.Equal(t, "30s", *r.Timeout)
	require.NotNil(t, r.IdleTimeout)
	assert.Equal(t, "120s", *r.IdleTimeout)

	// The PRM route (present for spec >= 2025-06-18) is still skipped.
	assert.Nil(t, mcpOpResilience(ops, api.OperationMethodGET, constants.MCP_PRM_RESOURCE_PATH))
}

// A user timeout with no idle override still disables nothing extra: timeout is the user value,
// idle stays unset (global default).
func TestMCPTransform_Resilience_TimeoutOnly(t *testing.T) {
	context := "/everything"
	latest := LATEST_SUPPORTED_MCP_SPEC_VERSION
	in := &api.MCPProxyConfiguration{
		Spec: api.MCPProxyConfigData{
			DisplayName: "everything",
			Version:     "v1.0",
			Context:     &context,
			SpecVersion: &latest,
			Upstream:    api.MCPProxyConfigData_Upstream{Url: stringPtr("http://backend:8080")},
			Resilience:  &api.Resilience{Timeout: stringPtr("45s")},
		},
	}

	res, err := (&MCPTransformer{}).Transform(in, &api.RestAPI{})
	require.NoError(t, err)

	r := mcpOpResilience(res.Spec.Operations, api.OperationMethodPOST, constants.MCP_RESOURCE_PATH)
	require.NotNil(t, r)
	require.NotNil(t, r.Timeout)
	assert.Equal(t, "45s", *r.Timeout)
	assert.Nil(t, r.IdleTimeout)
}
