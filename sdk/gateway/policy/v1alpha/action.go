package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy"

// DropHeaderAction controls which headers appear in the analytics event.
type DropHeaderAction = core.DropHeaderAction

// ImmediateResponse terminates the policy chain and returns a response to the
// downstream client immediately. Return as *ImmediateResponse (pointer) from any
// action method; the kernel dispatches on the pointer type via type assertion.
// Returning nil from a method that returns a sealed action interface means "no action".
type ImmediateResponse = core.ImmediateResponse

// ─── Header phase actions (sealed oneof) ─────────────────────────────────────

// RequestHeaderAction is a sealed oneof returned by OnRequestHeaders.
// Implement either UpstreamRequestHeaderModifications or return ImmediateResponse.
type RequestHeaderAction = core.RequestHeaderAction

// UpstreamRequestHeaderModifications continues the request to upstream with the
// specified header and routing modifications. Returned when no short-circuit is needed.
type UpstreamRequestHeaderModifications = core.UpstreamRequestHeaderModifications

// ResponseHeaderAction is a sealed oneof returned by OnResponseHeaders.
// Implement either DownstreamResponseHeaderModifications or return ImmediateResponse.
type ResponseHeaderAction = core.ResponseHeaderAction

// DownstreamResponseHeaderModifications continues with the specified response header
// modifications applied before the response is forwarded to the client.
type DownstreamResponseHeaderModifications = core.DownstreamResponseHeaderModifications

// ─── Buffered body actions (sealed oneof) ────────────────────────────────────

// RequestAction is a sealed oneof returned by RequestPolicy.OnRequestBody.
// Implement either UpstreamRequestModifications or return ImmediateResponse.
type RequestAction = core.RequestAction

// UpstreamRequestModifications forwards the request to upstream with the
// specified mutations. Returned when processing should continue normally.
type UpstreamRequestModifications = core.UpstreamRequestModifications

// ResponseAction is a sealed oneof returned by ResponsePolicy.OnResponseBody.
// Implement either DownstreamResponseModifications or return ImmediateResponse.
type ResponseAction = core.ResponseAction

// DownstreamResponseModifications forwards the response to the client with the
// specified mutations.
type DownstreamResponseModifications = core.DownstreamResponseModifications

// UpstreamResponseModifications is a backward-compatible alias for DownstreamResponseModifications.
// Deprecated: Use DownstreamResponseModifications instead.
type UpstreamResponseModifications = core.UpstreamResponseModifications

// ─── Streaming body actions ───────────────────────────────────────────────────

// RequestChunkAction is returned by StreamingRequestPolicy.OnRequestBodyChunk.
// Only the chunk payload can be modified.
type RequestChunkAction = core.RequestChunkAction

// ResponseChunkAction is returned by StreamingResponsePolicy.OnResponseBodyChunk.
// Only the chunk payload can be modified.
type ResponseChunkAction = core.ResponseChunkAction
