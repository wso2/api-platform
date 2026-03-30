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

package testutils

import (
	"context"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// =============================================================================
// NoopPolicy - A policy that does nothing
// =============================================================================

type NoopPolicy struct{}

func (p *NoopPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{}
}

func (p *NoopPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return nil
}

func (p *NoopPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return nil
}

// =============================================================================
// HeaderModifyingPolicy - A policy that modifies headers
// =============================================================================

type HeaderModifyingPolicy struct {
	Key   string
	Value string
}

func (p *HeaderModifyingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess,
		ResponseHeaderMode: policy.HeaderModeProcess,
	}
}

func (p *HeaderModifyingPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.UpstreamRequestModifications{
		HeadersToSet: map[string]string{p.Key: p.Value},
	}
}

func (p *HeaderModifyingPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return policy.DownstreamResponseModifications{
		HeadersToSet: map[string]string{p.Key: p.Value},
	}
}

// =============================================================================
// ShortCircuitingPolicy - A policy that returns an immediate response
// =============================================================================

type ShortCircuitingPolicy struct {
	StatusCode int
	Body       []byte
}

func (p *ShortCircuitingPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode: policy.HeaderModeProcess,
	}
}

func (p *ShortCircuitingPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return policy.ImmediateResponse{
		StatusCode: p.StatusCode,
		Body:       p.Body,
	}
}

func (p *ShortCircuitingPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return nil
}

// =============================================================================
// ConfigurableMockPolicy - A flexible mock policy with callbacks
// =============================================================================

type ConfigurableMockPolicy struct {
	Name     string
	Version  string
	MockMode policy.ProcessingMode
	OnReqFn  func(*policy.RequestContext, map[string]interface{}) policy.RequestAction
	OnRespFn func(*policy.ResponseContext, map[string]interface{}) policy.ResponseAction
}

func (p *ConfigurableMockPolicy) Mode() policy.ProcessingMode {
	return p.MockMode
}

func (p *ConfigurableMockPolicy) OnRequestBody(_ context.Context, ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	if p.OnReqFn != nil {
		return p.OnReqFn(ctx, params)
	}
	return nil
}

func (p *ConfigurableMockPolicy) OnResponseBody(_ context.Context, ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	if p.OnRespFn != nil {
		return p.OnRespFn(ctx, params)
	}
	return nil
}

// =============================================================================
// SimpleMockPolicy - A simple mock policy with name and version
// =============================================================================

type SimpleMockPolicy struct {
	Name    string
	Version string
}

func (p *SimpleMockPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{}
}

func (p *SimpleMockPolicy) OnRequestBody(_ context.Context, _ *policy.RequestContext, _ map[string]interface{}) policy.RequestAction {
	return nil
}

func (p *SimpleMockPolicy) OnResponseBody(_ context.Context, _ *policy.ResponseContext, _ map[string]interface{}) policy.ResponseAction {
	return nil
}

// =============================================================================
// NewMockPolicyFactory - Creates a PolicyFactory that returns a NoopPolicy
// =============================================================================

func NewMockPolicyFactory(name, version string) policy.PolicyFactory {
	return func(metadata policy.PolicyMetadata, params map[string]interface{}) (policy.Policy, error) {
		return &NoopPolicy{}, nil
	}
}
