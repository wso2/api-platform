/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package kinds

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/dto"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

func newMCPArtifact(upstreamURL string) *model.MCPProxyDeploymentYAML {
	a := &model.MCPProxyDeploymentYAML{ApiVersion: constants.GatewayApiVersion, Kind: constants.MCPProxy}
	a.Spec.Upstream.URL = upstreamURL
	return a
}

func TestMCP_StripUpstreamResourcePath(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		want     string // "" means unchanged
		warnings int
	}{
		{"root mcp endpoint", "https://h/mcp", "https://h", 0},
		{"nested mcp endpoint", "https://h/api/mcp", "https://h/api", 0},
		{"trailing slash tolerated", "https://h/api/mcp/", "https://h/api", 0},
		{"query kept", "https://h/mcp?x=1", "https://h?x=1", 0},
		{"userinfo kept", "https://u:p@h/api/mcp", "https://u:p@h/api", 0},
		{"percent-encoding kept", "https://h/a%20b/mcp", "https://h/a%20b", 0},
		{"port kept", "http://h:8080/v1/mcp", "http://h:8080/v1", 0},
		{"no mcp segment", "https://h/", "", 1},
		{"no path", "https://h", "", 1},
		{"mcp is a prefix not a segment", "https://h/mcpx", "", 1},
		{"case sensitive", "https://h/MCP", "", 1},
		{"mcp not final", "https://h/mcp/tools", "", 1},
		{"placeholder url", `{{ secret "u" }}`, "", 1},
		{"empty url", "", "", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newMCPArtifact(tt.in)
			var r translate.Report
			require.NoError(t, mcpStripUpstreamResourcePath(a, &r))
			want := tt.want
			if want == "" {
				want = tt.in
			}
			assert.Equal(t, want, a.Spec.Upstream.URL)
			ws := r.Warnings()
			assert.Len(t, ws, tt.warnings)
			for _, w := range ws {
				assert.Equal(t, "spec.upstream.url", w.Field)
				if tt.in != "" {
					assert.NotContains(t, w.Msg, tt.in, "the warning never echoes the URL")
				}
			}
		})
	}
}

func TestMCP_FoldSpecVersions(t *testing.T) {
	tests := []struct {
		name         string
		declared     []string
		wantSingular string
		warnings     int
	}{
		{"both legacy revisions: newest wins, the other is reported dropped", []string{"2025-06-18", "2025-11-25"}, "2025-11-25", 1},
		{"only the older revision", []string{"2025-06-18"}, "2025-06-18", 0},
		{"only a modern revision: cleared, gateway default applies", []string{"2026-07-28"}, "", 1},
		{"modern plus legacy: legacy kept, modern dropped", []string{"2026-07-28", "2025-06-18"}, "2025-06-18", 1},
		{"nothing declared: untouched", nil, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newMCPArtifact("https://h/mcp")
			a.Spec.SpecVersions = tt.declared
			var r translate.Report
			require.NoError(t, mcpFoldSpecVersions(a, &r))
			assert.Equal(t, tt.wantSingular, a.Spec.SpecVersion)
			assert.Nil(t, a.Spec.SpecVersions, "the list is always cleared")
			assert.Len(t, r.Warnings(), tt.warnings)
		})
	}

	t.Run("an existing singular value is left alone when no list is declared", func(t *testing.T) {
		a := newMCPArtifact("https://h/mcp")
		a.Spec.SpecVersion = "2025-06-18"
		var r translate.Report
		require.NoError(t, mcpFoldSpecVersions(a, &r))
		assert.Equal(t, "2025-06-18", a.Spec.SpecVersion)
		assert.True(t, r.Empty())
	})
}

func TestMCP_UpstreamAuth(t *testing.T) {
	tests := []struct {
		authType string
		wantNil  bool
		warnings int
	}{
		{"api-key", false, 0},
		{"", false, 0},
		{"none", true, 0},
		{"other", false, 1},
		{"basic", false, 1},
		{"bearer", false, 0}, // MCP validators have handled bearer since 1.0.0
	}
	for _, tt := range tests {
		t.Run("type="+tt.authType, func(t *testing.T) {
			a := newMCPArtifact("https://h/mcp")
			a.Spec.Upstream.Auth = &model.UpstreamAuth{Type: tt.authType, Header: "Authorization", Value: `{{ secret "k" }}`}
			var r translate.Report
			require.NoError(t, mcpUpstreamAuth(a, &r))
			if tt.wantNil {
				assert.Nil(t, a.Spec.Upstream.Auth)
			} else {
				require.NotNil(t, a.Spec.Upstream.Auth)
				assert.Equal(t, tt.authType, a.Spec.Upstream.Auth.Type, "kept unchanged")
			}
			assert.Len(t, r.Warnings(), tt.warnings)
		})
	}

	t.Run("no auth block", func(t *testing.T) {
		a := newMCPArtifact("https://h/mcp")
		var r translate.Report
		require.NoError(t, mcpUpstreamAuth(a, &r))
		assert.Nil(t, a.Spec.Upstream.Auth)
		assert.True(t, r.Empty())
	})
}

// The whole kind through the engine, per target gateway.
func TestMCP_Run(t *testing.T) {
	newArtifact := func() *model.MCPProxyDeploymentYAML {
		a := newMCPArtifact("https://b/api/mcp")
		a.Spec.SpecVersions = []string{"2026-07-28", "2025-11-25"}
		return a
	}

	for _, old := range []string{"1.0.0", "1.1.0"} {
		t.Run("gateway "+old, func(t *testing.T) {
			a := newArtifact()
			rep, err := translate.Run(MCP, "1.1", old, a)
			require.NoError(t, err)
			assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.ApiVersion)
			assert.Equal(t, "https://b/api", a.Spec.Upstream.URL)
			assert.Equal(t, "2025-11-25", a.Spec.SpecVersion)
			assert.Nil(t, a.Spec.SpecVersions)
			require.Len(t, rep.Warnings(), 1, "only the dropped 2026-07-28 revision")
			assert.Equal(t, "spec.specVersions", rep.Warnings()[0].Field)
		})
	}

	t.Run("gateway 1.2.0 folds the list only", func(t *testing.T) {
		a := newArtifact()
		rep, err := translate.Run(MCP, "1.1", "1.2.0", a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
		assert.Equal(t, "https://b/api/mcp", a.Spec.Upstream.URL)
		assert.Equal(t, "2025-11-25", a.Spec.SpecVersion)
		assert.Nil(t, a.Spec.SpecVersions)
		assert.Len(t, rep.Warnings(), 1)
	})

	// 1.2.0 lists none/other in its MCP schema but its validator still demands
	// a header and value for every type, so the auth step runs there too.
	for _, gw := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		t.Run("gateway "+gw+" drops auth type none", func(t *testing.T) {
			a := newMCPArtifact("https://b/api/mcp")
			a.Spec.Upstream.Auth = &model.UpstreamAuth{Type: "none"}
			rep, err := translate.Run(MCP, "1.1", gw, a)
			require.NoError(t, err)
			assert.Nil(t, a.Spec.Upstream.Auth)
			assert.True(t, rep.Empty())
		})
	}

	t.Run("unversioned gateway keeps auth type none", func(t *testing.T) {
		a := newMCPArtifact("https://b/api/mcp")
		a.Spec.Upstream.Auth = &model.UpstreamAuth{Type: "none"}
		rep, err := translate.Run(MCP, "1.1", "", a)
		require.NoError(t, err)
		require.NotNil(t, a.Spec.Upstream.Auth)
		assert.Equal(t, "none", a.Spec.Upstream.Auth.Type)
		assert.True(t, rep.Empty())
	})

	t.Run("unversioned gateway is untouched", func(t *testing.T) {
		a := newArtifact()
		rep, err := translate.Run(MCP, "1.1", "", a)
		require.NoError(t, err)
		assert.Equal(t, constants.GatewayApiVersion, a.ApiVersion)
		assert.Equal(t, "https://b/api/mcp", a.Spec.Upstream.URL)
		assert.Equal(t, []string{"2026-07-28", "2025-11-25"}, a.Spec.SpecVersions)
		assert.Empty(t, a.Spec.SpecVersion)
		assert.True(t, rep.Empty())
	})
}

func TestMCP_RejectsAnotherKindsArtifact(t *testing.T) {
	_, err := translate.Run(MCP, "1.1", "1.2.0", &dto.LLMProxyDeploymentYAML{})
	assert.Error(t, err)
}
