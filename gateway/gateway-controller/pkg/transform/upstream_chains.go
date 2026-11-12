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

package transform

import (
	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/utils"
)

// buildUpstreamPolicyChains compiles the per-attempt chain for every provider on an LlmProxy,
// keyed by the alias an attempt will be routed to.
//
// ORDER IS A SECURITY PROPERTY, not a preference (FR-039):
//
//  1. the author's own policies for this provider
//  2. that provider's transformer
//  3. that provider's authentication — ALWAYS last
//
// Authentication goes last so that no author-written policy ever observes the credential. An
// author policy placed after it could read the provider's key out of the request headers and
// copy it anywhere it liked; placed before, there is nothing yet to read. The ordering is fixed
// by the controller rather than authored, so a publisher cannot reorder their way into seeing it.
//
// The transformer sits between them because it reshapes the request body for the provider's API,
// and the credential must be applied to the request that is actually going to be sent.
// Keyed by CLUSTER NAME, not by provider alias. Envoy tells the engine which cluster an attempt
// is routed to (xds.cluster_name, confirmed per-attempt by Probe E), so keying by cluster makes
// the runtime lookup direct. Keying by alias would force the engine to carry a cluster-to-alias
// table and translate on every attempt, for no gain.
func buildUpstreamPolicyChains(spec *api.LLMProxyConfigData, clusters map[string]interface{},
	transformerFor, authFor func(alias string) *models.Policy,
	perAttempt []models.Policy) map[string]*models.PolicyChain {

	if spec == nil {
		return nil
	}

	chains := map[string]*models.PolicyChain{}

	// An alias with no cluster cannot receive an attempt, so a chain for it could never run.
	clusterFor := func(alias string) string {
		if cluster, ok := clusters[alias].(string); ok {
			return cluster
		}
		return ""
	}

	// Built from the NORMALISED attachments, never from a raw provider shape. Reading
	// spec.AdditionalProviders directly meant a proxy authored in the canonical
	// `providers` form got no per-attempt chain for any non-primary provider — so a
	// failover attempt reached that provider with neither its transformer nor its
	// credential, silently, which is the exact failure this feature exists to prevent.
	attachments, err := models.NormaliseLLMProxyAttachments(*spec)
	if err != nil {
		// A shape this malformed is rejected by validation before translation. Should
		// one reach here, no chains is the safe answer: a partial set built from a
		// half-understood shape would route attempts to providers whose credential and
		// transformer were silently dropped.
		return nil
	}

	for _, attachment := range attachments {
		// The primary provider is reached by a target that names no provider, which is the
		// empty alias — matching BuildProviderClusterMap's keying so the runtime can look
		// up either by the same string.
		alias := ""
		if !attachment.IsPrimary {
			alias = attachment.EffectiveName()
		}
		addChain(chains, clusterFor(alias), attachment.Policies,
			transformerFor(alias), authFor(alias), perAttempt)
	}

	if len(chains) == 0 {
		return nil
	}
	return chains
}

// addChain assembles one provider's chain in the fixed order and records it only when it has
// something to run — an upstream with no per-attempt work should generate no chain at all, so
// an attempt routed there pays nothing (FR-043).
func addChain(chains map[string]*models.PolicyChain, cluster string,
	authored *[]api.Policy, transformer, auth *models.Policy, perAttempt []models.Policy) {

	if cluster == "" {
		// No cluster resolved for this provider, so no attempt can be routed to it and the
		// chain could never run. Dropped rather than stored under an empty key, which would
		// collide across providers.
		return
	}

	policies := make([]models.Policy, 0, 4)

	// First, always. A per-attempt policy decides what this attempt is FOR -- the model it
	// asks for -- and everything after it, the author's own policies included, reads the
	// payload it leaves behind. A transformer translating before the model was substituted
	// would faithfully translate the wrong request.
	policies = append(policies, perAttempt...)

	if authored != nil {
		for _, p := range *authored {
			params := map[string]interface{}{}
			if p.Params != nil {
				params = *p.Params
			}
			policies = append(policies, models.Policy{
				Name:               p.Name,
				Version:            p.Version,
				Params:             params,
				ExecutionCondition: p.ExecutionCondition,
			})
		}
	}
	if transformer != nil {
		policies = append(policies, *transformer)
	}
	// Last, always.
	if auth != nil {
		policies = append(policies, *auth)
	}

	if len(policies) == 0 {
		return
	}
	chains[cluster] = &models.PolicyChain{Policies: policies}
}

// transformerFor and authFor adapt the LLM layer's per-provider policies to the lookups
// buildUpstreamPolicyChains expects. Named fields, not list positions: a provider with a
// credential and no transformer would otherwise have its credential applied as a translator.
func transformerFor(perProvider map[string]utils.ProviderPolicies) func(string) *models.Policy {
	return func(alias string) *models.Policy {
		return toModelPolicy(perProvider[alias].Transformer)
	}
}

func authFor(perProvider map[string]utils.ProviderPolicies) func(string) *models.Policy {
	return func(alias string) *models.Policy {
		return toModelPolicy(perProvider[alias].Auth)
	}
}

func toModelPolicy(p *api.Policy) *models.Policy {
	if p == nil {
		return nil
	}
	params := map[string]interface{}{}
	if p.Params != nil {
		params = *p.Params
	}
	return &models.Policy{Name: p.Name, Version: p.Version, Params: params}
}
