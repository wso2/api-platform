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
	management "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// FailoverPolicyName is the policy whose presence on an operation makes that operation
// generate a priority_group cluster specifier and a retry policy.
const FailoverPolicyName = "model-failover"

// Defaults mirror contracts/model-failover-policy.yaml and the policy's own parsing. They are
// duplicated here because the control plane must size the attempt budget and the timeouts
// without loading the policy module, which lives in a separate repository.
const (
	// 0 leaves the route's overall timeout in effect, matching the reviewed schema's
	// perTryTimeout default.
	defaultPerAttemptTimeoutMs = 0

	// per_try_idle_timeout bounds SILENCE within an attempt — and silence means TIME TO
	// FIRST BYTE, not merely a mid-stream stall. Probed 2026-10-30: a target that waited 20s
	// before sending anything was cancelled at 5.3s against a 5s idle timeout, while healthy
	// and about to answer 200 (poc-results.md, "Timeouts").
	//
	// That makes the old 10s default actively wrong for a NON-STREAMING completion, which
	// sends nothing at all until the answer is ready. From the proxy's side that is
	// indistinguishable from a hang, so any non-streaming request over 10s was cancelled and
	// failed over — routine for LLM traffic, and it burned a second target for an answer the
	// first was about to give. Streaming is unaffected either way: its chunks reset the timer.
	//
	// 50s rather than a round number because the value is bounded above, not chosen freely:
	// the route's overall timeout (RouteTimeoutMs, 60s by default) is terminal and fires
	// first, so anything at or above it is inert. T030p covers validating that relationship
	// at deploy time instead of leaving an operator to discover a configured value does
	// nothing. Splitting streaming and non-streaming routes would let both bounds be correct
	// at once; deferred 2026-10-30 as a large change for ~40s of detection latency (T030o).
	defaultPerAttemptIdleTimeoutMs = 50000
	// wildcardModel matches any client-requested model. It must agree with the policy's
	// WildcardModel.
	wildcardModel = "*"

	// ProviderClustersParam carries the provider-to-cluster map the policy needs in order to
	// publish COMPLETE priority groups rather than names alone.
	//
	// Injected by the controller after validation, never authored: the policy's parameter
	// schema sets additionalProperties:false, so an author writing this key would be rejected.
	// The double underscore marks it as belonging to the platform.
	//
	// Without it the policy can only publish group names, and a name resolves only if the
	// controller pre-configured a group for that exact (provider, model). That limits the
	// reachable set to what the control plane anticipated — the deploy-time constraint that
	// ruled out composite clusters in the first place (FR-057, FR-058).
	ProviderClustersParam = "__providerClusters"
)

var defaultFailoverStatusCodes = []uint32{429, 500, 502, 503, 504}

// buildFailoverConfig reads the model-failover policy off an operation's policy list and
// returns the resolved configuration, or nil when the policy is absent.
//
// Resolution is deliberately forgiving. A target that names an unknown model or provider is
// dropped rather than rejected, and a policy left with no usable targets yields nil — the
// operation then generates no specifier and no retry policy, and behaves exactly as if the
// policy were not attached (FR-006a, FR-030b, FR-030e). The gateway rejects no deployment for
// an unresolvable policy reference, and failover must not become the first policy that blocks
// a GitOps deployment over an environment mismatch.
//
// resolveCluster maps a target's provider alias to the upstream cluster serving it. Cluster
// names are derived from the upstream hostname or upstream-definition name, so only the
// control plane can perform this mapping — which is why the generated groups carry the
// clusters and the policy reorders them by group name alone.
func buildFailoverConfig(policies *[]management.Policy, resolveCluster func(provider string) string) *models.FailoverConfig {
	params, found := failoverParams(policies)
	if !found {
		return nil
	}

	cfg := &models.FailoverConfig{
		FailureConditions: models.FailoverFailureConditions{
			StatusCodes: append([]uint32(nil), defaultFailoverStatusCodes...),
		},
		PerAttemptTimeoutMs:     defaultPerAttemptTimeoutMs,
		PerAttemptIdleTimeoutMs: defaultPerAttemptIdleTimeoutMs,
	}

	// statusCodes REPLACES the default list entirely rather than extending it.
	if codes := toUint32Slice(params["statusCodes"]); len(codes) > 0 {
		cfg.FailureConditions.StatusCodes = codes
	}
	// perTryTimeout is authored in SECONDS; the runtime config carries milliseconds.
	if v, ok := toInt(params["perTryTimeout"]); ok && v > 0 {
		cfg.PerAttemptTimeoutMs = v * 1000
	}

	// Every member of every chain needs a priority group, because the controller cannot know
	// which chain a request will select — that depends on the model the client asks for, which
	// is only known at request time. The policy publishes the ordering for the chain it picked;
	// Envoy resolves those names against the groups configured here (FR-045, T024p).
	//
	// De-duplicated by identity: two chains that both fall back to the same (model, provider)
	// share one group, and one group is what a shared health circuit requires.
	seen := make(map[string]struct{})
	for _, member := range flattenChainMembers(params["targets"]) {
		cluster := resolveCluster(member.Provider)
		if cluster == "" {
			// Unresolvable: skipped, never a deployment failure. The rest of the chain
			// still works.
			continue
		}
		member.Cluster = cluster
		key := member.Provider + "\x00" + member.Model
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		cfg.Targets = append(cfg.Targets, member)
	}

	if cfg.Inert() {
		return nil
	}
	return cfg
}

// failoverParams finds the model-failover entry in an operation's policy list and returns its
// parameters. An entry with no params is still an attachment: defaults apply.
func failoverParams(policies *[]management.Policy) (map[string]interface{}, bool) {
	if policies == nil {
		return nil, false
	}
	for _, p := range *policies {
		if p.Name != FailoverPolicyName {
			continue
		}
		if p.Params == nil {
			return map[string]interface{}{}, true
		}
		return *p.Params, true
	}
	return nil, false
}

// flattenChainMembers walks the nested targets[].fallbacks[] shape and returns every member of
// every chain, in authored order.
//
// A chain's own model/provider is its first member; its fallbacks follow. A fallback with no
// provider inherits the CHAIN'S provider, not the proxy primary (FR-048) — the controller has to
// apply the same inheritance rule the policy does, or the two disagree about which cluster a
// fallback belongs to.
//
// A "*" member is kept as-is: it resolves to the client's requested model at request time, so
// the group it needs is the one for (provider, "*"). Its cluster is the provider's, which is all
// the controller has to supply.
func flattenChainMembers(raw interface{}) []models.FailoverTarget {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	members := make([]models.FailoverTarget, 0, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		model, _ := entry["model"].(string)
		if model == "" {
			// No requested-model key means the chain can never be selected.
			continue
		}
		provider, _ := entry["provider"].(string)
		members = append(members, models.FailoverTarget{Model: model, Provider: provider})

		fallbacks, ok := entry["fallbacks"].([]interface{})
		if !ok {
			continue
		}
		for _, f := range fallbacks {
			fe, ok := f.(map[string]interface{})
			if !ok {
				continue
			}
			fModel, _ := fe["model"].(string)
			if fModel == "" {
				continue
			}
			fProvider, _ := fe["provider"].(string)
			if fProvider == "" {
				fProvider = provider
			}
			members = append(members, models.FailoverTarget{Model: fModel, Provider: fProvider})
		}
	}
	return members
}

func toUint32Slice(raw interface{}) []uint32 {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]uint32, 0, len(list))
	for _, item := range list {
		if v, ok := toInt(item); ok && v > 0 {
			out = append(out, uint32(v))
		}
	}
	return out
}

func toInt(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

// BuildProviderClusterMap returns the provider alias to cluster mapping the failover policy needs
// to construct a priority group for any model at request time.
//
// Keyed by the alias an author writes in a target's `provider` field — an LlmProxy
// additionalProviders[].as value, or the provider id when no alias is set. The empty key is the
// API's own main upstream, used by a target that names no provider.
//
// The cluster depends on the PROVIDER alone, never the model, which is what makes this map
// sufficient: a "*" member changes the model and keeps its provider, so the policy can build a
// group for a model nobody enumerated (FR-058, FR-059).
func BuildProviderClusterMap(mainClusterKey string, defs *[]management.UpstreamDefinition,
	kind, uuid string) map[string]interface{} {

	clusters := map[string]interface{}{}
	if mainClusterKey != "" {
		clusters[""] = mainClusterKey
	}
	if defs == nil {
		return clusters
	}
	for _, def := range *defs {
		if len(def.Upstreams) == 0 || def.Upstreams[0].Url == "" {
			continue
		}
		clusters[def.Name] = "upstream_" + kind + "_" + uuid + "_" + SanitizeUpstreamDefinitionName(def.Name)
	}
	return clusters
}

// InjectProviderClusters adds the provider-to-cluster map to the model-failover policy in a
// chain, if the chain carries one. A chain without the policy is left untouched.
func InjectProviderClusters(chain *models.PolicyChain, clusters map[string]interface{}) {
	if chain == nil || len(clusters) == 0 {
		return
	}
	for i := range chain.Policies {
		if chain.Policies[i].Name != FailoverPolicyName {
			continue
		}
		if chain.Policies[i].Params == nil {
			chain.Policies[i].Params = map[string]interface{}{}
		}
		chain.Policies[i].Params[ProviderClustersParam] = clusters
	}
}

// perAttemptPolicies returns the policies that must also run once per retry attempt, at the
// upstream position, for this API.
//
// Only model-failover today, and only when the API actually attaches it. Two reasons not to add
// it unconditionally: a chain containing it declares RequiresRequestBody, so every provider on
// every LlmProxy would start buffering request bodies for a policy with nothing to do; and an
// inert instance is still a gRPC round trip per attempt.
//
// No params. The upstream instance reads the ranking this request actually installed out of
// dynamic metadata and its own target out of the engine's reserved attempt namespace -- it needs
// nothing configured. That is also why one instance serves every operation: the per-operation
// configuration was already consumed by the downstream pass that produced the ranking.
func perAttemptPolicies(operations *[]management.Operation) []models.Policy {
	if operations == nil {
		return nil
	}
	for _, op := range *operations {
		if op.Policies == nil {
			continue
		}
		for _, p := range *op.Policies {
			if p.Name != FailoverPolicyName {
				continue
			}
			// The publisher's own version, not a pinned one: the per-attempt instance is
			// the same policy doing the other half of the same job, so the two must not be
			// able to drift to different implementations within one request.
			return []models.Policy{{
				Name:    FailoverPolicyName,
				Version: p.Version,
				Params:  map[string]interface{}{},
			}}
		}
	}
	return nil
}

// moveFailoverLast puts model-failover at the END of an operation's request chain.
//
// WHY IT IS AUTOMATIC, and not the author's job. This policy consumes routing decisions that
// earlier policies publish into shared request metadata, so it has to run after them. Chain
// order looks like it already says that, but it only half does, and the half it misses is
// invisible in the YAML: the engine runs EVERY policy's OnRequestHeaders before ANY policy's
// OnRequestBody (two separate sweeps — ExecuteRequestHeaderPolicies, then
// ExecuteRequestPolicies), and this policy resolves its chain in the body phase.
//
// So of the eight steering policies in wso2/gateway-controllers, four publish in the header
// phase (llm-header-router, cost-based-model-routing, model-round-robin,
// model-weighted-round-robin) and are honoured wherever they sit, while four publish only in
// the body phase (semantic-, time-based-, intelligent- and typesafe-jev-model-routing) and are
// silently ignored if written after this one. Identical-looking configuration, opposite
// outcomes, decided by which Go interfaces a policy happens to implement. An author cannot see
// that, and the control plane cannot tell them: phases are declared by interface
// implementation, not in the policy-definition YAML the controller loads.
//
// The control plane does know the operation has failover, so it says so itself — the same
// argument prependFailoverFaultEntry makes for the fault chain. Position follows from what the
// policy IS: last on the request path because it consumes what every earlier policy decided,
// first on the fault path because it writes a default they can reshape.
//
// WHAT IT ALSO MOVES. One list drives every phase, and the response phases iterate it in
// REVERSE (ExecuteResponseHeaderPolicies and friends), so last-on-request is first-on-response.
// That is wanted rather than tolerated: grading reads the upstream's own status and headers
// before another response policy reshapes them. The status is unaffected either way — a status
// rewrite propagates only in the response BODY phase, which runs after the whole header phase
// where grading happens — but response HEADERS do propagate, and content-type is what
// distinguishes a provisional streamed 200 from a complete one.
//
// Returns whether anything actually moved, so the caller can say so at generation time rather
// than silently rearranging what an author deliberately wrote.
func moveFailoverLast(failover *models.FailoverConfig, policies []models.Policy) ([]models.Policy, bool) {
	// No resolved failover means the policy is inert for this operation — it publishes nothing
	// and grades nothing — so there is nothing to order around. Same guard as
	// prependFailoverFaultEntry, for the same reason.
	if failover == nil {
		return policies, false
	}

	kept := make([]models.Policy, 0, len(policies))
	var entries []models.Policy
	for _, p := range policies {
		if p.Name == FailoverPolicyName {
			entries = append(entries, p)
			continue
		}
		kept = append(kept, p)
	}
	if len(entries) == 0 {
		return policies, false
	}
	// Already last (the common case once an author follows the documented order): return the
	// original slice untouched so the caller logs nothing.
	if indexOfFirst(policies, FailoverPolicyName) == len(kept) {
		return policies, false
	}
	return append(kept, entries...), true
}

// indexOfFirst reports the position of the first entry with the given name, or -1.
func indexOfFirst(policies []models.Policy, name string) int {
	for i, p := range policies {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// prependFailoverFaultEntry puts model-failover at the FRONT of an operation's fault chain
// whenever that operation actually resolved a failover configuration.
//
// WHY IT IS AUTOMATIC. With policy_engine.fault_policies.handle_upstream_faults enabled, the
// engine skips every response policy for a response of 400 or above and runs the fault chain
// instead. For this policy that is exactly the set of responses it exists to grade, so without
// an entry here health stops being recorded the moment that flag is turned on and no target
// can ever be suspended. Requiring a publisher to list one policy twice, in two different
// blocks, to get one behaviour is a contract nobody would guess; the control plane knows the
// operation has failover and can say so itself.
//
// WHY IT IS FIRST. It writes the exhaustion error as a DEFAULT. An operator's own fault
// entries run after it and may reshape status, body or description — the operator keeps the
// final say — and the fault-path system policies still run last, so analytics records what is
// actually delivered rather than what this entry proposed.
//
// The entry is CLONED from the resolved response-chain entry rather than rebuilt, so the two
// cannot drift: same version resolution, same parameters, and the same injected
// provider-to-cluster map, which is added before this runs and which the policy needs in order
// to resolve a chain at all.
func prependFailoverFaultEntry(failover *models.FailoverConfig, chain, authored []models.Policy) []models.Policy {
	// No resolved failover means the policy is inert for this operation — it publishes
	// nothing and grades nothing — so adding a fault entry would put a policy on the error
	// path of an API that failover does not apply to.
	if failover == nil {
		return authored
	}
	for _, p := range chain {
		if p.Name != FailoverPolicyName {
			continue
		}
		return append([]models.Policy{p}, authored...)
	}
	return authored
}
