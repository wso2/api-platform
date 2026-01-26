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

package removequeryparameter

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// RemoveQueryParameterPolicy implements removing multiple query parameters from requests
type RemoveQueryParameterPolicy struct{}

var ins = &RemoveQueryParameterPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *RemoveQueryParameterPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, // Process request headers to remove query params
		RequestBodyMode:    policy.BodyModeSkip,      // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip,    // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,      // Don't need response body
	}
}

// OnRequest modifies request path by removing query parameters
func (p *RemoveQueryParameterPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check if queryParameters are configured
	queryParametersRaw, ok := params["queryParameters"]
	if !ok {
		// No query parameters configured, pass through
		return policy.UpstreamRequestModifications{}
	}

	// Parse queryParameters array
	queryParametersSlice, ok := queryParametersRaw.([]interface{})
	if !ok {
		// Invalid queryParameters format, pass through
		return policy.UpstreamRequestModifications{}
	}

	// Build list of query parameter names to remove
	var paramNamesToRemove []string

	for _, paramRaw := range queryParametersSlice {
		paramMap, ok := paramRaw.(map[string]interface{})
		if !ok {
			// Skip invalid parameter entries
			continue
		}

		name, nameOk := paramMap["name"].(string)
		if !nameOk || name == "" {
			// Skip invalid parameter entries
			continue
		}

		// Add the parameter name to the removal list
		paramNamesToRemove = append(paramNamesToRemove, name)
	}

	// Return modifications if we have any query parameters to remove
	if len(paramNamesToRemove) > 0 {
		return policy.UpstreamRequestModifications{
			RemoveQueryParameters: paramNamesToRemove,
		}
	}

	return policy.UpstreamRequestModifications{}
}

// OnResponse is a no-op for this policy
func (p *RemoveQueryParameterPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil
}
