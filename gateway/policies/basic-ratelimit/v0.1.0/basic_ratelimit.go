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

package basicratelimit

import (
	ratelimit "github.com/policy-engine/policies/advanced-ratelimit"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// BasicRateLimitPolicy provides a simplified rate limiting policy that delegates
// to the core ratelimit policy. It uses routename as the rate limit key and
// does not support cost extraction or multi-quota configurations.
type BasicRateLimitPolicy struct {
	delegate policy.Policy
}

// GetPolicy creates and initializes the basic rate limit policy.
// It transforms the simple limits configuration to a full ratelimit quota config
// and delegates to the core ratelimit policy.
func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	// Transform simple limits to full ratelimit config
	rlParams := transformToRatelimitParams(params)

	// Create the delegate ratelimit policy
	delegate, err := ratelimit.GetPolicy(metadata, rlParams)
	if err != nil {
		return nil, err
	}

	return &BasicRateLimitPolicy{delegate: delegate}, nil
}

// transformToRatelimitParams converts the simple limits array to a full ratelimit
// quota configuration with routename key extraction, and passes through system
// parameters (algorithm, backend, redis, memory).
func transformToRatelimitParams(params map[string]interface{}) map[string]interface{} {
	limits, _ := params["limits"].([]interface{})

	rlParams := map[string]interface{}{
		"quotas": []interface{}{
			map[string]interface{}{
				"name":   "default",
				"limits": limits,
				"keyExtraction": []interface{}{
					map[string]interface{}{
						"type": "routename",
					},
				},
			},
		},
	}

	// Pass through system parameters
	if algorithm, ok := params["algorithm"]; ok {
		rlParams["algorithm"] = algorithm
	}
	if backend, ok := params["backend"]; ok {
		rlParams["backend"] = backend
	}
	if redis, ok := params["redis"]; ok {
		rlParams["redis"] = redis
	}
	if memory, ok := params["memory"]; ok {
		rlParams["memory"] = memory
	}

	return rlParams
}

// Mode returns the processing mode for this policy.
// Since basic-ratelimit does not use cost extraction from request/response bodies,
// it only needs header processing and skips body buffering.
func (p *BasicRateLimitPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		RequestBodyMode:    policy.BodyModeSkip,
		ResponseHeaderMode: policy.HeaderModeProcess,
		ResponseBodyMode:   policy.BodyModeSkip,
	}
}

// OnRequest delegates to the core ratelimit policy's OnRequest method.
func (p *BasicRateLimitPolicy) OnRequest(
	ctx *policy.RequestContext,
	params map[string]interface{},
) policy.RequestAction {
	return p.delegate.OnRequest(ctx, params)
}

// OnResponse delegates to the core ratelimit policy's OnResponse method.
func (p *BasicRateLimitPolicy) OnResponse(
	ctx *policy.ResponseContext,
	params map[string]interface{},
) policy.ResponseAction {
	return p.delegate.OnResponse(ctx, params)
}
