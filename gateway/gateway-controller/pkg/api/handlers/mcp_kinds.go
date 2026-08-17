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

package handlers

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// mcpManifestContentType is the content type every manifest submitted through an
// MCP tool is parsed as. The YAML parser accepts JSON, so this covers both.
const mcpManifestContentType = "application/yaml"

// listedResource is one row of a list result. Resource carries the full
// k8s-shaped body; the flat fields let list_resources render a compact
// cross-kind inventory without re-parsing it.
type listedResource struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version,omitempty"`
	State       string `json:"state,omitempty"`
	Resource    any    `json:"-"`
}

// kindClass splits the registry along the one line the write tools care about:
// routable kinds take traffic and belong to deploy_api / undeploy_api; config
// kinds support them and belong to apply_config / delete_config. Read tools
// accept both.
type kindClass int

const (
	classAny      kindClass = iota // list_resources, get_resource
	classRoutable                  // deploy_api, undeploy_api
	classConfig                    // apply_config, delete_config
)

// kindOps adapts one artifact kind onto the verbs the six tools expose.
type kindOps struct {
	Kind     string
	Routable bool

	// Collection is this kind's management REST collection path
	// ("/rest-apis"). It is the authorization anchor: an MCP call is authorized
	// as the equivalent REST operation on this path, so the role map in
	// cmd/controller/main.go stays the single source of truth for both
	// surfaces. See mcp_authz.go.
	Collection string

	Create func(manifest []byte, correlationID string, log *slog.Logger) (any, error)
	Update func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error)
	Delete func(handle, correlationID string, log *slog.Logger) error
	Get    func(handle string) (any, error)

	// List takes no filter arguments by design: it returns every resource of the
	// kind and the caller narrows the set. Server-side filters would have to be
	// exact matches, which fail silently when a model guesses a value slightly
	// wrong; returning the full summarised set degrades gracefully instead.
	List func() ([]listedResource, error)
}

// kindAliases maps a normalised (lowercased, separator-stripped) kind string to
// its canonical form. Models are inconsistent about casing and separators, so
// "rest_api", "REST-API" and "RestApi" all settle on one value before reaching
// any service or database query.
var kindAliases = map[string]string{
	"restapi":             models.KindRestApi,
	"api":                 models.KindRestApi,
	"mcp":                 models.KindMcp,
	"mcpproxy":            models.KindMcp,
	"llmproxy":            models.KindLlmProxy,
	"llmprovider":         models.KindLlmProvider,
	"llmprovidertemplate": models.KindLlmProviderTemplate,
}

var kindSeparatorStripper = strings.NewReplacer("-", "", "_", "", " ", "")

// normalizeKind resolves any accepted spelling of a kind to its canonical value.
func normalizeKind(raw string) (string, error) {
	key := strings.ToLower(kindSeparatorStripper.Replace(strings.TrimSpace(raw)))
	canonical, ok := kindAliases[key]
	if !ok {
		return "", fmt.Errorf("unknown kind %q; supported kinds: %s",
			raw, strings.Join(canonicalKinds(), ", "))
	}
	return canonical, nil
}

// canonicalKinds returns every canonical kind name, sorted, for error messages
// and for the cross-kind inventory.
func canonicalKinds() []string {
	seen := make(map[string]bool, len(kindAliases))
	out := make([]string, 0, len(kindAliases))
	for _, v := range kindAliases {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// manifestEnvelope is the minimum read from a manifest to route it. Every kind
// carries apiVersion/kind/metadata, so the kind never has to be passed
// alongside the body.
type manifestEnvelope struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

// readManifestEnvelope extracts kind and metadata.name. JSON is valid YAML, so
// this handles both content types the services accept. Everything beyond these
// two fields is left to the service layer to parse and validate.
//
// This function is called twice per write call — once by the authorization gate
// and once by the tool — deliberately: both decisions must be made from the
// same bytes by the same code, or they can disagree.
func readManifestEnvelope(manifest []byte) (manifestEnvelope, error) {
	var env manifestEnvelope
	if err := yaml.Unmarshal(manifest, &env); err != nil {
		return env, fmt.Errorf("manifest is not valid YAML or JSON: %w", err)
	}
	if strings.TrimSpace(env.Kind) == "" {
		return env, fmt.Errorf(
			`manifest has no "kind" field; every manifest must declare one of: %s`,
			strings.Join(canonicalKinds(), ", "))
	}
	return env, nil
}

// buildKindRegistry wires every supported kind to its service calls. Called once
// from newMcpHandler.
func (h *McpHandler) buildKindRegistry() map[string]*kindOps {
	registry := make(map[string]*kindOps)
	for _, ops := range []*kindOps{
		h.restAPIOps(),
		h.mcpProxyOps(),
		h.llmProxyOps(),
		h.llmProviderOps(),
		h.llmProviderTemplateOps(),
	} {
		registry[ops.Kind] = ops
	}
	return registry
}

// resolveKind looks up a kind's operations, rejecting unknown kinds and kinds
// outside the requested class. A rejection names the tool that does own the
// kind, so the caller can correct itself without another round trip.
func (h *McpHandler) resolveKind(raw string, class kindClass) (*kindOps, error) {
	kind, err := normalizeKind(raw)
	if err != nil {
		return nil, err
	}
	ops, ok := h.kinds[kind]
	if !ok {
		return nil, fmt.Errorf("kind %q is not supported by this gateway", kind)
	}
	switch {
	case class == classRoutable && !ops.Routable:
		return nil, fmt.Errorf(
			"kind %q is a supporting configuration, not a routable resource; use apply_config or delete_config for it. deploy_api and undeploy_api accept: %s",
			kind, strings.Join(h.kindsInClass(classRoutable), ", "))
	case class == classConfig && ops.Routable:
		return nil, fmt.Errorf(
			"kind %q is a routable resource, not a supporting configuration; use deploy_api or undeploy_api for it. apply_config and delete_config accept: %s",
			kind, strings.Join(h.kindsInClass(classConfig), ", "))
	}
	return ops, nil
}

// kindsInClass lists the kinds belonging to a class, sorted, for error messages.
func (h *McpHandler) kindsInClass(class kindClass) []string {
	out := make([]string, 0, len(h.kinds))
	for kind, ops := range h.kinds {
		if class == classAny || (class == classRoutable) == ops.Routable {
			out = append(out, kind)
		}
	}
	sort.Strings(out)
	return out
}

// Per-kind adapters
// Each calls the same service the corresponding REST
// handler calls, so parsing, validation and deployment behave identically
// whether a resource arrives over HTTP or over MCP.

func (h *McpHandler) restAPIOps() *kindOps {
	return &kindOps{
		Kind:       models.KindRestApi,
		Routable:   true,
		Collection: "/rest-apis",

		Create: func(manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.restAPIService.Create(restapi.CreateParams{
				Body:          manifest,
				ContentType:   mcpManifestContentType,
				Kind:          models.KindRestApi,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildResourceResponseFromStored(result.StoredConfig.SourceConfiguration, result.StoredConfig), nil
		},

		Update: func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.restAPIService.Update(restapi.UpdateParams{
				Handle:        handle,
				Body:          manifest,
				ContentType:   mcpManifestContentType,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildResourceResponseFromStored(result.Config.SourceConfiguration, result.Config), nil
		},

		Delete: func(handle, correlationID string, log *slog.Logger) error {
			result, err := h.restAPIService.Delete(restapi.DeleteParams{
				Handle:        handle,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return err
			}
			h.notifyUndeploy(result.Config, log)
			return nil
		},

		Get: func(handle string) (any, error) {
			result, err := h.restAPIService.GetByHandle(handle)
			if err != nil {
				return nil, err
			}
			return buildResourceResponseFromStored(result.Config.SourceConfiguration, result.Config), nil
		},

		List: func() ([]listedResource, error) {
			// Empty params: every filter field nil means "return everything".
			result, err := h.restAPIService.List(api.ListRestAPIsParams{})
			if err != nil {
				return nil, err
			}
			rows := make([]listedResource, 0, len(result.Items))
			for _, cfg := range result.Items {
				rows = append(rows, storedConfigRow(cfg,
					buildResourceResponseFromStored(cfg.SourceConfiguration, cfg)))
			}
			return rows, nil
		},
	}
}

func (h *McpHandler) mcpProxyOps() *kindOps {
	return &kindOps{
		Kind:       models.KindMcp,
		Routable:   true,
		Collection: "/mcp-proxies",

		Create: func(manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.mcpDeploymentService.CreateMCPProxy(utils.MCPDeploymentParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				Origin:        models.OriginGatewayAPI,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return h.mcpProxyBody(log, result.StoredConfig)
		},

		Update: func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			updated, err := h.mcpDeploymentService.UpdateMCPProxy(handle, utils.MCPDeploymentParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				Origin:        models.OriginGatewayAPI,
				CorrelationID: correlationID,
				Logger:        log,
			}, log)
			if err != nil {
				return nil, err
			}
			return h.mcpProxyBody(log, updated)
		},

		Delete: func(handle, correlationID string, log *slog.Logger) error {
			cfg, err := h.mcpDeploymentService.DeleteMCPProxy(handle, correlationID, log)
			if err != nil {
				return err
			}
			h.notifyUndeploy(cfg, log)
			return nil
		},

		Get: func(handle string) (any, error) {
			cfg, err := h.mcpDeploymentService.GetMCPProxyByHandle(handle)
			if err != nil {
				return nil, err
			}
			return h.mcpProxyBody(h.logger, cfg)
		},

		List: func() ([]listedResource, error) {
			configs, err := h.mcpDeploymentService.ListMCPProxies()
			if err != nil {
				return nil, err
			}
			rows := make([]listedResource, 0, len(configs))
			for _, cfg := range configs {
				body, err := h.mcpProxyBody(h.logger, cfg)
				if err != nil {
					return nil, err
				}
				rows = append(rows, storedConfigRow(cfg, body))
			}
			return rows, nil
		},
	}
}

func (h *McpHandler) llmProxyOps() *kindOps {
	return &kindOps{
		Kind:       models.KindLlmProxy,
		Routable:   true,
		Collection: "/llm-proxies",

		Create: func(manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.llmDeploymentService.CreateLLMProxy(utils.LLMDeploymentParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				Origin:        models.OriginGatewayAPI,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return h.llmProxyBody(log, result.StoredConfig)
		},

		Update: func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.llmDeploymentService.UpdateLLMProxy(handle, utils.LLMDeploymentParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				Origin:        models.OriginGatewayAPI,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return h.llmProxyBody(log, result.StoredConfig)
		},

		Delete: func(handle, correlationID string, log *slog.Logger) error {
			cfg, err := h.llmDeploymentService.DeleteLLMProxy(handle, correlationID, log)
			if err != nil {
				return err
			}
			h.notifyUndeploy(cfg, log)
			return nil
		},

		Get: func(handle string) (any, error) {
			cfg, err := h.llmDeploymentService.GetLLMProxyByHandle(handle)
			if err != nil {
				return nil, err
			}
			return h.llmProxyBody(h.logger, cfg)
		},

		List: func() ([]listedResource, error) {
			configs := h.llmDeploymentService.ListLLMProxies(api.ListLLMProxiesParams{})
			rows := make([]listedResource, 0, len(configs))
			for _, cfg := range configs {
				body, err := h.llmProxyBody(h.logger, cfg)
				if err != nil {
					return nil, err
				}
				rows = append(rows, storedConfigRow(cfg, body))
			}
			return rows, nil
		},
	}
}

func (h *McpHandler) llmProviderOps() *kindOps {
	return &kindOps{
		Kind:       models.KindLlmProvider,
		Routable:   true,
		Collection: "/llm-providers",

		Create: func(manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.llmDeploymentService.CreateLLMProvider(utils.LLMDeploymentParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				Origin:        models.OriginGatewayAPI,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildResourceResponseFromStored(
				result.StoredConfig.SourceConfiguration, result.StoredConfig), nil
		},

		Update: func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.llmDeploymentService.UpdateLLMProvider(handle, utils.LLMDeploymentParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				Origin:        models.OriginGatewayAPI,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildResourceResponseFromStored(
				result.StoredConfig.SourceConfiguration, result.StoredConfig), nil
		},

		Delete: func(handle, correlationID string, log *slog.Logger) error {
			cfg, err := h.llmDeploymentService.DeleteLLMProvider(handle, correlationID, log)
			if err != nil {
				return err
			}
			h.notifyUndeploy(cfg, log)
			return nil
		},

		Get: func(handle string) (any, error) {
			cfg, err := h.llmDeploymentService.GetLLMProviderByHandle(handle)
			if err != nil {
				return nil, err
			}
			return buildResourceResponseFromStored(cfg.SourceConfiguration, cfg), nil
		},

		List: func() ([]listedResource, error) {
			configs := h.llmDeploymentService.ListLLMProviders(api.ListLLMProvidersParams{})
			rows := make([]listedResource, 0, len(configs))
			for _, cfg := range configs {
				rows = append(rows, storedConfigRow(cfg,
					buildResourceResponseFromStored(cfg.SourceConfiguration, cfg)))
			}
			return rows, nil
		},
	}
}

// llmProviderTemplateOps is non-routable: templates are supporting config, so
// deploy_api / undeploy_api reject them and apply_config / delete_config own
// their write path instead.
func (h *McpHandler) llmProviderTemplateOps() *kindOps {
	return &kindOps{
		Kind:       models.KindLlmProviderTemplate,
		Routable:   false,
		Collection: "/llm-provider-templates",

		Create: func(manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			stored, err := h.llmDeploymentService.CreateLLMProviderTemplate(utils.LLMTemplateParams{
				Spec:          manifest,
				ContentType:   mcpManifestContentType,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildTemplateResourceResponse(stored), nil
		},

		Update: func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			updated, err := h.llmDeploymentService.UpdateLLMProviderTemplate(handle, utils.LLMTemplateParams{
				Spec:          manifest,
				ContentType:   mcpManifestContentType,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildTemplateResourceResponse(updated), nil
		},

		Delete: func(handle, correlationID string, log *slog.Logger) error {
			_, err := h.llmDeploymentService.DeleteLLMProviderTemplate(handle, correlationID, log)
			return err
		},

		Get: func(handle string) (any, error) {
			tmpl, err := h.llmDeploymentService.GetLLMProviderTemplateByHandle(handle)
			if err != nil {
				return nil, err
			}
			return buildTemplateResourceResponse(tmpl), nil
		},

		List: func() ([]listedResource, error) {
			// nil displayName: return every template.
			templates := h.llmDeploymentService.ListLLMProviderTemplates(nil)
			rows := make([]listedResource, 0, len(templates))
			for _, tmpl := range templates {
				rows = append(rows, listedResource{
					ID:          tmpl.GetHandle(),
					DisplayName: tmpl.Configuration.Spec.DisplayName,
					Version:     derefString(tmpl.Configuration.Spec.Version),
					Resource:    buildTemplateResourceResponse(tmpl),
				})
			}
			return rows, nil
		},
	}
}

// Shared adapter helpers

// mcpProxyBody re-materialises a stored MCP proxy into the typed k8s-shaped
// response body GET /mcp-proxies/{id} returns.
func (h *McpHandler) mcpProxyBody(log *slog.Logger, cfg *models.StoredConfig) (any, error) {
	proxy, err := rematerializeMCPProxyConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
	if err != nil {
		return nil, fmt.Errorf("failed to read stored MCP proxy configuration")
	}
	return buildResourceResponseFromStored(proxy, cfg), nil
}

// llmProxyBody is the LLM proxy counterpart of mcpProxyBody.
func (h *McpHandler) llmProxyBody(log *slog.Logger, cfg *models.StoredConfig) (any, error) {
	proxy, err := rematerializeLLMProxyConfig(log, cfg.UUID, cfg.DisplayName, cfg.SourceConfiguration)
	if err != nil {
		return nil, fmt.Errorf("failed to read stored LLM proxy configuration")
	}
	return buildResourceResponseFromStored(proxy, cfg), nil
}

// notifyUndeploy forwards a delete to the control plane (DP-&gt;CP) exactly as the
// REST handlers do, so an artifact removed through MCP is marked undeployed
// upstream rather than left stale. No-op when no control plane is configured.
func (h *McpHandler) notifyUndeploy(cfg *models.StoredConfig, log *slog.Logger) {
	if h.pushArtifactUndeploy != nil && cfg != nil {
		h.pushArtifactUndeploy(cfg, log)
	}
}

// storedConfigRow builds a list row from a StoredConfig plus its rendered body.
func storedConfigRow(cfg *models.StoredConfig, body any) listedResource {
	return listedResource{
		ID:          cfg.Handle,
		DisplayName: cfg.DisplayName,
		Version:     cfg.Version,
		State:       string(cfg.DesiredState),
		Resource:    body,
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
