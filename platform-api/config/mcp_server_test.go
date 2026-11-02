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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMCPServerConfig(t *testing.T) {
	idp := &Auth{Mode: AuthModeIDP}
	idpRESTAudience := &Auth{Mode: AuthModeIDP, IDP: IDP{Audience: []string{"rest"}}}
	idpBothAudiences := &Auth{Mode: AuthModeIDP, IDP: IDP{Audience: []string{"rest", "mcp"}}}
	const publicURL = "https://platform-api.example.com"
	const mb = 1 << 20
	unchecked := &Auth{Mode: AuthModeInternalToken, InternalToken: InternalToken{SkipValidation: true}}

	tests := []struct {
		name    string
		m       MCPServer
		auth    *Auth
		wantErr string
	}{
		{"off is always fine", MCPServer{}, unchecked, ""},
		{"idp needs an audience", MCPServer{Enabled: true, MaxRequestBytes: mb}, idp, "audience is required"},
		{"idp with an audience", MCPServer{Enabled: true, MaxRequestBytes: mb, PublicURL: publicURL, Auth: MCPServerAuth{Audience: "mcp"}}, idp, ""},
		{"idp needs a public URL", MCPServer{Enabled: true, MaxRequestBytes: mb, Auth: MCPServerAuth{Audience: "mcp"}}, idp, "public_url is required"},
		{"idp audience list must hold the MCP one", MCPServer{Enabled: true, MaxRequestBytes: mb, PublicURL: publicURL, Auth: MCPServerAuth{Audience: "mcp"}}, idpRESTAudience, "must also be listed in auth.idp.audience"},
		{"idp audience list with the MCP one", MCPServer{Enabled: true, MaxRequestBytes: mb, PublicURL: publicURL, Auth: MCPServerAuth{Audience: "mcp"}}, idpBothAudiences, ""},
		{"file mode", MCPServer{Enabled: true, MaxRequestBytes: mb}, &Auth{Mode: AuthModeFile}, ""},
		// Upstream supports skip_validation as a trust boundary, and /mcp signs in like any route.
		{"unvalidated internal tokens", MCPServer{Enabled: true, MaxRequestBytes: mb}, unchecked, ""},
		{"a request limit of 0", MCPServer{Enabled: true, Auth: MCPServerAuth{Audience: "mcp"}}, idp, "max_request_bytes"},
		{"a negative request limit", MCPServer{Enabled: true, MaxRequestBytes: -1, Auth: MCPServerAuth{Audience: "mcp"}}, idp, "max_request_bytes"},
		{"public_url without a scheme", MCPServer{Enabled: true, MaxRequestBytes: mb, PublicURL: "platform-api.example.com", Auth: MCPServerAuth{Audience: "mcp"}}, idp, "public_url"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMCPServerConfig(&tc.m, tc.auth)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
