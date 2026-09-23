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
// An upstream credential is write-only: accepted on create/update and never
// returned by the management API on a read, for any role. A credential reaches
// an auth block through one of two mechanisms, and both must be cleared:
//
//   - `value`, the deprecated api-key form. The corresponding properties in
//     management-openapi.yaml are marked `writeOnly: true`.
//   - `policyParams`, the free-form bucket. For "oauth2" and "other" it is the
//     only credential mechanism there is — it carries `clientSecret`,
//     `bearerToken` and whatever else the named policy expects. For "api-key"
//     it replaces the deprecated header/value pair.
//
// `policyParams` is dropped whole rather than scrubbed key by key. Its contents
// are defined by whichever policy consumes it, so there is no closed set of
// credential-bearing names to match on, and a denylist that misses one discloses
// it. Dropping the bucket costs nothing on update: an auth block that carries no
// credential inherits the stored one (see pkg/utils/credential_inheritance.go),
// which is what keeps read-modify-write working.
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

	case string(api.AgentConfigurationKindAgent):
		agentConfig, err := rematerializeAgentConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
		if err != nil {
			return nil, err
		}
		return buildResourceResponseFromStored(agentConfig, cfg), nil

	default:
		// Kinds whose schema has no upstream auth block. A credential-carrying
		// field added to one of these needs a case above, not this passthrough.
		return buildResourceResponseFromStored(cfg.SourceConfiguration, cfg), nil
	}
}

// redactAuthCredential clears both credential mechanisms from one auth block.
// Every redaction path below funnels through it, so a newly added auth-bearing
// field cannot end up with a partial redaction written somewhere else.
//
// nil, not empty: both fields carry omitempty, so each is absent from the
// response rather than present-but-blank.
func redactAuthCredential(value **string, policyParams **map[string]interface{}) {
	*value = nil
	*policyParams = nil
}

// redactLLMProviderCredentials clears the upstream credential from an LLM
// provider configuration bound for a response body.
func redactLLMProviderCredentials(cfg *api.LLMProviderConfiguration) {
	if cfg == nil || cfg.Spec.Upstream.Auth == nil {
		return
	}
	redactAuthCredential(&cfg.Spec.Upstream.Auth.Value, &cfg.Spec.Upstream.Auth.PolicyParams)
}

// redactLLMProxyCredentials clears the upstream credentials from an LLM proxy
// configuration bound for a response body — the primary provider's auth, every
// additionalProviders[] entry's auth, and every providers[] entry's auth in the
// canonical shape. A credential must not survive a read in either shape.
func redactLLMProxyCredentials(cfg *api.LLMProxyConfiguration) {
	if cfg == nil {
		return
	}
	if cfg.Spec.Provider != nil && cfg.Spec.Provider.Auth != nil {
		redactAuthCredential(&cfg.Spec.Provider.Auth.Value, &cfg.Spec.Provider.Auth.PolicyParams)
	}
	if cfg.Spec.AdditionalProviders != nil {
		additional := *cfg.Spec.AdditionalProviders
		for i := range additional {
			if additional[i].Auth != nil {
				redactAuthCredential(&additional[i].Auth.Value, &additional[i].Auth.PolicyParams)
			}
		}
	}
	if cfg.Spec.Providers != nil {
		entries := *cfg.Spec.Providers
		for i := range entries {
			if entries[i].Auth != nil {
				redactAuthCredential(&entries[i].Auth.Value, &entries[i].Auth.PolicyParams)
			}
		}
	}
}

// redactMCPProxyCredentials clears the upstream credential from an MCP proxy
// configuration bound for a response body.
func redactMCPProxyCredentials(cfg *api.MCPProxyConfiguration) {
	if cfg == nil || cfg.Spec.Upstream.Auth == nil {
		return
	}
	redactAuthCredential(&cfg.Spec.Upstream.Auth.Value, &cfg.Spec.Upstream.Auth.PolicyParams)
}

// redactAgentCredentials clears the upstream credential from an Agent
// configuration bound for a response body.
func redactAgentCredentials(cfg *api.AgentConfiguration) {
	if cfg == nil || cfg.Spec.Upstream.Auth == nil {
		return
	}
	redactAuthCredential(&cfg.Spec.Upstream.Auth.Value, &cfg.Spec.Upstream.Auth.PolicyParams)
}
