/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package setheaders

import (
	"fmt"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// HeaderEntry represents a single header to be set
type HeaderEntry struct {
	Name  string
	Value string
}

// SetHeadersPolicy implements header setting for both request and response
type SetHeadersPolicy struct{}

var ins = &SetHeadersPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *SetHeadersPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Can set request headers
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeProcess, // Can set response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// Validate validates the policy configuration parameters
func (p *SetHeadersPolicy) Validate(params map[string]interface{}) error {
	// At least one of requestHeaders or responseHeaders must be specified
	requestHeadersRaw, hasRequestHeaders := params["requestHeaders"]
	responseHeadersRaw, hasResponseHeaders := params["responseHeaders"]

	if !hasRequestHeaders && !hasResponseHeaders {
		return fmt.Errorf("at least one of 'requestHeaders' or 'responseHeaders' must be specified")
	}

	// Validate requestHeaders if present
	if hasRequestHeaders {
		if err := p.validateHeaderEntries(requestHeadersRaw, "requestHeaders"); err != nil {
			return err
		}
	}

	// Validate responseHeaders if present
	if hasResponseHeaders {
		if err := p.validateHeaderEntries(responseHeadersRaw, "responseHeaders"); err != nil {
			return err
		}
	}

	return nil
}

// validateHeaderEntries validates a list of header entries
func (p *SetHeadersPolicy) validateHeaderEntries(headersRaw interface{}, fieldName string) error {
	headers, ok := headersRaw.([]interface{})
	if !ok {
		return fmt.Errorf("%s must be an array", fieldName)
	}

	if len(headers) == 0 {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}

	for i, headerRaw := range headers {
		headerMap, ok := headerRaw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s[%d] must be an object with 'name' and 'value' fields", fieldName, i)
		}

		// Validate name
		nameRaw, ok := headerMap["name"]
		if !ok {
			return fmt.Errorf("%s[%d] missing required 'name' field", fieldName, i)
		}
		name, ok := nameRaw.(string)
		if !ok {
			return fmt.Errorf("%s[%d].name must be a string", fieldName, i)
		}
		if len(strings.TrimSpace(name)) == 0 {
			return fmt.Errorf("%s[%d].name cannot be empty", fieldName, i)
		}

		// Validate value
		valueRaw, ok := headerMap["value"]
		if !ok {
			return fmt.Errorf("%s[%d] missing required 'value' field", fieldName, i)
		}
		_, ok = valueRaw.(string)
		if !ok {
			return fmt.Errorf("%s[%d].value must be a string", fieldName, i)
		}
	}

	return nil
}

// parseHeaderEntries parses header entries from config
func (p *SetHeadersPolicy) parseHeaderEntries(headersRaw interface{}) []HeaderEntry {
	headers, ok := headersRaw.([]interface{})
	if !ok {
		return nil
	}

	entries := make([]HeaderEntry, 0, len(headers))
	for _, headerRaw := range headers {
		headerMap, ok := headerRaw.(map[string]interface{})
		if !ok {
			continue
		}

		entry := HeaderEntry{
			Name:  strings.ToLower(strings.TrimSpace(headerMap["name"].(string))), // Normalize to lowercase
			Value: headerMap["value"].(string),
		}

		entries = append(entries, entry)
	}

	return entries
}

// convertToSetHeaderMap converts header entries to a map for policy actions
// Returns map[string]string for SetHeaders (overwrites existing headers)
// Multiple headers with the same name will have the last value win (map behavior)
func (p *SetHeadersPolicy) convertToSetHeaderMap(entries []HeaderEntry) map[string]string {
	headerMap := make(map[string]string)
	for _, entry := range entries {
		headerMap[entry.Name] = entry.Value // Last value wins for duplicate names
	}
	return headerMap
}

// OnRequest sets headers on the request
// Uses SetHeaders to overwrite existing headers instead of appending
func (p *SetHeadersPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check if requestHeaders are configured
	requestHeadersRaw, ok := params["requestHeaders"]
	if !ok {
		// No request headers to set, pass through
		return policy.UpstreamRequestModifications{}
	}

	// Parse header entries
	entries := p.parseHeaderEntries(requestHeadersRaw)
	if len(entries) == 0 {
		return policy.UpstreamRequestModifications{}
	}

	// Convert to set header map - this will overwrite existing headers
	setHeaders := p.convertToSetHeaderMap(entries)

	return policy.UpstreamRequestModifications{
		SetHeaders: setHeaders,
	}
}

// OnResponse sets headers on the response
// Uses SetHeaders to overwrite existing headers instead of appending
func (p *SetHeadersPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Check if responseHeaders are configured
	responseHeadersRaw, ok := params["responseHeaders"]
	if !ok {
		// No response headers to set, pass through
		return policy.UpstreamResponseModifications{}
	}

	// Parse header entries
	entries := p.parseHeaderEntries(responseHeadersRaw)
	if len(entries) == 0 {
		return policy.UpstreamResponseModifications{}
	}

	// Convert to set header map - this will overwrite existing headers
	setHeaders := p.convertToSetHeaderMap(entries)

	return policy.UpstreamResponseModifications{
		SetHeaders: setHeaders,
	}
}
