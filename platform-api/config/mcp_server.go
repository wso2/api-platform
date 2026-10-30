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
	"errors"
	"net/url"
	"slices"
)

// MCPServer configures the MCP server at /api/v0.9/mcp, which lets an MCP client
// manage APIs, projects and gateways. Not to be confused with MCP proxies.
type MCPServer struct {
	Enabled         bool  `koanf:"enabled"`
	MaxRequestBytes int64 `koanf:"max_request_bytes"`
	// PublicURL is how clients reach this server. Required in idp mode, for login discovery.
	PublicURL string        `koanf:"public_url"`
	Auth      MCPServerAuth `koanf:"auth"`
}

// MCPServerAuth is the token check /mcp adds to the Platform API's sign-in.
type MCPServerAuth struct {
	// Audience is required in idp mode, so a token issued for the REST API is
	// not accepted here. When auth.idp.audience is set, it must list this too.
	Audience string `koanf:"audience"`
}

func validateMCPServerConfig(m *MCPServer, auth *Auth) error {
	if !m.Enabled {
		return nil
	}
	if m.MaxRequestBytes <= 0 {
		return errors.New("mcp_server.max_request_bytes must be greater than 0")
	}
	if m.PublicURL != "" && !isHTTPURL(m.PublicURL) {
		return errors.New("mcp_server.public_url must be an absolute http(s) URL")
	}
	if auth.Mode == AuthModeIDP {
		switch {
		case m.Auth.Audience == "":
			return errors.New("mcp_server.auth.audience is required in idp mode")
		case m.PublicURL == "":
			return errors.New("mcp_server.public_url is required in idp mode, so clients can find where to log in")
		case len(auth.IDP.Audience) > 0 && !slices.Contains(auth.IDP.Audience, m.Auth.Audience):
			// The sign-in checks auth.idp.audience first, so a token for /mcp must pass that too.
			return errors.New("mcp_server.auth.audience must also be listed in auth.idp.audience")
		}
	}
	return nil
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
