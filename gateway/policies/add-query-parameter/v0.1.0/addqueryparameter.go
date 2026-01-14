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

package addqueryparameter

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// AddQueryParameterPolicy implements adding a query parameter to requests
type AddQueryParameterPolicy struct{}

var ins = &AddQueryParameterPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (p *AddQueryParameterPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip, // Don't process request headers
		RequestBodyMode:    policy.BodyModeSkip,   // Don't need request body
		ResponseHeaderMode: policy.HeaderModeSkip, // Don't process response headers
		ResponseBodyMode:   policy.BodyModeSkip,   // Don't need response body
	}
}

// OnRequest modifies request path by adding query parameter
func (p *AddQueryParameterPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	// Check if name parameter is configured
	name, ok := params["name"].(string)
	if !ok || name == "" {
		// No name for the query parameter, pass through
		return policy.UpstreamRequestModifications{}
	}

	value, ok := params["value"].(string)
	if !ok {
		// Invalid value for the query parameter, pass through
		return policy.UpstreamRequestModifications{}
	}

	return policy.UpstreamRequestModifications{
		AddQueryParameters: map[string]string{
			name: value,
		},
	}
}

// OnResponse is a no-op for this policy
func (p *AddQueryParameterPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	return nil
}
