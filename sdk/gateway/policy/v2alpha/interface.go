// Package policyv2alpha provides the v2 policy interface for the WSO2 API Gateway.
//
// Unlike policyv1alpha, policies here declare capabilities by implementing
// phase-specific sub-interfaces. The kernel discovers which phases a policy
// participates in via type assertions at chain-build time — once at startup,
// with zero per-request overhead.
//
// Context and action types are shared with policyv1alpha. Import both packages
// if you need to reference those types:
//
//	import (
//	    v1 "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
//	    v2 "github.com/wso2/api-platform/sdk/gateway/policy/v2alpha"
//	)
package policyv2alpha

import v1 "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"

// Policy is the marker interface all v2alpha policies must implement.
// Capabilities are declared by additionally implementing the phase-specific
// sub-interfaces below.
type Policy interface{}

// PolicyFactory is the function signature for creating v2alpha policy instances.
//
//	func GetPolicy(metadata v1.PolicyMetadata, params map[string]interface{}) (Policy, error)
type PolicyFactory func(metadata v1.PolicyMetadata, params map[string]interface{}) (Policy, error)

// ─── Phase-specific sub-interfaces ───────────────────────────────────────────
//
// Each method receives a params map for parity with the v1alpha interface and
// to ease migration. Params are the same static values resolved at chain-build
// time — they do not change per request. Prefer storing parsed config in the
// policy struct inside GetPolicy rather than re-parsing params on every call.
// TODO: remove params once all policies have migrated to the struct-based pattern.

// RequestHeaderPolicy processes request headers.
// Implement this to modify or inspect headers before the request body is read.
type RequestHeaderPolicy interface {
	OnRequestHeaders(ctx *v1.RequestHeaderContext, params map[string]interface{}) v1.RequestHeaderAction
}

// ResponseHeaderPolicy processes response headers.
// Implement this to modify or inspect response headers before the response body is read.
type ResponseHeaderPolicy interface {
	OnResponseHeaders(ctx *v1.ResponseHeaderContext, params map[string]interface{}) v1.ResponseHeaderAction
}

// RequestPolicy processes the complete buffered request body.
// If any policy in the chain implements this interface, the request body is
// buffered before any policy in the chain executes.
type RequestPolicy interface {
	OnRequestBody(ctx *v1.RequestContext, params map[string]interface{}) v1.RequestAction
}

// ResponsePolicy processes the complete buffered response body.
// If any policy in the chain implements only ResponsePolicy (not
// StreamingResponsePolicy), the entire chain uses BUFFERED mode.
type ResponsePolicy interface {
	OnResponseBody(ctx *v1.ResponseContext, params map[string]interface{}) v1.ResponseAction
}

// StreamingRequestPolicy processes the request body chunk-by-chunk.
// RequestPolicy is embedded as a buffered fallback — the kernel calls
// OnRequestBody when streaming is not possible (e.g. the chain has a
// non-streaming policy). NeedsMoreRequestData is called after each chunk;
// return true to accumulate before OnRequestBodyChunk is invoked.
type StreamingRequestPolicy interface {
	RequestPolicy
	OnRequestBodyChunk(ctx *v1.RequestStreamContext, chunk *v1.StreamBody, params map[string]interface{}) v1.RequestChunkAction
	NeedsMoreRequestData(accumulated []byte) bool
}

// StreamingResponsePolicy processes the response body chunk-by-chunk.
// ResponsePolicy is embedded as a buffered fallback — the kernel falls back to
// buffered mode when any policy in the chain does not implement this interface.
// The kernel upgrades to FULL_DUPLEX_STREAMED only when every response policy
// in the chain implements StreamingResponsePolicy.
type StreamingResponsePolicy interface {
	ResponsePolicy
	OnResponseBodyChunk(ctx *v1.ResponseStreamContext, chunk *v1.StreamBody, params map[string]interface{}) v1.ResponseChunkAction
	NeedsMoreResponseData(accumulated []byte) bool
}
