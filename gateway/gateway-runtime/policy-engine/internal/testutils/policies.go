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

package testutils

import (
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

// =============================================================================
// NoopPolicy - A policy that does nothing
// =============================================================================

// NoopPolicy satisfies the Policy marker interface with no processing.
// Useful for testing policy chains without side effects.
type NoopPolicy struct{}

// =============================================================================
// HeaderModifyingPolicy - A policy that modifies request and response headers
// =============================================================================

// HeaderModifyingPolicy implements RequestHeaderPolicy and ResponseHeaderPolicy.
type HeaderModifyingPolicy struct {
	Key   string
	Value string
}

// OnRequestHeaders returns modifications to set the configured header.
func (p *HeaderModifyingPolicy) OnRequestHeaders(*policy.RequestHeaderContext) policy.RequestHeaderAction {
	return policy.RequestHeaderAction{
		Set: map[string]string{p.Key: p.Value},
	}
}

// OnResponseHeaders returns modifications to set the configured header.
func (p *HeaderModifyingPolicy) OnResponseHeaders(*policy.ResponseHeaderContext) policy.ResponseHeaderAction {
	return policy.ResponseHeaderAction{
		Set: map[string]string{p.Key: p.Value},
	}
}

// =============================================================================
// ShortCircuitingPolicy - A policy that returns an immediate response
// =============================================================================

// ShortCircuitingPolicy implements RequestHeaderPolicy and short-circuits with
// an ImmediateResponse.
type ShortCircuitingPolicy struct {
	StatusCode int
	Body       []byte
}

// OnRequestHeaders returns an ImmediateResponse to short-circuit the request.
func (p *ShortCircuitingPolicy) OnRequestHeaders(*policy.RequestHeaderContext) policy.RequestHeaderAction {
	return policy.RequestHeaderAction{
		ImmediateResponse: &policy.ImmediateResponse{
			StatusCode: p.StatusCode,
			Body:       p.Body,
		},
	}
}

// =============================================================================
// ConfigurableMockPolicy - A flexible mock policy with callbacks
// =============================================================================

// ConfigurableMockPolicy is a mock policy with configurable behavior via callbacks.
type ConfigurableMockPolicy struct {
	Name     string
	Version  string
	OnReqFn  func(*policy.RequestHeaderContext) policy.RequestHeaderAction
	OnRespFn func(*policy.ResponseHeaderContext) policy.ResponseHeaderAction
}

// OnRequestHeaders calls the configured callback or returns an empty action.
func (m *ConfigurableMockPolicy) OnRequestHeaders(ctx *policy.RequestHeaderContext) policy.RequestHeaderAction {
	if m.OnReqFn != nil {
		return m.OnReqFn(ctx)
	}
	return policy.RequestHeaderAction{}
}

// OnResponseHeaders calls the configured callback or returns an empty action.
func (m *ConfigurableMockPolicy) OnResponseHeaders(ctx *policy.ResponseHeaderContext) policy.ResponseHeaderAction {
	if m.OnRespFn != nil {
		return m.OnRespFn(ctx)
	}
	return policy.ResponseHeaderAction{}
}

// =============================================================================
// SimpleMockPolicy - A simple mock policy for registry tests
// =============================================================================

// SimpleMockPolicy is a minimal mock policy that satisfies the Policy marker interface.
type SimpleMockPolicy struct {
	Name    string
	Version string
}

// =============================================================================
// Factory Functions
// =============================================================================

// NewMockPolicyFactory creates a PolicyFactory that returns SimpleMockPolicy instances.
func NewMockPolicyFactory(name, version string) policy.PolicyFactory {
	return func(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
		return &SimpleMockPolicy{Name: name, Version: version}, nil
	}
}
