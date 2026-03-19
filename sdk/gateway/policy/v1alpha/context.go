package policyv1alpha

import core "github.com/wso2/api-platform/sdk/core/policy"

// Body represents HTTP request or response body data.
type Body = core.Body

// SharedContext contains data shared across request and response phases.
type SharedContext = core.SharedContext

// RequestHeaderContext is passed to RequestHeaderPolicy.OnRequestHeaders.
// The request body is not yet available at this phase.
type RequestHeaderContext = core.RequestHeaderContext

// RequestContext is passed to RequestPolicy.OnRequestBody.
// The complete buffered request body is available.
type RequestContext = core.RequestContext

// ResponseHeaderContext is passed to ResponseHeaderPolicy.OnResponseHeaders.
// The response body is not yet available at this phase.
type ResponseHeaderContext = core.ResponseHeaderContext

// ResponseContext is passed to ResponsePolicy.OnResponseBody.
// The complete buffered response body is available.
type ResponseContext = core.ResponseContext

// StreamBody holds a single chunk of body data delivered during streaming processing.
// Unlike Body, it does not carry the "Present" flag — a chunk is always present by definition.
type StreamBody = core.StreamBody

// RequestStreamContext is the per-chunk context passed to StreamingRequestBodyPolicy.
// It is structurally identical to RequestHeaderContext today, but kept as a distinct
// type so that streaming-specific fields (e.g. accumulated byte count, chunk index)
// can be added in the future without changing the header-phase contract.
// Headers are read-only; body data arrives via the StreamBody argument, not this struct.
type RequestStreamContext = core.RequestStreamContext

// ResponseStreamContext is the per-chunk context passed to StreamingResponseBodyPolicy.
// It mirrors ResponseHeaderContext — response body is delivered chunk-by-chunk
// via the StreamBody argument.
type ResponseStreamContext = core.ResponseStreamContext
