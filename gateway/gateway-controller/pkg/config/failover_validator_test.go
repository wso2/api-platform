/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 */

package config

import (
	"testing"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

func TestValidateFailoverParams_IgnoresOtherPolicies(t *testing.T) {
	params := map[string]interface{}{
		"suspendDuration":      float64(0),
		"maxSuspendMultiplier": float64(8),
	}
	if errs := validateFailoverParams("model-round-robin", params, "p"); len(errs) != 0 {
		t.Fatalf("another policy's params must not be checked against failover rules: %v", errs)
	}
}

// A multiplier with no base to multiply is a configuration that silently does nothing.
func TestValidateFailoverParams_RejectsMultiplierWithoutDuration(t *testing.T) {
	params := map[string]interface{}{"maxSuspendMultiplier": float64(8)}
	errs := validateFailoverParams(FailoverPolicyName, params, "policies[0]")
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %v", errs)
	}
	if errs[0].Field != "policies[0].maxSuspendMultiplier" {
		t.Fatalf("field = %q", errs[0].Field)
	}
}

func TestValidateFailoverParams_AcceptsMultiplierWithDuration(t *testing.T) {
	params := map[string]interface{}{
		"suspendDuration":      float64(30),
		"maxSuspendMultiplier": float64(8),
	}
	if errs := validateFailoverParams(FailoverPolicyName, params, "p"); len(errs) != 0 {
		t.Fatalf("a multiplier with a base is valid: %v", errs)
	}
}

// suspendDuration of 0 disables suspension; leaving the multiplier at its default is not an
// error, only setting it above 1 deliberately is.
func TestValidateFailoverParams_DefaultMultiplierWithoutDurationIsFine(t *testing.T) {
	params := map[string]interface{}{"maxSuspendMultiplier": float64(1)}
	if errs := validateFailoverParams(FailoverPolicyName, params, "p"); len(errs) != 0 {
		t.Fatalf("a multiplier of 1 disables doubling and is always valid: %v", errs)
	}
}

// An unresolvable target is dropped at transform time, never rejected here — failover must not
// be the policy that blocks a GitOps rollout over an environment mismatch (FR-006a).
func TestValidateFailoverParams_NeverRejectsUnresolvableTargets(t *testing.T) {
	params := map[string]interface{}{
		"targets": []interface{}{
			map[string]interface{}{
				"model": "gpt-4o",
				"fallbacks": []interface{}{
					map[string]interface{}{"model": "x", "provider": "provider-not-in-this-environment"},
				},
			},
		},
	}
	if errs := validateFailoverParams(FailoverPolicyName, params, "p"); len(errs) != 0 {
		t.Fatalf("an unresolvable provider must not fail validation: %v", errs)
	}
}

func TestValidateFailoverAgainstRequestTimeout(t *testing.T) {
	params := map[string]interface{}{"perAttemptTimeoutMs": float64(30000)}

	if errs := validateFailoverAgainstRequestTimeout(params, 60000, "p"); len(errs) != 0 {
		t.Fatalf("a per-attempt timeout inside the request budget is valid: %v", errs)
	}
	// Equal is already too large: the first attempt alone can consume the whole budget.
	if errs := validateFailoverAgainstRequestTimeout(params, 30000, "p"); len(errs) != 1 {
		t.Fatalf("expected one error for an equal timeout, got %v", errs)
	}
	if errs := validateFailoverAgainstRequestTimeout(params, 15000, "p"); len(errs) != 1 {
		t.Fatalf("expected one error for a per-attempt timeout above the request timeout, got %v", errs)
	}
	// An unknown request timeout is not grounds for an error; there is nothing to compare to.
	if errs := validateFailoverAgainstRequestTimeout(params, 0, "p"); len(errs) != 0 {
		t.Fatalf("expected no error when the request timeout is unknown: %v", errs)
	}
}

// model-failover belongs in operationPolicies, once. Attaching it per upstream looks reasonable
// — "fail over from this provider" — and would appear to deploy fine, which is exactly why it
// has to be rejected rather than ignored: each provider would carry its own health view and its
// own chain, so a target suspended while serving provider A would still be selected on an
// attempt routed to provider B.
func TestValidateUpstreamPolicies_RejectsFailoverOnAnUpstream(t *testing.T) {
	policies := []api.Policy{
		{Name: "set-headers"},
		{Name: FailoverPolicyName},
	}
	errs := ValidateUpstreamPolicies(&policies, "spec.additionalProviders[0].policies")
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %v", errs)
	}
	if errs[0].Field != "spec.additionalProviders[0].policies[1].name" {
		t.Fatalf("field = %q, want the offending index", errs[0].Field)
	}
}

func TestValidateUpstreamPolicies_AllowsOrdinaryPolicies(t *testing.T) {
	policies := []api.Policy{{Name: "set-headers"}, {Name: "pii-masking-regex"}}
	if errs := ValidateUpstreamPolicies(&policies, "p"); len(errs) != 0 {
		t.Fatalf("ordinary per-attempt policies must be allowed: %v", errs)
	}
	if errs := ValidateUpstreamPolicies(nil, "p"); len(errs) != 0 {
		t.Fatalf("an upstream with no policies is valid: %v", errs)
	}
}
