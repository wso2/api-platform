/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wso2/api-platform/platform-api/api"
	"github.com/wso2/api-platform/platform-api/internal/apperror"
	"github.com/wso2/api-platform/platform-api/internal/model"
	"github.com/wso2/api-platform/platform-api/internal/utils"
)

func apiSpecVersions(vs ...string) *[]string {
	out := append([]string(nil), vs...)
	return &out
}

func apiSpecVersion(v string) *string {
	sv := v
	return &sv
}

func TestMCPSpecVersionsFromRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *api.MCPProxy
		want []string
	}{
		{name: "neither form declared", req: &api.MCPProxy{}, want: nil},
		{
			name: "deprecated scalar is folded into the list",
			req:  &api.MCPProxy{McpSpecVersion: apiSpecVersion("2025-06-18")},
			want: []string{"2025-06-18"},
		},
		{
			name: "list is stored as sent",
			req:  &api.MCPProxy{McpSpecVersions: apiSpecVersions("2025-06-18", "2026-07-28")},
			want: []string{"2025-06-18", "2026-07-28"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mcpSpecVersionsFromRequest(tt.req))
		})
	}
}

func TestValidateMCPSpecVersions(t *testing.T) {
	both := &api.MCPProxy{
		McpSpecVersion:  apiSpecVersion("2025-06-18"),
		McpSpecVersions: apiSpecVersions("2026-07-28"),
	}
	err := validateMCPSpecVersions(both)
	if !apperror.ValidationFailed.Is(err) {
		t.Fatalf("declaring both forms: got %v, want a validation failure", err)
	}

	for _, req := range []*api.MCPProxy{
		{},
		{McpSpecVersion: apiSpecVersion("2025-06-18")},
		{McpSpecVersions: apiSpecVersions("2026-07-28")},
	} {
		if err := validateMCPSpecVersions(req); err != nil {
			t.Errorf("single form or none: got %v, want nil", err)
		}
	}
}

// A row written before this change carries only the deprecated scalar. It must read back as the
// canonical list without the stored configuration being rewritten.
func TestMCPSpecVersionsLegacyRowReadsAsList(t *testing.T) {
	cfg := model.MCPProxyConfiguration{SpecVersion: "2025-11-25"}

	got := mcpSpecVersionsToAPI(cfg)
	assert.Equal(t, apiSpecVersions("2025-11-25"), got)
	assert.Equal(t, &[]string{"2025-11-25"}, utils.StringSlicePtr(cfg.EffectiveSpecVersions()))
	assert.Equal(t, "2025-11-25", cfg.SpecVersion, "reading must not rewrite the stored configuration")
	assert.Nil(t, cfg.SpecVersions, "reading must not populate the stored list")
}

func TestMCPSpecVersionsResponseOmitsDeprecatedField(t *testing.T) {
	proxy := &model.MCPProxy{
		Handle:        "weather-mcp",
		Name:          "Weather",
		Version:       "v1.0",
		Configuration: model.MCPProxyConfiguration{SpecVersions: []string{"2026-07-28"}},
	}

	assert.Nil(t, mapMCPProxyModelToAPI(proxy).McpSpecVersion)
	assert.Equal(t, apiSpecVersions("2026-07-28"), mapMCPProxyModelToAPI(proxy).McpSpecVersions)
	assert.Nil(t, mapMCPProxyModelToListItem(proxy).McpSpecVersion)
	assert.Equal(t, &[]string{"2026-07-28"}, mapMCPProxyModelToListItem(proxy).McpSpecVersions)
}

func TestMCPSpecVersionsNeitherFormYieldsNothing(t *testing.T) {
	proxy := &model.MCPProxy{Handle: "h", Name: "n", Version: "v1.0"}
	assert.Nil(t, mapMCPProxyModelToAPI(proxy).McpSpecVersions)
	assert.Nil(t, mapMCPProxyModelToListItem(proxy).McpSpecVersions)
}
