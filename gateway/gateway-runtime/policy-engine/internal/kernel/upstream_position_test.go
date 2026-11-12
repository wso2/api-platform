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
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/constants"
)

func TestPositionFromStream_UpstreamIsDeclaredNotInferred(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		constants.PolicyPositionMetadataKey, constants.PolicyPositionUpstream))

	if got := positionFromStream(ctx); got != PositionUpstream {
		t.Fatalf("position = %v, want upstream", got)
	}
}

// Every ambiguous case resolves to downstream. That is the safe direction: a per-attempt policy
// that fails to run leaves the attempt unmodified, whereas a once-per-request policy wrongly run
// per attempt double-charges quota, rate limits and analytics (FR-012d).
func TestPositionFromStream_DefaultsToDownstream(t *testing.T) {
	cases := map[string]context.Context{
		"no metadata at all": context.Background(),
		"metadata without the key": metadata.NewIncomingContext(
			context.Background(), metadata.Pairs("x-something-else", "value")),
		"unrecognised value": metadata.NewIncomingContext(
			context.Background(), metadata.Pairs(constants.PolicyPositionMetadataKey, "sideways")),
		"empty value": metadata.NewIncomingContext(
			context.Background(), metadata.Pairs(constants.PolicyPositionMetadataKey, "")),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			if got := positionFromStream(ctx); got != PositionDownstream {
				t.Fatalf("position = %v, want downstream", got)
			}
		})
	}
}

func requestWithHeaders(pairs ...string) *extprocv3.ProcessingRequest {
	headers := make([]*corev3.HeaderValue, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		headers = append(headers, &corev3.HeaderValue{Key: pairs[i], Value: pairs[i+1]})
	}
	return &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: headers},
			},
		},
	}
}

func TestAttemptNumberFromRequest(t *testing.T) {
	if got := attemptNumberFromRequest(requestWithHeaders(constants.AttemptCountHeader, "3")); got != 3 {
		t.Fatalf("attempt = %d, want 3", got)
	}

	// Absent, unparseable or nonsensical all read as the first attempt, so a policy indexing
	// a chain by attempt number stays in range rather than inventing a position in it.
	for name, req := range map[string]*extprocv3.ProcessingRequest{
		"header absent":      requestWithHeaders("content-type", "application/json"),
		"unparseable":        requestWithHeaders(constants.AttemptCountHeader, "lots"),
		"zero":               requestWithHeaders(constants.AttemptCountHeader, "0"),
		"negative":           requestWithHeaders(constants.AttemptCountHeader, "-2"),
		"not a header phase": {},
	} {
		t.Run(name, func(t *testing.T) {
			if got := attemptNumberFromRequest(req); got != 1 {
				t.Fatalf("attempt = %d, want 1", got)
			}
		})
	}
}

// Envoy may send a header as raw bytes rather than a string value.
func TestAttemptNumberFromRequest_ReadsRawValue(t *testing.T) {
	req := &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{Headers: []*corev3.HeaderValue{{
					Key:      constants.AttemptCountHeader,
					RawValue: []byte("2"),
				}}},
			},
		},
	}
	if got := attemptNumberFromRequest(req); got != 2 {
		t.Fatalf("attempt = %d, want 2 from the raw value", got)
	}
}

// CROSS-MODULE CONTRACT with the controller's pkg/policyxds UpstreamChainKey. The two are
// separate Go modules and cannot share the constant, so the literal is pinned on both sides.
//
// A mismatch is SILENT: the engine finds no chain, the attempt runs unmodified, and the request
// still succeeds carrying whatever the previous attempt left in place. No test that merely
// asserts "failover works" would catch it, which is why this one pins the exact string.
func TestUpstreamChainKey_CrossModuleContract(t *testing.T) {
	if UpstreamChainPrefix != "upstream:" {
		t.Fatalf("prefix = %q, want %q — the controller publishes chains under this exact "+
			"string in pkg/policyxds/snapshot.go", UpstreamChainPrefix, "upstream:")
	}
	if got := UpstreamChainKey("upstream_LlmProxy_abc_anthropic"); got != "upstream:upstream_LlmProxy_abc_anthropic" {
		t.Fatalf("key = %q", got)
	}
}

func requestWithClusterAttribute(cluster string) *extprocv3.ProcessingRequest {
	return &extprocv3.ProcessingRequest{
		Attributes: map[string]*structpb.Struct{
			constants.ExtProcFilter: {
				Fields: map[string]*structpb.Value{
					constants.ExtProcAttributeClusterName: structpb.NewStringValue(cluster),
				},
			},
		},
	}
}

func TestClusterFromRequest(t *testing.T) {
	if got := clusterFromRequest(requestWithClusterAttribute("upstream_azure")); got != "upstream_azure" {
		t.Fatalf("cluster = %q, want upstream_azure", got)
	}

	// Every absent case yields "", which callers treat as "no per-attempt work" rather than
	// an error — an upstream with nothing to do generates no chain at all.
	for name, req := range map[string]*extprocv3.ProcessingRequest{
		"no attributes":    {},
		"wrong filter key": {Attributes: map[string]*structpb.Struct{"other.filter": {Fields: map[string]*structpb.Value{}}}},
		"no cluster field": {Attributes: map[string]*structpb.Struct{constants.ExtProcFilter: {Fields: map[string]*structpb.Value{}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := clusterFromRequest(req); got != "" {
				t.Fatalf("cluster = %q, want empty", got)
			}
		})
	}
}
