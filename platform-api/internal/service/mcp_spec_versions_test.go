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

func apiUpstreamSpecVersions(vs ...string) *[]string {
	out := append([]string(nil), vs...)
	return &out
}

// The snapshot is stored as the server named it. Older revisions and ones this build has never
// heard of are kept: narrowing them would make the record a claim about this platform rather
// than about the server.
func TestUpstreamSpecVersionsFromRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *api.MCPProxy
		want []string
	}{
		{name: "absent", req: &api.MCPProxy{}, want: nil},
		{
			name: "every revision the server named, however old or unknown",
			req: &api.MCPProxy{UpstreamMcpSpecVersions: apiUpstreamSpecVersions(
				"2024-11-05", "2025-03-26", "2025-06-18", "2026-07-28", "2099-01-01")},
			want: []string{"2024-11-05", "2025-03-26", "2025-06-18", "2026-07-28", "2099-01-01"},
		},
		{
			// A caller may send [] where the workspace omits the field; both end up storing
			// nothing, and the response omits the field either way (StringSlicePtr).
			name: "an explicitly empty list stores an empty list",
			req:  &api.MCPProxy{UpstreamMcpSpecVersions: &[]string{}},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, upstreamSpecVersionsFromRequest(tt.req))
		})
	}
}

// The two fields answer different questions, so neither may be read from the other: what the
// proxy declares stays empty when only the upstream reported something, and the reverse.
func TestUpstreamSpecVersionsAreIndependentOfTheDeclaredList(t *testing.T) {
	reportedOnly := &model.MCPProxy{
		Handle: "h", Name: "n", Version: "v1.0",
		Configuration: model.MCPProxyConfiguration{
			UpstreamSpecVersions: []string{"2024-11-05", "2026-07-28"},
		},
	}
	out := mapMCPProxyModelToAPI(reportedOnly)
	assert.Equal(t, apiUpstreamSpecVersions("2024-11-05", "2026-07-28"), out.UpstreamMcpSpecVersions)
	assert.Nil(t, out.McpSpecVersions, "a reported revision must not become a declared one")

	declaredOnly := &model.MCPProxy{
		Handle: "h", Name: "n", Version: "v1.0",
		Configuration: model.MCPProxyConfiguration{SpecVersions: []string{"2025-06-18"}},
	}
	out = mapMCPProxyModelToAPI(declaredOnly)
	assert.Equal(t, apiSpecVersions("2025-06-18"), out.McpSpecVersions)
	assert.Nil(t, out.UpstreamMcpSpecVersions, "a declared revision says nothing about the server")
}

// The list endpoint is a summary and carries only what a proxy declares. Recorded so that
// adding the snapshot there later is a decision rather than an accident.
func TestUpstreamSpecVersionsAreAbsentFromTheListItem(t *testing.T) {
	proxy := &model.MCPProxy{
		Handle: "h", Name: "n", Version: "v1.0",
		Configuration: model.MCPProxyConfiguration{
			SpecVersions:         []string{"2025-06-18"},
			UpstreamSpecVersions: []string{"2026-07-28"},
		},
	}
	item := mapMCPProxyModelToListItem(proxy)
	assert.Equal(t, &[]string{"2025-06-18"}, item.McpSpecVersions)
}

