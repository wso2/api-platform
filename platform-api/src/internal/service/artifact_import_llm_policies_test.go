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

package service

import (
	"testing"

	"platform-api/src/internal/dto"
	"platform-api/src/internal/model"
	"platform-api/src/internal/utils"

	"gopkg.in/yaml.v3"
)

func bptr(b bool) *bool { return &b }

// TestLiftLLMPolicies_RoundTrip drives a provider config that uses the first-class
// Security and RateLimiting fields through the actual CP->DP forward conversion
// (generateLLMProviderDeploymentYAML), then through the DP->CP import decode + lift,
// and asserts the first-class fields are reconstructed. This guards the inverse
// mapping against drift in the forward conversion.
func TestLiftLLMPolicies_RoundTrip(t *testing.T) {
	provider := &model.LLMProvider{
		ID:      "round-trip-provider",
		Name:    "Round Trip Provider",
		Version: "v1.0",
		Configuration: model.LLMProviderConfig{
			Upstream: &model.UpstreamConfig{Main: &model.UpstreamEndpoint{URL: "https://api.openai.com"}},
			Security: &model.SecurityConfig{
				Enabled: bptr(true),
				APIKey:  &model.APIKeySecurity{Enabled: bptr(true), Key: "secret-key", In: "header"},
			},
			RateLimiting: &model.LLMRateLimitingConfig{
				// Provider scope: resource-wise (default token + one resource request).
				ProviderLevel: &model.RateLimitingScopeConfig{
					ResourceWise: &model.ResourceWiseRateLimitingConfig{
						Default: model.RateLimitingLimitConfig{
							Token: &model.TokenRateLimit{Enabled: true, Count: 1000, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
						},
						Resources: []model.RateLimitingResourceLimit{
							{
								Resource: "/v1/chat",
								Limit: model.RateLimitingLimitConfig{
									Request: &model.RequestRateLimit{Enabled: true, Count: 60, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "minute"}},
								},
							},
						},
					},
				},
				// Consumer scope: global cost.
				ConsumerLevel: &model.RateLimitingScopeConfig{
					Global: &model.RateLimitingLimitConfig{
						Cost: &model.CostRateLimit{Enabled: true, Amount: 50, Reset: model.RateLimitResetWindow{Duration: 1, Unit: "hour"}},
					},
				},
			},
			// A genuine, non-system policy that must survive the round-trip untouched.
			Policies: []model.LLMPolicy{
				{Name: "custom-guardrail", Version: "v1", Paths: []model.LLMPolicyPath{{Path: "/*", Methods: []string{"*"}}}},
			},
		},
	}

	// Forward: CP -> gateway YAML (security/rate-limit become policies).
	yamlDoc, err := generateLLMProviderDeploymentYAML(provider, "openai")
	if err != nil {
		t.Fatalf("generateLLMProviderDeploymentYAML: %v", err)
	}
	yamlBytes, err := yaml.Marshal(yamlDoc)
	if err != nil {
		t.Fatalf("marshal forward YAML: %v", err)
	}

	// Extract the spec block, mirroring what the gateway pushes back on import.
	var doc map[string]interface{}
	if err := yaml.Unmarshal(yamlBytes, &doc); err != nil {
		t.Fatalf("unmarshal forward YAML: %v", err)
	}
	specMap, _ := doc["spec"].(map[string]interface{})
	if specMap == nil {
		t.Fatalf("forward YAML missing spec block")
	}

	// Import decode + reverse map, exactly as the LLM provider importer does:
	// decode into the deployment spec, then reconstruct the stored config.
	var spec dto.LLMProviderDeploymentSpec
	if err := utils.DecodeSpec(specMap, &spec); err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	cfg := mapLLMProviderSpecToConfig(spec)
	sec, rl, remaining := cfg.Security, cfg.RateLimiting, cfg.Policies

	// --- Security reconstructed ---
	if sec == nil || sec.APIKey == nil {
		t.Fatalf("security not reconstructed: %+v", sec)
	}
	if sec.APIKey.Key != "secret-key" || sec.APIKey.In != "header" {
		t.Errorf("security apiKey = %+v, want key=secret-key in=header", sec.APIKey)
	}
	if sec.Enabled == nil || !*sec.Enabled {
		t.Errorf("security.Enabled = %v, want true", sec.Enabled)
	}

	// --- RateLimiting reconstructed ---
	if rl == nil || rl.ProviderLevel == nil || rl.ProviderLevel.ResourceWise == nil {
		t.Fatalf("provider rate limiting not reconstructed as resource-wise: %+v", rl)
	}
	pdef := rl.ProviderLevel.ResourceWise.Default
	if pdef.Token == nil || pdef.Token.Count != 1000 || pdef.Token.Reset.Unit != "hour" || pdef.Token.Reset.Duration != 1 {
		t.Errorf("provider default token = %+v, want count=1000 reset=1h", pdef.Token)
	}
	if len(rl.ProviderLevel.ResourceWise.Resources) != 1 {
		t.Fatalf("provider resources = %d, want 1", len(rl.ProviderLevel.ResourceWise.Resources))
	}
	res := rl.ProviderLevel.ResourceWise.Resources[0]
	if res.Resource != "/v1/chat" || res.Limit.Request == nil || res.Limit.Request.Count != 60 || res.Limit.Request.Reset.Unit != "minute" {
		t.Errorf("provider resource = %+v, want /v1/chat request count=60 reset=1m", res)
	}
	if rl.ConsumerLevel == nil || rl.ConsumerLevel.Global == nil || rl.ConsumerLevel.Global.Cost == nil {
		t.Fatalf("consumer cost not reconstructed: %+v", rl.ConsumerLevel)
	}
	if rl.ConsumerLevel.Global.Cost.Amount != 50 || rl.ConsumerLevel.Global.Cost.Reset.Unit != "hour" {
		t.Errorf("consumer cost = %+v, want amount=50 reset=1h", rl.ConsumerLevel.Global.Cost)
	}

	// --- Genuine policies preserved; security/rate-limit policies stripped ---
	// The llm-cost tracker that the forward conversion auto-attaches alongside cost
	// limits is preserved (not dropped) so the AI Workspace can surface it.
	names := make(map[string]bool, len(remaining))
	for _, p := range remaining {
		names[p.Name] = true
	}
	if !names["custom-guardrail"] {
		t.Errorf("remaining policies = %+v, want custom-guardrail preserved", remaining)
	}
	if !names[importPolicyLLMCost] {
		t.Errorf("remaining policies = %+v, want the llm-cost tracker preserved", remaining)
	}
	for _, p := range remaining {
		switch p.Name {
		case importPolicyAPIKeyAuth, importPolicyTokenRateLimit, importPolicyAdvancedRateLimit,
			importPolicyCostRateLimit:
			t.Errorf("security/rate-limit policy %q leaked into remaining policies", p.Name)
		}
	}
}

// TestLiftLLMPolicies_NoSpecialPolicies verifies plain policies pass through and no
// security/rate-limiting is fabricated.
func TestLiftLLMPolicies_NoSpecialPolicies(t *testing.T) {
	in := []model.LLMPolicy{
		{Name: "custom-a"},
		{Name: "custom-b"},
	}
	sec, rl, remaining := liftLLMPolicies(in)
	if sec != nil {
		t.Errorf("security = %+v, want nil", sec)
	}
	if rl != nil {
		t.Errorf("rateLimiting = %+v, want nil", rl)
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %d, want 2", len(remaining))
	}
}
