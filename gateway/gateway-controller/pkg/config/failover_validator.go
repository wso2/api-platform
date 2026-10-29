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

// validateFailoverAgainstRequestTimeout reports a per-attempt timeout that does not fit inside
// the operation's overall request timeout.
//
// Both bounds are in milliseconds. A per-attempt timeout at or above the request timeout means
// a single slow target consumes the entire budget, and the request is cut off before a second
// target is ever attempted — the policy is attached, configured, and silently unable to fail
// over. That is worth refusing at deployment time rather than discovering during an outage.
func validateFailoverAgainstRequestTimeout(params map[string]interface{}, requestTimeoutMs int64, fieldPath string) []ValidationError {
	if params == nil || requestTimeoutMs <= 0 {
		return nil
	}
	perAttempt, ok := failoverNumber(params["perAttemptTimeoutMs"])
	if !ok || perAttempt < requestTimeoutMs {
		return nil
	}
	return []ValidationError{{
		Field: fieldPath + ".perAttemptTimeoutMs",
		Message: fmt.Sprintf(
			"perAttemptTimeoutMs (%d) must be less than the operation's request timeout (%d); "+
				"otherwise one slow target consumes the whole budget and no fallback is ever attempted",
			perAttempt, requestTimeoutMs),
	}}
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
// model-failover is attached ONCE, in the operation's policies, and the controller adds whatever
// per-attempt work it needs. Listing it per-provider would be a category error with a quiet
// failure mode: each provider would carry its own health view and its own chain, so a target
// suspended while serving provider A would still be freely selected on an attempt routed to
// provider B, and the ordering each instance published would overwrite the last (FR-040).
//
// Worth rejecting rather than ignoring, because the authored intent — "fail over from this
// provider" — is reasonable on its face and the configuration would otherwise appear to work.
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
				"operationPolicies and the gateway applies it per attempt. Attaching it per " +
				"upstream would give each provider a separate health view and a separate chain",
		})
	}
	return errors
}
