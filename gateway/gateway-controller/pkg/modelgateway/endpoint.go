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

package modelgateway

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// TransformerTypeNone is the transformer type that attaches no transformer: the
// provider already speaks the endpoint's inbound format.
const TransformerTypeNone = "none"

// Placeholders a transformer entry may use. They are filled from gateway
// configuration — the provider's spec and the target — and never from a request.
const (
	PlaceholderUpstreamModel = "{upstreamModel}"
	placeholderAPIVersion    = "apiVersion"
)

// apiVersionPlaceholder matches `{apiVersion}` and `{apiVersion:<default>}`.
var apiVersionPlaceholder = regexp.MustCompile(`\{apiVersion(?::([^{}]*))?\}`)

// Endpoint is what binding a Model to one model endpoint needs to know about the
// endpoint.
type Endpoint struct {
	Handle string

	// InboundTemplate is the endpoint's inbound template handle, and
	// InboundGroup that template's group, when it has one. A provider on either
	// speaks the endpoint's format and needs no transformer.
	InboundTemplate string
	InboundGroup    string

	// Transformers are the endpoint's spec.transformers entries.
	Transformers []api.ModelEndpointTransformer

	// DispatchPaths are the endpoint's dispatch operations, relative to its
	// context. `{model}` names the model's path segment. A provider that passes
	// through is called on these paths.
	DispatchPaths []string
}

// Transformer is the transformer one provider gets on one endpoint.
type Transformer struct {
	// PolicyName and Version (major only) name the installed policy.
	PolicyName string
	Version    string

	// Params are the entry's parameters with their placeholders filled. providerId
	// is added by the compiler, which owns the provider's upstream name.
	Params map[string]interface{}
}

// Binding is a Model bound to one endpoint: the transformer each of its providers
// gets there.
type Binding struct {
	// Transformers maps a provider handle to its transformer; a provider mapped to
	// nil passes through.
	Transformers map[string]*Transformer

	// Warnings are things that do not stop the Model being served but that an
	// operator should hear about, such as a routed target whose paths could not be
	// checked.
	Warnings []string
}

// Bind decides how each of a resolved Model's providers is reached on one
// endpoint, and checks what can be checked there.
//
// For each provider the transformer is, in order:
//
//  1. the entry naming the provider by handle;
//  2. the entry naming the provider's template, by handle or group;
//  3. none, when the provider's template is the endpoint's inbound template;
//  4. otherwise the endpoint cannot serve the Model, and Bind fails naming the
//     provider and its template.
//
// A routed target's paths must also pass its provider route's access control:
// the paths the matched entry declares, or the endpoint's own paths for a provider
// that passes through. An entry with a transformer and no declared paths is
// accepted with a warning, as an LLM proxy's transformer is never checked.
func Bind(m *Model, ep Endpoint) (*Binding, error) {
	var errs Errors
	add := func(field, format string, args ...interface{}) {
		errs = append(errs, config.ValidationError{Field: field, Message: fmt.Sprintf(format, args...)})
	}

	b := &Binding{Transformers: make(map[string]*Transformer)}
	for _, handle := range m.ProviderHandles() {
		target := firstTarget(m, handle)
		entry, passthrough := selectEntry(target, ep)
		if entry == nil && !passthrough {
			// The format is named because the shipped transformers all read OpenAI
			// requests: on an endpoint of another format, the entry must name a
			// transformer that reads that format.
			add("spec.transformers", "endpoint '%s' (inbound format '%s') has no transformer for provider '%s' "+
				"(template '%s'); add a spec.transformers entry on the endpoint, for the provider or its template, "+
				"with a transformer that reads '%s' requests, or remove the endpoint from the Model",
				ep.Handle, ep.InboundTemplate, handle, templateHandle(target), ep.InboundTemplate)
			continue
		}

		var tr *Transformer
		if entry != nil && strings.TrimSpace(entry.Type) != TransformerTypeNone {
			params, err := fillParams(entry.Params, target.Provider)
			if err != nil {
				add("spec.transformers", "endpoint '%s', provider '%s': %s", ep.Handle, handle, err.Error())
				continue
			}
			version := ""
			if entry.Version != nil {
				version = strings.TrimSpace(*entry.Version)
			}
			tr = &Transformer{PolicyName: strings.TrimSpace(entry.Type), Version: version, Params: params}
		}
		b.Transformers[handle] = tr

		// The paths each routed target on this provider is called on.
		for i := range m.Targets {
			t := &m.Targets[i]
			if t.ProviderHandle != handle || t.Mode != ModeRoute {
				continue
			}
			paths, declared, err := targetPaths(entry, tr, t, ep)
			if err != nil {
				add("spec.transformers", "endpoint '%s', target '%s': %s", ep.Handle, t.Name, err.Error())
				continue
			}
			if !declared {
				b.Warnings = append(b.Warnings, fmt.Sprintf("target '%s' on provider '%s' is reached through the "+
					"provider's route, and transformer '%s' declares no upstreamPaths, so its paths are not checked "+
					"against the route's accessControl", t.Name, handle, tr.PolicyName))
				continue
			}
			for _, path := range paths {
				if !accessControlAllows(t.Provider.Config.Spec.AccessControl, "POST", path) {
					add("spec.targets", "provider '%s' does not allow POST %s through its route (accessControl), "+
						"which target '%s' needs on endpoint '%s'", handle, path, t.Name, ep.Handle)
				}
			}
		}
	}

	if len(errs) > 0 {
		return nil, errs
	}
	return b, nil
}

// selectEntry picks the transformer entry for a target's provider, or reports
// that the provider passes through with none.
func selectEntry(t *Target, ep Endpoint) (*api.ModelEndpointTransformer, bool) {
	for i := range ep.Transformers {
		e := &ep.Transformers[i]
		if e.Provider != nil && strings.TrimSpace(*e.Provider) == t.ProviderHandle {
			return e, false
		}
	}
	for i := range ep.Transformers {
		e := &ep.Transformers[i]
		if e.Template != nil && TemplateMatches(t.Template, strings.TrimSpace(*e.Template)) {
			return e, false
		}
	}
	if TemplateMatches(t.Template, ep.InboundTemplate) ||
		(ep.InboundGroup != "" && t.Template != nil && strings.TrimSpace(t.Template.GetGroupID()) == ep.InboundGroup) {
		return nil, true
	}
	return nil, false
}

// TemplateMatches reports whether a provider template is the named template: by
// handle, or by group, so a customised copy of an out-of-box template (same group,
// another handle) matches the same entries. An out-of-box group is also matched by
// its short name ("anthropic" for "wso2-anthropic").
func TemplateMatches(tmpl *models.StoredLLMProviderTemplate, name string) bool {
	if tmpl == nil || name == "" {
		return false
	}
	handle := strings.TrimSpace(tmpl.GetHandle())
	group := strings.TrimSpace(tmpl.GetGroupID())
	return handle == name || (group != "" && (group == name || group == "wso2-"+name))
}

// targetPaths returns the paths a routed target is called on, and whether they are
// known. For a provider that passes through, they are the endpoint's dispatch
// paths with the model segment filled; for a transformer, the entry's declared
// paths with their placeholders filled.
func targetPaths(entry *api.ModelEndpointTransformer, tr *Transformer, t *Target, ep Endpoint) ([]string, bool, error) {
	if entry != nil && entry.UpstreamPaths != nil && len(*entry.UpstreamPaths) > 0 {
		out := make([]string, 0, len(*entry.UpstreamPaths))
		for _, p := range *entry.UpstreamPaths {
			filled, err := fillString(p, t.Provider)
			if err != nil {
				return nil, false, err
			}
			out = append(out, strings.ReplaceAll(filled, PlaceholderUpstreamModel, url.PathEscape(t.UpstreamModel)))
		}
		return out, true, nil
	}
	if tr != nil {
		return nil, false, nil
	}
	out := make([]string, 0, len(ep.DispatchPaths))
	for _, p := range ep.DispatchPaths {
		out = append(out, strings.ReplaceAll(p, "{model}", url.PathEscape(t.UpstreamModel)))
	}
	return out, true, nil
}

// fillParams copies a transformer entry's params with every placeholder in every
// string value filled.
func fillParams(params *map[string]interface{}, provider *Provider) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if params == nil {
		return out, nil
	}
	for k, v := range *params {
		filled, err := fillValue(v, provider)
		if err != nil {
			return nil, fmt.Errorf("params.%s: %w", k, err)
		}
		out[k] = filled
	}
	return out, nil
}

func fillValue(v interface{}, provider *Provider) (interface{}, error) {
	switch val := v.(type) {
	case string:
		return fillString(val, provider)
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, inner := range val {
			filled, err := fillValue(inner, provider)
			if err != nil {
				return nil, err
			}
			out[k] = filled
		}
		return out, nil
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, inner := range val {
			filled, err := fillValue(inner, provider)
			if err != nil {
				return nil, err
			}
			out[i] = filled
		}
		return out, nil
	default:
		return v, nil
	}
}

// fillString fills `{apiVersion}` and `{apiVersion:<default>}` from the provider's
// spec.upstream.apiVersion. `{apiVersion}` with no default needs the provider to
// state one: some provider APIs (Azure OpenAI) refuse a request without it.
func fillString(s string, provider *Provider) (string, error) {
	if !strings.Contains(s, "{"+placeholderAPIVersion) {
		return s, nil
	}
	version := ""
	name := ""
	if provider != nil {
		name = provider.Config.Metadata.Name
		if v := provider.Config.Spec.Upstream.ApiVersion; v != nil {
			version = strings.TrimSpace(*v)
		}
	}
	var missing bool
	out := apiVersionPlaceholder.ReplaceAllStringFunc(s, func(match string) string {
		if version != "" {
			return version
		}
		sub := apiVersionPlaceholder.FindStringSubmatch(match)
		if len(sub) == 2 && strings.Contains(match, ":") {
			return sub[1]
		}
		missing = true
		return match
	})
	if missing {
		return "", fmt.Errorf("provider '%s' must set spec.upstream.apiVersion: its transformer entry uses "+
			"{apiVersion} with no default", name)
	}
	return out, nil
}

func firstTarget(m *Model, provider string) *Target {
	for i := range m.Targets {
		if m.Targets[i].ProviderHandle == provider {
			return &m.Targets[i]
		}
	}
	return nil
}

func templateHandle(t *Target) string {
	if t == nil || t.Template == nil {
		return ""
	}
	return t.Template.GetHandle()
}
