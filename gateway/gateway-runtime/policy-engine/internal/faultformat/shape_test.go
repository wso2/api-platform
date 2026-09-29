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

package faultformat

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wso2/api-platform/gateway/common/agentproto"
	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

func TestNegotiate(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want ShapeID
	}{
		// Kind wins over Accept: an MCP client speaks JSON-RPC whatever it asks for,
		// so honouring Accept here would hand it a body it cannot parse.
		{"MCP is always JSON-RPC", Request{APIKind: policy.APIKindMCP}, ShapeJSONRPC},
		{"MCP ignores an XML Accept", Request{APIKind: policy.APIKindMCP, Accept: "application/xml"}, ShapeJSONRPC},

		// SOAP 1.2 is identifiable from the request alone, which is the whole reason
		// shape resolution is per-request rather than per-deployment.
		{"SOAP 1.2 media type", Request{ContentType: mediaSOAP12}, ShapeSOAP12},
		{"SOAP 1.2 with charset", Request{ContentType: "application/soap+xml; charset=utf-8"}, ShapeSOAP12},
		{"SOAP 1.2 media type is case-insensitive", Request{ContentType: "Application/SOAP+XML"}, ShapeSOAP12},

		// text/xml is ambiguous between SOAP 1.1 and plain XML. A plain XML document is
		// merely incomplete for a SOAP caller; JSON would be unreadable to it.
		{"text/xml gets XML, the lesser wrong for an unresolved SOAP 1.1", Request{ContentType: mediaTextXML}, ShapeXML},

		// An explicit XML ask is honoured over the JSON default.
		{"explicit XML Accept", Request{Accept: "application/xml"}, ShapeXML},
		{"XML preferred by q-value", Request{Accept: "application/json;q=0.5, application/xml;q=0.9"}, ShapeXML},
		{"JSON preferred by q-value", Request{Accept: "application/xml;q=0.3, application/json;q=0.8"}, ShapeJSON},
		{"equal q-values favour the explicit XML ask", Request{Accept: "application/xml, application/json"}, ShapeXML},
		{"q=0 is a refusal, not a request", Request{Accept: "application/xml;q=0"}, ShapeJSON},

		// JSON is the default. Negotiate is only consulted when no policy authored a body,
		// so the alternative here is an EMPTY body, not the caller's current one — which is
		// why defaulting to a real shape is the safe choice rather than the risky one.
		{"no signals at all", Request{}, ShapeJSON},
		{"wildcard Accept is not an XML request", Request{Accept: "*/*"}, ShapeJSON},
		{"browser-style Accept is not an XML request", Request{Accept: "text/html,application/xhtml+xml,*/*;q=0.8"}, ShapeJSON},
		{"plain REST JSON caller", Request{APIKind: policy.APIKindRestApi, Accept: "application/json"}, ShapeJSON},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Negotiate(tc.req); got != tc.want {
				t.Fatalf("Negotiate(%+v) = %q, want %q", tc.req, got, tc.want)
			}
		})
	}
}

// A declared-but-unregistered shape must degrade to "leave the body alone" rather than
// erroring. Without this, adding a ShapeID before its renderer would turn every matching
// error into a broken response instead of an unimproved one.
func TestRender_UnimplementedShapeDegradesToPassthrough(t *testing.T) {
	r := NewRegistry()
	for _, shape := range []ShapeID{ShapeSOAP11, ShapeSOAP12, ShapePassthrough, ShapeID("not-a-shape")} {
		if _, _, ok := r.Render(shape, RenderInput{Err: policy.FaultDetails{Message: "x"}, Status: 0}); ok {
			t.Fatalf("shape %q reported ok=true but has no renderer", shape)
		}
	}
}

func TestRender_JSON(t *testing.T) {
	body, ct, ok := NewRegistry().Render(ShapeJSON, RenderInput{Err: policy.FaultDetails{Code: "906000", Type: "guardrail", Message: "Request rejected"}, Status: 422})
	if !ok {
		t.Fatal("expected JSON shape to render")
	}
	if ct != "application/json" {
		t.Fatalf("content type = %q", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("rendered body is not valid JSON: %v (%s)", err, body)
	}
	if got["message"] != "Request rejected" || got["type"] != "guardrail" || got["code"] != "906000" {
		t.Fatalf("unexpected body: %s", body)
	}
	// Description was not set, so it must be absent rather than present-and-empty —
	// a caller must be able to distinguish "no detail" from "empty detail".
	if _, present := got["description"]; present {
		t.Fatalf("empty description must be omitted: %s", body)
	}
}

// The detail of a guardrail rejection IS the content the guardrail blocked, so no renderer
// here may forward it — the summary goes to the client, the detail does not.
//
// The earlier version of this test only checked that an EMPTY description was omitted, which
// every renderer satisfied via `omitempty` while still forwarding a populated one. That gap
// was found by running the demo, not by this test, so the populated case is now the case.
func TestRender_DescriptionIsNeverForwarded(t *testing.T) {
	const blocked = "THE-BLOCKED-CONTENT-MUST-NOT-BE-FORWARDED"
	r := NewRegistry()
	for _, shape := range []ShapeID{ShapeJSON, ShapeJSONRPC, ShapeXML} {
		for _, e := range []policy.FaultDetails{
			{Message: "Rejected"},                       // absent
			{Message: "Rejected", Description: blocked}, // populated
		} {
			body, _, ok := r.Render(shape, RenderInput{Err: e, Status: 422})
			if !ok {
				t.Fatalf("shape %q did not render", shape)
			}
			if strings.Contains(string(body), blocked) {
				t.Fatalf("shape %q leaked the description: %s", shape, body)
			}
			if strings.Contains(string(body), "description") || strings.Contains(string(body), `"data"`) {
				t.Fatalf("shape %q emitted a detail field: %s", shape, body)
			}
			if !strings.Contains(string(body), "Rejected") {
				t.Fatalf("shape %q dropped the client-facing message: %s", shape, body)
			}
		}
	}
}

func TestRender_JSONRPC(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		wantCode float64
	}{
		{"4xx maps to invalid request", 401, jsonRPCInvalidRequest},
		{"5xx maps to internal error", 503, jsonRPCInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, _, ok := NewRegistry().Render(ShapeJSONRPC, RenderInput{Err: policy.FaultDetails{Message: "Denied", Description: "token expired"}, Status: tc.status})
			if !ok {
				t.Fatal("expected JSON-RPC shape to render")
			}
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("not valid JSON: %v", err)
			}
			if got["jsonrpc"] != "2.0" {
				t.Fatalf("missing jsonrpc version: %s", body)
			}
			// id must be present AND null: absent is not a valid JSON-RPC response,
			// and a guessed id would correlate the error to the wrong call.
			id, present := got["id"]
			if !present || id != nil {
				t.Fatalf("id must be present and null, got %v (present=%v): %s", id, present, body)
			}
			errObj, _ := got["error"].(map[string]any)
			if errObj == nil || errObj["code"] != tc.wantCode {
				t.Fatalf("want code %v, got %s", tc.wantCode, body)
			}
			// Description must NOT surface, here or in any other renderer: for a guardrail
			// it is the content that was blocked. Rendered as `data` it went straight to
			// the client, which is exactly what the guardrail prevented.
			if _, present := errObj["data"]; present {
				t.Fatalf("description must not be forwarded to the client as data: %s", body)
			}
		})
	}
}

// Renderers must not put the status in the body. The convention is that a protocol error
// object travels in the body while the HTTP status keeps its own meaning — collapsing a 401
// to a SOAP-conformant 500 would destroy the signal analytics and client retry logic depend
// on. The type carrying no status makes that structural; this checks no renderer reintroduces
// it from the status argument.
func TestRender_StatusNeverAppearsInTheBody(t *testing.T) {
	r := NewRegistry()
	for _, shape := range []ShapeID{ShapeJSON, ShapeJSONRPC, ShapeXML} {
		body, _, ok := r.Render(shape, RenderInput{Err: policy.FaultDetails{Message: "Too many requests"}, Status: 429})
		if !ok {
			t.Fatalf("shape %q did not render", shape)
		}
		if strings.Contains(string(body), "429") {
			t.Fatalf("shape %q leaked the HTTP status into the body: %s", shape, body)
		}
	}
}

// An error derived from a bare Envoy local reply carries almost nothing. Every
// shape must still produce a valid document with a usable message.
func TestRender_SparseErrorStillValid(t *testing.T) {
	r := NewRegistry()

	body, _, _ := r.Render(ShapeJSON, RenderInput{Err: policy.FaultDetails{}, Status: 503})
	var jsonGot map[string]any
	if err := json.Unmarshal(body, &jsonGot); err != nil {
		t.Fatalf("sparse JSON invalid: %v", err)
	}
	if msg, _ := jsonGot["message"].(string); msg == "" {
		t.Fatalf("sparse error must still carry a message: %s", body)
	}

	body, _, _ = r.Render(ShapeXML, RenderInput{Err: policy.FaultDetails{}, Status: 503})
	if !strings.Contains(string(body), "<message>") {
		t.Fatalf("sparse XML must still carry a message: %s", body)
	}
}

// A message containing markup must not be able to break out of its XML element.
func TestRender_XMLEscapesContent(t *testing.T) {
	body, _, _ := NewRegistry().Render(ShapeXML, RenderInput{Err: policy.FaultDetails{Message: `</message><injected>x</injected>`}, Status: 400})
	if strings.Contains(string(body), "<injected>") {
		t.Fatalf("XML content was not escaped: %s", body)
	}
}

// A policy that parsed the request knows two things the engine cannot work out: the specific
// JSON-RPC code, and the id of the call that failed. The status-derived code covers only
// -32600/-32603, so without this a policy's -32602 Invalid params collapses into the generic
// -32600, and a client with several calls in flight cannot tell which one was rejected.
func TestRender_JSONRPC_PolicySuppliedCodeAndID(t *testing.T) {
	code := -32602
	body, _, ok := NewRegistry().Render(ShapeJSONRPC, RenderInput{
		Err: policy.FaultDetails{
			Message: "Invalid MCP request params",
			JSONRPC: &policy.JSONRPCError{Code: &code, ID: "call-7"},
		},
		// A status that would otherwise derive -32600, so a passing test cannot be the
		// derivation happening to agree.
		Status: 400,
	})
	if !ok {
		t.Fatal("expected JSON-RPC shape to render")
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if got["id"] != "call-7" {
		t.Fatalf("policy-supplied id must be echoed, got %v: %s", got["id"], body)
	}
	errObj, _ := got["error"].(map[string]any)
	if errObj == nil || errObj["code"] != float64(code) {
		t.Fatalf("want policy code %d, got %s", code, body)
	}
}

// A JSON-RPC id is whatever the client sent, so a number has to survive as a number. Encoding
// it as the string "7" would break correlation for any client matching on the value it sent.
func TestRender_JSONRPC_NumericIDKeepsItsType(t *testing.T) {
	body, _, ok := NewRegistry().Render(ShapeJSONRPC, RenderInput{
		Err:    policy.FaultDetails{Message: "Denied", JSONRPC: &policy.JSONRPCError{ID: float64(7)}},
		Status: 403,
	})
	if !ok {
		t.Fatal("expected JSON-RPC shape to render")
	}
	if !bytes.Contains(body, []byte(`"id":7`)) {
		t.Fatalf("numeric id must stay a JSON number: %s", body)
	}
}

// An id with no code still derives the code from the status, and a code with no id still
// renders id as null. The two halves are independent because a policy may know one and not
// the other — a rate limiter reading the body knows the id but has no code of its own.
func TestRender_JSONRPC_CodeAndIDAreIndependent(t *testing.T) {
	r := NewRegistry()

	body, _, _ := r.Render(ShapeJSONRPC, RenderInput{
		Err:    policy.FaultDetails{Message: "Slow down", JSONRPC: &policy.JSONRPCError{ID: "abc"}},
		Status: 429,
	})
	if !bytes.Contains(body, []byte(`"code":-32600`)) {
		t.Fatalf("an id-only block must leave the code derived from the status: %s", body)
	}

	code := -32700
	body, _, _ = r.Render(ShapeJSONRPC, RenderInput{
		Err:    policy.FaultDetails{Message: "Parse error", JSONRPC: &policy.JSONRPCError{Code: &code}},
		Status: 400,
	})
	if !bytes.Contains(body, []byte(`"id":null`)) {
		t.Fatalf("a code-only block must still render id as null: %s", body)
	}
}

// The JSON and XML renderers must ignore the block entirely. It is protocol detail for one
// wire format, and neither of those has anywhere to put a JSON-RPC code — leaking it would
// put a field in the body that means nothing to the caller receiving it.
func TestRender_JSONRPCBlockIsInvisibleToOtherShapes(t *testing.T) {
	code := -32602
	err := policy.FaultDetails{
		Message: "Invalid params",
		JSONRPC: &policy.JSONRPCError{Code: &code, ID: "call-7"},
	}
	for _, shape := range []ShapeID{ShapeJSON, ShapeXML} {
		body, _, ok := NewRegistry().Render(shape, RenderInput{Err: err, Status: 400})
		if !ok {
			t.Fatalf("shape %q did not render", shape)
		}
		for _, leak := range []string{"32602", "call-7", "jsonrpc"} {
			if bytes.Contains(body, []byte(leak)) {
				t.Fatalf("shape %q leaked JSON-RPC detail %q: %s", shape, leak, body)
			}
		}
	}
}

// An MCP caller that posted an event stream reads FRAMES. A bare JSON object arrives as an
// unterminated event it waits on rather than an error it can report, so the framing — not just
// the object — has to match what the caller opened.
func TestNegotiate_MCPEventStreamFraming(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		want ShapeID
	}{
		{
			"an event-stream request gets framed",
			Request{APIKind: policy.APIKindMCP, ContentType: "text/event-stream"},
			ShapeJSONRPCEventStream,
		},
		{
			"charset and case do not change the answer",
			Request{APIKind: policy.APIKindMCP, ContentType: "Text/Event-Stream; charset=utf-8"},
			ShapeJSONRPCEventStream,
		},
		{
			"a plain JSON post stays unframed",
			Request{APIKind: policy.APIKindMCP, ContentType: "application/json"},
			ShapeJSONRPC,
		},
		{
			// The case that makes Accept unusable here: MCP clients advertise both on an
			// ordinary POST, so honouring Accept would frame every MCP error.
			"Accept advertising event-stream does not frame a JSON post",
			Request{APIKind: policy.APIKindMCP, ContentType: "application/json", Accept: "application/json, text/event-stream"},
			ShapeJSONRPC,
		},
		{
			// Only MCP. A REST API posting an event stream is not speaking JSON-RPC.
			"event-stream on a REST API is not JSON-RPC",
			Request{APIKind: policy.APIKindRestApi, ContentType: "text/event-stream"},
			ShapeJSON,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Negotiate(tc.req); got != tc.want {
				t.Fatalf("want %q, got %q", tc.want, got)
			}
		})
	}
}

// The framed body must carry the SAME error object as the unframed one — only the framing
// differs. Two renderers producing two different objects is the drift this composition exists
// to prevent.
func TestRender_JSONRPCEventStream(t *testing.T) {
	code := -32000
	in := RenderInput{
		Err: policy.FaultDetails{
			Message: "MCP capability not allowed",
			JSONRPC: &policy.JSONRPCError{Code: &code, ID: "call-9"},
		},
		Status: 400,
	}
	r := NewRegistry()

	framed, contentType, ok := r.Render(ShapeJSONRPCEventStream, in)
	if !ok {
		t.Fatal("expected the event-stream shape to render")
	}
	if contentType != "text/event-stream" {
		t.Fatalf("want text/event-stream, got %q", contentType)
	}
	if !bytes.HasPrefix(framed, []byte("data: ")) || !bytes.HasSuffix(framed, []byte("\n\n")) {
		t.Fatalf("body is not a terminated SSE event: %q", framed)
	}

	plain, _, _ := r.Render(ShapeJSONRPC, in)
	unwrapped := bytes.TrimSuffix(bytes.TrimPrefix(framed, []byte("data: ")), []byte("\n\n"))
	if !bytes.Equal(unwrapped, plain) {
		t.Fatalf("framed payload differs from the unframed object:\n framed: %s\n plain:  %s", unwrapped, plain)
	}
}

// The gateway code and class must reach an MCP caller. error.code is the JSON-RPC protocol code,
// so without a home in `data` an MCP client is the one client that cannot see the six-digit
// code — which defeats the point of reusing the existing numbering, since the whole argument for it
// is that a client already keying off an existing code keeps working.
func TestRender_JSONRPC_CarriesTheAPIMCodeAndType(t *testing.T) {
	jrpc := -32602
	body, _, ok := NewRegistry().Render(ShapeJSONRPC, RenderInput{
		Err: policy.FaultDetails{
			Code: "960800", Type: "validation", Message: "Invalid MCP request params",
			JSONRPC: &policy.JSONRPCError{Code: &jrpc, ID: "call-7"},
		},
		Status: 400,
	})
	if !ok {
		t.Fatal("expected the JSON-RPC shape to render")
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	errObj, _ := got["error"].(map[string]any)
	data, _ := errObj["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected a data object: %s", body)
	}
	if data["code"] != "960800" {
		t.Fatalf("want gateway code in data.code, got %v: %s", data["code"], body)
	}
	if data["type"] != "validation" {
		t.Fatalf("want type in data.type, got %v: %s", data["type"], body)
	}

	// The two `code` fields are different things at different levels, and both must survive.
	// error.code is the JSON-RPC protocol code; data.code is the gateway's. Collapsing either into
	// the other loses a distinct piece of information.
	if errObj["code"] != float64(jrpc) {
		t.Fatalf("the protocol code must stay in error.code, got %v: %s", errObj["code"], body)
	}
}

// data is omitted entirely when there is nothing returnable to put in it. An empty object
// would say "there is detail" and carry none — the case a router failure hits, where nothing
// described the error at all.
func TestRender_JSONRPC_DataOmittedWhenNothingDescribed(t *testing.T) {
	body, _, _ := NewRegistry().Render(ShapeJSONRPC, RenderInput{
		Err:    policy.FaultDetails{Message: "Denied"},
		Status: 503,
	})
	if bytes.Contains(body, []byte(`"data"`)) {
		t.Fatalf("data must be absent when nothing is describable: %s", body)
	}
}

// Each field independently brings data into existence, so a partially-described error still
// carries what it has rather than dropping the lot.
func TestRender_JSONRPC_DataFieldsAreIndependent(t *testing.T) {
	r := NewRegistry()
	for _, tc := range []struct {
		name string
		in   RenderInput
		want string
	}{
		{"code only", RenderInput{Err: policy.FaultDetails{Code: "900902"}, Status: 401}, `"code":"900902"`},
		{"type only", RenderInput{Err: policy.FaultDetails{Type: "throttling"}, Status: 429}, `"type":"throttling"`},
		{"error_id only", RenderInput{Err: policy.FaultDetails{}, Status: 500, ErrorID: "abc"}, `"error_id":"abc"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _, _ := r.Render(ShapeJSONRPC, tc.in)
			if !bytes.Contains(body, []byte(tc.want)) {
				t.Fatalf("want %s in %s", tc.want, body)
			}
		})
	}
}

// Description must not follow code and type into data. For a response guardrail it is the
// content the guardrail existed to stop, and data is a client-visible field like any other.
func TestRender_JSONRPC_DescriptionStillNeverReachesData(t *testing.T) {
	body, _, _ := NewRegistry().Render(ShapeJSONRPC, RenderInput{
		Err: policy.FaultDetails{
			Code: "906000", Type: "guardrail", Message: "Blocked",
			Description: "THE BLOCKED CONTENT ITSELF",
		},
		Status: 422,
	})
	if bytes.Contains(body, []byte("THE BLOCKED CONTENT")) {
		t.Fatalf("Description leaked into the rendered body: %s", body)
	}
}

// An Agent API is the one kind whose protocol is not settled by the kind: the same Agent
// serves JSON-RPC and HTTP+JSON, on routes that are indistinguishable by content type. Only
// the JSON-RPC half needs an envelope, and giving one to the other half would be as wrong as
// withholding it from the first.
func TestNegotiate_AgentShapeFollowsTheTransport(t *testing.T) {
	for _, tc := range []struct {
		name        string
		transport   string
		contentType string
		want        ShapeID
	}{
		{"json-rpc gets the envelope", string(agentproto.TransportJSONRPC), "application/json", ShapeJSONRPC},
		{"json-rpc over an event stream is framed", string(agentproto.TransportJSONRPC), "text/event-stream", ShapeJSONRPCEventStream},
		{"http+json is left alone", string(agentproto.TransportHTTPJSON), "application/json", ShapePassthrough},
		// Before resolution there is no transport, and no operation or request id either, so
		// an envelope could carry only nulls. Leaving the body alone claims nothing.
		{"unresolved transport is left alone", "", "application/json", ShapePassthrough},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Negotiate(Request{
				APIKind:     policy.APIKindAgent,
				Transport:   tc.transport,
				ContentType: tc.contentType,
			})
			assert.Equal(t, tc.want, got)
		})
	}
}

// The transport decides an Agent's shape and nothing else's. A stray value on another kind
// must not pull it into a JSON-RPC envelope.
func TestNegotiate_TransportIsIgnoredOffAgent(t *testing.T) {
	for _, kind := range []policy.APIKind{policy.APIKindRestApi, policy.APIKindLlmProxy} {
		got := Negotiate(Request{
			APIKind:     kind,
			Transport:   string(agentproto.TransportJSONRPC),
			ContentType: "application/json",
		})
		assert.Equal(t, ShapeJSON, got, "%s must not follow an Agent transport", kind)
	}
}

// Accept is not consulted for a JSON-RPC Agent, for the same reason it is not for MCP: an A2A
// client advertises what it can read, not what its protocol permits, and an XML error would
// be unreadable to it whatever it said it accepted.
func TestNegotiate_AgentJSONRPCIgnoresAccept(t *testing.T) {
	got := Negotiate(Request{
		APIKind:     policy.APIKindAgent,
		Transport:   string(agentproto.TransportJSONRPC),
		ContentType: "application/json",
		Accept:      "application/xml",
	})
	assert.Equal(t, ShapeJSONRPC, got)
}
