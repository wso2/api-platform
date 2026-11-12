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

package kernel

import (
	"context"
	"strconv"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	"google.golang.org/grpc/metadata"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

// FilterPosition is which Envoy filter chain an ext_proc stream is serving.
//
// The distinction is not cosmetic. Envoy runs the downstream chain ONCE per client request and
// never re-enters it on a retry; it runs the upstream chain once per attempt. A policy that must
// differ between attempts — provider credentials, request translation, target steering — can
// only do its work from the upstream position, and a policy with external side effects must only
// ever run downstream or it charges the client once per attempt.
type FilterPosition int

const (
	// PositionDownstream runs once per client request. The default: a stream that says
	// nothing about its position is downstream, so an Envoy that predates upstream
	// processing behaves exactly as before.
	PositionDownstream FilterPosition = iota

	// PositionUpstream runs once per upstream attempt, including every failover retry.
	PositionUpstream
)

func (p FilterPosition) String() string {
	if p == PositionUpstream {
		return "upstream"
	}
	return "downstream"
}

// positionFromStream reads the position the router declared for this stream.
//
// Read from gRPC initial metadata rather than inferred from the payload, deliberately. Both
// processors dial the same policy-engine cluster, and Envoy keys ext_proc attributes by the
// filter's CANONICAL type (envoy.filters.http.ext_proc) rather than its instance name — so the
// two streams are indistinguishable by filter name, and the only payload-level difference is
// which attributes each happens to request.
//
// Inferring from that would be a heuristic deciding which policy chain to run, and the upstream
// chain is the one that injects provider credentials. A request shape nobody anticipated would
// silently pick the wrong chain with no error signal (GO-AUTH-017).
//
// An unrecognised value is treated as downstream, which is the safe direction: a per-attempt
// policy that fails to run leaves the attempt unmodified, whereas a once-per-request policy
// wrongly run per attempt would double-charge quota and analytics (FR-012d).
func positionFromStream(ctx context.Context) FilterPosition {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return PositionDownstream
	}
	values := md.Get(constants.PolicyPositionMetadataKey)
	if len(values) == 0 {
		return PositionDownstream
	}
	if values[0] == constants.PolicyPositionUpstream {
		return PositionUpstream
	}
	return PositionDownstream
}

// attemptNumberFromRequest reads which attempt this upstream invocation is serving, from the
// x-envoy-attempt-count request header the virtual host is generated to include.
//
// Returns 1 when absent or unparseable. That is the safe reading: it treats the invocation as
// the first attempt rather than inventing a position in a chain, so a policy indexing anything
// by attempt number stays in range.
func attemptNumberFromRequest(req *extprocv3.ProcessingRequest) int {
	headers := req.GetRequestHeaders()
	if headers == nil {
		return 1
	}
	for _, h := range headers.GetHeaders().GetHeaders() {
		if h.GetKey() != constants.AttemptCountHeader {
			continue
		}
		value := h.GetValue()
		if value == "" && len(h.GetRawValue()) > 0 {
			value = string(h.GetRawValue())
		}
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

// UpstreamChainPrefix namespaces a per-attempt chain's key so a cluster name can never be
// mistaken for a route key.
//
// CROSS-MODULE CONTRACT: the controller publishes chains under this exact prefix
// (pkg/policyxds/snapshot.go). The two are separate Go modules and cannot share the constant, so
// it is duplicated and pinned by a test on each side. A mismatch is SILENT — the engine simply
// finds no chain, the attempt runs unmodified, and the request still succeeds carrying whatever
// the previous attempt left in place. Nothing errors.
const UpstreamChainPrefix = "upstream:"

// UpstreamChainKey is the policy-chain key for the per-attempt chain of one cluster.
func UpstreamChainKey(cluster string) string {
	return UpstreamChainPrefix + cluster
}

// clusterFromRequest reads the cluster Envoy selected for THIS attempt.
//
// Probe E confirmed xds.cluster_name carries per-attempt values — provider_a on attempt 1 and
// provider_b on attempt 2 — which is what lets a per-attempt policy learn its target without
// knowing anything about the failover chain.
//
// Attributes arrive keyed by the filter's CANONICAL type rather than its instance name, so both
// the downstream and upstream processors appear under the same key; which position a stream
// serves is established separately, by positionFromStream.
func clusterFromRequest(req *extprocv3.ProcessingRequest) string {
	if req.GetAttributes() == nil {
		return ""
	}
	attrs, ok := req.GetAttributes()[constants.ExtProcFilter]
	if !ok || attrs.GetFields() == nil {
		return ""
	}
	if value, ok := attrs.GetFields()[constants.ExtProcAttributeClusterName]; ok {
		return value.GetStringValue()
	}
	return ""
}

// UpstreamChainFor returns the per-attempt chain for the cluster this attempt is routed to, or
// nil when that cluster has no upstream work.
//
// A nil result is the ordinary case, not an error: an upstream with nothing to do generates no
// chain at all, so the attempt proceeds unmodified.
func (k *Kernel) UpstreamChainFor(req *extprocv3.ProcessingRequest) *registry.PolicyChain {
	return k.UpstreamChainForCluster(clusterFromRequest(req))
}

// UpstreamChainForCluster is the lookup the body phase needs, because it cannot derive the
// cluster itself -- see upstreamAttemptState.
func (k *Kernel) UpstreamChainForCluster(cluster string) *registry.PolicyChain {
	if cluster == "" {
		return nil
	}
	return k.GetPolicyChainForKey(UpstreamChainKey(cluster))
}

// upstreamAttemptState carries what an attempt's headers phase resolved into that same attempt's
// body phase.
//
// Envoy attaches request_attributes and the metadata context to the FIRST message of an ext_proc
// stream only. A RequestBody message therefore arrives with no xds.cluster_name, so on its own it
// cannot say which provider this attempt targets: the chain lookup finds nothing and a
// transformer that rewrites the payload is silently skipped, sending the fallback provider a
// request in the primary's format.
//
// An upstream stream serves exactly one attempt (Envoy opens a fresh one per retry), so a single
// value per stream is the whole story -- no keying, no eviction.
type upstreamAttemptState struct {
	cluster  string
	metadata map[string]interface{}

	// provider/route base paths this attempt was re-pointed with, so its body phase can compose
	// a policy's path rewrite the same way (a transformer rewrites both payload and path -- e.g.
	// Anthropic's /v1/messages -- and that path still has to land inside the provider's context).
	providerBasePath string
	routeBasePath    string

	// headers as this attempt's HEADERS phase left them. The body phase needs them for two
	// reasons: the executor writes a policy's HeadersToSet straight into this map (a nil one
	// panics), and a body-rewriting policy routinely adjusts headers alongside the payload --
	// content-length, a vendor API version -- which would otherwise be lost.
	headers map[string][]string
}

// cloneHeaderValues copies the header map across phases so a body-phase mutation cannot reach
// back into the header phase's context.
func cloneHeaderValues(src map[string][]string) map[string][]string {
	out := make(map[string][]string, len(src))
	for k, v := range src {
		out[k] = append([]string(nil), v...)
	}
	return out
}
