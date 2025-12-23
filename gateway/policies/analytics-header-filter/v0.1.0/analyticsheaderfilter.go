/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package analyticsheaderfilter

import (
	"fmt"
	"log/slog"
	"strings"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	// Analytics metadata keys for filtered headers
	// These keys are used to pass the filtered header information to the analytics publisher
	RequestHeadersFilteredKey  = "analytics:request-headers-filtered"
	ResponseHeadersFilteredKey = "analytics:response-headers-filtered"
)

// AnalyticsHeaderFilterPolicy filters specified headers from analytics data
type AnalyticsHeaderFilterPolicy struct {
	analyticsEnabled bool
}

// GetPolicy creates a new instance of the AnalyticsHeaderFilterPolicy
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	// Extract analyticsEnabled from system parameters
	analyticsEnabled := false
	if enabled, ok := params["analyticsEnabled"].(bool); ok {
		analyticsEnabled = enabled
	}

	return &AnalyticsHeaderFilterPolicy{
		analyticsEnabled: analyticsEnabled,
	}, nil
}

// Mode returns the processing mode for this policy
// Only processes headers (request and response), no body processing needed
func (p *AnalyticsHeaderFilterPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequest processes request headers and filters specified headers from analytics
func (p *AnalyticsHeaderFilterPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// If analytics is not enabled, skip processing
	if !p.analyticsEnabled {
		return nil
	}

	// Get the list of request headers to filter
	headersToFilter := p.parseHeaderList(params, "requestHeadersToFilter")
	if len(headersToFilter) == 0 {
		slog.Debug("[analytics-header-filter] No request headers provided from the user")
		return nil
	}

	// Build the filtered headers list by checking which headers actually exist
	filteredHeaders := p.getMatchingHeaders(ctx.Headers, headersToFilter)
	if len(filteredHeaders) == 0 {
		slog.Debug("[analytics-header-filter] No matching headers found in request headers")
		return nil
	}

	// Return analytics metadata with the list of filtered headers
	// The analytics publisher will use this to exclude these headers
	slog.Debug(fmt.Sprintf("[analytics-header-filter] Returning analytics metadata with the list of filtered request headers: %s", strings.Join(filteredHeaders, ",")))
	return policy.UpstreamRequestModifications{
		AnalyticsMetadata: map[string]any{
			RequestHeadersFilteredKey: strings.Join(filteredHeaders, ","),
		},
	}
}

// OnResponse processes response headers and filters specified headers from analytics
func (p *AnalyticsHeaderFilterPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	// If analytics is not enabled, skip processing
	if !p.analyticsEnabled {
		return nil
	}

	// Get the list of response headers to filter
	headersToFilter := p.parseHeaderList(params, "responseHeadersToFilter")
	if len(headersToFilter) == 0 {
		slog.Debug("[analytics-header-filter] No response headers provided from the user to filter")
		return nil
	}

	// Build the filtered headers list by checking which headers actually exist
	filteredHeaders := p.getMatchingHeaders(ctx.ResponseHeaders, headersToFilter)
	if len(filteredHeaders) == 0 {
		slog.Debug("[analytics-header-filter] No matching headers found in response headers")
		return nil
	}

	// Return analytics metadata with the list of filtered headers
	// The analytics publisher will use this to exclude these headers
	slog.Debug(fmt.Sprintf("[analytics-header-filter] Returning analytics metadata with the list of filtered response headers: %s", strings.Join(filteredHeaders, ",")))
	return policy.UpstreamResponseModifications{
		AnalyticsMetadata: map[string]any{
			ResponseHeadersFilteredKey: strings.Join(filteredHeaders, ","),
		},
	}
}

// parseHeaderList extracts a list of header names from the parameters
func (p *AnalyticsHeaderFilterPolicy) parseHeaderList(params map[string]interface{}, key string) []string {
	headersRaw, ok := params[key]
	if !ok {
		return nil
	}

	headersList, ok := headersRaw.([]interface{})
	if !ok {
		return nil
	}

	headers := make([]string, 0, len(headersList))
	for _, h := range headersList {
		if headerName, ok := h.(string); ok && headerName != "" {
			// Normalize header names to lowercase for case-insensitive matching
			headers = append(headers, strings.ToLower(headerName))
		}
	}

	return headers
}

// getMatchingHeaders returns headers that exist in the context and match the filter list
func (p *AnalyticsHeaderFilterPolicy) getMatchingHeaders(headers *policy.Headers, filterList []string) []string {
	if headers == nil {
		return nil
	}

	matched := make([]string, 0)
	for _, headerToFilter := range filterList {
		// Headers.Has performs case-insensitive lookup
		if headers.Has(headerToFilter) {
			matched = append(matched, headerToFilter)
		}
	}

	return matched
}
