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

package kernel

import (
	"context"
	"log/slog"
	"strings"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/faultformat"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// errorFormatRegistry is the shared renderer set. Immutable after construction, so one
// instance serves every request.
var errorFormatRegistry = faultformat.NewRegistry()

// contentTypeHeader is the header the formatter must overwrite when it replaces a body.
const contentTypeHeader = "content-type"

// formatFaultResponse supplies a protocol-appropriate body for a gateway error when no
// policy authored one.
//
// It runs at the end of every rejection path, including the paths where no fault policies are
// configured or the chain failed. That is the point of moving this into the engine: a SOAP
// caller's error has to be a SOAP fault whether or not an operator configured anything, and
// a chain entry could only ever run when a chain existed.
//
// Returns immResp untouched whenever it has nothing to add, which is the common case.
func (ec *PolicyExecutionContext) formatFaultResponse(
	ctx context.Context,
	immResp policy.ImmediateResponse,
) policy.ImmediateResponse {
	decision := faultformat.ShouldFormat(errorFormatRegistry,
		ec.faultFormatInput(ec.errorFor(immResp.Fault), immResp.StatusCode))
	if !decision.Format {
		slog.DebugContext(ctx, "Error formatting skipped",
			"request_id", ec.requestID, "route_key", ec.routeKey, "reason", decision.Reason)
		return immResp
	}

	slog.DebugContext(ctx, "Error body formatted",
		"request_id", ec.requestID, "route_key", ec.routeKey,
		"reason", decision.Reason, "status", immResp.StatusCode)

	immResp.Body = decision.Body
	if immResp.Headers == nil {
		immResp.Headers = make(map[string]string, 1)
	}
	// Overwrite rather than add: the existing value describes the body being replaced.
	immResp.Headers[contentTypeHeader] = decision.ContentType
	return immResp
}

// faultFormatInput fills the seven fields every execution-context ShouldFormat call shares,
// leaving the caller only the two that actually vary: which description to render, and the
// status it belongs to.
//
// Three call sites built this literal identically apart from those two, which is a drift
// risk rather than mere repetition: a field added to faultformat.Input would have to be
// filled in three places, and a formatter that saw it on two of them would behave
// differently depending on which phase produced the failure.
//
// The server-level caller (formatSterileFault) deliberately does NOT use this: it has no
// execution context to read, which is the whole reason it exists separately.
func (ec *PolicyExecutionContext) faultFormatInput(
	declared policy.FaultDetails,
	status int,
) faultformat.Input {
	return faultformat.Input{
		FormatterEnabled: ec.errorFormatterEnabled(),
		BodyAuthored:     ec.faultBodyAuthored,
		Err:              declared,
		Status:           status,
		ErrorID:          ec.engineErrorID,
		APIKind:          ec.apiKind(),
		ContentType:      ec.requestHeader(headerContentType),
		Accept:           ec.requestHeader(headerAccept),
		RequestMethod:    ec.requestMethod(),
	}
}

// errorFor resolves what to render, preferring the error carried by the action being handled.
//
// The action in hand is the most direct source: it is the very response being formatted. The
// recorded copy (errorResponseForFormatting) is the fallback, for the paths that reach here
// without an action carrying one — the body phase, and a router failure where no policy
// produced an action at all.
func (ec *PolicyExecutionContext) errorFor(declared *policy.FaultDetails) policy.FaultDetails {
	if declared == nil {
		return ec.errorResponseForFormatting()
	}
	out := *declared
	// Policy is gateway-owned, so the chain's attribution overwrites whatever the policy
	// claimed — the same rule attributedFault applies.
	out.Policy = ec.faultPolicyName
	return out
}

// errorResponseForFormatting returns what to render.
//
// A zero value is normal and deliberate. Most errors reaching here today carry no
// FaultDetails at all: the router produced them, or the producing policy still puts its
// message in the body rather than in the error object. Rendering a sparse error still
// produces a valid document with a usable fallback message, which for an MCP or SOAP caller
// beats a body they cannot parse — so absence is not a reason to skip.
func (ec *PolicyExecutionContext) errorResponseForFormatting() policy.FaultDetails {
	// Delegates rather than repeating the resolution. Two copies of this existed briefly and
	// immediately diverged: router-failure descriptions were taught to one of them, so fault
	// policies received them and the formatter did not — which is exactly the split this
	// function's caller cannot detect.
	if err := ec.attributedFault(); err != nil {
		return *err
	}
	return policy.FaultDetails{Policy: ec.faultPolicyName}
}

// errorFormatterEnabled reports whether the operator enabled error-body synthesis for this
// route's API kind.
//
// False when the server was built without the option, and false for a request that matched
// no route — an unknown route has no kind, so no configuration could have named it, and the
// reply Envoy already wrote stands.
func (ec *PolicyExecutionContext) errorFormatterEnabled() bool {
	if ec.server == nil {
		return false
	}
	return ec.server.errorFormatterKinds.Enabled(ec.apiKind())
}

// apiKind reads the API kind, tolerating a context that has none (an unmatched request).
func (ec *PolicyExecutionContext) apiKind() policy.APIKind {
	if ec.sharedCtx == nil {
		return ""
	}
	return ec.sharedCtx.APIKind
}

// requestMethod returns the request's method, or "" when the context has none.
func (ec *PolicyExecutionContext) requestMethod() string {
	if ec.requestHeaderCtx == nil {
		return ""
	}
	return ec.requestHeaderCtx.Method
}

// requestHeader returns the first value of a request header, or "" when absent.
func (ec *PolicyExecutionContext) requestHeader(name string) string {
	if ec.requestHeaderCtx == nil {
		return ""
	}
	values := ec.requestHeaderCtx.Headers.Get(name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Request headers the shape decision reads.
const (
	headerContentType = "content-type"
	headerAccept      = "accept"
)

// ─── Sterile engine failures ─────────────────────────────────────────────────
//
// Two failures reach the client before any policy chain is bound, so neither has a
// PolicyExecutionContext: a route the engine holds no chain for, and a resolver that could
// not turn the request into an operation.
//
// They are still formattable, since formatting needs only the API kind and the request's own
// shape signals. It is the fault POLICIES that cannot run here, having no chain to be
// selected from.

// requestShapeSignals is everything shape negotiation needs about a request the engine never
// built a policy context for.
//
// A separate type rather than four loose parameters because all four travel together, and
// three of the four are easy to pass in the wrong order as bare strings.
type requestShapeSignals struct {
	APIKind     policy.APIKind
	Method      string
	ContentType string
	Accept      string
}

// shapeSignalsFromHeaders reads the shape signals straight off the ext_proc request-headers
// message.
//
// Used on the paths with no execution context. Header names arrive lower-cased from Envoy
// (HTTP/2 requires it, and Envoy normalizes HTTP/1 to match), and are compared case-
// insensitively anyway so a test constructing a message by hand cannot silently miss.
func shapeSignalsFromHeaders(kind policy.APIKind, headers *extprocv3.HttpHeaders) requestShapeSignals {
	sig := requestShapeSignals{APIKind: kind}
	if headers == nil || headers.Headers == nil {
		return sig
	}
	for _, h := range headers.Headers.GetHeaders() {
		value := string(h.RawValue)
		if value == "" {
			value = h.Value
		}
		switch strings.ToLower(h.Key) {
		case ":method":
			// Upper-cased at extraction, per GO-AUTH-006, so the HEAD check cannot miss.
			sig.Method = strings.ToUpper(value)
		case headerContentType:
			sig.ContentType = value
		case headerAccept:
			sig.Accept = value
		}
	}
	return sig
}

// formatSterileError renders an engine failure in the caller's protocol, returning the body
// and content type to send.
//
// Returns the sterile body and content type UNCHANGED when the operator has not enabled this
// API kind, when the request was a HEAD, or when the shape has no renderer — every "leave it
// alone" answer ShouldFormat can give. The caller does not need to distinguish them; the
// reason is logged.
func (s *ExternalProcessorServer) formatSterileError(
	ctx context.Context,
	sig requestShapeSignals,
	err policy.FaultDetails,
	status int,
	errorID string,
	body []byte,
	contentType string,
) ([]byte, string) {
	decision := faultformat.ShouldFormat(errorFormatRegistry, faultformat.Input{
		FormatterEnabled: s.errorFormatterKinds.Enabled(sig.APIKind),
		// Nothing can have authored a body here: no policy chain ran. The sterile body is
		// the engine's own, and the engine is not an author — same rule as its 500s.
		BodyAuthored:  false,
		Err:           err,
		Status:        status,
		ErrorID:       errorID,
		APIKind:       sig.APIKind,
		ContentType:   sig.ContentType,
		Accept:        sig.Accept,
		RequestMethod: sig.Method,
	})
	if !decision.Format {
		slog.DebugContext(ctx, "Sterile error left unformatted",
			"error_id", errorID, "api_kind", string(sig.APIKind),
			"status", status, "reason", decision.Reason)
		return body, contentType
	}

	slog.DebugContext(ctx, "Sterile error formatted",
		"error_id", errorID, "api_kind", string(sig.APIKind),
		"status", status, "reason", decision.Reason)
	return decision.Body, decision.ContentType
}

// shapeSignals reads the shape signals off an execution context.
//
// The counterpart to shapeSignalsFromHeaders, for a failure raised once a context exists —
// a resolver that deferred to the request-body phase and then denied.
func (ec *PolicyExecutionContext) shapeSignals() requestShapeSignals {
	return requestShapeSignals{
		APIKind:     ec.apiKind(),
		Method:      ec.requestMethod(),
		ContentType: ec.requestHeader(headerContentType),
		Accept:      ec.requestHeader(headerAccept),
	}
}
