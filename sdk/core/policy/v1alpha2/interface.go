/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 *
 */

// Package policyv1alpha2 provides the policy interface for the WSO2 API Gateway.
//
// Policies declare phase participation via Mode(), which is the authoritative
// source for all six processing phases. Phase-specific sub-interfaces provide
// the method implementations; the kernel uses type assertions only for method
// dispatch and streaming capability validation at chain-build time.
package policyv1alpha2

import "context"

// Policy is the base interface all policies must implement.
// Mode declares the policy's processing requirements for each phase; the kernel
// uses this at chain-build time to configure buffering and ext_proc modes.
// Capabilities are declared by additionally implementing the phase-specific
// sub-interfaces below.
type Policy interface {
	Mode() ProcessingMode
}

// PolicyFactory is the function signature for creating policy instances.
// Policy implementations must export a GetPolicy function with this signature:
//
//	func GetPolicy(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)
type PolicyFactory func(metadata PolicyMetadata, params map[string]interface{}) (Policy, error)

// PolicyMetadata contains metadata passed to GetPolicy for instance creation.
type PolicyMetadata struct {
	// RouteName is the unique identifier for the route this policy is attached to.
	RouteName string

	// APIId is the unique identifier of the API this policy belongs to.
	APIId string

	// APIName is the name of the API this policy belongs to.
	APIName string

	// APIVersion is the version of the API this policy belongs to.
	APIVersion string

	// AttachedTo indicates where the policy is attached (e.g., LevelAPI, LevelRoute).
	AttachedTo Level
}

// Level defines the attachment level of a policy.
type Level string

const (
	LevelAPI   Level = "api"
	LevelRoute Level = "route"
)

// ─── Processing mode ─────────────────────────────────────────────────────────

// ProcessingMode declares a policy's processing requirements for each phase.
// The kernel uses this at chain-build time to decide whether to buffer bodies
// and which Envoy ext_proc modes to request.
type ProcessingMode struct {
	RequestHeaderMode  HeaderProcessingMode
	RequestBodyMode    BodyProcessingMode
	ResponseHeaderMode HeaderProcessingMode
	ResponseBodyMode   BodyProcessingMode
}

// HeaderProcessingMode defines how a policy processes headers.
type HeaderProcessingMode string

const (
	// HeaderModeSkip — don't process headers; the phase method is not called.
	HeaderModeSkip HeaderProcessingMode = "SKIP"

	// HeaderModeProcess — process headers.
	HeaderModeProcess HeaderProcessingMode = "PROCESS"
)

// BodyProcessingMode defines how a policy processes body content.
type BodyProcessingMode string

const (
	// BodyModeSkip — don't process body; the phase method is not called.
	BodyModeSkip BodyProcessingMode = "SKIP"

	// BodyModeBuffer — buffer the complete body before invoking OnRequestBody/OnResponseBody.
	BodyModeBuffer BodyProcessingMode = "BUFFER"

	// BodyModeStream — process body in streaming chunks.
	BodyModeStream BodyProcessingMode = "STREAM"
)

// ─── Phase-specific sub-interfaces ───────────────────────────────────────────

// RequestHeaderPolicy processes request headers.
// Implement this to modify or inspect headers before the request body is read.
type RequestHeaderPolicy interface {
	OnRequestHeaders(ctx context.Context, reqCtx *RequestHeaderContext, params map[string]interface{}) RequestHeaderAction
}

// ResponseHeaderPolicy processes response headers.
// Implement this to modify or inspect response headers before the response body is read.
type ResponseHeaderPolicy interface {
	OnResponseHeaders(ctx context.Context, respCtx *ResponseHeaderContext, params map[string]interface{}) ResponseHeaderAction
}

// RequestPolicy processes the complete buffered request body.
// If any policy in the chain implements this interface, the request body is
// buffered before any policy in the chain executes.
type RequestPolicy interface {
	OnRequestBody(ctx context.Context, reqCtx *RequestContext, params map[string]interface{}) RequestAction
}

// ResponsePolicy processes the complete buffered response body.
// If any policy in the chain implements only ResponsePolicy (not
// StreamingResponsePolicy), the entire chain uses BUFFERED mode.
type ResponsePolicy interface {
	OnResponseBody(ctx context.Context, respCtx *ResponseContext, params map[string]interface{}) ResponseAction
}

// StreamingRequestPolicy processes the request body chunk-by-chunk.
// RequestPolicy is embedded as a buffered fallback — the kernel calls
// OnRequestBody when streaming is not possible (e.g. the chain has a
// non-streaming policy). NeedsMoreRequestData is called after each chunk;
// return true to accumulate before OnRequestBodyChunk is invoked.
//
// Error handling limitation: if an error occurs after one or more chunks have
// already been forwarded to upstream, the upstream connection is already in
// progress and the error cannot be cleanly surfaced — the stream will be
// aborted. Policies that hold per-stream resources should handle cleanup in
// their own error paths; there is currently no dedicated error-notification
// hook on this interface.
type StreamingRequestPolicy interface {
	RequestPolicy
	OnRequestBodyChunk(ctx context.Context, reqCtx *RequestStreamContext, chunk *StreamBody, params map[string]interface{}) StreamingRequestAction
	NeedsMoreRequestData(accumulated []byte) bool
}

// StreamingResponsePolicy processes the response body chunk-by-chunk.
// ResponsePolicy is embedded as a buffered fallback — the kernel falls back to
// buffered mode when any policy in the chain does not implement this interface.
// The kernel upgrades to FULL_DUPLEX_STREAMED only when every response policy
// in the chain implements StreamingResponsePolicy.
//
// Error handling limitation: once the kernel has entered FULL_DUPLEX_STREAMED
// mode and flushed at least one chunk downstream, the response status and
// headers are committed to the client. A mid-stream error cannot be surfaced
// as a clean HTTP error response — Envoy will abort the HTTP/2 stream or reset
// the connection. ImmediateResponse is silently ignored in this context. There
// is currently no dedicated error-notification hook on this interface; a future
// OnStreamError method is planned to allow cleanup of per-stream resources
// (open connections, partial buffers, token counters for billing).
type StreamingResponsePolicy interface {
	ResponsePolicy
	OnResponseBodyChunk(ctx context.Context, respCtx *ResponseStreamContext, chunk *StreamBody, params map[string]interface{}) StreamingResponseAction
	NeedsMoreResponseData(accumulated []byte) bool
}

// FaultPolicy is the contract for a policy that runs on the FAULT path: the ordered list
// of policies a route may declare to run only when a request has failed.
//
// A fault policy is a distinct kind of policy, not a response policy reused: implementing
// this interface says "this policy is for the fault path", which the gateway checks at
// configuration time. A policy that does not implement OnFault cannot be used in a fault
// sequence — existing policies gain the capability by adding this method, not automatically.
//
// The context is a FaultContext: the response the client is receiving, the request that
// produced it, and a typed description of why the flow is running.
//
// Returning nil is the ordinary case and says "I changed nothing" — what a notification,
// audit or metrics entry does.
//
// OnFault must not fail the request. It runs when the client is already receiving an error,
// so a panic or an unusable return leaves the original error intact rather than escalating.
type FaultPolicy interface {
	OnFault(ctx context.Context, faultCtx *FaultContext, params map[string]interface{}) *FaultResponse
}

// AttemptCompletionPolicy is the contract for a policy that needs to know how an upstream
// attempt actually ENDED, after the exchange is over.
//
// Every other hook on a policy runs while the request is in flight, which means none of them can
// see the end of a response. A target that sends `200`, streams part of a body and then dies is,
// from OnResponseHeaders, indistinguishable from one that served the whole answer — the headers
// are identical, and the failure happens long after the policy has returned. For a policy that
// grades upstream health, recording that as a success is worse than recording nothing: it
// actively credits a target for an answer it never delivered.
//
// Envoy is the only component that observes the end of the exchange, and reports it as a
// response flag on the access-log entry. This hook carries that verdict back.
//
// Three properties follow from WHERE the verdict comes from, and a policy implementing this must
// be written for all three:
//
//   - It is OUT OF BAND. The call arrives roughly a second after the client's request finished,
//     on the access-log stream's goroutine, with no request context. Nothing it does can affect
//     the request it describes; the only useful action is updating state that outlives the
//     request, such as health. Correlate by RequestID with something recorded during the request.
//   - It is NOT GUARANTEED. The gateway delivers it only when the deployment's access-log sink
//     carries the entry. A policy must behave correctly when the hook is never called at all —
//     conservatively, not by assuming success.
//   - It is BODY-INDEPENDENT. Unlike a streaming hook, participation does not require the policy
//     to process the response body, so it does not force the body out of its skip/streaming mode
//     and does not make this policy's correctness depend on what the OTHER policies in the chain
//     support.
//
// Implementations must not block: the hook runs inline on the access-log stream, so a slow
// implementation stalls delivery for every other route. They must not panic: the request is
// already finished, so there is nothing to fail, and a panic takes the stream down with it.
type AttemptCompletionPolicy interface {
	OnAttemptComplete(ctx context.Context, attemptCtx *AttemptCompletionContext, params map[string]interface{})
}
