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
	"io"
	"log/slog"
	"strconv"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpstreamExternalProcessorServer is the second, minimal ext_proc gRPC server
// hosted in this same policy-engine process — wired into Envoy's per-cluster
// UPSTREAM HTTP filter chain (see gateway-controller's Task 8), not the
// per-listener downstream chain ExternalProcessorServer (extproc.go) serves.
// It handles only the request-headers phase: this filter attachment point
// has no sensible response phase or body phase for this feature (see the
// design doc). It resolves route -> policy chain via the exact same
// in-memory registry the downstream server uses (s.kernel.GetPolicyChain),
// so a policy's cached state (e.g. oauth2-generator's token cache) is
// naturally shared between both entry points with zero duplication.
type UpstreamExternalProcessorServer struct {
	extprocv3.UnimplementedExternalProcessorServer
	kernel *Kernel
}

// NewUpstreamExternalProcessorServer constructs the server. k must be the
// same *Kernel instance the downstream ExternalProcessorServer uses (see
// cmd/policy-engine/main.go, Task 4) — this is what makes chain/state sharing
// automatic rather than something this type has to arrange itself.
func NewUpstreamExternalProcessorServer(k *Kernel) *UpstreamExternalProcessorServer {
	return &UpstreamExternalProcessorServer{kernel: k}
}

// Process implements extprocv3.ExternalProcessorServer. Unlike the downstream
// server's Process (extproc.go), this one only ever expects RequestHeaders
// messages (the cluster's upstream filter is configured with
// RequestHeaderMode: SEND and every other mode left at its default NONE, see
// Task 8) — any other message type gets an empty continue response rather
// than an error, since failing this path must never break the retry itself
// (see Global Constraints: fail open).
func (s *UpstreamExternalProcessorServer) Process(stream extprocv3.ExternalProcessor_ProcessServer) error {
	ctx := stream.Context()
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var resp *extprocv3.ProcessingResponse
		switch req.Request.(type) {
		case *extprocv3.ProcessingRequest_RequestHeaders:
			resp, err = s.processRequestHeaders(ctx, req)
			if err != nil {
				slog.ErrorContext(ctx, "upstream ext_proc: failed to process request headers, failing open", "error", err)
				resp = emptyContinueRequestHeadersResponse()
			}
		default:
			resp = emptyContinueRequestHeadersResponse()
		}

		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "upstream ext_proc: failed to send response: %v", err)
		}
	}
}

// emptyContinueRequestHeadersResponse is the fail-open / no-op response: no
// header mutation, request proceeds unchanged.
func emptyContinueRequestHeadersResponse() *extprocv3.ProcessingResponse {
	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{},
			},
		},
	}
}

// processRequestHeaders resolves the route's policy chain and dispatches to
// every policy implementing UpstreamAttemptPolicy, in chain order. A policy
// that doesn't implement it (the common case — rate limiting, analytics,
// transforms) is silently skipped via the type assertion; this is what makes
// the mechanism generic with zero per-policy wiring in this server.
func (s *UpstreamExternalProcessorServer) processRequestHeaders(ctx context.Context, req *extprocv3.ProcessingRequest) (*extprocv3.ProcessingResponse, error) {
	routeKey := extractRouteKeyFromAttributes(req)
	chain := s.kernel.GetPolicyChain(routeKey)
	if chain == nil {
		return emptyContinueRequestHeadersResponse(), nil
	}

	headers := req.GetRequestHeaders()
	attemptCount := 1
	headersMap := make(map[string][]string)
	if headers.GetHeaders() != nil {
		for _, h := range headers.GetHeaders().GetHeaders() {
			key := h.Key
			value := string(h.RawValue)
			headersMap[key] = append(headersMap[key], value)
			if key == "x-envoy-attempt-count" {
				if n, err := strconv.Atoi(value); err == nil && n > 0 {
					attemptCount = n
				}
			}
		}
	}

	actx := &policy.UpstreamAttemptContext{
		AttemptCount: attemptCount,
		Headers:      policy.NewHeaders(headersMap),
	}

	headersToSet := make(map[string]string)
	for _, p := range chain.Policies {
		attemptPolicy, ok := p.(policy.UpstreamAttemptPolicy)
		if !ok {
			continue
		}
		action := attemptPolicy.OnUpstreamAttemptRequestHeaders(ctx, actx)
		mods, ok := action.(policy.UpstreamAttemptHeaderModifications)
		if !ok {
			continue
		}
		for k, v := range mods.HeadersToSet {
			headersToSet[k] = v
		}
	}

	if len(headersToSet) == 0 {
		return emptyContinueRequestHeadersResponse(), nil
	}

	return &extprocv3.ProcessingResponse{
		Response: &extprocv3.ProcessingResponse_RequestHeaders{
			RequestHeaders: &extprocv3.HeadersResponse{
				Response: &extprocv3.CommonResponse{
					HeaderMutation: buildHeaderValueOptions(headersToSet),
				},
			},
		},
	}, nil
}
