package policyv1alpha2

// This file provides convenience accessors that resolve the pre-mutation
// request/response snapshots carried on each policy context, falling back to the
// live (possibly peer-mutated) values on gateways built before the snapshot
// feature.
//
// Use these for every read that feeds an authentication, authorization, or
// gating decision. The kernel runs every policy's header phase before any
// policy's body phase, mutating one shared header set in place, so a decision
// that reads the live values can observe something another policy rewrote —
// regardless of policy order. The snapshot always holds what the client
// actually sent / what the upstream actually returned.
//
// The accessors are nil-safe: they may be called even when the Downstream/
// Upstream context (or its Request/Response) is nil, which is the case on
// gateways that predate the snapshot-header-context feature.

// resolveDownstreamRequest returns the downstream request snapshot when the
// gateway provides it, otherwise a snapshot synthesized from the live request
// values so callers always get the same shape.
func resolveDownstreamRequest(ds *DownstreamContext, liveHeaders *Headers, livePath, liveMethod, liveAuthority, liveScheme string) *DownstreamRequest {
	if ds != nil && ds.Request != nil {
		return ds.Request // fields may be nil/empty; Headers reads (Get/Has/Iterate) are nil-safe
	}
	return &DownstreamRequest{
		Headers:   liveHeaders,
		Path:      livePath,
		Method:    liveMethod,
		Authority: liveAuthority,
		Scheme:    liveScheme,
	}
}

// resolveUpstreamResponse returns the upstream response snapshot when the
// gateway provides it, otherwise a snapshot synthesized from the live response
// values.
func resolveUpstreamResponse(us *UpstreamResponseContext, liveHeaders *Headers, liveStatus int) *UpstreamResponse {
	if us != nil && us.Response != nil {
		return us.Response // Headers may be nil; Headers reads are nil-safe
	}
	return &UpstreamResponse{
		Headers:    liveHeaders,
		StatusCode: liveStatus,
	}
}

// ─── Request-phase accessors ─────────────────────────────────────────────────

// DownstreamRequest returns the pre-mutation snapshot of the client request,
// falling back to a snapshot built from the live request values on gateways
// that predate the snapshot feature.
func (c *RequestHeaderContext) DownstreamRequest() *DownstreamRequest {
	return resolveDownstreamRequest(c.Downstream, c.Headers, c.Path, c.Method, c.Authority, c.Scheme)
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *RequestHeaderContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// DownstreamRequest returns the pre-mutation snapshot of the client request,
// falling back to a snapshot built from the live request values on gateways
// that predate the snapshot feature.
func (c *RequestContext) DownstreamRequest() *DownstreamRequest {
	return resolveDownstreamRequest(c.Downstream, c.Headers, c.Path, c.Method, c.Authority, c.Scheme)
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *RequestContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// DownstreamRequest returns the pre-mutation snapshot of the client request,
// falling back to a snapshot built from the live request values on gateways
// that predate the snapshot feature.
func (c *RequestStreamContext) DownstreamRequest() *DownstreamRequest {
	return resolveDownstreamRequest(c.Downstream, c.Headers, c.Path, c.Method, c.Authority, c.Scheme)
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *RequestStreamContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// ─── Response-phase accessors ────────────────────────────────────────────────
//
// Response contexts carry only the live request echoes (RequestHeaders,
// RequestPath, RequestMethod); there is no live Authority/Scheme to fall back
// to, so those come from the snapshot only and are empty on pre-snapshot
// gateways.

// DownstreamRequest returns the pre-mutation snapshot of the client request,
// falling back to the live request echoes on gateways that predate the snapshot
// feature.
func (c *ResponseHeaderContext) DownstreamRequest() *DownstreamRequest {
	return resolveDownstreamRequest(c.Downstream, c.RequestHeaders, c.RequestPath, c.RequestMethod, "", "")
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *ResponseHeaderContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// UpstreamResponse returns the pre-mutation snapshot of the upstream response,
// falling back to a snapshot built from the live response values on gateways
// that predate the snapshot feature.
func (c *ResponseHeaderContext) UpstreamResponse() *UpstreamResponse {
	return resolveUpstreamResponse(c.Upstream, c.ResponseHeaders, c.ResponseStatus)
}

// UpstreamHeaders is a shortcut for UpstreamResponse().Headers.
func (c *ResponseHeaderContext) UpstreamHeaders() *Headers {
	return c.UpstreamResponse().Headers
}

// DownstreamRequest returns the pre-mutation snapshot of the client request,
// falling back to the live request echoes on gateways that predate the snapshot
// feature.
func (c *ResponseContext) DownstreamRequest() *DownstreamRequest {
	return resolveDownstreamRequest(c.Downstream, c.RequestHeaders, c.RequestPath, c.RequestMethod, "", "")
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *ResponseContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// UpstreamResponse returns the pre-mutation snapshot of the upstream response,
// falling back to a snapshot built from the live response values on gateways
// that predate the snapshot feature.
func (c *ResponseContext) UpstreamResponse() *UpstreamResponse {
	return resolveUpstreamResponse(c.Upstream, c.ResponseHeaders, c.ResponseStatus)
}

// UpstreamHeaders is a shortcut for UpstreamResponse().Headers.
func (c *ResponseContext) UpstreamHeaders() *Headers {
	return c.UpstreamResponse().Headers
}

// DownstreamRequest returns the pre-mutation snapshot of the client request,
// falling back to the live request echoes on gateways that predate the snapshot
// feature.
func (c *ResponseStreamContext) DownstreamRequest() *DownstreamRequest {
	return resolveDownstreamRequest(c.Downstream, c.RequestHeaders, c.RequestPath, c.RequestMethod, "", "")
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *ResponseStreamContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// UpstreamResponse returns the pre-mutation snapshot of the upstream response,
// falling back to a snapshot built from the live response values on gateways
// that predate the snapshot feature.
func (c *ResponseStreamContext) UpstreamResponse() *UpstreamResponse {
	return resolveUpstreamResponse(c.Upstream, c.ResponseHeaders, c.ResponseStatus)
}

// UpstreamHeaders is a shortcut for UpstreamResponse().Headers.
func (c *ResponseStreamContext) UpstreamHeaders() *Headers {
	return c.UpstreamResponse().Headers
}
