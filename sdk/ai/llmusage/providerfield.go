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
	"encoding/json"

	"github.com/wso2/api-platform/sdk/core/utils"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// decodedBodyKey holds the response decoded once per request, so a consumer
// reading several provider fields does not re-parse the body for each.
const (
	decodedBodyKey    = "llm:usage:decoded-body"
	decodedRequestKey = "llm:usage:decoded-request"
)

// ProviderField returns the value at the location the route's template declares
// for name under providerFields. The value is returned as decoded JSON — a
// string, a number, a list or an object — so a caller can walk a list or a map
// without knowing where the provider puts it. Reports false when the route has
// no template, the name is not declared, or the location holds nothing.
//
// This is the escape hatch for values the closed vocabulary does not cover: the
// template still owns where the data is, and the caller owns what to do with it.
func ProviderField(sc *policy.SharedContext, body []byte, requestPath, name string) (interface{}, bool) {
	return providerFieldFrom(sc, body, requestPath, name, decodedBodyKey)
}

// ProviderFieldFromRequest is ProviderField against the request body, for the
// values a provider reports only on the way in — a requested search depth, for
// instance, that the response never echoes.
func ProviderFieldFromRequest(sc *policy.SharedContext, requestBody []byte, requestPath, name string) (interface{}, bool) {
	return providerFieldFrom(sc, requestBody, requestPath, name, decodedRequestKey)
}

func providerFieldFrom(sc *policy.SharedContext, body []byte, requestPath, name, cacheKey string) (interface{}, bool) {
	if sc == nil || name == "" {
		return nil, false
	}

	template, err := templateForRoute(sc)
	if err != nil {
		return nil, false
	}

	spec, ok := providerFieldSpec(template, name)
	if !ok || spec.Location != locationPayload {
		return nil, false
	}

	document, ok := decodedDocument(sc, body, requestPath, cacheKey)
	if !ok {
		return nil, false
	}

	for _, identifier := range spec.Identifiers {
		value, err := utils.ExtractValueFromJsonpath(document, identifier)
		if err != nil || value == nil {
			continue
		}
		return value, true
	}
	return nil, false
}

// providerFieldSpec reads one entry from the template's providerFields map.
func providerFieldSpec(template map[string]interface{}, name string) (fieldSpec, bool) {
	fields, ok := template["providerFields"].(map[string]interface{})
	if !ok {
		return fieldSpec{}, false
	}
	return readFieldSpec(fields, name)
}

// decodedDocument decodes the response once and keeps it for the rest of the
// request, since a provider needing several fields would otherwise pay for a
// parse per field.
func decodedDocument(sc *policy.SharedContext, body []byte, requestPath, cacheKey string) (map[string]interface{}, bool) {
	if sc.Metadata == nil {
		sc.Metadata = make(map[string]interface{})
	}
	if cached, ok := sc.Metadata[cacheKey].(map[string]interface{}); ok {
		return cached, true
	}

	if len(body) == 0 {
		return nil, false
	}
	var document map[string]interface{}
	if err := json.Unmarshal(decodeBody(body, requestPath), &document); err != nil {
		return nil, false
	}

	sc.Metadata[cacheKey] = document
	return document, true
}
