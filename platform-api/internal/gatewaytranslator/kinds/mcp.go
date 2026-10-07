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
	"net/url"
	"slices"
	"strings"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/translate"
	"github.com/wso2/api-platform/platform-api/internal/model"
)

// MCP is the MCP proxy kind (gateway and control-plane kind "Mcp").
//
// MCP artifacts have had a single stored shape. Older gateways need three
// adaptations, applied in this order.
var MCP = translate.Kind{
	GatewayKind: constants.MCPProxy,
	Normalize:   onlyType[*model.MCPProxyDeploymentYAML](),
	Steps: []translate.Step{
		{
			Below: gwversion.MinVerbatimMCPUpstreamPathVersion,
			Name:  "strip the trailing /mcp segment from spec.upstream.url",
			Apply: mcpStripUpstreamResourcePath,
		},
		{
			Below: gwversion.MinMCPSpecVersionListGatewayVersion,
			Name:  "fold spec.specVersions into the singular spec.specVersion",
			Apply: mcpFoldSpecVersions,
		},
		{
			Below: gwversion.MinUpstreamAuthTypeNoneOtherVersion,
			Name:  "adapt spec.upstream.auth to the auth types older validators accept",
			Apply: mcpUpstreamAuth,
		},
	},
}

// mcpResourcePath is the gateway-facing MCP endpoint path. Platform-api stores
// the full MCP endpoint URL as the upstream, so the stored path already ends
// in this segment.
const mcpResourcePath = "/mcp"

// mcpLegacySpecVersionsNewestFirst are the MCP specification revisions every
// gateway below MinMCPSpecVersionListGatewayVersion accepts in its singular
// spec.specVersion field, newest first.
var mcpLegacySpecVersionsNewestFirst = []string{
	constants.MCPSpecVersion20251125,
	constants.MCPSpecVersion20250618,
}

// mcpStripUpstreamResourcePath removes the final "/mcp" segment from the
// upstream URL, because the target gateway appends the "/mcp" operation path
// to the upstream path itself and would otherwise call ".../mcp/mcp".
// Scheme, host, userinfo and query are kept; one trailing slash is tolerated.
//
// A URL whose path does not end in exactly "/mcp" cannot be expressed for
// such a gateway (it always appends the segment), so it is shipped unchanged
// with a warning rather than failing the deploy. The warning never includes
// the URL, which may carry credentials.
func mcpStripUpstreamResourcePath(artifact any, r *translate.Report) error {
	a, err := expect[*model.MCPProxyDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	const field = "spec.upstream.url"
	raw := a.Spec.Upstream.URL
	u, parseErr := url.Parse(raw)
	if raw == "" || parseErr != nil || u.Host == "" {
		r.Warn(constants.MCPProxy, field,
			"upstream URL could not be parsed; gateways below %s append %s to the upstream path, so the artifact is shipped unchanged",
			gwversion.MinVerbatimMCPUpstreamPathVersion, mcpResourcePath)
		return nil
	}
	escaped := strings.TrimSuffix(u.EscapedPath(), "/")
	if !strings.HasSuffix(escaped, mcpResourcePath) {
		r.Warn(constants.MCPProxy, field,
			"upstream URL path does not end in %s; gateways below %s append %s to the upstream path, so the artifact is shipped unchanged",
			mcpResourcePath, gwversion.MinVerbatimMCPUpstreamPathVersion, mcpResourcePath)
		return nil
	}
	trimmed, err := url.PathUnescape(strings.TrimSuffix(escaped, mcpResourcePath))
	if err != nil {
		r.Warn(constants.MCPProxy, field,
			"upstream URL path could not be decoded; gateways below %s append %s to the upstream path, so the artifact is shipped unchanged",
			gwversion.MinVerbatimMCPUpstreamPathVersion, mcpResourcePath)
		return nil
	}
	u.RawPath = ""
	u.Path = trimmed
	a.Spec.Upstream.URL = u.String()
	return nil
}

// mcpFoldSpecVersions collapses the spec.specVersions list into the singular
// spec.specVersion the target gateway understands, choosing the newest
// revision that gateway accepts. Entries it does not accept are dropped with a
// warning; when none is accepted both fields are cleared, with a warning, so
// the gateway falls back to its own default. An artifact without the list is
// left untouched.
func mcpFoldSpecVersions(artifact any, r *translate.Report) error {
	a, err := expect[*model.MCPProxyDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	declared := a.Spec.SpecVersions
	if len(declared) == 0 {
		return nil
	}
	const field = "spec.specVersions"
	chosen := ""
	for _, v := range mcpLegacySpecVersionsNewestFirst {
		if slices.Contains(declared, v) {
			chosen = v
			break
		}
	}
	var dropped []string
	for _, v := range declared {
		if v != chosen {
			dropped = append(dropped, v)
		}
	}
	a.Spec.SpecVersion = chosen
	a.Spec.SpecVersions = nil
	switch {
	case chosen == "":
		r.Warn(constants.MCPProxy, field,
			"none of the declared MCP spec versions %v is supported by gateways below %s; specVersion omitted so the gateway default applies",
			declared, gwversion.MinMCPSpecVersionListGatewayVersion)
	case len(dropped) > 0:
		r.Warn(constants.MCPProxy, field,
			"gateways below %s accept a single specVersion; using %s and dropping %v",
			gwversion.MinMCPSpecVersionListGatewayVersion, chosen, dropped)
	}
	return nil
}

// mcpUpstreamAuth applies the shared upstream-auth rule to spec.upstream.auth.
func mcpUpstreamAuth(artifact any, r *translate.Report) error {
	a, err := expect[*model.MCPProxyDeploymentYAML](artifact)
	if err != nil {
		return err
	}
	downConvertModelUpstreamAuth(&a.Spec.Upstream.Auth, constants.MCPProxy, "spec.upstream.auth", r)
	return nil
}
