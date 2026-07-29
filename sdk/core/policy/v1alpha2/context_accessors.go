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

package policyv1alpha2

// This file provides convenience accessors that return the pre-mutation
// request/response snapshot carried on each policy context, falling back to the
// live values when the gateway does not provide a snapshot.
//
// Snapshots and live values carry DIFFERENT guarantees — do not treat the
// accessor's return value as always being the unmutated client request /
// upstream response:
//
//   - When the gateway populates the snapshot (Downstream.Request /
//     Upstream.Response present), the accessor returns it. This is what the
//     client actually sent / what the upstream actually returned, immune to
//     rewrites by earlier policies in the chain. The kernel runs every policy's
//     header phase before any policy's body phase against one shared, mutable
//     header set, so reading the live values can otherwise observe a value
//     another policy rewrote — regardless of policy order. Prefer the snapshot
//     for any authentication, authorization, or gating decision.
//
//   - When the gateway does NOT provide the snapshot (gateways released before
//     the snapshot feature, where Downstream/Upstream — or its Request/Response
//     — is nil), the accessor falls back to the live values. These are exactly
//     what policies observed before this feature existed, and may already
//     reflect an earlier in-chain mutation. The immunity guarantee above holds
//     ONLY where the snapshot is present.
//
// The fallback is deliberate compatibility behaviour and does NOT fail closed:
// on a gateway that never populates the snapshot, live is the only data that
// exists, and refusing to return it would break every such deployment. A policy
// that must have the unmutated values (and can require a snapshot-capable
// gateway) should read the Downstream/Upstream snapshot directly and handle a
// nil snapshot itself, rather than relying on the fallback.
//
// All accessors are nil-safe: they may be called even when Downstream/Upstream
// (or its Request/Response) is nil.

// downstreamSnapshot returns the client request snapshot, or nil when the
// gateway does not provide one. Single home for the snapshot-presence check.
func downstreamSnapshot(ds *DownstreamContext) *DownstreamRequest {
	if ds != nil && ds.Request != nil {
		return ds.Request // fields may be nil/empty; Headers reads (Get/Has/Iterate) are nil-safe
	}
	return nil
}

// upstreamSnapshot returns the upstream response snapshot, or nil when the
// gateway does not provide one.
func upstreamSnapshot(us *UpstreamResponseContext) *UpstreamResponse {
	if us != nil && us.Response != nil {
		return us.Response // Headers may be nil; Headers reads are nil-safe
	}
	return nil
}

// ─── Request-phase accessors ─────────────────────────────────────────────────

// DownstreamRequest returns the client request snapshot, or the live request
// values when the gateway does not provide a snapshot. See the file header for
// the guarantee difference between the two.
func (c *RequestHeaderContext) DownstreamRequest() *DownstreamRequest {
	if snap := downstreamSnapshot(c.Downstream); snap != nil {
		return snap
	}
	return &DownstreamRequest{Headers: c.Headers, Path: c.Path, Method: c.Method, Authority: c.Authority, Scheme: c.Scheme}
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *RequestHeaderContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// DownstreamRequest returns the client request snapshot, or the live request
// values when the gateway does not provide a snapshot.
func (c *RequestContext) DownstreamRequest() *DownstreamRequest {
	if snap := downstreamSnapshot(c.Downstream); snap != nil {
		return snap
	}
	return &DownstreamRequest{Headers: c.Headers, Path: c.Path, Method: c.Method, Authority: c.Authority, Scheme: c.Scheme}
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *RequestContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// DownstreamRequest returns the client request snapshot, or the live request
// values when the gateway does not provide a snapshot.
func (c *RequestStreamContext) DownstreamRequest() *DownstreamRequest {
	if snap := downstreamSnapshot(c.Downstream); snap != nil {
		return snap
	}
	return &DownstreamRequest{Headers: c.Headers, Path: c.Path, Method: c.Method, Authority: c.Authority, Scheme: c.Scheme}
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *RequestStreamContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// ─── Response-phase accessors ────────────────────────────────────────────────
//
// Response contexts carry only the live request echoes (RequestHeaders,
// RequestPath, RequestMethod); there is no live Authority/Scheme to fall back
// to, so those come from the snapshot only and are empty when the gateway does
// not provide a snapshot.

// DownstreamRequest returns the client request snapshot, or the live request
// echoes when the gateway does not provide a snapshot.
func (c *ResponseHeaderContext) DownstreamRequest() *DownstreamRequest {
	if snap := downstreamSnapshot(c.Downstream); snap != nil {
		return snap
	}
	return &DownstreamRequest{Headers: c.RequestHeaders, Path: c.RequestPath, Method: c.RequestMethod}
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *ResponseHeaderContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// UpstreamResponse returns the upstream response snapshot, or the live response
// values when the gateway does not provide a snapshot.
func (c *ResponseHeaderContext) UpstreamResponse() *UpstreamResponse {
	if snap := upstreamSnapshot(c.Upstream); snap != nil {
		return snap
	}
	return &UpstreamResponse{Headers: c.ResponseHeaders, StatusCode: c.ResponseStatus}
}

// UpstreamHeaders is a shortcut for UpstreamResponse().Headers.
func (c *ResponseHeaderContext) UpstreamHeaders() *Headers {
	return c.UpstreamResponse().Headers
}

// DownstreamRequest returns the client request snapshot, or the live request
// echoes when the gateway does not provide a snapshot.
func (c *ResponseContext) DownstreamRequest() *DownstreamRequest {
	if snap := downstreamSnapshot(c.Downstream); snap != nil {
		return snap
	}
	return &DownstreamRequest{Headers: c.RequestHeaders, Path: c.RequestPath, Method: c.RequestMethod}
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *ResponseContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// UpstreamResponse returns the upstream response snapshot, or the live response
// values when the gateway does not provide a snapshot.
func (c *ResponseContext) UpstreamResponse() *UpstreamResponse {
	if snap := upstreamSnapshot(c.Upstream); snap != nil {
		return snap
	}
	return &UpstreamResponse{Headers: c.ResponseHeaders, StatusCode: c.ResponseStatus}
}

// UpstreamHeaders is a shortcut for UpstreamResponse().Headers.
func (c *ResponseContext) UpstreamHeaders() *Headers {
	return c.UpstreamResponse().Headers
}

// DownstreamRequest returns the client request snapshot, or the live request
// echoes when the gateway does not provide a snapshot.
func (c *ResponseStreamContext) DownstreamRequest() *DownstreamRequest {
	if snap := downstreamSnapshot(c.Downstream); snap != nil {
		return snap
	}
	return &DownstreamRequest{Headers: c.RequestHeaders, Path: c.RequestPath, Method: c.RequestMethod}
}

// DownstreamHeaders is a shortcut for DownstreamRequest().Headers.
func (c *ResponseStreamContext) DownstreamHeaders() *Headers {
	return c.DownstreamRequest().Headers
}

// UpstreamResponse returns the upstream response snapshot, or the live response
// values when the gateway does not provide a snapshot.
func (c *ResponseStreamContext) UpstreamResponse() *UpstreamResponse {
	if snap := upstreamSnapshot(c.Upstream); snap != nil {
		return snap
	}
	return &UpstreamResponse{Headers: c.ResponseHeaders, StatusCode: c.ResponseStatus}
}

// UpstreamHeaders is a shortcut for UpstreamResponse().Headers.
func (c *ResponseStreamContext) UpstreamHeaders() *Headers {
	return c.UpstreamResponse().Headers
}
