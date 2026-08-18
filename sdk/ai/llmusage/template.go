/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package llmusage

import (
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// fieldSpec is one extraction identifier from a template, with any fallback
// locations to try after it.
type fieldSpec struct {
	Location    string
	Identifier  string
	Identifiers []string

	// ValueMap translates values the provider reports into the vocabulary the
	// gateway works in. A value that is not a key is used as reported.
	ValueMap map[string]string
}

// extractionFields are the template keys this library reads. Names match the
// LlmProviderTemplate schema exactly.
var extractionFields = []string{
	"promptTokens",
	"completionTokens",
	"totalTokens",
	"cachedTokens",
	"cacheWriteTokens",
	"cacheWrite1hTokens",
	"reasoningTokens",
	"audioInputTokens",
	"audioOutputTokens",
	"serviceTier",
	"requestModel",
	"responseModel",
}

// resolveFields returns the effective extraction fields for a request path and
// the effective cache accounting mode. A resourceMappings entry matching the
// path overrides individual fields; every field it does not set falls back to
// the template root on its own.
func resolveFields(template map[string]interface{}, requestPath string) (map[string]fieldSpec, string) {
	fields := make(map[string]fieldSpec, len(extractionFields))
	if template == nil {
		return fields, ""
	}

	for _, name := range extractionFields {
		if spec, ok := readFieldSpec(template, name); ok {
			fields[name] = spec
		}
	}
	accounting, _ := template["cacheAccounting"].(string)

	mapping := selectMapping(template, requestPath)
	if mapping == nil {
		return fields, accounting
	}

	for _, name := range extractionFields {
		if spec, ok := readFieldSpec(mapping, name); ok {
			fields[name] = spec
		}
	}
	if override, ok := mapping["cacheAccounting"].(string); ok && override != "" {
		accounting = override
	}

	return fields, accounting
}

// readFieldSpec reads one extraction identifier. A field that is absent,
// malformed, or missing its identifier is treated as not set. Identifiers
// holds the primary identifier followed by any fallbackIdentifiers, in order.
func readFieldSpec(source map[string]interface{}, name string) (fieldSpec, bool) {
	raw, ok := source[name].(map[string]interface{})
	if !ok {
		return fieldSpec{}, false
	}
	identifier, ok := raw["identifier"].(string)
	if !ok || identifier == "" {
		return fieldSpec{}, false
	}
	location, _ := raw["location"].(string)

	identifiers := []string{identifier}
	if fallbacks, ok := raw["fallbackIdentifiers"].([]interface{}); ok {
		for _, f := range fallbacks {
			fallback, ok := f.(string)
			if !ok || fallback == "" {
				continue
			}
			identifiers = append(identifiers, fallback)
		}
	}

	var valueMap map[string]string
	if entries, ok := raw["valueMap"].(map[string]interface{}); ok {
		for key, value := range entries {
			mapped, ok := value.(string)
			if !ok || key == "" {
				continue
			}
			if valueMap == nil {
				valueMap = make(map[string]string, len(entries))
			}
			valueMap[key] = mapped
		}
	}

	return fieldSpec{
		Location:    location,
		Identifier:  identifier,
		Identifiers: identifiers,
		ValueMap:    valueMap,
	}, true
}

// selectMapping returns the resourceMappings entry that best matches the
// request path, preferring an exact match over a wildcard and a longer pattern
// over a shorter one.
func selectMapping(template map[string]interface{}, requestPath string) map[string]interface{} {
	mappings, ok := template["resourceMappings"].(map[string]interface{})
	if !ok {
		return nil
	}
	resources, ok := mappings["resources"].([]interface{})
	if !ok {
		return nil
	}

	var selected map[string]interface{}
	var selectedPath string
	for _, entry := range resources {
		candidate, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		candidatePath, ok := candidate["resource"].(string)
		if !ok || !pathsMatch(requestPath, candidatePath) {
			continue
		}
		if selected == nil || preferMoreSpecificPath(candidatePath, selectedPath) {
			selected, selectedPath = candidate, candidatePath
		}
	}

	return selected
}

// FieldPaths returns the response path each usage field is declared at for this
// route, so a consumer can label per-field output with the provider's own field
// names. Only payload fields are returned; header and path-parameter
// identifiers do not name a location in the response body. The result reflects
// any resourceMappings override that applies to requestPath.
func FieldPaths(sc *policy.SharedContext, requestPath string) map[string]string {
	if sc == nil {
		return map[string]string{}
	}

	template, err := templateForRoute(sc)
	if err != nil {
		return map[string]string{}
	}

	fields, _ := resolveFields(template, requestPath)
	paths := make(map[string]string, len(fields))
	for name, spec := range fields {
		if spec.Location == locationPayload && spec.Identifier != "" {
			paths[name] = spec.Identifier
		}
	}
	return paths
}
