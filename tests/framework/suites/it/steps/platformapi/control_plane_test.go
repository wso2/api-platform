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

package platformapi

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
)

func TestArtifactPath(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		handle  string
		want    string
		wantErr bool
	}{
		{name: "template", kind: "LlmProviderTemplate", handle: "tmpl-1", want: "/llm-provider-templates/tmpl-1"},
		{name: "provider", kind: "LlmProvider", handle: "prov-1", want: "/llm-providers/prov-1"},
		{name: "proxy", kind: "LlmProxy", handle: "proxy-1", want: "/llm-proxies/proxy-1"},
		{name: "mcp", kind: "Mcp", handle: "mcp-1", want: "/mcp-proxies/mcp-1"},
		{name: "rest api", kind: "RestApi", handle: "api-1", want: "/rest-apis/api-1"},
		{name: "unknown kind", kind: "WebSubApi", handle: "x", wantErr: true},
		{name: "empty kind", kind: "", handle: "x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := artifactPath(tt.kind, tt.handle)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDeploymentStatusMatches(t *testing.T) {
	tests := []struct {
		name string
		resp *httpx.Response
		want string
		ok   bool
	}{
		{
			name: "matching status present",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[{"status":"DEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   true,
		},
		{
			name: "different status present",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[{"status":"UNDEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "multiple deployments, one matches",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[{"status":"ARCHIVED"},{"status":"DEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   true,
		},
		{
			name: "empty list",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`{"list":[]}`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "non-200 status",
			resp: &httpx.Response{StatusCode: http.StatusNotFound, Body: []byte(`{"list":[{"status":"DEPLOYED"}]}`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "malformed body",
			resp: &httpx.Response{StatusCode: http.StatusOK, Body: []byte(`not json`)},
			want: "DEPLOYED",
			ok:   false,
		},
		{
			name: "nil response",
			resp: nil,
			want: "DEPLOYED",
			ok:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.ok, deploymentStatusMatches(tt.resp, tt.want))
		})
	}
}
