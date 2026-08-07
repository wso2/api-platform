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

// Upstream credential redaction for management API responses.
//
// An upstream `auth.value` is write-only: accepted on create/update and never
// returned by the management API on a read, for any role. The corresponding
// `value` properties in management-openapi.yaml are marked `writeOnly: true`.
//
// These functions operate on the re-materialised configuration produced by each
// rematerialize*Config round-trip — a response-bound copy — never on a
// StoredConfig's own SourceConfiguration, which is what each replica re-renders
// on consumption.
package handlers

import (
	"log/slog"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// buildDeploymentListItem builds the response body for one stored configuration
// on the generic search/list path. Kinds whose schema can carry an upstream
// credential are routed through their rematerialize helper, where redaction
// lives. Dispatch is on the stored artifact's own Kind, not the requested kind.
func buildDeploymentListItem(log *slog.Logger, cfg *models.StoredConfig) (any, error) {
	switch cfg.Kind {
	case string(api.MCPProxyConfigurationKindMcp):
		mcp, err := rematerializeMCPProxyConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
		if err != nil {
			return nil, err
		}
		return buildResourceResponseFromStored(mcp, cfg), nil

	case string(api.LLMProviderConfigurationKindLlmProvider):
		prov, err := rematerializeLLMProviderConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
		if err != nil {
			return nil, err
		}
		return buildResourceResponseFromStored(prov, cfg), nil

	case string(api.LLMProxyConfigurationKindLlmProxy):
		proxy, err := rematerializeLLMProxyConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
		if err != nil {
			return nil, err
		}
		return buildResourceResponseFromStored(proxy, cfg), nil

	default:
		// Kinds whose schema has no upstream auth block. A credential-carrying
		// field added to one of these needs a case above, not this passthrough.
		return buildResourceResponseFromStored(cfg.SourceConfiguration, cfg), nil
	}
}

// redactLLMProviderCredentials clears the upstream credential from an LLM
// provider configuration bound for a response body.
func redactLLMProviderCredentials(cfg *api.LLMProviderConfiguration) {
	if cfg == nil || cfg.Spec.Upstream.Auth == nil {
		return
	}
	// nil, not empty string: `value` carries omitempty, so the field is absent
	// from the response rather than present-but-blank.
	cfg.Spec.Upstream.Auth.Value = nil
}

// redactLLMProxyCredentials clears the upstream credentials from an LLM proxy
// configuration bound for a response body — both the primary provider's auth
// and every additionalProviders[] entry's auth.
func redactLLMProxyCredentials(cfg *api.LLMProxyConfiguration) {
	if cfg == nil {
		return
	}
	if cfg.Spec.Provider.Auth != nil {
		cfg.Spec.Provider.Auth.Value = nil
	}
	if cfg.Spec.AdditionalProviders == nil {
		return
	}
	additional := *cfg.Spec.AdditionalProviders
	for i := range additional {
		if additional[i].Auth != nil {
			additional[i].Auth.Value = nil
		}
	}
}

// redactMCPProxyCredentials clears the upstream credential from an MCP proxy
// configuration bound for a response body.
func redactMCPProxyCredentials(cfg *api.MCPProxyConfiguration) {
	if cfg == nil || cfg.Spec.Upstream.Auth == nil {
		return
	}
	cfg.Spec.Upstream.Auth.Value = nil
}
