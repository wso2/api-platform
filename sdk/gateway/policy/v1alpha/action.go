package policyv1alpha

// DropHeaderAction controls which headers appear in the analytics event.
type DropHeaderAction struct {
	Action  string   // "allow" (allowlist) or "deny" (denylist)
	Headers []string // Header name list to drop or allow
}

// ─── Short-circuit ────────────────────────────────────────────────────────────

// ImmediateResponse terminates the policy chain and returns a response to the
// downstream client immediately. Embed as a pointer in HeaderAction or
// RequestBodyAction/ResponseBodyAction; a non-nil pointer short-circuits the chain.
type ImmediateResponse struct {
	StatusCode               int
	Headers                  map[string]string
	Body                     []byte
	AnalyticsMetadata        map[string]any            // Custom analytics metadata
	DynamicMetadata          map[string]map[string]any // Dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // Headers to exclude from analytics
}

// ─── Header phase action ──────────────────────────────────────────────────────

// RequestHeaderAction is returned by OnRequestHeaders.
// Only header mutations are allowed at this phase — the body is not yet available.
// If ImmediateResponse is non-nil the chain short-circuits and the response is
// returned to the downstream client without forwarding to upstream.
type RequestHeaderAction struct {
	Set               map[string]string   // overwrite header (last write wins)
	Remove            []string            // remove by name (case-insensitive)
	Append            map[string][]string // append values alongside existing
	ImmediateResponse *ImmediateResponse  // non-nil → stop chain, return to client

	// Routing mutations — TODO: check if we need this.
	// UpstreamName  *string             // route to a named upstream definition (nil = no change)
	// PathMutation  *string             // rewrite the request path (nil = no change)
	// MethodMutation *string            // rewrite the HTTP method (nil = no change)
	// QueryParametersToAdd    map[string][]string // add/append query parameters
	// QueryParametersToRemove []string            // remove query parameters by name

	AnalyticsMetadata        map[string]any            // custom analytics metadata
	DynamicMetadata          map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

// ResponseHeaderAction is returned by OnResponseHeaders.
// Only header mutations are allowed at this phase
type ResponseHeaderAction struct {
	Set               map[string]string   // overwrite header (last write wins)
	Remove            []string            // remove by name (case-insensitive)
	Append            map[string][]string // append values alongside existing
	AnalyticsMetadata        map[string]any            // custom analytics metadata
	DynamicMetadata          map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

// ─── Buffered body actions ────────────────────────────────────────────────────

// RequestAction is returned by RequestPolicy.OnRequestBody.
// Because the request body is fully buffered before being forwarded upstream,
// header and routing mutations applied here are still effective.
type RequestAction struct {
	BodyMutation      []byte             // nil = passthrough; []byte{} = clear body
	ImmediateResponse *ImmediateResponse // non-nil → reject, return to client now

	// Header mutations valid here: the request has not left yet.
	HeaderMutation *RequestHeaderAction

	// Routing mutations (also valid before the request is forwarded).
	PathMutation   *string
	MethodMutation *string
	QueryParametersToAdd    map[string][]string
	QueryParametersToRemove []string
	UpstreamName   *string // route to a named upstream definition (nil = no change)

	AnalyticsMetadata        map[string]any
	DynamicMetadata          map[string]map[string]any
	AnalyticsHeaderFilter DropHeaderAction
}

// ResponseAction is returned by ResponsePolicy.OnResponseBody.
// By this phase the request headers are already committed to upstream, but the
// response has not yet been forwarded to the downstream client, so status, body,
// and response headers can still be changed.
type ResponseAction struct {
	BodyMutation      []byte             // nil = passthrough; []byte{} = clear body
	ImmediateResponse *ImmediateResponse // non-nil → replace entire response
	StatusCode        *int               // nil = no change

	// Header mutations applied to the response — valid because the response has
	// not yet been forwarded to the downstream client.
	HeaderMutation *ResponseHeaderAction

	AnalyticsMetadata        map[string]any
	DynamicMetadata          map[string]map[string]any
	AnalyticsHeaderFilter DropHeaderAction
}

// ─── Streaming body actions ───────────────────────────────────────────────────
//
// Streaming hooks receive one chunk at a time. By the time chunks arrive, both
// request headers (sent upstream) and response headers (sent downstream) are
// already committed. Only the chunk content can be changed.
//
// ImmediateResponse is NOT available in streaming chunk actions:
//   - For request chunks: the upstream connection is already open; use
//     RequestHeaderPolicy or RequestBodyPolicy to reject before the body starts.
//   - For response chunks: the client has already received the response headers
//     and status; injecting a new response mid-stream is physically impossible.

// RequestChunkAction is returned by StreamingRequestPolicy.OnRequestBodyChunk.
// Only the chunk payload can be modified. Request headers, path, method, and
// query parameters are all committed — mutations to those fields are ignored.
type RequestChunkAction struct {
	BodyMutation []byte // nil = passthrough; non-nil bytes replace the chunk

	// Analytics — accumulates incremental data across chunks (e.g. token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}

// ResponseChunkAction is returned by StreamingResponsePolicy.OnResponseBodyChunk.
// Only the chunk payload can be modified. Response status and headers are already
// committed to the downstream client — mutations to those fields are ignored.
type ResponseChunkAction struct {
	BodyMutation []byte // nil = passthrough; non-nil bytes replace the chunk

	// Analytics — accumulates incremental data across chunks (e.g. per-SSE-event token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}
