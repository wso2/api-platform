/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package kernel

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/registry"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

// fakeUpstreamAttemptPolicy proves the dispatch loop invokes exactly the
// policies implementing UpstreamAttemptPolicy, via type assertion, ignoring
// every other policy in the chain (rate-limit/analytics-shaped policies that
// don't implement it).
type fakeUpstreamAttemptPolicy struct{ lastAttempt int }

func (p *fakeUpstreamAttemptPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }
func (p *fakeUpstreamAttemptPolicy) OnUpstreamAttemptRequestHeaders(_ context.Context, actx *policy.UpstreamAttemptContext) policy.UpstreamAttemptAction {
	p.lastAttempt = actx.AttemptCount
	if actx.AttemptCount <= 1 {
		return policy.UpstreamAttemptHeaderModifications{}
	}
	return policy.UpstreamAttemptHeaderModifications{HeadersToSet: map[string]string{"Authorization": "Bearer refreshed"}}
}

// nonParticipatingPolicy implements only the base Policy interface — proves
// the dispatch loop skips it via type assertion, not a hardcoded name check.
type nonParticipatingPolicy struct{}

func (nonParticipatingPolicy) Mode() policy.ProcessingMode { return policy.ProcessingMode{} }

// newTestRouteConfigAndChain builds a Kernel with a policy chain registered
// under routeKey, using the package's real chain-registration entry point
// (RegisterRoute — see mapper.go and its use throughout kernel_test.go /
// body_mode_test.go / extproc_test.go) rather than a new test-only setter:
// processRequestHeaders only ever reads the chain via Kernel.GetPolicyChain,
// which RegisterRoute already populates, so no additional mechanism is
// needed.
func newTestRouteConfigAndChain(t *testing.T, routeKey string, chain *registry.PolicyChain) *Kernel {
	t.Helper()
	k := NewKernel()
	k.RegisterRoute(routeKey, chain)
	return k
}

func attrsFor(routeKey string) map[string]*structpb.Struct {
	return map[string]*structpb.Struct{
		"envoy.filters.http.ext_proc": {
			Fields: map[string]*structpb.Value{
				"xds.route_name": structpb.NewStringValue(routeKey),
			},
		},
	}
}

func TestUpstreamExtProc_DispatchesOnlyToImplementingPolicies(t *testing.T) {
	fp := &fakeUpstreamAttemptPolicy{}
	chain := &registry.PolicyChain{Policies: []policy.Policy{nonParticipatingPolicy{}, fp}}
	k := newTestRouteConfigAndChain(t, "test-route", chain)
	s := NewUpstreamExternalProcessorServer(k)

	req := &extprocv3.ProcessingRequest{
		Attributes: attrsFor("test-route"),
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{
					{Key: "x-envoy-attempt-count", RawValue: []byte("2")},
				}},
			},
		},
	}

	resp, err := s.processRequestHeaders(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp.lastAttempt != 2 {
		t.Fatalf("expected the implementing policy to observe AttemptCount=2, got %d", fp.lastAttempt)
	}
	rh, ok := resp.Response.(*extprocv3.ProcessingResponse_RequestHeaders)
	if !ok {
		t.Fatalf("expected a RequestHeaders response, got %T", resp.Response)
	}
	mutation := rh.RequestHeaders.GetResponse().GetHeaderMutation()
	if mutation == nil || len(mutation.SetHeaders) != 1 || string(mutation.SetHeaders[0].Header.RawValue) != "Bearer refreshed" {
		t.Fatalf("expected the refreshed Authorization header to be set, got %#v", mutation)
	}
}

func TestUpstreamExtProc_MissingAttemptCountHeaderTreatedAsOne(t *testing.T) {
	fp := &fakeUpstreamAttemptPolicy{}
	chain := &registry.PolicyChain{Policies: []policy.Policy{fp}}
	k := newTestRouteConfigAndChain(t, "test-route", chain)
	s := NewUpstreamExternalProcessorServer(k)

	req := &extprocv3.ProcessingRequest{
		Attributes: attrsFor("test-route"),
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		},
	}
	if _, err := s.processRequestHeaders(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fp.lastAttempt != 1 {
		t.Fatalf("expected a missing attempt-count header to be treated as attempt 1, got %d", fp.lastAttempt)
	}
}

func TestUpstreamExtProc_UnknownRouteReturnsEmptyContinue(t *testing.T) {
	k := NewKernel()
	s := NewUpstreamExternalProcessorServer(k)
	req := &extprocv3.ProcessingRequest{
		Attributes: attrsFor("no-such-route"),
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{Headers: &corev3.HeaderMap{}},
		},
	}
	resp, err := s.processRequestHeaders(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rh, ok := resp.Response.(*extprocv3.ProcessingResponse_RequestHeaders)
	if !ok || rh.RequestHeaders.GetResponse().GetHeaderMutation() != nil {
		t.Fatalf("expected an empty continue response for an unknown route, got %#v", resp.Response)
	}
}
