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

// NoopPolicy is a policy implementation that does nothing.
// Useful for testing policy chains without side effects.
type NoopPolicy struct{}

// Mode returns an empty ProcessingMode.
func (p *NoopPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{}
}

// OnRequest returns nil (no action).
func (p *NoopPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return nil
}

// OnResponse returns nil (no action).
func (p *NoopPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil
}

// =============================================================================
// HeaderModifyingPolicy - A policy that modifies headers
// =============================================================================

// HeaderModifyingPolicy is a policy that sets a header on request and response.
type HeaderModifyingPolicy struct {
	Key   string
	Value string
}

// Mode returns a ProcessingMode that processes headers.
func (p *HeaderModifyingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		ResponseHeaderMode: policy.HeaderModeProcess,
	}
}

// OnRequest returns modifications to set the configured header.
func (p *HeaderModifyingPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{
		SetHeaders: map[string]string{p.Key: p.Value},
	}
}

// OnResponse returns modifications to set the configured header.
func (p *HeaderModifyingPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return policy.UpstreamResponseModifications{
		SetHeaders: map[string]string{p.Key: p.Value},
	}
}

// =============================================================================
// ShortCircuitingPolicy - A policy that returns an immediate response
// =============================================================================

// ShortCircuitingPolicy is a policy that short-circuits with an immediate response.
type ShortCircuitingPolicy struct {
	StatusCode int
	Body       []byte
}

// Mode returns a ProcessingMode that processes request headers.
func (p *ShortCircuitingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode: policy.HeaderModeProcess,
	}
}

// OnRequest returns an ImmediateResponse to short-circuit the request.
func (p *ShortCircuitingPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return policy.ImmediateResponse{
		StatusCode: p.StatusCode,
		Body:       p.Body,
	}
}

// OnResponse returns nil (no action).
func (p *ShortCircuitingPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil
}

// =============================================================================
// ConfigurableMockPolicy - A flexible mock policy with callbacks
// =============================================================================

// ConfigurableMockPolicy is a mock policy with configurable behavior via callbacks.
type ConfigurableMockPolicy struct {
	Name    string
	Version string
	MockMode policy.ProcessingMode
	OnReqFn  func(*policy.RequestContext, map[string]interface{}) policy.RequestAction
	OnRespFn func(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction
}

// Mode returns the configured ProcessingMode.
func (m *ConfigurableMockPolicy) Mode() policy.ProcessingMode {
	return m.MockMode
}

// OnRequest calls the configured callback or returns nil.
func (m *ConfigurableMockPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if m.OnReqFn != nil {
		return m.OnReqFn(ctx, params)
	}
	return nil
}

// OnResponse calls the configured callback or returns nil.
func (m *ConfigurableMockPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	if m.OnRespFn != nil {
		return m.OnRespFn(ctx, params)
	}
	return nil
}

// =============================================================================
// SimpleMockPolicy - A simple mock policy for registry tests
// =============================================================================

// SimpleMockPolicy is a minimal mock policy that stores name and version.
type SimpleMockPolicy struct {
	Name    string
	Version string
}

// Mode returns an empty ProcessingMode.
func (m *SimpleMockPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{}
}

// OnRequest returns nil (no action).
func (m *SimpleMockPolicy) OnRequest(*policy.RequestContext, map[string]interface{}) policy.RequestAction {
	return nil
}

// OnResponse returns nil (no action).
func (m *SimpleMockPolicy) OnResponse(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction {
	return nil
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
