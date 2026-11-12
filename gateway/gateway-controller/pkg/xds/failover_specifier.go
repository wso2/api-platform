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
	"fmt"
	"log/slog"
	"slices"
	"time"

	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v3"
	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	mutationrules "github.com/envoyproxy/go-control-plane/envoy/config/common/mutation_rules/v3"
	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	extproc "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_proc/v3"
	upstreamcodec "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/upstream_codec/v3"
	hcm "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	prioritygroup "github.com/envoyproxy/go-control-plane/envoy/extensions/router/cluster_specifiers/priority_group/v3"
	upstreamsv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/wso2/api-platform/common/collector"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/constants"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

const (
	// FailoverSpecifierPluginName is the cluster specifier plugin extension registered by
	// Envoy for attempt-indexed group selection. Requires Envoy >= 1.40.0.
	FailoverSpecifierPluginName = "envoy.router.cluster_specifier_plugin.priority_group"

	// FailoverOverrideMetadataNamespace is the dynamic-metadata namespace the model-failover
	// policy writes its per-request priority groups into. The specifier reads the ordered
	// groups from here, which is what lets a routing policy's per-request ranking be obeyed
	// without generating configuration per ordering (FR-021).
	//
	// This namespace is gateway-internal: it must never be derivable from client input, and
	// an inbound header or metadata of the same name is stripped at ingress.
	FailoverOverrideMetadataNamespace = "wso2.failover"

	// failoverGroupNamePrefix prefixes each group name. A group is named for the TARGET it
	// serves, not its rank, because the policy reorders groups per request by name.
	//
	// Naming by rank would be useless: "rank-1" means a different target on every request once
	// a ranking is applied. Naming by target identity lets the policy publish an ordered list
	// of names and have Envoy resolve each to the clusters configured here — so the policy
	// never needs to know cluster names, which it cannot derive anyway (they come from the
	// upstream hostname or upstream-definition name, not the provider alias).
	failoverGroupNamePrefix = "t:"

	// failoverRetryOn always includes retriable-status-codes: without it Envoy never
	// evaluates RetriableStatusCodes, and the configured failover status codes silently
	// do nothing.
	//
	// `reset` is retained deliberately though it is already subsumed by `5xx` (T030l, decided
	// 2026-11-05). Envoy retries a reset under `5xx` independently of `reset` — retry_state_impl.cc
	// carries the comment "Currently we count an upstream reset as a '5xx' (since it will result
	// in one)" — and `gateway-error` behaves the same way. So swapping `reset` for the narrower
	// `reset-before-request`, which retries only while no bytes have reached the upstream, would
	// change nothing at all while reading as though it had.
	//
	// Narrowing it is possible, but not here: it would mean dropping `5xx` AND `gateway-error`
	// and rebuilding the trigger set from `retriable-status-codes` + `connect-failure` +
	// `reset-before-request`. That buys protection against re-sending a prompt a provider has
	// already begun billing for, and costs the most common real failure this feature exists to
	// handle — a provider that accepts the request, starts inference, then dies without
	// responding, leaving the client a hard failure with a healthy fallback sitting idle. The
	// client never sees a double answer in either case (only one response is committed
	// downstream), and paying a second provider is inherent to failover rather than specific to
	// resets, so the retry is kept.
	//
	// Keeping the now-redundant `reset` token is the safer of the two spellings: should `5xx`
	// ever be narrowed, `reset` becomes load-bearing again, where deleting it now would leave a
	// silent trap for whoever does that.
	failoverRetryOn = "5xx,connect-failure,refused-stream,reset,retriable-status-codes"
)

// buildFailoverSpecifierPlugin returns the priority_group cluster specifier plugin for one
// failover-enabled operation.
//
// Each configured target becomes a single-cluster group, in configured order. That order is
// only the default: when the model-failover policy writes priority groups into
// FailoverOverrideMetadataNamespace, Envoy uses those instead, for that request alone. Attempt
// N selects group N; once attempts exceed the group count Envoy stays on the last group rather
// than reaching for an unranked cluster.
//
// Clusters are referenced by name, so each provider keeps its own cluster with its own TLS
// context, credentials and health handling. Nothing about the per-provider clusters changes.
func buildFailoverSpecifierPlugin(pluginName string, cfg *models.FailoverConfig) (*route.ClusterSpecifierPlugin, error) {
	if cfg.Inert() {
		// Nothing resolved: the policy is inert and generates no configuration at all, so the
		// operation behaves exactly as if failover were not attached (FR-030b, FR-030e).
		return nil, nil
	}

	groups := make([]*prioritygroup.PriorityGroup, 0, len(cfg.Targets))
	for _, target := range cfg.Targets {
		if target.Cluster == "" {
			// An unresolved target is skipped, never rejected (FR-006a).
			continue
		}
		groups = append(groups, &prioritygroup.PriorityGroup{
			Name: FailoverGroupName(target.Provider, target.Model),
			Clusters: []*prioritygroup.ClusterWeight{{
				ClusterName: target.Cluster,
				Weight:      wrapperspb.UInt32(1),
			}},
		})
	}
	if len(groups) == 0 {
		return nil, nil
	}

	specifier := &prioritygroup.PriorityGroupClusterSpecifier{
		PriorityGroups:            groups,
		OverrideMetadataNamespace: FailoverOverrideMetadataNamespace,
	}

	typedConfig, err := anypb.New(specifier)
	if err != nil {
		return nil, fmt.Errorf("marshalling failover cluster specifier: %w", err)
	}

	return &route.ClusterSpecifierPlugin{
		Extension: &core.TypedExtensionConfig{
			Name:        pluginName,
			TypedConfig: typedConfig,
		},
	}, nil
}

// FailoverGroupName is the deterministic name for the priority group serving one target.
//
// Both sides depend on this agreeing exactly: the controller names configured groups with it,
// and the model-failover policy publishes the same names in its per-request ordering. A
// mismatch makes Envoy fall back to the configured order silently — the request still
// succeeds, so the only symptom is that the published ranking is quietly ignored.
func FailoverGroupName(provider, model string) string {
	if provider == "" {
		provider = "default"
	}
	return failoverGroupNamePrefix + provider + ":" + model
}

// buildFailoverRetryPolicy returns the route retry policy that drives the attempt sequence.
//
// Two properties here are load-bearing rather than cosmetic:
//
//   - NumRetries is exactly ChainLength-1. This is what guarantees at-most-once (FR-010): a
//     surplus attempt would be served outside the published groups, so the budget must match
//     the chain exactly rather than being a generous round number.
//   - RefreshClusterOnRetry must be true. Without it Envoy keeps the cluster chosen for the
//     first attempt and retries the *same* failing provider, returning its error while the
//     healthy next target receives nothing — a silent no-op that also multiplies load on a
//     backend that is already failing.
//
// RateLimitedRetryBackOff is deliberately never set: it honours the failing provider's
// Retry-After (300s default ceiling) before attempting a *different* provider, which is
// backwards for failover. See TestFailoverRetryPolicy_OmitsRateLimitedBackOff.
func buildFailoverRetryPolicy(cfg *models.FailoverConfig) *route.RetryPolicy {
	if cfg.Inert() {
		return nil
	}

	numRetries := cfg.ChainLength() - 1
	if numRetries < 0 {
		numRetries = 0
	}

	retryPolicy := &route.RetryPolicy{
		RetryOn:               failoverRetryOn,
		NumRetries:            wrapperspb.UInt32(uint32(numRetries)),
		RetriableStatusCodes:  cfg.FailureConditions.StatusCodes,
		RefreshClusterOnRetry: true,
	}

	if cfg.PerAttemptTimeoutMs > 0 {
		retryPolicy.PerTryTimeout = durationpb.New(
			time.Duration(cfg.PerAttemptTimeoutMs) * time.Millisecond)
	}
	// Distinct from PerTryTimeout: this bounds a stalled stream rather than total attempt
	// duration, which is the case that matters for a provider that accepts a streaming
	// request and then stops producing tokens.
	if cfg.PerAttemptIdleTimeoutMs > 0 {
		retryPolicy.PerTryIdleTimeout = durationpb.New(
			time.Duration(cfg.PerAttemptIdleTimeoutMs) * time.Millisecond)
	}

	return retryPolicy
}

// AttemptCountHeader is the response header Envoy populates when a virtual host sets
// IncludeAttemptCountInResponse. It carries the number of upstream attempts made for the
// request.
//
// This is how the model-failover policy attributes an outcome to the right target. The
// downstream response phase sees only the FINAL response — Envoy does not re-run downstream
// filters per attempt — so without the attempt count the policy cannot tell whether the
// response came from the first target or the third. Grading the first target for a response
// the third produced would mean a consistently failing target is recorded healthy every time
// failover rescued the request, and so never suspended.
const AttemptCountHeader = "x-envoy-attempt-count"

// failoverSpecifierPluginName derives a per-operation plugin name. The name is referenced by
// the route's ClusterSpecifierPlugin field and must be unique within the route configuration.
//
// Scoping the plugin to the attaching route is what keeps failover from affecting anything
// else: an API that does not attach the policy — including one routing to the same shared
// provider cluster — generates no plugin, no retry policy, and is byte-for-byte unchanged
// (FR-030c).
func failoverSpecifierPluginName(routeKey string) string {
	return "failover-groups/" + routeKey
}

// buildUpstreamExtProcFilters returns the UPSTREAM HTTP filter chain for the router.
//
// Envoy runs the downstream filter chain once per client request and never re-enters it on a
// retry. Only this chain repeats — once per attempt — so it is the only place that can adapt a
// request for the target it is about to reach: the provider's context path, its request
// transformation, its credential. Probe E measured the invocation pattern directly: one
// downstream call, two upstream calls, for a single client request that failed over once.
//
// Returns nil when upstream processing is off, which leaves the router's filter list empty and
// Envoy's behaviour exactly as before.
func buildUpstreamExtProcFilters(cfg *config.PolicyEngineConfig) ([]*hcm.HttpFilter, error) {
	if cfg == nil || !cfg.UpstreamProcessing {
		return nil, nil
	}

	processor := &extproc.ExternalProcessor{
		GrpcService: &core.GrpcService{
			TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
				EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
					ClusterName: constants.PolicyEngineClusterName,
				},
			},
			Timeout: durationpb.New(time.Duration(cfg.TimeoutMs) * time.Millisecond),
			// Tell the engine which position this stream serves. Both processors dial the
			// same cluster and Envoy keys attributes by canonical filter type, so nothing
			// else in the payload distinguishes them.
			InitialMetadata: []*core.HeaderValue{{
				Key:   constants.PolicyPositionMetadataKey,
				Value: constants.PolicyPositionUpstream,
			}},
		},
		// Fail closed, matching the downstream filter: an attempt that cannot run its
		// per-attempt chain must not reach the provider untransformed or unauthenticated
		// (FR-042).
		FailureModeAllow: false,
		// REQUEST ONLY. The response is handled downstream instead (FR-037a).
		//
		// A response at this position carries no cluster attribute, so resolving its chain
		// means carrying per-stream state from the request phase — and that state is exactly
		// the retry/upstream-filter interaction Agent Router avoids by skipping the response
		// here. The downstream response phase already knows which target answered, from the
		// attempt count, so it can run a provider's response translation with no such state.
		//
		// Bodies default to NONE and are requested per request instead: AllowModeOverride
		// lets the engine ask for a buffered body only when the chain actually needs one, so
		// an attempt whose chain is header-only pays no buffering cost.
		ProcessingMode: &extproc.ProcessingMode{
			RequestHeaderMode:   extproc.ProcessingMode_SEND,
			ResponseHeaderMode:  extproc.ProcessingMode_SKIP,
			RequestBodyMode:     extproc.ProcessingMode_NONE,
			ResponseBodyMode:    extproc.ProcessingMode_NONE,
			RequestTrailerMode:  extproc.ProcessingMode_SKIP,
			ResponseTrailerMode: extproc.ProcessingMode_SKIP,
		},
		AllowModeOverride: true,
		// WITHOUT allow_all_routing, ext_proc silently discards any mutation of :path,
		// :method, :authority, :scheme or host — no error, no log, the rewrite simply does
		// not happen (Probe E). Re-pointing a request at another provider is exactly such a
		// mutation, so this is mandatory rather than optional hardening. It does widen what
		// every upstream policy may rewrite, which is why it is set here deliberately and
		// documented rather than copied from the downstream filter.
		MutationRules: &mutationrules.HeaderMutationRules{
			AllowAllRouting: wrapperspb.Bool(true),
			DisallowIsError: wrapperspb.Bool(true),
		},
		// Target identity for THIS attempt. Probe E confirmed these carry per-attempt values,
		// which removes any need for the policy to infer its target by indexing the published
		// chain with an attempt counter.
		RequestAttributes: []string{
			constants.ExtProcRequestAttributeClusterName,
			constants.ExtProcRequestAttributeUpstreamAddress,
			// The route this attempt belongs to. Lets the engine reach the route's
			// upstream-definition base paths, which is how it re-points an attempt at the
			// provider's own context without any new transport — those paths already travel
			// in the route config.
			constants.ExtProcRequestAttributeRouteName,
		},
		MessageTimeout: durationpb.New(time.Duration(cfg.MessageTimeoutMs) * time.Millisecond),
		MetadataOptions: &extproc.MetadataOptions{
			// The chain published by the downstream pass has to be readable here, since this
			// is where a policy learns what the request was supposed to do.
			ReceivingNamespaces: &extproc.MetadataOptions_MetadataNamespaces{
				Untyped: []string{constants.ExtProcMetadataNamespace, FailoverOverrideMetadataNamespace},
			},
			ForwardingNamespaces: &extproc.MetadataOptions_MetadataNamespaces{
				// wso2.failover travels here too, so a per-attempt policy can read back the
				// ranking the downstream pass published. Without it the only copy reaching
				// this position is the one embedded in the analytics blob, which would tie
				// per-attempt behaviour to an unrelated serialisation format.
				Untyped: []string{constants.ExtProcMetadataNamespace, FailoverOverrideMetadataNamespace},
			},
		},
	}

	processorAny, err := anypb.New(processor)
	if err != nil {
		return nil, fmt.Errorf("marshalling upstream ext_proc config: %w", err)
	}

	codecAny, err := anypb.New(&upstreamcodec.UpstreamCodec{})
	if err != nil {
		return nil, fmt.Errorf("marshalling upstream codec config: %w", err)
	}

	return []*hcm.HttpFilter{
		{
			Name:       constants.UpstreamExtProcFilterName,
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: processorAny},
		},
		{
			// Must be last: upstream_codec terminates an upstream chain the way the router
			// filter terminates a downstream one. Envoy rejects the config without it.
			Name:       constants.UpstreamCodecFilterName,
			ConfigType: &hcm.HttpFilter_TypedConfig{TypedConfig: codecAny},
		},
	}, nil
}

// httpProtocolOptionsName is where a cluster's UPSTREAM HTTP filter chain lives.
const httpProtocolOptionsName = "envoy.extensions.upstreams.http.v3.HttpProtocolOptions"

// attachUpstreamFilters puts the per-attempt filter chain on ONE cluster.
//
// Per cluster rather than on the Router filter, whose `upstream_http_filters` apply to every
// route on the listener (FR-043a). The cluster is the provider here, so a cluster with no
// per-attempt work simply carries no filter and pays nothing — which satisfies FR-043 by
// construction, with no gating mechanism and no opt-in flag needed to protect unrelated traffic.
//
// Merges into any options the cluster already carries: some clusters set explicit HTTP/2 config,
// and overwriting that would silently downgrade them to HTTP/1.
func attachUpstreamFilters(cluster *clusterv3.Cluster, filters []*hcm.HttpFilter) error {
	if cluster == nil || len(filters) == 0 {
		return nil
	}

	options := &upstreamsv3.HttpProtocolOptions{}
	if existing, ok := cluster.GetTypedExtensionProtocolOptions()[httpProtocolOptionsName]; ok {
		if err := existing.UnmarshalTo(options); err != nil {
			return fmt.Errorf("unmarshalling existing protocol options for cluster %q: %w",
				cluster.GetName(), err)
		}
	} else {
		// No options yet: state the protocol explicitly rather than leaving it unset, since
		// HttpProtocolOptions requires an upstream protocol to be chosen.
		options.UpstreamProtocolOptions = &upstreamsv3.HttpProtocolOptions_ExplicitHttpConfig_{
			ExplicitHttpConfig: &upstreamsv3.HttpProtocolOptions_ExplicitHttpConfig{
				ProtocolConfig: &upstreamsv3.HttpProtocolOptions_ExplicitHttpConfig_HttpProtocolOptions{},
			},
		}
	}

	// Replace rather than append, so a re-translation of the same cluster does not stack
	// duplicate filters.
	options.HttpFilters = slices.DeleteFunc(options.HttpFilters, func(f *hcm.HttpFilter) bool {
		return f.GetName() == constants.UpstreamExtProcFilterName ||
			f.GetName() == constants.UpstreamCodecFilterName
	})
	options.HttpFilters = append(options.HttpFilters, filters...)

	optionsAny, err := anypb.New(options)
	if err != nil {
		return fmt.Errorf("marshalling protocol options for cluster %q: %w", cluster.GetName(), err)
	}
	if cluster.TypedExtensionProtocolOptions == nil {
		cluster.TypedExtensionProtocolOptions = map[string]*anypb.Any{}
	}
	cluster.TypedExtensionProtocolOptions[httpProtocolOptionsName] = optionsAny
	return nil
}

// failoverTargetClusters collects every cluster reachable as a failover attempt on this
// deployment.
//
// These need per-attempt processing even with no authored policies, because the engine
// re-points an attempt at its own upstream's base path BEFORE the chain runs. The set is known
// statically: a target that did not resolve to a cluster is dropped at config time, so
// Targets[].Cluster is exactly the set Envoy can select.
func failoverTargetClusters(rdc *models.RuntimeDeployConfig) map[string]bool {
	clusters := make(map[string]bool)
	for _, route := range rdc.Routes {
		if route == nil || route.Failover == nil || route.Failover.Inert() {
			continue
		}
		for _, target := range route.Failover.Targets {
			if target.Cluster != "" {
				clusters[target.Cluster] = true
			}
		}
	}
	return clusters
}

// attachUpstreamChainTo gives a cluster the per-attempt filter chain, but only when that cluster
// actually has per-attempt work to do.
//
// This is what makes FR-043 hold by construction. A cluster that neither carries an upstream
// policy chain nor is reachable as a failover attempt gets no filter, so a request reaching it
// pays no ext_proc round trip — no gating mechanism, no opt-in flag, and no cost to APIs that
// use neither feature.
//
// Participating in a failover chain qualifies on its own, with no authored policies required.
// The engine re-points each attempt at its own upstream's base path before running the chain,
// and that must happen for EVERY attempt — the route's regex_rewrite is compiled once and sends
// all of them to the primary's base path otherwise. An LlmProxy gets the filter regardless,
// since its providers always carry a transformer and auth chain; a RestApi failing over between
// upstream definitions has no such chain, and without this clause its fallback attempts
// silently arrived on the primary's base path (FR-030a).
func (t *Translator) attachUpstreamChainTo(cluster *clusterv3.Cluster, rdc *models.RuntimeDeployConfig) error {
	if cluster == nil || rdc == nil {
		return nil
	}
	chain, hasChain := rdc.UpstreamPolicyChains[cluster.GetName()]
	hasPolicies := hasChain && chain != nil && len(chain.Policies) > 0
	if !hasPolicies && !failoverTargetClusters(rdc)[cluster.GetName()] {
		return nil
	}

	filters, err := buildUpstreamExtProcFilters(&t.routerConfig.PolicyEngine)
	if err != nil {
		return fmt.Errorf("building upstream filters for cluster %q: %w", cluster.GetName(), err)
	}
	if len(filters) == 0 {
		return nil
	}
	if err := attachUpstreamFilters(cluster, filters); err != nil {
		return err
	}
	policyCount := 0
	if hasPolicies {
		policyCount = len(chain.Policies)
	}
	t.logger.Debug("attached per-attempt policy chain to cluster",
		slog.String("cluster", cluster.GetName()),
		slog.Int("policies", policyCount))
	return nil
}

// inertPerTryTimeouts reports per-attempt bounds that cannot fire, because the route's own
// timeout is terminal and no longer than they are.
//
// The three timeouts on a failover route do not compose the way their names suggest. The route
// timeout spans the WHOLE request, retries included, and firing it produces a local 504 — which
// Envoy does not retry, so failover never engages. The per-try bounds are retriable and are the
// ones failover depends on. Setting a per-try bound at or above the route timeout therefore does
// not extend anything: the route timeout fires first and ends the request outright.
//
// That is invisible from the outside. The deployment is accepted, the value appears in the
// configuration an operator reads back, and the only symptom is a request dying at the route
// timeout with no failover when the operator believed they had allowed longer. Surfaced here
// rather than rejected because an ineffective-but-valid value is not a schema error (FR-006);
// rejecting it would also make an API-level timeout change able to break an unrelated
// operation's deployment.
//
// Equality counts as inert, not merely exceeding it: attempt 1 starts when the request does, so
// an equal per-try bound races a timeout that has already been running, and every later attempt
// starts later still.
//
// A route timeout of zero disables it in Envoy, which leaves the per-try bounds as the only
// limits and makes nothing inert.
func inertPerTryTimeouts(failover *models.FailoverConfig, routeTimeout *durationpb.Duration) []string {
	if failover == nil || failover.Inert() || routeTimeout == nil {
		return nil
	}
	routeMs := routeTimeout.AsDuration().Milliseconds()
	if routeMs <= 0 {
		return nil
	}

	var inert []string
	if ms := int64(failover.PerAttemptTimeoutMs); ms > 0 && ms >= routeMs {
		inert = append(inert, fmt.Sprintf(
			"perTryTimeout (%dms) is not below the route timeout (%dms), so the route timeout "+
				"fires first and the request ends with no failover", ms, routeMs))
	}
	if ms := int64(failover.PerAttemptIdleTimeoutMs); ms > 0 && ms >= routeMs {
		inert = append(inert, fmt.Sprintf(
			"perTryIdleTimeout (%dms) is not below the route timeout (%dms), so the route timeout "+
				"fires first and the request ends with no failover", ms, routeMs))
	}
	return inert
}

// failoverGradingResponseFlags is the agreement this sink's filter is built from. It is defined
// in `common` because the policy-engine decodes exactly the same set back out of the entries it
// receives — see collector.FailoverGradingResponseFlags for why the two sides must not drift.
var failoverGradingResponseFlags = collector.FailoverGradingResponseFlags

// failoverResponseFlagFilter scopes an access-log sink to attempts that failed upstream.
//
// This is what makes the failover-driven sink free for healthy traffic: a request the target
// served to completion carries no flag, matches nothing here, and is never serialized, never
// buffered, and never sent (FR-030o). Only the failures the policy needs to see cost anything.
func failoverResponseFlagFilter() *accesslog.AccessLogFilter {
	return &accesslog.AccessLogFilter{
		FilterSpecifier: &accesslog.AccessLogFilter_ResponseFlagFilter{
			ResponseFlagFilter: &accesslog.ResponseFlagFilter{
				Flags: slices.Clone(failoverGradingResponseFlags),
			},
		},
	}
}

// needsALSStream reports whether this translation must stand up the gRPC access-log stream.
//
// Two independent consumers want it — the collector (analytics / traffic logging) and failover
// health grading — and both the ALS *cluster* and the access-log *sink* must follow the same
// answer. They are emitted at two distant call sites, and a sink pointing at a cluster that was
// never emitted is a listener Envoy rejects. One predicate for both removes the drift rather
// than leaving a test to notice it later.
func (t *Translator) needsALSStream() bool {
	return t.config.IsCollectorEnabled() || t.anyRouteHasFailover()
}

// anyRouteHasFailover reports whether this translation produced at least one failover route.
//
// failoverPlugins is reset at the start of every translation and filled while routes are built,
// which is before listeners and clusters are assembled — so this is an accurate listener-wide
// answer at both the ALS-cluster and access-log-config call sites.
func (t *Translator) anyRouteHasFailover() bool {
	return len(t.failoverPlugins) > 0
}
