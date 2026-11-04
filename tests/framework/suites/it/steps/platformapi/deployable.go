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
	"context"
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// keyDeployableBase prefixes the context key under which an LLM proxy created by
// createDeployable records the base provider it routes to.
const keyDeployableBase = "platformDeployableBase:"

// createDeployable creates one artifact of kind, ready to deploy, so a Scenario Outline can run
// the same lifecycle steps once per kind. Every variant avoids secrets, so it deploys on every
// gateway release the kind supports. An LLM proxy gets a base provider of its own, which
// deployArtifactToGateway deploys to the same gateway before the proxy.
func (s *Steps) createDeployable(ctx context.Context, kind, name, project, resourceContext string) error {
	switch kind {
	case "RestApi":
		// POST /health too, so the outlines' POST probe reaches a REST API like the other kinds.
		return s.createRestAPIWithSecret(ctx, name, project, resourceContext, []map[string]any{
			{"request": map[string]any{"method": "GET", "path": "/health"}},
			{"request": map[string]any{"method": "POST", "path": "/health"}},
		}, map[string]any{"url": "http://testbench:3000"})
	case "GraphQLApi":
		return s.createGraphQLAPI(ctx, name, project, resourceContext)
	case "LlmProvider":
		return s.createLLMProvider(ctx, name, resourceContext, "openai", "")
	case "LlmProxy":
		return s.createDeployableLLMProxy(ctx, name, project, resourceContext)
	case "Mcp":
		return s.createDeployableMCPProxy(ctx, name, project, resourceContext)
	default:
		return fmt.Errorf("unknown deployable kind %q: use RestApi, GraphQLApi, LlmProvider, LlmProxy or Mcp", kind)
	}
}

// createDeployableLLMProxy creates a base provider and an LLM proxy routing to it, with no
// credential override.
func (s *Steps) createDeployableLLMProxy(ctx context.Context, name, project, resourceContext string) error {
	resolved := make([]string, 3)
	for i, value := range []string{name, project, resourceContext} {
		expanded, err := stepscommon.Expand(ctx, value)
		if err != nil {
			return err
		}
		resolved[i] = expanded
	}
	proxy, resolvedProject, proxyContext := resolved[0], resolved[1], resolved[2]
	provider := proxy + "-base"
	if err := s.createLLMProvider(ctx, provider, proxyContext+"-base", "openai", ""); err != nil {
		return err
	}
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	if err := s.postJSON(ctx, base, bearer, "/llm-proxies", map[string]any{
		"id":          proxy,
		"displayName": proxy,
		"version":     "v1.0",
		"projectId":   resolvedProject,
		"context":     proxyContext,
		"provider":    map[string]any{"id": provider},
	}, nil); err != nil {
		return err
	}
	if err := s.registerPlatformResource(ctx, platformProxyKind, proxy, "/llm-proxies"); err != nil {
		return err
	}
	return tcontext.Set(ctx, keyDeployableBase+proxy, provider)
}

// createDeployableMCPProxy creates an MCP proxy in front of the testbench MCP server, at the
// upstream path the gateway release under test expects.
func (s *Steps) createDeployableMCPProxy(ctx context.Context, name, project, resourceContext string) error {
	resolved := make([]string, 4)
	for i, value := range []string{name, project, resourceContext, "${CTX:gatewayMCPUpstreamPath}"} {
		expanded, err := stepscommon.Expand(ctx, value)
		if err != nil {
			return err
		}
		resolved[i] = expanded
	}
	proxy, resolvedProject, proxyContext, upstreamPath := resolved[0], resolved[1], resolved[2], resolved[3]
	base, bearer, err := s.authed(ctx)
	if err != nil {
		return err
	}
	if err := s.postJSON(ctx, base, bearer, "/mcp-proxies", map[string]any{
		"id":             proxy,
		"displayName":    proxy,
		"version":        "v1.0",
		"context":        proxyContext,
		"projectId":      resolvedProject,
		"mcpSpecVersion": "2025-06-18",
		"upstream":       map[string]any{"main": map[string]any{"url": "http://testbench:3009" + upstreamPath}},
	}, nil); err != nil {
		return err
	}
	return s.registerPlatformResource(ctx, platformMCPKind, proxy, "/mcp-proxies")
}

// deployableBase returns the base provider createDeployable made for an LLM proxy, if any.
func deployableBase(ctx context.Context, proxy string) (string, bool) {
	value, ok := tcontext.Get(ctx, keyDeployableBase+proxy)
	provider, isString := value.(string)
	return provider, ok && isString && provider != ""
}
