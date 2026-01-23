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

package addheaders

import (
	"fmt"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// HeaderEntry represents a single header to be added
type HeaderEntry struct {
	Name  string
	Value string
}

// AddHeadersPolicy implements header addition for both request and response
type AddHeadersPolicy struct{}

var ins = &AddHeadersPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *AddHeadersPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Can add request headers
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeProcess, // Can add response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// Validate validates the policy configuration parameters
func (p *AddHeadersPolicy) Validate(params map[string]interface{}) error {
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
func (p *AddHeadersPolicy) validateHeaderEntries(headersRaw interface{}, fieldName string) error {
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
func (p *AddHeadersPolicy) parseHeaderEntries(headersRaw interface{}) []HeaderEntry {
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

// convertToAppendHeaderMap converts header entries to a map for policy actions
// Returns map[string][]string for AppendHeaders (appends to existing headers instead of replacing)
// Multiple headers with the same name will have their values accumulated in the slice
func (p *AddHeadersPolicy) convertToAppendHeaderMap(entries []HeaderEntry) map[string][]string {
	headerMap := make(map[string][]string)
	for _, entry := range entries {
		headerMap[entry.Name] = append(headerMap[entry.Name], entry.Value)
	}
	return headerMap
}

// OnRequest adds headers to the request
// Uses AppendHeaders to preserve existing headers and allow multiple values for the same header name
func (p *AddHeadersPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check if requestHeaders are configured
	requestHeadersRaw, ok := params["requestHeaders"]
	if !ok {
		// No request headers to add, pass through
		return policy.UpstreamRequestModifications{}
	}

	// Parse header entries
	entries := p.parseHeaderEntries(requestHeadersRaw)
	if len(entries) == 0 {
		return policy.UpstreamRequestModifications{}
	}

	// Convert to append header map - this allows multiple values per header name
	// and ensures headers are appended to existing ones rather than replacing them
	appendHeaders := p.convertToAppendHeaderMap(entries)

	return policy.UpstreamRequestModifications{
		AppendHeaders: appendHeaders,
	}
}

// OnResponse adds headers to the response
// Uses AppendHeaders to preserve existing headers and allow multiple values for the same header name
func (p *AddHeadersPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// Check if responseHeaders are configured
	responseHeadersRaw, ok := params["responseHeaders"]
	if !ok {
		// No response headers to add, pass through
		return policy.UpstreamResponseModifications{}
	}

	// Parse header entries
	entries := p.parseHeaderEntries(responseHeadersRaw)
	if len(entries) == 0 {
		return policy.UpstreamResponseModifications{}
	}

	// Convert to append header map - this allows multiple values per header name
	// and ensures headers are appended to existing ones rather than replacing them
	appendHeaders := p.convertToAppendHeaderMap(entries)

	return policy.UpstreamResponseModifications{
		AppendHeaders: appendHeaders,
	}
}
