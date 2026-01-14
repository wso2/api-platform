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

package rewriteresourcepath

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	"net/url"
	"strings"
)

// RewriteResourcePathPolicy implements rewriting resource path of the request
type RewriteResourcePathPolicy struct{}

var ins = &RewriteResourcePathPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *RewriteResourcePathPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip, // Don't process request headers
		RequestBodyMode:    policy.BodyModeSkip,   // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip, // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,   // Don't need response body
	}
}

// OnRequest modifies request by rewriting the resource path
func (p *RewriteResourcePathPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check if path parameter is configured
	path, ok := params["path"].(string)
	if !ok || path == "" {
		// No path parameter configured, pass through
		return policy.UpstreamRequestModifications{}
	}

	// Trim leading slash from user-provided path to avoid double slashes
	path = strings.TrimPrefix(path, "/")

	apiContext := ctx.SharedContext.APIContext
	fullPath := apiContext + "/" + path

	// Parse the user-provided path to handle URL components properly
	parsedPath, err := url.Parse(path)
	if err != nil {
		// If parsing fails, use the path as-is for backward compatibility
		return policy.UpstreamRequestModifications{
			Path: &fullPath,
		}
	}

	// Parse the original request path to extract query parameters
	originalParsedURL, err := url.Parse(ctx.Path)
	if err != nil {
		// If URL parsing fails, return the new path as-is
		return policy.UpstreamRequestModifications{
			Path: &fullPath,
		}
	}

	// Get the API context (e.g., "/weather/v2.0") and append the parsed path
	// If the user-provided path has query parameters, do not preserve them
	fullPath = apiContext + "/" + parsedPath.Path

	// Preserve original query parameters from the request URL
	if originalParsedURL.RawQuery != "" {
		fullPath = fullPath + "?" + originalParsedURL.RawQuery
	}

	return policy.UpstreamRequestModifications{
		Path: &fullPath,
	}
}

// OnResponse is a no-op for this policy
func (p *RewriteResourcePathPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil
}
