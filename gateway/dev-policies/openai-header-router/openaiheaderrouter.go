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

package openaiheaderrouter

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

const (
	PolicyName = "openai-header-router"

	// DefaultHeaderName is the request header consulted when the operator
	// does not override "headerName" in the policy parameters.
	DefaultHeaderName = "x-provider"

	// MetadataKeySelectedProvider is the SharedContext.Metadata key the
	// router writes after picking a provider. Downstream consumer
	// policies read this key to decide whether to run.
	MetadataKeySelectedProvider = "selected_provider"
)

// HeaderMapping is a single header-value → provider-id rule.
type HeaderMapping struct {
	// HeaderValue is matched against the incoming header value
	// case-insensitively after trimming whitespace.
	HeaderValue string
	// Provider is the id published when this mapping wins.
	Provider string
}

// PolicyParams is the parsed, validated configuration for a single
// instantiation of the policy.
type PolicyParams struct {
	// HeaderName is the request header read for selection. Comparison
	// against incoming headers is case-insensitive.
	HeaderName string

	// DefaultProvider is selected when the header is missing, empty, or
	// no mapping matches. parseParams enforces it is non-empty.
	DefaultProvider string

	// Mappings is checked in order; the first match wins. Order is the
	// order the operator declared in the policy YAML.
	Mappings []HeaderMapping
}

// RouterPolicy is the Policy implementation registered with the gateway
// runtime. It is stateless across requests — selection depends only on the
// incoming header and the immutable params.
type RouterPolicy struct {
	params PolicyParams
}

// GetPolicy is the v1alpha2 factory entry point invoked by the gateway
// kernel for each policy instantiation on a route.
func GetPolicy(
	_ policy.PolicyMetadata,
	rawParams map[string]interface{},
) (policy.Policy, error) {
	parsed, err := parseParams(rawParams)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid params: %w", PolicyName, err)
	}
	return &RouterPolicy{params: parsed}, nil
}

// Mode declares the phases this policy participates in. Selection runs in the
// request-header phase so header-phase consumers observe the choice — notably
// the proxy->provider upstream auth injection (a set-headers policy gated on
// selected_provider), which executes in the header phase and would otherwise
// evaluate its gate before the selection exists. The body phase repeats the
// publish as an idempotent fallback; both phases share one *SharedContext, so
// a value written in the header phase is visible to body-phase consumers
// (e.g. the openai-to-* translators).
func (p *RouterPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequestHeaders performs provider selection during the request-header phase
// so the selection is published before header-phase consumers (upstream auth
// injection) evaluate their selected_provider gate.
func (p *RouterPolicy) OnRequestHeaders(
	_ context.Context,
	reqCtx *policy.RequestHeaderContext,
	_ map[string]interface{},
) policy.RequestHeaderAction {
	if reqCtx.SharedContext == nil {
		return policy.UpstreamRequestHeaderModifications{}
	}
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = map[string]interface{}{}
	}
	p.publishSelection(reqCtx.Metadata, reqCtx.Headers)
	return policy.UpstreamRequestHeaderModifications{}
}

// OnRequestBody republishes the selection in the body phase as an idempotent
// fallback (e.g. if an upstream policy cleared metadata between phases). If
// selected_provider is already set, publishSelection leaves it untouched.
func (p *RouterPolicy) OnRequestBody(
	_ context.Context,
	reqCtx *policy.RequestContext,
	_ map[string]interface{},
) policy.RequestAction {
	// SharedContext is required to publish the selection downstream.
	if reqCtx.SharedContext == nil {
		return policy.UpstreamRequestModifications{}
	}
	if reqCtx.Metadata == nil {
		reqCtx.Metadata = map[string]interface{}{}
	}
	p.publishSelection(reqCtx.Metadata, reqCtx.Headers)
	return policy.UpstreamRequestModifications{}
}

// publishSelection reads the configured header, picks the provider via the
// mappings (falling back to defaultProvider), and writes the result into
// metadata. If selected_provider is already set by an upstream policy or an
// earlier phase, it is left untouched. Shared by the header and body phases.
func (p *RouterPolicy) publishSelection(metadata map[string]interface{}, headers *policy.Headers) {
	if existing, ok := metadata[MetadataKeySelectedProvider].(string); ok && existing != "" {
		return
	}

	headerValue := readHeader(headers, p.params.HeaderName)
	provider, source := p.selectProvider(headerValue)
	metadata[MetadataKeySelectedProvider] = provider

	slog.Debug(PolicyName+": provider selected",
		"headerName", p.params.HeaderName, "headerValue", headerValue,
		"provider", provider, "source", source)
}

// selectProvider picks a provider id for the given header value and
// returns both the chosen provider and a short tag describing why it was
// chosen (used for telemetry / logging).
//
//   - "header"  — a mapping matched the header value.
//   - "default" — fell back to defaultProvider (header missing, empty, or
//     no mapping matched).
func (p *RouterPolicy) selectProvider(headerValue string) (string, string) {
	if headerValue != "" {
		for _, m := range p.params.Mappings {
			if strings.EqualFold(m.HeaderValue, headerValue) {
				return m.Provider, "header"
			}
		}
	}
	return p.params.DefaultProvider, "default"
}

// readHeader extracts the configured header from the request, trimmed of
// surrounding whitespace. Returns "" when the header is missing.
func readHeader(headers *policy.Headers, name string) string {
	if headers == nil {
		return ""
	}
	values := headers.Get(name)
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

// parseParams validates and copies the raw parameter map into PolicyParams.
// All shape and value validation lives here so OnRequestBody can assume
// the configuration is sound.
func parseParams(params map[string]interface{}) (PolicyParams, error) {
	result := PolicyParams{
		HeaderName: DefaultHeaderName,
	}

	if v, err := optionalString(params, "headerName"); err != nil {
		return result, err
	} else if v != "" {
		result.HeaderName = v
	}

	defaultProvider, err := optionalString(params, "defaultProvider")
	if err != nil {
		return result, err
	}
	if defaultProvider == "" {
		return result, fmt.Errorf("'defaultProvider' is required")
	}
	result.DefaultProvider = defaultProvider

	rawMappings, ok := params["mappings"]
	if !ok || rawMappings == nil {
		return result, fmt.Errorf("'mappings' is required")
	}
	list, ok := rawMappings.([]interface{})
	if !ok {
		return result, fmt.Errorf("'mappings' must be an array")
	}
	if len(list) == 0 {
		return result, fmt.Errorf("'mappings' must contain at least one entry")
	}

	seenHeaderValues := map[string]struct{}{}
	for i, raw := range list {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			return result, fmt.Errorf("mappings[%d] must be an object", i)
		}
		headerValue, err := requiredEntryString(entry, "headerValue", i)
		if err != nil {
			return result, err
		}
		provider, err := requiredEntryString(entry, "provider", i)
		if err != nil {
			return result, err
		}

		// Reject duplicate header values up-front so operators get a
		// clear error instead of a silently-shadowed mapping.
		key := strings.ToLower(headerValue)
		if _, dup := seenHeaderValues[key]; dup {
			return result, fmt.Errorf("mappings[%d].headerValue %q is duplicated (case-insensitive)", i, headerValue)
		}
		seenHeaderValues[key] = struct{}{}

		result.Mappings = append(result.Mappings, HeaderMapping{
			HeaderValue: headerValue,
			Provider:    provider,
		})
	}

	return result, nil
}

// optionalString reads a string parameter, trims it, and returns "" when
// the key is absent or its value is null. It rejects non-string values.
func optionalString(params map[string]interface{}, key string) (string, error) {
	raw, ok := params[key]
	if !ok || raw == nil {
		return "", nil
	}
	v, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("'%s' must be a string", key)
	}
	return strings.TrimSpace(v), nil
}

// requiredEntryString reads a required string field from a mappings entry,
// trimming whitespace and producing a structured error message that points
// at the offending array index.
func requiredEntryString(entry map[string]interface{}, key string, idx int) (string, error) {
	raw, ok := entry[key]
	if !ok || raw == nil {
		return "", fmt.Errorf("mappings[%d].%s is required", idx, key)
	}
	v, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("mappings[%d].%s must be a string", idx, key)
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", fmt.Errorf("mappings[%d].%s must not be empty", idx, key)
	}
	return v, nil
}
