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

package policy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// ptr is a helper to get a pointer to a string literal.
func ptr(s string) *string { return &s }

// makeStoredConfig builds a minimal RestApi StoredConfig with one operation and a
// single API-level policy so that DerivePolicyFromAPIConfig returns a non-nil result.
func makeStoredConfig(t *testing.T, sandbox *api.Upstream) *models.StoredConfig {
	t.Helper()

	apiData := api.APIConfigData{
		DisplayName: "Test API",
		Context:     "/test",
		Version:     "1.0.0",
		Operations: []api.Operation{
			{Method: "GET", Path: "/hello"},
		},
		Policies: &[]api.Policy{
			{Name: "header-mutate", Version: "v1"},
		},
		Upstream: struct {
			Main    api.Upstream  `json:"main" yaml:"main"`
			Sandbox *api.Upstream `json:"sandbox,omitempty" yaml:"sandbox,omitempty"`
		}{
			Main:    api.Upstream{Url: ptr("http://backend:8080")},
			Sandbox: sandbox,
		},
	}

	apiConfig := api.RestAPI{
		Kind: api.RestApi,
		Metadata: api.Metadata{
			Name: "test-api",
		},
		Spec: apiData,
	}

	return &models.StoredConfig{
		UUID:                "test-api",
		Kind:                string(api.RestApi),
		Configuration:       apiConfig,
		SourceConfiguration: apiConfig,
		Origin:              models.OriginGatewayAPI,
	}
}

// testRouterConfig returns a minimal RouterConfig suitable for builder tests.
func testRouterConfig() *config.RouterConfig {
	return &config.RouterConfig{
		GatewayHost: "gw.local",
		VHosts: config.VHostsConfig{
			Main:    config.VHostEntry{Default: "main.local"},
			Sandbox: config.VHostEntry{Default: "sandbox.local"},
		},
	}
}

// policyDefs contains the single policy definition used across all test cases.
var policyDefs = map[string]api.PolicyDefinition{
	"header-mutate|v1.0.0": {Name: "header-mutate", Version: "v1.0.0"},
}

func TestDerivePolicyFromAPIConfig_SandboxVhosts(t *testing.T) {
	tests := []struct {
		name           string
		sandbox        *api.Upstream
		wantRouteCount int
	}{
		{
			name:           "no sandbox",
			sandbox:        nil,
			wantRouteCount: 1,
		},
		{
			name:           "sandbox with url",
			sandbox:        &api.Upstream{Url: ptr("http://sandbox-backend:8080")},
			wantRouteCount: 2,
		},
		{
			name:           "sandbox with ref",
			sandbox:        &api.Upstream{Ref: ptr("my-upstream-def")},
			wantRouteCount: 2,
		},
		{
			name:           "sandbox present but url and ref both nil",
			sandbox:        &api.Upstream{},
			wantRouteCount: 2,
		},
	}

	routerCfg := testRouterConfig()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := makeStoredConfig(t, tc.sandbox)
			result := DerivePolicyFromAPIConfig(cfg, routerCfg, &config.Config{}, policyDefs)
			require.NotNil(t, result, "expected non-nil policy config (API has policies)")
			assert.Len(t, result.Configuration.Routes, tc.wantRouteCount,
				"expected %d route key(s) for case %q", tc.wantRouteCount, tc.name)

			if tc.sandbox != nil {
				// Verify that the sandbox vhost ("sandbox.local") appears in at least one route key.
				hasSandboxRoute := false
				for _, r := range result.Configuration.Routes {
					if strings.Contains(r.RouteKey, "sandbox.local") {
						hasSandboxRoute = true
						break
					}
				}
				assert.True(t, hasSandboxRoute, "expected a route key containing 'sandbox.local'")
			}
		})
	}
}
