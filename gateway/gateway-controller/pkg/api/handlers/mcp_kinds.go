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

	commonmodels "github.com/wso2/api-platform/common/models"
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/secrets"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/service/restapi"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// mcpManifestContentType is the content type every manifest submitted through an
// MCP tool is parsed as. The YAML parser accepts JSON, so this covers both.
const mcpManifestContentType = "application/yaml"

// listedResource is one row of a list result. Resource carries the full
// k8s-shaped body; the flat fields let wso2_apip_gw_list_resources render a compact
// cross-kind inventory without re-parsing it.
type listedResource struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	Version     string `json:"version,omitempty"`
	State       string `json:"state,omitempty"`
	Resource    any    `json:"-"`
}

// kindClass splits the registry along the one line the write tools care about:
// routable kinds take traffic and belong to wso2_apip_gw_deploy_api /
// wso2_apip_gw_undeploy_api; config kinds support them and belong to
// wso2_apip_gw_apply_config / wso2_apip_gw_delete_config. Read tools accept
// both.
type kindClass int

const (
	classAny      kindClass = iota // wso2_apip_gw_list_resources, wso2_apip_gw_get_resource
	classRoutable                  // wso2_apip_gw_deploy_api, wso2_apip_gw_undeploy_api
	classConfig                    // wso2_apip_gw_apply_config, wso2_apip_gw_delete_config
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

	// Keys is non-nil only for kinds that bear API keys (RestApi, LlmProvider,
	// LlmProxy). The four api-key tools refuse any kind whose block is nil,
	// which is also what keeps a nil dereference impossible when the api-key
	// service was never wired.
	Keys *keyOps
}

// keyOps adapts one key-bearing kind onto the four api-key verbs. Every function
// takes the verified caller explicitly rather than reading it from a context:
// APIKeyService scopes each operation on caller.UserID, so making it a required
// argument means no call site can silently omit it.
type keyOps struct {
	Issue  func(parentID string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error)
	List   func(parentID string, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error)
	Rotate func(parentID, keyName string, req api.APIKeyRegenerationRequest, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error)
	// Update installs a caller-supplied key value under an existing name —
	// the external-key-injection operation behind PUT .../{apiKeyName}. It is
	// the second half of the rotate tool; see rotateIsInjection in mcp_tools.go
	// for how a call is routed to it rather than to Rotate.
	Update func(parentID, keyName string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error)
	Revoke func(parentID, keyName string, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error)
}

// apiKeyOps builds the api-key adapter for one kind. APIKeyService takes Kind as
// a plain string, so all three key-bearing kinds are this one parameterised
// block — there is no per-kind api-key code.
//
// Kind is always passed explicitly. The REST handlers omit it on three of five
// operations and lean on the service defaulting to RestApi; that implicit
// contract is not inherited here, and it would be wrong for the other two kinds.
func (h *McpHandler) apiKeyOps(kind string) *keyOps {
	if h.apiKeyService == nil {
		return nil
	}
	return &keyOps{
		Issue: func(parentID string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.apiKeyService.CreateAPIKey(utils.APIKeyCreationParams{
				Kind:          kind,
				Handle:        parentID,
				Request:       req,
				User:          caller,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return result.Response, nil
		},

		List: func(parentID string, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.apiKeyService.ListAPIKeys(utils.ListAPIKeyParams{
				Kind:          kind,
				Handle:        parentID,
				User:          caller,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			// Already masked by the service.
			return result.Response, nil
		},

		Rotate: func(parentID, keyName string, req api.APIKeyRegenerationRequest, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.apiKeyService.RegenerateAPIKey(utils.APIKeyRegenerationParams{
				Kind:          kind,
				Handle:        parentID,
				APIKeyName:    keyName,
				Request:       req,
				User:          caller,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return result.Response, nil
		},

		Update: func(parentID, keyName string, req api.APIKeyCreationRequest, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.apiKeyService.UpdateAPIKey(utils.APIKeyUpdateParams{
				Kind:          kind,
				Handle:        parentID,
				APIKeyName:    keyName,
				Request:       req,
				User:          caller,
				CorrelationID: correlationID,
				Logger:        log,
				// As on Revoke: explicit though it is the zero value, because
				// this flag suppresses the creator check and only the
				// platform-api event path is a pre-validated origin.
				TrustedOrigin: false,
			})
			if err != nil {
				return nil, err
			}
			// Masked, not plaintext — UpdateAPIKey returns the masked form.
			return result.Response, nil
		},

		Revoke: func(parentID, keyName string, caller *commonmodels.AuthContext, correlationID string, log *slog.Logger) (any, error) {
			result, err := h.apiKeyService.RevokeAPIKey(utils.APIKeyRevocationParams{
				Kind:          kind,
				Handle:        parentID,
				APIKeyName:    keyName,
				User:          caller,
				CorrelationID: correlationID,
				Logger:        log,
				// Explicit though it is the zero value: this flag is what
				// suppresses the creator check, and it must never be set on a
				// caller-originated request. Only the platform-api event path
				// (RevokeExternalAPIKeyFromEvent) is a pre-validated origin.
				TrustedOrigin: false,
			})
			if err != nil {
				return nil, err
			}
			return result.Response, nil
		},
	}
}

// resolveKeyBearing resolves a kind and asserts it bears API keys, so the four
// key tools reject Mcp, LlmProviderTemplate and Secret with a message naming the
// kinds that would have worked.
func (h *McpHandler) resolveKeyBearing(raw string) (*kindOps, error) {
	ops, err := h.resolveKind(raw, classAny)
	if err != nil {
		return nil, err
	}
	if ops.Keys == nil {
		return nil, fmt.Errorf(
			"kind %q does not have API keys; the API key tools accept: %s",
			ops.Kind, strings.Join(h.keyBearingKinds(), ", "))
	}
	return ops, nil
}

// keyBearingKinds lists the kinds with an api-key block, sorted, for error
// messages.
func (h *McpHandler) keyBearingKinds() []string {
	out := make([]string, 0, len(h.kinds))
	for kind, ops := range h.kinds {
		if ops.Keys != nil {
			out = append(out, kind)
		}
	}
	sort.Strings(out)
	return out
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
	"secret":              models.KindSecret,
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
//
// A constructor returns nil when the service backing that kind was not wired
// (secrets need an encryption provider, which not every deployment configures).
// Skipping the entry means the kind is simply not advertised, rather than a nil
// dereference the first time a cross-kind list touches it.
func (h *McpHandler) buildKindRegistry() map[string]*kindOps {
	registry := make(map[string]*kindOps)
	for _, ops := range []*kindOps{
		h.restAPIOps(),
		h.mcpProxyOps(),
		h.llmProxyOps(),
		h.llmProviderOps(),
		h.llmProviderTemplateOps(),
		h.secretOps(),
	} {
		if ops == nil {
			continue
		}
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
			"kind %q is a supporting configuration, not a routable resource; "+
				"use wso2_apip_gw_apply_config or wso2_apip_gw_delete_config for it. "+
				"wso2_apip_gw_deploy_api and wso2_apip_gw_undeploy_api accept: %s",
			kind, strings.Join(h.kindsInClass(classRoutable), ", "))
	case class == classConfig && ops.Routable:
		return nil, fmt.Errorf(
			"kind %q is a routable resource, not a supporting configuration; "+
				"use wso2_apip_gw_deploy_api or wso2_apip_gw_undeploy_api for it. "+
				"wso2_apip_gw_apply_config and wso2_apip_gw_delete_config accept: %s",
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
		Keys:       h.apiKeyOps(models.KindRestApi),

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
		Keys:       h.apiKeyOps(models.KindLlmProxy),

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
		Keys:       h.apiKeyOps(models.KindLlmProvider),

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
// wso2_apip_gw_deploy_api / wso2_apip_gw_undeploy_api reject them and
// wso2_apip_gw_apply_config / wso2_apip_gw_delete_config own their write path
// instead.
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

// secretOps is non-routable: a Secret is supporting config, so it belongs to
// wso2_apip_gw_apply_config / wso2_apip_gw_delete_config alongside
// LlmProviderTemplate.
//
// Returns nil when no secret service is wired (the gateway has no encryption
// provider configured), which drops the kind from the registry entirely.
func (h *McpHandler) secretOps() *kindOps {
	if h.secretService == nil {
		return nil
	}
	return &kindOps{
		Kind:       models.KindSecret,
		Routable:   false,
		Collection: "/secrets",

		Create: func(manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			// No handle argument: CreateSecret takes the handle from the
			// manifest's metadata.name, the same way the REST handler does.
			secret, err := h.secretService.CreateSecret(secrets.SecretParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildSecretResourceResponse(secret, false), nil
		},

		Update: func(handle string, manifest []byte, correlationID string, log *slog.Logger) (any, error) {
			secret, err := h.secretService.UpdateSecret(handle, secrets.SecretParams{
				Data:          manifest,
				ContentType:   mcpManifestContentType,
				CorrelationID: correlationID,
				Logger:        log,
			})
			if err != nil {
				return nil, err
			}
			return buildSecretResourceResponse(secret, false), nil
		},

		// SecretService.Delete takes no logger; the correlation ID is the only
		// context it records.
		Delete: func(handle, correlationID string, _ *slog.Logger) error {
			return h.secretService.Delete(handle, correlationID)
		},

		Get: func(handle string) (any, error) {
			// Empty correlation ID: it is only a log field on this method.
			secret, err := h.secretService.Get(handle, "")
			if err != nil {
				return nil, err
			}
			// includeValue=true matches GET /secrets/{id}
			// (secret_handler.go), which returns the plaintext so a caller can
			// read back what it wrote. MCP mirrors the REST API rather than
			// applying its own policy, so a secret read here carries the value
			// too — treat an MCP transcript as credential-bearing.
			//
			// Create and Update pass false, and List returns SecretMeta which
			// has no value field at all; both also match REST.
			return buildSecretResourceResponse(secret, true), nil
		},

		List: func() ([]listedResource, error) {
			metas, err := h.secretService.GetSecrets("")
			if err != nil {
				return nil, err
			}
			// Built by hand rather than via storedConfigRow: secrets are not
			// StoredConfigs and carry no version or deployment state.
			// SecretMeta structurally cannot hold a plaintext value.
			rows := make([]listedResource, 0, len(metas))
			for _, meta := range metas {
				rows = append(rows, listedResource{
					ID:          meta.Handle,
					DisplayName: meta.DisplayName,
					Resource:    buildSecretMetaResourceResponse(meta),
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

// notifyUndeploy forwards a delete to the control plane exactly as the
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
