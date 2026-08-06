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

import "strings"

// This file provides the WRITE counterpart of the read accessors in
// context_accessors.go. It lets an earlier policy influence what a LATER policy
// in the same request observes when it reads the pre-mutation snapshot via
// DownstreamHeaders() (request phase) or UpstreamHeaders() (response phase) —
// for example, injecting a header that a downstream JWT/auth policy validates
// against the client-request snapshot.
//
// These setters are deliberately IMPERATIVE and CHAIN-INTERNAL, mirroring how
// policies mutate SharedContext.Metadata. A snapshot header set here is NEVER
// translated into an Envoy header mutation:
//
//   - It does NOT change what the backend receives. To change the upstream
//     request, return UpstreamRequestModifications.HeadersToSet/Append/Remove.
//   - It does NOT change what the client receives. To change the downstream
//     response, return DownstreamResponseModifications /
//     DownstreamResponseHeaderModifications.
//
// Its only effect is on subsequent in-chain reads of the snapshot (the kernel
// shares one snapshot object across every phase context of a request, so a
// header-phase write is visible to the body/stream phases, and a request-phase
// downstream write is visible to the response phase).
//
// SNAPSHOT-ONLY, never the live fallback: unlike the read accessors, these
// setters do NOT fall back to the live working headers when the gateway does not
// provide a snapshot. Mutating the live request headers would leak to the
// backend, and mutating the live response headers would leak to the client —
// precisely what a snapshot write must not do. On a gateway that does not
// populate the snapshot, these setters are a no-op; a policy that requires the
// write to take effect must run on a snapshot-capable gateway (see the
// context_accessors.go header for the snapshot-presence guarantee).
//
// Header names are matched case-insensitively (lower-cased), consistent with the
// read accessors and NewHeaders.

// ─── Internal writable-target resolvers ──────────────────────────────────────

// downstreamWritableHeaders returns the stored client-request snapshot's Headers
// for mutation, lazily creating an empty Headers on the snapshot when it is
// present but header-less. Returns nil when the gateway did not provide a
// snapshot — in which case the caller must no-op rather than touch live headers.
func downstreamWritableHeaders(ds *DownstreamContext) *Headers {
	snap := downstreamSnapshot(ds)
	if snap == nil {
		return nil
	}
	if snap.Headers == nil {
		snap.Headers = NewHeaders(nil)
	}
	return snap.Headers
}

// upstreamWritableHeaders returns the stored upstream-response snapshot's Headers
// for mutation, lazily creating an empty Headers on the snapshot when it is
// present but header-less. Returns nil when the gateway did not provide a
// snapshot — in which case the caller must no-op rather than touch live headers.
func upstreamWritableHeaders(us *UpstreamResponseContext) *Headers {
	snap := upstreamSnapshot(us)
	if snap == nil {
		return nil
	}
	if snap.Headers == nil {
		snap.Headers = NewHeaders(nil)
	}
	return snap.Headers
}

// setSnapshotHeader overwrites all values of name with a single value. No-op on a
// nil target (absent snapshot).
func setSnapshotHeader(h *Headers, name, value string) {
	if h == nil {
		return
	}
	h.UnsafeInternalValues()[strings.ToLower(name)] = []string{value}
}

// addSnapshotHeader appends value to the existing values of name, preserving any
// already present. No-op on a nil target (absent snapshot).
func addSnapshotHeader(h *Headers, name, value string) {
	if h == nil {
		return
	}
	lk := strings.ToLower(name)
	m := h.UnsafeInternalValues()
	m[lk] = append(m[lk], value)
}

// removeSnapshotHeader deletes name and all its values. No-op on a nil target
// (absent snapshot).
func removeSnapshotHeader(h *Headers, name string) {
	if h == nil {
		return
	}
	delete(h.UnsafeInternalValues(), strings.ToLower(name))
}

// ─── Request phase: client-request (downstream) snapshot writers ──────────────

// SetDownstreamHeader overwrites a header in the client-request snapshot that
// later policies read via DownstreamHeaders(). See this file's header for the
// chain-internal, snapshot-only semantics.
func (c *RequestHeaderContext) SetDownstreamHeader(name, value string) {
	setSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// AddDownstreamHeader appends a value to a header in the client-request snapshot.
func (c *RequestHeaderContext) AddDownstreamHeader(name, value string) {
	addSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// RemoveDownstreamHeader removes a header from the client-request snapshot.
func (c *RequestHeaderContext) RemoveDownstreamHeader(name string) {
	removeSnapshotHeader(downstreamWritableHeaders(c.Downstream), name)
}

// SetDownstreamHeader overwrites a header in the client-request snapshot that
// later policies read via DownstreamHeaders().
func (c *RequestContext) SetDownstreamHeader(name, value string) {
	setSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// AddDownstreamHeader appends a value to a header in the client-request snapshot.
func (c *RequestContext) AddDownstreamHeader(name, value string) {
	addSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// RemoveDownstreamHeader removes a header from the client-request snapshot.
func (c *RequestContext) RemoveDownstreamHeader(name string) {
	removeSnapshotHeader(downstreamWritableHeaders(c.Downstream), name)
}

// ─── Response phase: client-request (downstream) snapshot writers ─────────────
//
// The kernel carries the same client-request snapshot forward from the request
// phase into the response phase, and response-phase policies read it via
// DownstreamHeaders(). These writers let an earlier response-phase policy
// influence what a later response-phase policy observes on that snapshot. They
// mutate only the carried-forward request snapshot — never what the client
// receives (use DownstreamResponseModifications for that).

// SetDownstreamHeader overwrites a header in the client-request snapshot that
// later response-phase policies read via DownstreamHeaders().
func (c *ResponseHeaderContext) SetDownstreamHeader(name, value string) {
	setSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// AddDownstreamHeader appends a value to a header in the client-request snapshot.
func (c *ResponseHeaderContext) AddDownstreamHeader(name, value string) {
	addSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// RemoveDownstreamHeader removes a header from the client-request snapshot.
func (c *ResponseHeaderContext) RemoveDownstreamHeader(name string) {
	removeSnapshotHeader(downstreamWritableHeaders(c.Downstream), name)
}

// SetDownstreamHeader overwrites a header in the client-request snapshot that
// later response-phase policies read via DownstreamHeaders().
func (c *ResponseContext) SetDownstreamHeader(name, value string) {
	setSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// AddDownstreamHeader appends a value to a header in the client-request snapshot.
func (c *ResponseContext) AddDownstreamHeader(name, value string) {
	addSnapshotHeader(downstreamWritableHeaders(c.Downstream), name, value)
}

// RemoveDownstreamHeader removes a header from the client-request snapshot.
func (c *ResponseContext) RemoveDownstreamHeader(name string) {
	removeSnapshotHeader(downstreamWritableHeaders(c.Downstream), name)
}

// ─── Response phase: upstream-response snapshot writers ───────────────────────

// SetUpstreamHeader overwrites a header in the upstream-response snapshot that
// later policies read via UpstreamHeaders(). See this file's header for the
// chain-internal, snapshot-only semantics.
func (c *ResponseHeaderContext) SetUpstreamHeader(name, value string) {
	setSnapshotHeader(upstreamWritableHeaders(c.Upstream), name, value)
}

// AddUpstreamHeader appends a value to a header in the upstream-response snapshot.
func (c *ResponseHeaderContext) AddUpstreamHeader(name, value string) {
	addSnapshotHeader(upstreamWritableHeaders(c.Upstream), name, value)
}

// RemoveUpstreamHeader removes a header from the upstream-response snapshot.
func (c *ResponseHeaderContext) RemoveUpstreamHeader(name string) {
	removeSnapshotHeader(upstreamWritableHeaders(c.Upstream), name)
}

// SetUpstreamHeader overwrites a header in the upstream-response snapshot that
// later policies read via UpstreamHeaders().
func (c *ResponseContext) SetUpstreamHeader(name, value string) {
	setSnapshotHeader(upstreamWritableHeaders(c.Upstream), name, value)
}

// AddUpstreamHeader appends a value to a header in the upstream-response snapshot.
func (c *ResponseContext) AddUpstreamHeader(name, value string) {
	addSnapshotHeader(upstreamWritableHeaders(c.Upstream), name, value)
}

// RemoveUpstreamHeader removes a header from the upstream-response snapshot.
func (c *ResponseContext) RemoveUpstreamHeader(name string) {
	removeSnapshotHeader(upstreamWritableHeaders(c.Upstream), name)
}
