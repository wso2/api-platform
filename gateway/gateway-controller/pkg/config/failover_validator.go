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
	"fmt"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
)

// FailoverPolicyName is the policy these checks apply to.
const FailoverPolicyName = "model-failover"

// validateFailoverParams checks the model-failover constraints that the policy's own JSON
// schema cannot express.
//
// Everything expressible as a single-field rule — minimums, maximums, required properties,
// additionalProperties — is already enforced generically from the policy definition, so this
// function deliberately covers only the cross-field relationships, which a JSON schema has no
// way to state.
//
// What it must NOT do is reject an unresolvable model or provider reference. A target naming a
// provider that is not deployed in this environment is dropped from the chain at transform time
// and the rest of the chain still works (FR-006a). Failing the deployment instead would make
// failover the one policy that blocks a GitOps rollout over an environment mismatch.
func validateFailoverParams(policyName string, params map[string]interface{}, fieldPath string) []ValidationError {
	if policyName != FailoverPolicyName || params == nil {
		return nil
	}

	var errors []ValidationError

	// The reviewed schema expresses the ceiling as maxSuspendMultiplier, a MULTIPLE of
	// suspendDuration rather than an absolute value, so the "ceiling below the base" mistake
	// is now unrepresentable — the schema's own minimum of 1 covers it. What remains worth
	// checking is a multiplier set without any base to multiply.
	multiplier, multiplierOK := failoverNumber(params["maxSuspendMultiplier"])
	duration, durationOK := failoverNumber(params["suspendDuration"])
	if multiplierOK && multiplier > 1 && (!durationOK || duration <= 0) {
		errors = append(errors, ValidationError{
			Field: fieldPath + ".maxSuspendMultiplier",
			Message: "maxSuspendMultiplier has no effect without a non-zero suspendDuration; " +
				"suspendDuration of 0 disables suspension tracking entirely",
		})
	}

	return errors
}

// failoverNumber reads a schema-declared integer out of the params map. JSON decoding yields
// float64, template coercion can yield int, and both must be accepted.
func failoverNumber(raw interface{}) (int64, bool) {
	switch v := raw.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case float32:
		return int64(v), true
	default:
		return 0, false
	}
}

// ValidateUpstreamPolicies rejects model-failover in an upstream's own policies list.
//
// model-failover is attached ONCE — in an operation's policies, or at the API level — and the
// controller adds whatever per-attempt work it needs (perAttemptPolicies, pkg/transform). There
// is nothing here for an author to add (FR-040).
//
// THE REASON IS PLACEMENT, NOT SAFETY. A provider's policies list runs at the upstream position,
// which Envoy enters once per attempt — AFTER it has already selected the cluster for that
// attempt. A policy there cannot change where the attempt goes: that was settled by the priority
// groups the downstream chain published. "Fail over from this provider" is therefore
// unimplementable at the only position this field offers.
//
// What such a configuration actually does is nothing. buildFailoverConfig reads the operation and
// API policy lists, never a provider's, so no route failover configuration, no cluster specifier
// and no retry policy are generated; the authored instance then runs at the upstream position,
// detects the per-attempt namespace, and finds no published ranking to apply. Worth rejecting
// rather than ignoring, because the authored intent — "fail over from this provider" — is
// reasonable on its face and the configuration would otherwise appear to work.
//
// NOT the reason, deliberately: FR-012f considered and WITHDREW the argument that a per-provider
// attachment is unsafe because it repeats work. Each provider's upstream chain runs only on
// attempts routed to that provider, so a per-provider policy is correctly scoped rather than
// duplicated. This refusal is specific to model-failover and rests on FR-040 alone.
func ValidateUpstreamPolicies(policies *[]api.Policy, fieldPath string) []ValidationError {
	if policies == nil {
		return nil
	}
	var errors []ValidationError
	for i, p := range *policies {
		if p.Name != FailoverPolicyName {
			continue
		}
		errors = append(errors, ValidationError{
			Field: fmt.Sprintf("%s[%d].name", fieldPath, i),
			Message: "model-failover cannot be attached to an upstream; attach it once in " +
				"operationPolicies or at the API level, and the gateway applies it per attempt. " +
				"An upstream's policies run after this attempt's target has already been " +
				"selected, so failover cannot steer from there",
		})
	}
	return errors
}
