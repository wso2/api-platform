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

package config

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// UpstreamNameMarker marks a string policy parameter whose value names one of the
// artifact's named upstreams: an upstreamDefinitions entry of a REST API or an LLM
// provider, or a provider (by alias when one is set) of an LLM proxy. A policy that
// routes a request by name (it sets the upstream name or selected_provider) declares
// it on that parameter, for example:
//
//	provider:
//	  type: string
//	  x-wso2-upstream-name: true
//
// The policy itself never learns which upstreams the artifact has, so a mistyped
// name would deploy cleanly and fail every request routed to it. The validator
// rejects it at deploy time instead, for every policy that declares the marker.
const UpstreamNameMarker = "x-wso2-upstream-name"

// policyParams is one attached policy's parameters and where they appear.
type policyParams struct {
	name, version string
	params        map[string]interface{}
	fieldPath     string
}

// upstreamDefinitionNames returns the names of an artifact's upstreamDefinitions.
func upstreamDefinitionNames(defs *[]api.UpstreamDefinition) []string {
	if defs == nil {
		return nil
	}
	names := make([]string, 0, len(*defs))
	for _, def := range *defs {
		if name := strings.TrimSpace(def.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// llmProxyProviderNames returns the names policies use to select a provider of an
// LLM proxy. ok is false when the provider shape cannot be normalised; the LLM
// validator reports that, so the names are not checked.
func llmProxyProviderNames(spec api.LLMProxyConfigData) ([]string, bool) {
	attachments, err := models.NormaliseLLMProxyAttachments(spec)
	if err != nil {
		return nil, false
	}
	names := make([]string, 0, len(attachments))
	for _, a := range attachments {
		names = append(names, a.EffectiveName())
	}
	return names, true
}

// llmPolicyParams lists the parameters of an LLM provider's or proxy's normal
// policies: global, operation-level and the deprecated list. Fault policies run on
// the error response and do not route, so they are not listed.
func llmPolicyParams(globalPolicies *[]api.Policy, operationPolicies *[]api.OperationPolicy, legacyPolicies *[]api.LLMPolicy) []policyParams {
	var out []policyParams
	if globalPolicies != nil {
		for i, p := range *globalPolicies {
			if p.Params != nil {
				out = append(out, policyParams{p.Name, p.Version, *p.Params, fmt.Sprintf("spec.globalPolicies[%d].params", i)})
			}
		}
	}
	if operationPolicies != nil {
		for i, p := range *operationPolicies {
			for j, path := range p.Paths {
				out = append(out, policyParams{p.Name, p.Version, path.Params, fmt.Sprintf("spec.operationPolicies[%d].paths[%d].params", i, j)})
			}
		}
	}
	if legacyPolicies != nil {
		for i, p := range *legacyPolicies {
			for j, path := range p.Paths {
				out = append(out, policyParams{p.Name, p.Version, path.Params, fmt.Sprintf("spec.policies[%d].paths[%d].params", i, j)})
			}
		}
	}
	return out
}

// restAPIPolicyParams lists the parameters of a REST API's normal policies.
func restAPIPolicyParams(spec api.APIConfigData) []policyParams {
	var out []policyParams
	if spec.Policies != nil {
		for i, p := range *spec.Policies {
			if p.Params != nil {
				out = append(out, policyParams{p.Name, p.Version, *p.Params, fmt.Sprintf("spec.policies[%d].params", i)})
			}
		}
	}
	for i, op := range spec.Operations {
		if op.Policies == nil {
			continue
		}
		for j, p := range *op.Policies {
			if p.Params != nil {
				out = append(out, policyParams{p.Name, p.Version, *p.Params, fmt.Sprintf("spec.operations[%d].policies[%d].params", i, j)})
			}
		}
	}
	return out
}

// validateUpstreamNameParams checks every parameter marked with UpstreamNameMarker
// against the artifact's upstream names. noun names what those are in the message
// ("upstream definition", "provider"). A policy that cannot be resolved is skipped:
// the reference validation reports it.
func (pv *PolicyValidator) validateUpstreamNameParams(attached []policyParams, names []string, noun string) []ValidationError {
	known := make(map[string]bool, len(names))
	for _, n := range names {
		known[n] = true
	}
	var errors []ValidationError
	for _, a := range attached {
		schema := pv.parameterSchema(a.name, a.version)
		if schema == nil || a.params == nil {
			continue
		}
		var refs []upstreamNameRef
		collectUpstreamNameRefs(a.fieldPath, a.params, schema, &refs)
		for _, ref := range refs {
			if known[ref.value] {
				continue
			}
			errors = append(errors, ValidationError{
				Field:   ref.path,
				Message: unknownUpstreamNameMessage(ref.value, names, noun),
			})
		}
	}
	return errors
}

func unknownUpstreamNameMessage(value string, names []string, noun string) string {
	if len(names) == 0 {
		return fmt.Sprintf("'%s' does not match any %s: this artifact has none", value, noun)
	}
	sorted := slices.Clone(names)
	slices.Sort(sorted)
	return fmt.Sprintf("'%s' does not match any %s of this artifact; use one of: %s", value, noun, strings.Join(sorted, ", "))
}

// parameterSchema returns the parameters schema of the named policy, or nil when
// the policy cannot be resolved or takes no parameters.
func (pv *PolicyValidator) parameterSchema(name, version string) map[string]interface{} {
	resolved, err := pv.resolvePolicyVersion(name, version)
	if err != nil {
		return nil
	}
	def, ok := pv.policyDefinitions[name+"|"+resolved]
	if !ok || def.Parameters == nil {
		return nil
	}
	return *def.Parameters
}

type upstreamNameRef struct {
	path, value string
}

// isUpstreamNameSchema reports whether a parameter schema carries the marker,
// directly or through one of its allOf, anyOf or oneOf subschemas.
func isUpstreamNameSchema(schema map[string]interface{}) bool {
	if marked, _ := schema[UpstreamNameMarker].(bool); marked {
		return true
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		subschemas, _ := schema[keyword].([]interface{})
		for _, s := range subschemas {
			if sub, ok := s.(map[string]interface{}); ok && isUpstreamNameSchema(sub) {
				return true
			}
		}
	}
	return false
}

// collectUpstreamNameRefs walks a parameter value alongside its JSON Schema, through
// nested objects and arrays, and collects every non-blank string whose schema
// carries the marker. A value still holding a template expression is skipped; it is
// checked once rendered.
func collectUpstreamNameRefs(path string, value interface{}, schema map[string]interface{}, out *[]upstreamNameRef) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed != "" && !strings.Contains(trimmed, "{{") && isUpstreamNameSchema(schema) {
			*out = append(*out, upstreamNameRef{path: path, value: trimmed})
		}
	case map[string]interface{}:
		props, _ := schema["properties"].(map[string]interface{})
		additional, _ := schema["additionalProperties"].(map[string]interface{})
		for _, key := range slices.Sorted(maps.Keys(v)) {
			childSchema, _ := props[key].(map[string]interface{})
			if childSchema == nil {
				childSchema = additional
			}
			if childSchema != nil {
				collectUpstreamNameRefs(path+"."+key, v[key], childSchema, out)
			}
		}
	case []interface{}:
		items, _ := schema["items"].(map[string]interface{})
		if items == nil {
			return
		}
		for i, child := range v {
			collectUpstreamNameRefs(fmt.Sprintf("%s[%d]", path, i), child, items, out)
		}
	}
}
