package policyv1alpha

// DropHeaderAction controls which headers appear in the analytics event.
type DropHeaderAction struct {
	Action  string   // "allow" (allowlist) or "deny" (denylist)
	Headers []string // Header name list to drop or allow
}

// ─── Short-circuit ────────────────────────────────────────────────────────────

// ImmediateResponse terminates the policy chain and returns a response to the
// downstream client immediately. Return as *ImmediateResponse (pointer) from any
// action method; the kernel dispatches on the pointer type via type assertion.
// Returning nil from a method that returns a sealed action interface means "no action".
type ImmediateResponse struct {
	StatusCode            int
	Headers               map[string]string
	Body                  []byte
	AnalyticsMetadata     map[string]any            // Custom analytics metadata
	DynamicMetadata       map[string]map[string]any // Dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // Headers to exclude from analytics

	// Deprecated: Use AnalyticsHeaderFilter instead.
	DropHeadersFromAnalytics DropHeaderAction
}

// ─── Header phase actions (sealed oneof) ─────────────────────────────────────
//
// RequestHeaderAction and ResponseHeaderAction are sealed interfaces.
// Each has two concrete implementations — one for header mutations, one for
// short-circuiting. The kernel uses a type switch to dispatch.

// RequestHeaderAction is a sealed oneof returned by OnRequestHeaders.
// Implement either UpstreamRequestHeaderModifications or return ImmediateResponse.
type RequestHeaderAction interface {
	isRequestHeaderAction()
}

// UpstreamRequestHeaderModifications continues the request to upstream with the
// specified header modifications. Returned when no short-circuit is needed.
type UpstreamRequestHeaderModifications struct {
	Set    map[string]string   // overwrite header (last write wins)
	Remove []string            // remove by name (case-insensitive)
	Append map[string][]string // append values alongside existing

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

func (UpstreamRequestHeaderModifications) isRequestHeaderAction() {}

// ImmediateResponse also implements RequestHeaderAction — returning it short-circuits
// the chain and sends the response directly to the downstream client.
func (ImmediateResponse) isRequestHeaderAction() {}

// ResponseHeaderAction is a sealed oneof returned by OnResponseHeaders.
// Implement either DownstreamResponseHeaderModifications or return ImmediateResponse.
type ResponseHeaderAction interface {
	isResponseHeaderAction()
}

// DownstreamResponseHeaderModifications continues with the specified response header
// modifications applied before the response is forwarded to the client.
type DownstreamResponseHeaderModifications struct {
	Set    map[string]string   // overwrite header (last write wins)
	Remove []string            // remove by name (case-insensitive)
	Append map[string][]string // append values alongside existing

	AnalyticsMetadata     map[string]any            // custom analytics metadata
	DynamicMetadata       map[string]map[string]any // dynamic metadata by namespace
	AnalyticsHeaderFilter DropHeaderAction          // headers to exclude from analytics
}

func (DownstreamResponseHeaderModifications) isResponseHeaderAction() {}

// ImmediateResponse also implements ResponseHeaderAction — returning it short-circuits
// the chain and sends the response directly to the downstream client.
func (ImmediateResponse) isResponseHeaderAction() {}

// ─── Buffered body actions (sealed oneof) ────────────────────────────────────
//
// RequestAction and ResponseAction are sealed interfaces. Each has two concrete
// implementations — one for mutations (continue to upstream/client), one for
// short-circuiting (ImmediateResponse). StopExecution() lets callers branch
// without a type assertion; a type switch is still needed to access fields.

// RequestAction is a sealed oneof returned by RequestPolicy.OnRequestBody.
// Implement either UpstreamRequestModifications or return ImmediateResponse.
type RequestAction interface {
	isRequestAction()
	// StopExecution returns true when the chain should be short-circuited and
	// the response returned directly to the downstream client.
	StopExecution() bool
}

// UpstreamRequestModifications forwards the request to upstream with the
// specified mutations. Returned when processing should continue normally.
// Because the request body is fully buffered, header and routing mutations
// applied here are still effective.
type UpstreamRequestModifications struct {
	Body []byte // nil = passthrough; []byte{} = clear body

	// Header allows modifying upstream request headers alongside body modifications.
	// Prefer this over the deprecated flat SetHeaders/RemoveHeaders/AppendHeaders fields.
	Header *UpstreamRequestHeaderModifications

	// Routing mutations (also valid before the request is forwarded).
	Path                    *string
	Method                  *string
	QueryParametersToAdd    map[string][]string
	QueryParametersToRemove []string
	UpstreamName            *string // route to a named upstream definition (nil = no change)

	AnalyticsMetadata     map[string]any
	DynamicMetadata       map[string]map[string]any
	AnalyticsHeaderFilter DropHeaderAction

	// Deprecated fields — retained for backward compatibility with policies written
	// against SDK ≤ v0.4.3. New policies should use Header and the renamed fields instead.

	// Deprecated: Use Header.Set instead.
	SetHeaders map[string]string
	// Deprecated: Use Header.Remove instead.
	RemoveHeaders []string
	// Deprecated: Use Header.Append instead.
	AppendHeaders map[string][]string
	// Deprecated: Use QueryParametersToAdd instead.
	AddQueryParameters map[string][]string
	// Deprecated: Use QueryParametersToRemove instead.
	RemoveQueryParameters []string
	// Deprecated: Use AnalyticsHeaderFilter instead.
	DropHeadersFromAnalytics DropHeaderAction
}

func (UpstreamRequestModifications) isRequestAction()    {}
func (UpstreamRequestModifications) StopExecution() bool { return false }

// ImmediateResponse also implements RequestAction — returning it short-circuits
// the chain and sends the response directly to the downstream client.
func (ImmediateResponse) isRequestAction()    {}
func (ImmediateResponse) StopExecution() bool { return true }

// ResponseAction is a sealed oneof returned by ResponsePolicy.OnResponseBody.
// Implement either DownstreamResponseModifications or return ImmediateResponse.
type ResponseAction interface {
	isResponseAction()
	// StopExecution returns true when the entire response should be replaced by
	// this ImmediateResponse rather than forwarding the upstream response body.
	StopExecution() bool
}

// DownstreamResponseModifications forwards the response to the client with the
// specified mutations. The request headers are already committed to upstream,
// but status, body, and response headers can still be changed.
type DownstreamResponseModifications struct {
	Body       []byte // nil = passthrough; []byte{} = clear body
	StatusCode *int   // nil = no change

	// Header allows modifying downstream response headers alongside body modifications.
	// Prefer this over the deprecated flat SetHeaders/RemoveHeaders/AppendHeaders fields.
	Header *DownstreamResponseHeaderModifications

	AnalyticsMetadata     map[string]any
	DynamicMetadata       map[string]map[string]any
	AnalyticsHeaderFilter DropHeaderAction

	// Deprecated fields — retained for backward compatibility with policies written
	// against SDK ≤ v0.4.3. New policies should use Header instead.

	// Deprecated: Use Header.Set instead.
	SetHeaders map[string]string
	// Deprecated: Use Header.Remove instead.
	RemoveHeaders []string
	// Deprecated: Use Header.Append instead.
	AppendHeaders map[string][]string
	// Deprecated: Use AnalyticsHeaderFilter instead.
	DropHeadersFromAnalytics DropHeaderAction
}

func (DownstreamResponseModifications) isResponseAction()   {}
func (DownstreamResponseModifications) StopExecution() bool { return false }

// ImmediateResponse also implements ResponseAction — returning it replaces the
// entire upstream response with the specified status, headers, and body.
func (ImmediateResponse) isResponseAction() {}

// UpstreamResponseModifications is a backward-compatible alias for DownstreamResponseModifications.
// Deprecated: Use DownstreamResponseModifications instead.
type UpstreamResponseModifications = DownstreamResponseModifications

// Compile-time interface satisfaction checks.
// These ensure ImmediateResponse satisfies all sealed action interfaces and that
// the concrete modification types satisfy their respective action interfaces.
var (
	_ RequestHeaderAction  = UpstreamRequestHeaderModifications{}
	_ RequestHeaderAction  = ImmediateResponse{}
	_ ResponseHeaderAction = DownstreamResponseHeaderModifications{}
	_ ResponseHeaderAction = ImmediateResponse{}
	_ RequestAction        = UpstreamRequestModifications{}
	_ RequestAction        = ImmediateResponse{}
	_ ResponseAction       = DownstreamResponseModifications{}
	_ ResponseAction       = ImmediateResponse{}
)

// ─── Streaming body actions ───────────────────────────────────────────────────
//
// Streaming hooks receive one chunk at a time. By the time chunks arrive, both
// request headers (sent upstream) and response headers (sent downstream) are
// already committed. Only the chunk content can be changed.
//
// ImmediateResponse is NOT available in streaming chunk actions:
//   - For request chunks: the upstream connection is already open; use
//     RequestHeaderPolicy or RequestPolicy to reject before the body starts.
//   - For response chunks: the client has already received the response headers
//     and status; injecting a new response mid-stream is physically impossible.

// RequestChunkAction is returned by StreamingRequestPolicy.OnRequestBodyChunk.
// Only the chunk payload can be modified. Request headers, path, method, and
// query parameters are all committed — mutations to those fields are ignored.
type RequestChunkAction struct {
	Body []byte // nil = passthrough; non-nil bytes replace the chunk

	// Analytics — accumulates incremental data across chunks (e.g. token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}

// ResponseChunkAction is returned by StreamingResponsePolicy.OnResponseBodyChunk.
// Only the chunk payload can be modified. Response status and headers are already
// committed to the downstream client — mutations to those fields are ignored.
type ResponseChunkAction struct {
	Body []byte // nil = passthrough; non-nil bytes replace the chunk

	// Analytics — accumulates incremental data across chunks (e.g. per-SSE-event token counts).
	AnalyticsMetadata map[string]any
	DynamicMetadata   map[string]map[string]any
}
