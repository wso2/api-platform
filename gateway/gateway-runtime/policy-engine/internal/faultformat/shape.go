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

// Package faultformat renders a gateway error into the wire format the caller's protocol
// requires, when nothing else has produced a body.
//
// It is a DEFAULT applied after the fault chain, not a chain entry: protocol conformance
// cannot be opt-in, cannot be ordered wrongly, and costs nothing on routes that never use it.
//
// A policy overrides it by authoring a body — see ShouldFormat. The override is
// presence-based, so a custom formatter policy does not have to run after anything.
//
// Negotiate reads only request-side signals (API kind, request Content-Type, request Accept),
// which are available at every point an error can be produced, including a router local reply.
// The shape is resolved per request, not per deployment, and is not configurable.
//
// Adding a protocol means two edits: a ShapeID plus Renderer registered in NewRegistry, and a
// branch in Negotiate. An unregistered ShapeID renders as ShapePassthrough rather than
// erroring.
package faultformat

import (
	"strings"

	"github.com/wso2/api-platform/gateway/common/agentproto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// ShapeID identifies a wire format for an error response.
type ShapeID string

const (
	// ShapePassthrough leaves the error body exactly as produced. It is the result for
	// anything this package has no opinion about, and the safe default everywhere.
	ShapePassthrough ShapeID = "passthrough"
	// ShapeJSON is the canonical JSON error object.
	ShapeJSON ShapeID = "json"
	// ShapeJSONRPC is a JSON-RPC 2.0 error response, required by MCP.
	ShapeJSONRPC ShapeID = "jsonrpc"
	// ShapeJSONRPCEventStream is the same error object wrapped in a single SSE event, for a
	// caller that opened the exchange as an event stream. An SSE client reads frames, so a
	// bare JSON object would arrive as an unterminated event rather than an error.
	ShapeJSONRPCEventStream ShapeID = "jsonrpc-eventstream"
	// ShapeXML is a plain XML error document, for a REST caller that asked for XML.
	ShapeXML ShapeID = "xml"

	// ShapeSOAP11 and ShapeSOAP12 are declared so Negotiate can name them, but are NOT
	// registered, so they render as passthrough.
	ShapeSOAP11 ShapeID = "soap11"
	ShapeSOAP12 ShapeID = "soap12"
)

// Media types that identify a protocol from the request alone.
const (
	mediaSOAP12      = "application/soap+xml" // SOAP 1.2 uses its own media type
	mediaTextXML     = "text/xml"             // SOAP 1.1 rides on text/xml
	mediaAppXML      = "application/xml"
	mediaJSON        = "application/json"
	mediaEventStream = "text/event-stream"
)

// Request carries the request-side signals shape selection is allowed to consult.
//
// Everything here is available at every error point, including one where no upstream
// was ever contacted. Nothing response-side appears, deliberately: a response may not
// exist.
type Request struct {
	APIKind     policy.APIKind
	ContentType string // the request's Content-Type
	Accept      string // the request's Accept
	// Transport is the wire protocol an Agent route serves. See format.Request.Transport.
	Transport string
}

// Renderer turns a gateway error into a body plus the content type describing it.
//
// Status and ErrorID are passed alongside rather than read from the error, because neither
// belongs to it: the status belongs to the response, and the correlation id to the engine that
// generated the failure. policy.FaultDetails deliberately carries no copy of either (a second
// copy could only drift, with no rule for which is authoritative).
type Renderer interface {
	// ContentType is the media type of the rendered body.
	ContentType() string
	// Render produces the body. It must not fail on any input: an error path that
	// itself errors leaves the client with nothing, so renderers degrade rather than
	// return an error.
	Render(in RenderInput) []byte
}

// RenderInput is everything a renderer may put in the body.
//
// A struct rather than a parameter list: what a rendered error may carry has grown twice
// already, and each growth otherwise touches every renderer's signature.
type RenderInput struct {
	// Err is what the policy — or, for a router failure, the gateway — said about the failure.
	Err policy.FaultDetails
	// Status is the response status. Only the JSON-RPC renderer needs it, to choose a code
	// from the reserved range.
	Status int
	// ErrorID correlates a client-visible error with the engine log entry that explains it.
	// Set only for the engine's OWN failures; empty for everything a policy produced, which
	// is correlated by request id instead.
	ErrorID string
}

// Registry resolves a ShapeID to its renderer.
type Registry struct {
	renderers map[ShapeID]Renderer
}

// NewRegistry returns the registry of implemented shapes.
//
// ShapeSOAP11/ShapeSOAP12 are intentionally absent — see their declaration.
func NewRegistry() *Registry {
	return &Registry{
		renderers: map[ShapeID]Renderer{
			ShapeJSON:               jsonRenderer{},
			ShapeJSONRPC:            jsonRPCRenderer{},
			ShapeJSONRPCEventStream: jsonRPCEventStreamRenderer{},
			ShapeXML:                xmlRenderer{},
		},
	}
}

// Render renders err in the given shape.
//
// ok is false when the shape is passthrough or not implemented, meaning the caller must
// leave the existing body untouched. That is the single most important property of this
// function: reshaping is an improvement over the status quo, never a precondition for
// returning a response, so every unhandled case degrades to "leave it alone".
func (r *Registry) Render(shape ShapeID, in RenderInput) (body []byte, contentType string, ok bool) {
	if shape == ShapePassthrough {
		return nil, "", false
	}
	renderer, found := r.renderers[shape]
	if !found {
		return nil, "", false
	}
	return renderer.Render(in), renderer.ContentType(), true
}

// Negotiate resolves the shape an error response must take for this request.
//
// Order matters. Kind-mandated protocols are decided first because they are not
// negotiable — an MCP client speaks JSON-RPC regardless of what it puts in Accept, so
// honouring Accept there would produce a body the client cannot parse. Only after that
// do content-type and Accept get a say.
func Negotiate(req Request) ShapeID {
	// 1. Protocols fixed by API kind. These ignore Accept entirely.
	switch req.APIKind {
	case policy.APIKindAgent:
		// An Agent serves two transports over routes a content type cannot tell apart, so
		// unlike MCP the kind does not settle the shape — the route does, and the resolver
		// publishes which.
		//
		// Only JSON-RPC formats. The rule for joining supportedKinds is that the protocol
		// leaves the caller unable to read anything else, and that is true of exactly this
		// half of A2A: an HTTP+JSON client reads the plain JSON error perfectly well, so
		// reshaping it would change bytes for no one's benefit. Passthrough, not a fall
		// through to the JSON renderer, for that reason — the two differ, and only
		// passthrough is a guarantee that nothing moves.
		//
		// An unresolved transport is passthrough on the same reasoning plus one more: a
		// request refused before resolution has no operation and no request id, so the
		// envelope it would get could carry only nulls.
		if req.Transport != string(agentproto.TransportJSONRPC) {
			return ShapePassthrough
		}
		if normalizeMediaType(req.ContentType) == mediaEventStream {
			return ShapeJSONRPCEventStream
		}
		return ShapeJSONRPC
	case policy.APIKindMCP:
		// The framing, unlike the object inside it, IS decided by the request: an MCP caller
		// that posted an event stream is reading frames and would hang on a bare JSON object.
		//
		// Content-Type only, deliberately NOT Accept. An MCP client routinely sends
		// `Accept: application/json, text/event-stream` on a plain POST to say it can handle
		// either, so reading Accept here would wrap every MCP error in SSE framing. What the
		// client SENT is the unambiguous signal, and it is the same one the MCP policies use
		// to decide how to parse the request in the first place.
		if normalizeMediaType(req.ContentType) == mediaEventStream {
			return ShapeJSONRPCEventStream
		}
		return ShapeJSONRPC
	}
	// 2. Protocols identified by the request's own media type. SOAP is the case this
	// exists for: a single SOAP API serves both versions, and the version is carried
	// by the request, so it cannot be resolved at deploy time.
	switch normalizeMediaType(req.ContentType) {
	case mediaSOAP12:
		return ShapeSOAP12
	case mediaTextXML:
		// text/xml is SOAP 1.1's media type, but it is also just XML, and telling the two
		// apart needs an API kind that does not exist yet. A plain XML document is the
		// choice that is merely incomplete for a SOAP 1.1 caller rather than unreadable,
		// which JSON would be.
		return ShapeXML
	}

	// 3. Content negotiation for everything else.
	if prefersXML(req.Accept) {
		return ShapeXML
	}

	// JSON is the default, including for a caller that expressed no preference.
	//
	// This was ShapePassthrough — "change nothing" — on the grounds that emitting canonical
	// JSON would rewrite the body of every error the product returns. That reasoning does
	// not survive the authorship gate: this function is only consulted when NO policy
	// authored a body (see ShouldFormat), so the alternative here is not the current body,
	// it is an EMPTY one.
	//
	// Which makes the default safe and the previous one harmful. A policy that still writes
	// its own body is untouched, so nothing visible changes today. A policy that has moved
	// its message into FaultDetails would, under passthrough, hand the client a bare status
	// with no body at all — worse than a generic body, since a client parsing JSON gets a
	// parse failure instead of a message.
	//
	// The consequence to keep in mind is one of SEQUENCING: this default has to be in place
	// BEFORE any policy migrates, or there is a window where migrated policies return empty
	// bodies to REST callers.
	return ShapeJSON
}

// normalizeMediaType strips parameters and whitespace and lowercases, so
// `Text/XML; charset=utf-8` compares equal to `text/xml`.
func normalizeMediaType(v string) string {
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, ';'); i >= 0 {
		v = v[:i]
	}
	return strings.ToLower(strings.TrimSpace(v))
}

// prefersXML reports whether Accept asks for XML ahead of JSON.
//
// Deliberately narrow: it returns true only when an XML type is present AND is not
// outranked by a JSON type, comparing q-values. A wildcard never counts as asking for
// XML — `*/*` is what every curl and browser sends, and treating it as an XML request
// would reshape errors for callers that expressed no preference at all.
func prefersXML(accept string) bool {
	if accept == "" {
		return false
	}
	xmlQ, jsonQ := -1.0, -1.0
	for _, part := range strings.Split(accept, ",") {
		mediaType, q := parseAcceptEntry(part)
		switch mediaType {
		case mediaAppXML, mediaTextXML, mediaSOAP12:
			if q > xmlQ {
				xmlQ = q
			}
		case mediaJSON:
			if q > jsonQ {
				jsonQ = q
			}
		}
	}
	return xmlQ > 0 && xmlQ >= jsonQ
}

// parseAcceptEntry splits one Accept entry into its media type and q-value. A missing
// or malformed q defaults to 1.0, per RFC 9110.
func parseAcceptEntry(entry string) (mediaType string, q float64) {
	q = 1.0
	segments := strings.Split(entry, ";")
	mediaType = strings.ToLower(strings.TrimSpace(segments[0]))
	for _, seg := range segments[1:] {
		seg = strings.TrimSpace(seg)
		if !strings.HasPrefix(seg, "q=") {
			continue
		}
		if parsed, err := parseQValue(seg[2:]); err == nil {
			q = parsed
		}
	}
	return mediaType, q
}
