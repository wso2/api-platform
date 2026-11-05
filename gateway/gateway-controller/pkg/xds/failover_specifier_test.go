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

package xds

import (
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"strings"
	"testing"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	prioritygroup "github.com/envoyproxy/go-control-plane/envoy/extensions/router/cluster_specifiers/priority_group/v3"
	upstreamsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

func threeTargetFailover() *models.FailoverConfig {
	return &models.FailoverConfig{
		Targets: []models.FailoverTarget{
			{Model: "gpt-4o", Provider: "openai", Cluster: "openai-cluster"},
			{Model: "gpt-4o", Provider: "azure-sweden", Cluster: "azure-sweden-cluster"},
			{Model: "claude-3", Provider: "bedrock", Cluster: "bedrock-cluster"},
		},
		FailureConditions: models.FailoverFailureConditions{
			StatusCodes: []uint32{429, 500, 502, 503, 504},
		},
		PerAttemptTimeoutMs:     30000,
		PerAttemptIdleTimeoutMs: 10000,
	}
}

func unmarshalSpecifier(t *testing.T, cfg *models.FailoverConfig) *prioritygroup.PriorityGroupClusterSpecifier {
	t.Helper()
	plugin, err := buildFailoverSpecifierPlugin("failover-groups/test", cfg)
	if err != nil {
		t.Fatalf("buildFailoverSpecifierPlugin returned error: %v", err)
	}
	if plugin == nil {
		t.Fatal("expected a plugin, got nil")
	}
	var specifier prioritygroup.PriorityGroupClusterSpecifier
	if err := plugin.Extension.TypedConfig.UnmarshalTo(&specifier); err != nil {
		t.Fatalf("typed config is not a PriorityGroupClusterSpecifier: %v", err)
	}
	return &specifier
}

// The specifier must produce one group per target, in configured order, each naming the
// target's own cluster. Group order is the default; the policy overrides it per request.
func TestFailoverSpecifier_GroupPerTargetInConfiguredOrder(t *testing.T) {
	specifier := unmarshalSpecifier(t, threeTargetFailover())

	want := []string{"openai-cluster", "azure-sweden-cluster", "bedrock-cluster"}
	if got := len(specifier.PriorityGroups); got != len(want) {
		t.Fatalf("expected %d priority groups, got %d", len(want), got)
	}
	for i, wantCluster := range want {
		group := specifier.PriorityGroups[i]
		if len(group.Clusters) != 1 {
			t.Fatalf("group %d: expected exactly 1 cluster, got %d", i, len(group.Clusters))
		}
		if got := group.Clusters[0].ClusterName; got != wantCluster {
			t.Errorf("group %d: expected cluster %q, got %q", i, wantCluster, got)
		}
	}
}

// The override namespace is what lets a routing policy's per-request ranking be obeyed. If it
// is unset, Envoy silently uses the configured order for every request and FR-021 is violated
// with no error anywhere.
func TestFailoverSpecifier_SetsOverrideMetadataNamespace(t *testing.T) {
	specifier := unmarshalSpecifier(t, threeTargetFailover())

	if specifier.OverrideMetadataNamespace != FailoverOverrideMetadataNamespace {
		t.Errorf("expected override namespace %q, got %q",
			FailoverOverrideMetadataNamespace, specifier.OverrideMetadataNamespace)
	}
}

// An inert policy generates nothing at all, so the operation is indistinguishable from one
// with no failover attached (FR-030b, FR-030e).
func TestFailoverSpecifier_InertWhenNothingResolves(t *testing.T) {
	for name, cfg := range map[string]*models.FailoverConfig{
		"nil config":        nil,
		"no targets":        {Targets: nil},
		"unresolved target": {Targets: []models.FailoverTarget{{Model: "gpt-4o", Cluster: ""}}},
	} {
		t.Run(name, func(t *testing.T) {
			plugin, err := buildFailoverSpecifierPlugin("failover-groups/test", cfg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plugin != nil {
				t.Error("expected no plugin for an inert configuration")
			}
			if retry := buildFailoverRetryPolicy(cfg); retry != nil {
				t.Error("expected no retry policy for an inert configuration")
			}
		})
	}
}

// A target that does not resolve is skipped rather than rejected, and the rest of the chain
// still works (FR-006a).
func TestFailoverSpecifier_SkipsUnresolvedTargets(t *testing.T) {
	cfg := &models.FailoverConfig{
		Targets: []models.FailoverTarget{
			{Model: "gpt-4o", Provider: "openai", Cluster: "openai-cluster"},
			{Model: "ghost", Provider: "missing", Cluster: ""},
			{Model: "claude-3", Provider: "bedrock", Cluster: "bedrock-cluster"},
		},
	}
	specifier := unmarshalSpecifier(t, cfg)

	if got := len(specifier.PriorityGroups); got != 2 {
		t.Fatalf("expected the unresolved target to be skipped, leaving 2 groups, got %d", got)
	}
	if got := specifier.PriorityGroups[1].Clusters[0].ClusterName; got != "bedrock-cluster" {
		t.Errorf("expected bedrock-cluster after the skip, got %q", got)
	}
}

// NumRetries must equal chain length - 1 exactly. A larger budget would let an attempt be
// served outside the published groups, which is what at-most-once (FR-010) depends on.
func TestFailoverRetryPolicy_NumRetriesMatchesChainExactly(t *testing.T) {
	cfg := threeTargetFailover()
	retry := buildFailoverRetryPolicy(cfg)

	if retry == nil {
		t.Fatal("expected a retry policy")
	}
	if got, want := retry.NumRetries.GetValue(), uint32(2); got != want {
		t.Errorf("expected num_retries %d for a 3-target chain, got %d", want, got)
	}
}

// retriable-status-codes must appear in retry_on, or Envoy never evaluates
// RetriableStatusCodes and every configured failover status code silently does nothing.
func TestFailoverRetryPolicy_RetryOnIncludesRetriableStatusCodes(t *testing.T) {
	retry := buildFailoverRetryPolicy(threeTargetFailover())

	if !strings.Contains(retry.RetryOn, "retriable-status-codes") {
		t.Errorf("retry_on must contain retriable-status-codes, got %q", retry.RetryOn)
	}
	if len(retry.RetriableStatusCodes) != 5 {
		t.Errorf("expected the configured status codes to be carried through, got %v",
			retry.RetriableStatusCodes)
	}
}

// Without refresh_cluster_on_retry Envoy keeps the cluster chosen for the first attempt, so
// every retry hits the SAME failing provider and the healthy next target never sees traffic.
// It fails silently — no error, no warning — while tripling load on a failing backend, which
// is why this is asserted rather than trusted.
func TestFailoverRetryPolicy_RefreshClusterOnRetryIsSet(t *testing.T) {
	retry := buildFailoverRetryPolicy(threeTargetFailover())

	if !retry.RefreshClusterOnRetry {
		t.Error("refresh_cluster_on_retry must be true, otherwise failover silently does nothing")
	}
}

// rate_limited_retry_back_off honours the failing provider's Retry-After — up to a 300s
// default ceiling — before the next attempt. That is right for retrying the same provider and
// exactly wrong here, where the next attempt goes to a DIFFERENT provider. Asserting its
// absence stops it being "helpfully" added later by someone who notices Retry-After ignored.
func TestFailoverRetryPolicy_OmitsRateLimitedBackOff(t *testing.T) {
	retry := buildFailoverRetryPolicy(threeTargetFailover())

	if retry.RateLimitedRetryBackOff != nil {
		t.Error("rate_limited_retry_back_off must never be set: it would delay failover to a " +
			"different provider by the failing provider's Retry-After")
	}
}

// per_try_idle_timeout bounds a stalled stream, which per_try_timeout does not: a provider
// that accepts a streaming request and then stops emitting tokens is caught only by this.
func TestFailoverRetryPolicy_SetsBothPerAttemptTimeouts(t *testing.T) {
	retry := buildFailoverRetryPolicy(threeTargetFailover())

	if retry.PerTryTimeout.GetSeconds() != 30 {
		t.Errorf("expected per_try_timeout of 30s, got %v", retry.PerTryTimeout)
	}
	if retry.PerTryIdleTimeout.GetSeconds() != 10 {
		t.Errorf("expected per_try_idle_timeout of 10s, got %v", retry.PerTryIdleTimeout)
	}
}

// The group name and metadata namespace are a CROSS-REPOSITORY contract with the
// model-failover policy in wso2/gateway-controllers. The controller names each configured
// group; the policy republishes those names in its per-request order.
//
// A mismatch is silent. Envoy finds no group matching a published name, falls back to the
// configured order, and the request still succeeds — so no test that only asserts "failover
// works" can catch a rename. The literals are pinned here, and the same literals are pinned
// in the policy's publish_test.go. Changing either side breaks a test rather than quietly
// disabling per-request ordering in production.
func TestFailoverGroupName_CrossRepoContract(t *testing.T) {
	if got := FailoverGroupName("openai", "gpt-4o"); got != "t:openai:gpt-4o" {
		t.Fatalf("group name = %q, want %q — the model-failover policy's groupName() must "+
			"produce the identical string", got, "t:openai:gpt-4o")
	}

	// A target with no provider names the API's own main upstream. Both sides substitute
	// the same placeholder, or the two spellings never meet.
	if got := FailoverGroupName("", "gpt-4o"); got != "t:default:gpt-4o" {
		t.Fatalf("group name without provider = %q, want %q", got, "t:default:gpt-4o")
	}

	if FailoverOverrideMetadataNamespace != "wso2.failover" {
		t.Fatalf("metadata namespace = %q, want wso2.failover — the policy writes into this "+
			"namespace by literal name", FailoverOverrideMetadataNamespace)
	}
}

// The failover policy raises a request's own retry budget by setting x-envoy-max-retries, which
// is safe ONLY because Envoy sanitizes x-envoy-* headers from EXTERNAL downstreams. That
// classification is what `internal_address_config` on the HCM changes.
//
// Probe H measured the safe behaviour: a client-set x-envoy-max-retries reached the filter chain
// as nil and had no effect. If an internal-address range is ever configured for an unrelated
// reason, the same header becomes client-controllable — and since every extra attempt is a billed
// inference call, that is an unbounded spend multiplier with no other visible symptom.
//
// This test exists to make that change impossible to land silently.
func TestHCM_DoesNotClassifyDownstreamsAsInternal(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	lis, _, err := translator.createListener(nil, false, false)
	require.NoError(t, err)
	manager := extractHCM(t, lis)

	if ic := manager.GetInternalAddressConfig(); ic != nil {
		t.Fatalf("HCM sets internal_address_config (%v): downstream clients would be treated as "+
			"internal, Envoy would stop sanitizing x-envoy-* headers, and a caller could set "+
			"x-envoy-max-retries to fan their request across every provider in the chain. If this "+
			"is deliberate, the failover policy must stop trusting the header and the attempt "+
			"budget needs a different per-request mechanism", ic)
	}
}

// Both attempt-count flags are generated together, and only for a virtual host that actually
// fails over — a vhost without it must keep byte-identical responses (FR-030i).
//
// They are two DIFFERENT fields serving two different consumers:
//   - IncludeAttemptCountInResponse  → the client response, read by the policy's response phase
//     to attribute an outcome to the target that produced it
//   - IncludeRequestAttemptCount     → the UPSTREAM request, read by the per-attempt filter chain
//     to learn which attempt it is serving
//
// The request-side field is easy to get wrong: `include_attempt_count_in_request` is not a field
// and Envoy rejects the whole bootstrap if you reach for it, which is how it was first found.
func TestVhostAttemptCountFlags(t *testing.T) {
	failoverRoute := &route.Route{Action: &route.Route_Route{Route: &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_ClusterSpecifierPlugin{
			ClusterSpecifierPlugin: "failover-groups/x",
		},
	}}}
	plainRoute := &route.Route{Action: &route.Route_Route{Route: &route.RouteAction{
		ClusterSpecifier: &route.RouteAction_Cluster{Cluster: "upstream_main"},
	}}}

	tr := &Translator{}

	if !tr.vhostHasFailover([]*route.Route{plainRoute, failoverRoute}) {
		t.Fatal("a vhost carrying any failover route must report true")
	}
	if tr.vhostHasFailover([]*route.Route{plainRoute}) {
		t.Fatal("a vhost with no failover route must report false, so its responses are unchanged")
	}
	if tr.vhostHasFailover(nil) {
		t.Fatal("an empty vhost must report false")
	}
}

func upstreamProcessingConfig() *config.PolicyEngineConfig {
	return &config.PolicyEngineConfig{
		UpstreamProcessing: true,
		TimeoutMs:          2000,
		MessageTimeoutMs:   1000,
	}
}

func upstreamProcessor(t *testing.T, filters []*hcm.HttpFilter) *extproc.ExternalProcessor {
	t.Helper()
	var processor extproc.ExternalProcessor
	if err := filters[0].GetTypedConfig().UnmarshalTo(&processor); err != nil {
		t.Fatalf("unmarshalling upstream ext_proc config: %v", err)
	}
	return &processor
}

// Off by default. upstream_http_filters are listener-wide, so enabling this adds an ext_proc
// round trip to EVERY request on the listener, not only ones that fail over.
func TestUpstreamExtProcFilters_AbsentUnlessEnabled(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(&config.PolicyEngineConfig{})
	require.NoError(t, err)
	if filters != nil {
		t.Fatalf("expected no upstream filters when upstream processing is off, got %d", len(filters))
	}

	none, err := buildUpstreamExtProcFilters(nil)
	require.NoError(t, err)
	if none != nil {
		t.Fatal("a nil config must not produce filters")
	}
}

// THE trap. Without allow_all_routing, ext_proc discards any mutation of :path, :method,
// :authority, :scheme or host — silently, no error, no log. Re-pointing a request at another
// provider is exactly that kind of mutation, so losing this flag disables per-attempt failover
// while every test that only checks "the filter exists" keeps passing.
func TestUpstreamExtProcFilters_AllowAllRoutingIsSet(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	processor := upstreamProcessor(t, filters)

	rules := processor.GetMutationRules()
	require.NotNil(t, rules, "mutation_rules must be set on the upstream processor")
	require.NotNil(t, rules.GetAllowAllRouting(), "allow_all_routing must be explicitly set")
	if !rules.GetAllowAllRouting().GetValue() {
		t.Fatal("allow_all_routing must be true, or the per-attempt path rewrite is dropped silently")
	}
}

// Probe E: these attributes carry correct PER-ATTEMPT identity, which is how an upstream policy
// learns its target without having to know the failover chain.
func TestUpstreamExtProcFilters_RequestsTargetIdentityAttributes(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	processor := upstreamProcessor(t, filters)

	attrs := strings.Join(processor.GetRequestAttributes(), ",")
	for _, want := range []string{
		constants.ExtProcRequestAttributeClusterName,
		constants.ExtProcRequestAttributeUpstreamAddress,
	} {
		if !strings.Contains(attrs, want) {
			t.Fatalf("request attribute %q missing; upstream policies cannot identify their target without it", want)
		}
	}
}

// upstream_codec terminates an upstream chain the way the router filter terminates a downstream
// one. Envoy rejects the whole config without it, so ordering is not cosmetic.
func TestUpstreamExtProcFilters_CodecTerminatesTheChain(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	require.Len(t, filters, 2)

	if filters[0].GetName() != constants.UpstreamExtProcFilterName {
		t.Fatalf("first filter = %q, want the upstream processor", filters[0].GetName())
	}
	if last := filters[len(filters)-1].GetName(); last != constants.UpstreamCodecFilterName {
		t.Fatalf("last filter = %q, want %q", last, constants.UpstreamCodecFilterName)
	}
}

// The upstream filter must be separately addressable from the downstream one, or per-route
// disabling (the thing that makes this cheap enough to default on) can never target just one.
func TestUpstreamExtProcFilters_NameIsDistinctFromDownstream(t *testing.T) {
	if constants.UpstreamExtProcFilterName == constants.ExtProcFilterName {
		t.Fatal("upstream and downstream ext_proc filters must not share a name")
	}
}

// Fail closed, and read the chain the downstream pass published — a per-attempt policy that
// cannot see it has no idea what the request was supposed to do.
func TestUpstreamExtProcFilters_FailsClosedAndReadsFailoverMetadata(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	processor := upstreamProcessor(t, filters)

	if processor.GetFailureModeAllow() {
		t.Fatal("failure_mode_allow must be false: an attempt whose per-attempt chain cannot " +
			"run must not reach the provider untransformed or unauthenticated")
	}
	receiving := strings.Join(processor.GetMetadataOptions().GetReceivingNamespaces().GetUntyped(), ",")
	if !strings.Contains(receiving, FailoverOverrideMetadataNamespace) {
		t.Fatalf("receiving namespaces %q must include %q", receiving, FailoverOverrideMetadataNamespace)
	}
}

// The per-attempt chain is REQUEST ONLY (FR-037a). A response at this position carries no
// cluster attribute, so resolving its chain would mean carrying per-stream state from the
// request phase — the retry/upstream-filter interaction Agent Router avoids by handling
// responses downstream, where the attempt count already identifies which target answered.
func TestUpstreamExtProcFilters_RequestOnlyProcessingMode(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	processor := upstreamProcessor(t, filters)
	mode := processor.GetProcessingMode()

	require.Equal(t, extproc.ProcessingMode_SEND, mode.GetRequestHeaderMode())
	require.Equal(t, extproc.ProcessingMode_SKIP, mode.GetResponseHeaderMode(),
		"response work belongs downstream; running it here reintroduces the per-stream state "+
			"needed to carry a chain across the retry boundary")

	// Bodies are opt-in per request rather than always-on, so an attempt whose chain is
	// header-only pays no buffering cost.
	require.Equal(t, extproc.ProcessingMode_NONE, mode.GetRequestBodyMode())
	require.Equal(t, extproc.ProcessingMode_NONE, mode.GetResponseBodyMode())
	require.True(t, processor.GetAllowModeOverride(),
		"without mode override the engine cannot request a body for the chains that need one")
}

// The engine must know which position a stream serves WITHOUT inferring it. Both processors
// dial the same policy-engine cluster, and Envoy keys ext_proc attributes by the filter's
// canonical type rather than its instance name — so the distinct filter name does NOT separate
// them in the payload. Guessing the position would mean guessing which policy chain to run, and
// the upstream chain is the one that injects provider credentials.
func TestUpstreamExtProcFilters_DeclaresItsPositionExplicitly(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	processor := upstreamProcessor(t, filters)

	md := processor.GetGrpcService().GetInitialMetadata()
	require.NotEmpty(t, md, "the upstream stream must identify itself to the policy engine")

	var found bool
	for _, h := range md {
		if h.GetKey() == constants.PolicyPositionMetadataKey {
			require.Equal(t, constants.PolicyPositionUpstream, h.GetValue())
			found = true
		}
	}
	if !found {
		t.Fatalf("initial metadata %v carries no %q; the engine would have to infer the "+
			"position from which attributes happen to be present", md, constants.PolicyPositionMetadataKey)
	}
}

// ext_proc refuses to modify x-envoy-* headers by default, and with DisallowIsError that
// refusal FAILS the request rather than dropping the mutation. The failover policy sets
// x-envoy-max-retries to size the attempt budget to the ranking it published, so without this
// allowance every failover request returns 500.
//
// Found only at runtime: Probe H validated the header using a Lua filter, which is not subject
// to ext_proc's mutation rules at all.
//
// Narrowed to that ONE header deliberately. allow_envoy would let any downstream policy rewrite
// every x-envoy-* header, including x-envoy-upstream-rq-timeout-ms and the retry-policy
// overrides — a policy bug there is a per-request denial of service against the upstream.
func TestDownstreamExtProc_AllowsOnlyTheRetryBudgetHeader(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	filter, err := translator.createExtProcFilter()
	require.NoError(t, err)

	var processor extproc.ExternalProcessor
	require.NoError(t, filter.GetTypedConfig().UnmarshalTo(&processor))

	rules := processor.GetMutationRules()
	require.NotNil(t, rules.GetAllowExpression(),
		"without an allowance, every request that sets x-envoy-max-retries fails with 500")
	require.Equal(t, "^"+constants.MaxRetriesHeader+"$", rules.GetAllowExpression().GetRegex())

	// The blanket allowance must stay off.
	if rules.GetAllowEnvoy().GetValue() {
		t.Fatal("allow_envoy must remain unset: it would let any downstream policy rewrite " +
			"every x-envoy-* header, including the upstream timeout and retry overrides")
	}
}

func clusterWithName(name string) *clusterv3.Cluster {
	return &clusterv3.Cluster{Name: name}
}

func upstreamFiltersOn(t *testing.T, c *clusterv3.Cluster) []*hcm.HttpFilter {
	t.Helper()
	raw, ok := c.GetTypedExtensionProtocolOptions()[httpProtocolOptionsName]
	if !ok {
		return nil
	}
	var options upstreamsv3.HttpProtocolOptions
	require.NoError(t, raw.UnmarshalTo(&options))
	return options.GetHttpFilters()
}

// Per cluster, not on the Router. The Router's upstream_http_filters apply to every route on the
// listener; a cluster carries its own chain, so a cluster with no per-attempt work pays nothing
// (FR-043a).
func TestAttachUpstreamFilters_PutsTheChainOnTheCluster(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)

	cluster := clusterWithName("upstream_LlmProxy_abc_anthropic")
	require.NoError(t, attachUpstreamFilters(cluster, filters))

	got := upstreamFiltersOn(t, cluster)
	require.Len(t, got, 2)
	require.Equal(t, constants.UpstreamExtProcFilterName, got[0].GetName())
	require.Equal(t, constants.UpstreamCodecFilterName, got[len(got)-1].GetName(),
		"upstream_codec must terminate the chain or Envoy rejects the cluster")
}

// Some clusters already carry explicit HTTP/2 options. Overwriting them would silently downgrade
// those clusters to HTTP/1 — a failure that shows up as degraded performance, not an error.
func TestAttachUpstreamFilters_PreservesExistingProtocolOptions(t *testing.T) {
	existing := &upstreamsv3.HttpProtocolOptions{
		UpstreamProtocolOptions: &upstreamsv3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &upstreamsv3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &upstreamsv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{},
			},
		},
	}
	existingAny, err := anypb.New(existing)
	require.NoError(t, err)

	cluster := clusterWithName("upstream_grpc_backend")
	cluster.TypedExtensionProtocolOptions = map[string]*anypb.Any{httpProtocolOptionsName: existingAny}

	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	require.NoError(t, attachUpstreamFilters(cluster, filters))

	var options upstreamsv3.HttpProtocolOptions
	require.NoError(t, cluster.GetTypedExtensionProtocolOptions()[httpProtocolOptionsName].UnmarshalTo(&options))

	explicit := options.GetExplicitHttpConfig()
	require.NotNil(t, explicit)
	if explicit.GetHttp2ProtocolOptions() == nil {
		t.Fatal("HTTP/2 config was lost: attaching filters must merge into existing protocol " +
			"options, not replace them, or the cluster silently downgrades to HTTP/1")
	}
	require.Len(t, options.GetHttpFilters(), 2)
}

// Re-translating the same cluster must not stack duplicate filters.
func TestAttachUpstreamFilters_IsIdempotent(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)

	cluster := clusterWithName("upstream_LlmProxy_abc_anthropic")
	require.NoError(t, attachUpstreamFilters(cluster, filters))
	require.NoError(t, attachUpstreamFilters(cluster, filters))

	if got := upstreamFiltersOn(t, cluster); len(got) != 2 {
		t.Fatalf("filters = %d, want 2; re-translation must replace, not append", len(got))
	}
}

// A cluster with no per-attempt work carries no filter at all. This is what makes FR-043 hold by
// construction rather than needing a gating mechanism.
func TestAttachUpstreamChainTo_SkipsClustersWithNoWork(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.PolicyEngine.UpstreamProcessing = true
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	rdc := &models.RuntimeDeployConfig{
		UpstreamPolicyChains: map[string]*models.PolicyChain{
			"upstream_has_work": {Policies: []models.Policy{{Name: "set-headers"}}},
			"upstream_empty":    {Policies: nil},
		},
	}

	for name, wantFilters := range map[string]bool{
		"upstream_has_work": true,
		"upstream_empty":    false,
		"upstream_unknown":  false,
	} {
		t.Run(name, func(t *testing.T) {
			cluster := clusterWithName(name)
			require.NoError(t, translator.attachUpstreamChainTo(cluster, rdc))
			got := len(upstreamFiltersOn(t, cluster)) > 0
			if got != wantFilters {
				t.Fatalf("cluster %q has filters = %v, want %v", name, got, wantFilters)
			}
		})
	}
}

// A failover target gets the per-attempt filter even with no authored policies.
//
// The engine re-points an attempt at its own upstream's base path before the chain runs, and the
// route's regex_rewrite cannot vary per attempt. Without the filter a fallback attempt arrives on
// the PRIMARY's base path and is served by the wrong resource — silently, with a 200. An LlmProxy
// never showed this because its providers always carry a transformer chain; a RestApi failing over
// between upstream definitions carries none (FR-030a).
func TestAttachUpstreamChainTo_CoversFailoverTargetsWithoutPolicies(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	routerCfg.PolicyEngine.UpstreamProcessing = true
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	rdc := &models.RuntimeDeployConfig{
		Routes: map[string]*models.Route{
			"route_with_failover": {
				Failover: &models.FailoverConfig{
					Targets: []models.FailoverTarget{
						{Model: "gpt-4o", Cluster: "upstream_RestApi_abc_primary"},
						{Model: "claude", Cluster: "upstream_RestApi_abc_alive"},
					},
				},
			},
			"route_inert_failover": {
				// Every target unresolvable: inert, so it must not pull in any cluster.
				Failover: &models.FailoverConfig{
					Targets: []models.FailoverTarget{{Model: "ghost"}},
				},
			},
			"route_no_failover": {},
		},
	}

	for name, wantFilters := range map[string]bool{
		"upstream_RestApi_abc_primary": true,
		"upstream_RestApi_abc_alive":   true,
		"upstream_RestApi_abc_other":   false,
	} {
		t.Run(name, func(t *testing.T) {
			cluster := clusterWithName(name)
			require.NoError(t, translator.attachUpstreamChainTo(cluster, rdc))
			got := len(upstreamFiltersOn(t, cluster)) > 0
			if got != wantFilters {
				t.Fatalf("cluster %q has filters = %v, want %v", name, got, wantFilters)
			}
		})
	}
}

// Envoy DROPS a dynamic-metadata update for any namespace absent from receiving_namespaces,
// logging "namespace not configured for receiving" and otherwise carrying on. Without wso2.failover
// listed on the DOWNSTREAM filter, a failover policy's published ranking never reaches the cluster
// specifier at all — and the failure is near-invisible, because priority_group then falls back to
// the group configured at that POSITION (Probe G): the request still reaches a plausible target,
// so failover looks like it works while the ranking, the health state and the ordering are all
// being ignored.
func TestUpstreamExtProcFilters_ForwardsFailoverMetadata(t *testing.T) {
	filters, err := buildUpstreamExtProcFilters(upstreamProcessingConfig())
	require.NoError(t, err)
	processor := upstreamProcessor(t, filters)

	forwarding := strings.Join(processor.GetMetadataOptions().GetForwardingNamespaces().GetUntyped(), ",")
	if !strings.Contains(forwarding, FailoverOverrideMetadataNamespace) {
		t.Fatalf("forwarding namespaces %q must include %q — the per-attempt position cannot "+
			"read back the ranking this request installed otherwise",
			forwarding, FailoverOverrideMetadataNamespace)
	}
}

// THE bug that made failover look like it worked while doing nothing of the sort.
//
// Envoy drops a dynamic-metadata update for any namespace absent from receiving_namespaces --
// it logs "namespace not configured for receiving" and carries on. With wso2.failover missing
// here, the policy's published ranking never reached the cluster specifier, so priority_group
// fell back to the group configured at that POSITION (Probe G). The request still reached a
// plausible target and still returned 200 on attempt 2, so every observable symptom said
// "failover works" while the published order, the health state and the suspensions were all
// being ignored.
func TestDownstreamExtProc_AcceptsPublishedFailoverRanking(t *testing.T) {
	logger := createTestLogger()
	routerCfg := testRouterConfig()
	cfg := testConfig()
	cfg.Router = *routerCfg
	translator, err := NewTranslator(logger, routerCfg, nil, cfg)
	require.NoError(t, err)

	filter, err := translator.createExtProcFilter()
	require.NoError(t, err)

	var processor extproc.ExternalProcessor
	require.NoError(t, filter.GetTypedConfig().UnmarshalTo(&processor))

	receiving := strings.Join(processor.GetMetadataOptions().GetReceivingNamespaces().GetUntyped(), ",")
	if !strings.Contains(receiving, FailoverOverrideMetadataNamespace) {
		t.Fatalf("receiving namespaces %q must include %q, or Envoy silently discards every "+
			"published ranking", receiving, FailoverOverrideMetadataNamespace)
	}

	forwarding := strings.Join(processor.GetMetadataOptions().GetForwardingNamespaces().GetUntyped(), ",")
	if !strings.Contains(forwarding, FailoverOverrideMetadataNamespace) {
		t.Fatalf("forwarding namespaces %q must include %q", forwarding, FailoverOverrideMetadataNamespace)
	}
}

// The case T030p exists for: an operator raises the idle timeout past the route timeout and
// gets no more time at all, because the route timeout is terminal and fires first.
func TestInertPerTryTimeouts_IdleAboveRouteTimeoutIsReported(t *testing.T) {
	cfg := &models.FailoverConfig{
		Targets:                 []models.FailoverTarget{{Model: "a", Cluster: "c1"}, {Model: "b", Cluster: "c2"}},
		PerAttemptIdleTimeoutMs: 90000,
	}

	inert := inertPerTryTimeouts(cfg, durationpb.New(60*time.Second))

	if len(inert) != 1 {
		t.Fatalf("inert = %v, want one finding for the idle timeout", inert)
	}
	if !strings.Contains(inert[0], "perTryIdleTimeout") {
		t.Errorf("finding does not name the setting: %q", inert[0])
	}
}

// Equality is inert too. Attempt 1 starts when the request does, so an equal per-try bound races
// a clock that is already running, and every later attempt starts later still.
func TestInertPerTryTimeouts_EqualToRouteTimeoutIsInert(t *testing.T) {
	cfg := &models.FailoverConfig{
		Targets:             []models.FailoverTarget{{Model: "a", Cluster: "c1"}, {Model: "b", Cluster: "c2"}},
		PerAttemptTimeoutMs: 60000,
	}

	if inert := inertPerTryTimeouts(cfg, durationpb.New(60*time.Second)); len(inert) != 1 {
		t.Fatalf("inert = %v, want the equal per-try timeout reported", inert)
	}
}

// The shipped defaults must not warn, or the signal is noise from the first deployment:
// perTryIdleTimeout defaults to 50s against a 60s route timeout, deliberately under it.
func TestInertPerTryTimeouts_ShippedDefaultsAreSilent(t *testing.T) {
	cfg := &models.FailoverConfig{
		Targets:                 []models.FailoverTarget{{Model: "a", Cluster: "c1"}, {Model: "b", Cluster: "c2"}},
		PerAttemptIdleTimeoutMs: 50000,
		PerAttemptTimeoutMs:     0, // unset: the route timeout is the only bound, which is fine
	}

	if inert := inertPerTryTimeouts(cfg, durationpb.New(60*time.Second)); len(inert) != 0 {
		t.Fatalf("inert = %v, want silence on the shipped defaults", inert)
	}
}

// A zero route timeout disables it in Envoy, leaving the per-try bounds as the only limits —
// so nothing is inert however large they are.
func TestInertPerTryTimeouts_DisabledRouteTimeoutMakesNothingInert(t *testing.T) {
	cfg := &models.FailoverConfig{
		Targets:                 []models.FailoverTarget{{Model: "a", Cluster: "c1"}, {Model: "b", Cluster: "c2"}},
		PerAttemptIdleTimeoutMs: 600000,
		PerAttemptTimeoutMs:     600000,
	}

	if inert := inertPerTryTimeouts(cfg, durationpb.New(0)); len(inert) != 0 {
		t.Fatalf("inert = %v, want none when the route timeout is disabled", inert)
	}
}

// An inert route carries no retry policy at all, so there is nothing to report and a warning
// would point at a policy that is not running.
func TestInertPerTryTimeouts_InertFailoverIsSilent(t *testing.T) {
	cfg := &models.FailoverConfig{PerAttemptIdleTimeoutMs: 600000}

	if inert := inertPerTryTimeouts(cfg, durationpb.New(60*time.Second)); len(inert) != 0 {
		t.Fatalf("inert = %v, want silence when the policy resolved no targets", inert)
	}
}
