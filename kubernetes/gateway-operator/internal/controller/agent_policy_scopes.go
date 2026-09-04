/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// agentPolicyRef is one Policy embedded in an Agent spec, together with a
// human-readable scope label used in error messages.
type agentPolicyRef struct {
	// Scope names the spec location, e.g. "a2a.agentCard.public".
	Scope string
	// Policy points into the caller's spec so a resolver can rewrite params
	// in place.
	Policy *apiv1.Policy
}

// agentPolicyRefs returns every Policy embedded in spec.
//
// Unlike RestApi (one API-level list plus per-operation lists), an Agent
// carries policies in three unrelated scopes: the common operation chain, the
// per-operation chains, and the public Agent Card chain. Every path that must
// see all of them — valueFrom resolution at deploy time, the
// external-dependency fingerprint, and the Secret/ConfigMap watch predicates —
// goes through this one function, so a scope added later cannot be wired into
// some of those paths and silently missed by the rest.
func agentPolicyRefs(spec *apiv1.AgentConfigData) []agentPolicyRef {
	if spec == nil {
		return nil
	}
	ops := &spec.A2A.OperationConfigs
	refs := make([]agentPolicyRef, 0, len(ops.Policies)+len(ops.Operations)+len(spec.A2A.AgentCard.Public.Policies))

	for i := range ops.Policies {
		refs = append(refs, agentPolicyRef{
			Scope:  "a2a.operationConfigs",
			Policy: &ops.Policies[i],
		})
	}
	for i := range ops.Operations {
		scope := "a2a.operationConfigs.operations[" + string(ops.Operations[i].Name) + "]"
		for j := range ops.Operations[i].Policies {
			refs = append(refs, agentPolicyRef{
				Scope:  scope,
				Policy: &ops.Operations[i].Policies[j],
			})
		}
	}
	card := &spec.A2A.AgentCard.Public
	for i := range card.Policies {
		refs = append(refs, agentPolicyRef{
			Scope:  "a2a.agentCard.public",
			Policy: &card.Policies[i],
		})
	}
	return refs
}
