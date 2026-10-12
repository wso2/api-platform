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
	"encoding/xml"
	"strconv"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// parseQValue parses an Accept q-value, rejecting anything outside [0,1].
func parseQValue(s string) (float64, error) {
	q, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	if q < 0 || q > 1 {
		return 0, strconv.ErrRange
	}
	return q, nil
}

// ─── JSON ────────────────────────────────────────────────────────────────────

// FaultDetails.Description is deliberately NOT rendered by ANY renderer here: for a response
// guardrail it is the content the guardrail existed to stop from leaving. Message is the
// client-facing summary; Description reaches logs and audit sinks, or a template the operator
// wrote with a ${Description} placeholder.
//
// FaultDetails.Guardrail IS rendered, and the decision is the policy's: the shipped guardrails
// attach the naming fields on every intervention and populate Assessments only when their own
// showAssessment parameter permits. The formatter renders what is there and makes no safety
// judgement of its own.

// jsonRenderer emits the canonical JSON error object.
//
// The field set is fixed. It is not the union of every shape the policy catalogue
// currently produces (`{"error","message"}` from jwt-auth, `{"type","message"}` from the
// guardrails, `{"error"}` from the router's 404): those disagree on both names and
// meaning — "message" is a human summary in one and a short code name in another — so
// reproducing them would mean codifying the inconsistency this package exists to end.
type jsonRenderer struct{}

func (jsonRenderer) ContentType() string { return "application/json" }

type jsonErrorBody struct {
	Code      string          `json:"code,omitempty"`
	Type      string          `json:"type,omitempty"`
	Message   string          `json:"message"`
	ErrorID   string          `json:"error_id,omitempty"`
	Guardrail *guardrailBlock `json:"guardrail,omitempty"`
}

func (jsonRenderer) Render(in RenderInput) []byte {
	e := in.Err
	body, err := json.Marshal(jsonErrorBody{
		Code:      e.Code,
		Type:      e.Type,
		Message:   fallbackMessage(e),
		ErrorID:   in.ErrorID,
		Guardrail: guardrailFor(e),
	})
	if err != nil {
		// Unreachable for this struct (all fields are strings), but an error path that
		// returns nothing is worse than one that returns a terse valid body.
		return []byte(`{"message":"An unexpected error occurred."}`)
	}
	return body
}

// ─── JSON-RPC 2.0 ────────────────────────────────────────────────────────────

// jsonRPCRenderer emits a JSON-RPC 2.0 error response, as MCP requires.
//
// Matches the shape mcp-auth already produces for protocol errors, so a client sees one
// consistent error format from the gateway rather than two.
type jsonRPCRenderer struct{}

func (jsonRPCRenderer) ContentType() string { return "application/json" }

const jsonRPCVersion = "2.0"

// JSON-RPC reserves -32768..-32000. Gateway errors are server-side failures, so they map
// onto the reserved server range rather than inventing codes that could collide with an
// application's own.
const (
	jsonRPCInvalidRequest = -32600 // the request was not a valid request object
	jsonRPCInternalError  = -32603 // the server failed to fulfil a valid request
)

type jsonRPCError struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Data    *jsonRPCData `json:"data,omitempty"`
}

// jsonRPCData is the protocol's own slot for implementation-defined detail. Only fields the
// policy or the engine stated are returnable go in it — never Description.
//
// Code and Type are here because error.code above is the JSON-RPC protocol code, which leaves
// the code with nowhere else to go. Without this an MCP caller is the one caller that
// cannot see it — and the numbering exists precisely so a client that already keys off an existing
// code keeps working. The names match the JSON and XML shapes, so a client reading `code` at
// the top level of one shape reads `data.code` in this one.
//
// Two fields named `code` in one document is a real cost, accepted deliberately: they sit in
// different objects, they are different types (protocol int vs code string), and renaming this
// one would break the cross-shape symmetry that makes the field findable at all.
type jsonRPCData struct {
	// Code is the gateway error code, NOT the JSON-RPC code in the enclosing error object.
	Code      string          `json:"code,omitempty"`
	Type      string          `json:"type,omitempty"`
	ErrorID   string          `json:"error_id,omitempty"`
	Guardrail *guardrailBlock `json:"guardrail,omitempty"`
}

type jsonRPCBody struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      any          `json:"id"`
	Error   jsonRPCError `json:"error"`
}

func (jsonRPCRenderer) Render(in RenderInput) []byte {
	e := in.Err
	body, err := json.Marshal(jsonRPCBody{
		JSONRPC: jsonRPCVersion,
		ID:      jsonRPCIDFor(e),
		Error: jsonRPCError{
			Code:    jsonRPCCodeFor(e, in.Status),
			Message: fallbackMessage(e),
			Data:    jsonRPCDataFor(in),
		},
	})
	if err != nil {
		// The only field that can fail to marshal is the id, which is whatever the policy
		// read out of the request. Retry without it rather than dropping to the constant:
		// a correct error missing its correlation beats a generic one.
		if body, err = json.Marshal(jsonRPCBody{
			JSONRPC: jsonRPCVersion,
			ID:      nil,
			Error: jsonRPCError{
				Code:    jsonRPCCodeFor(e, in.Status),
				Message: fallbackMessage(e),
				Data:    jsonRPCDataFor(in),
			},
		}); err != nil {
			return []byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"An unexpected error occurred."}}`)
		}
	}
	return body
}

// jsonRPCCodeFor picks the error code: the policy's if it stated one, otherwise derived
// from the HTTP status.
//
// The derivation is right for a failure the GATEWAY produced — it knows a status and nothing
// about the JSON-RPC call — and wrong for a policy that parsed the request and knows the call
// was -32602 Invalid params or -32700 Parse error. Neither of those is reachable from a
// status, which is why a policy has to be able to say so.
func jsonRPCCodeFor(e policy.FaultDetails, status int) int {
	if e.JSONRPC != nil && e.JSONRPC.Code != nil {
		return *e.JSONRPC.Code
	}
	if status >= 400 && status < 500 {
		return jsonRPCInvalidRequest
	}
	return jsonRPCInternalError
}

// jsonRPCIDFor echoes the request id when a policy supplied one, and is null otherwise.
//
// Null is the default because the id lives in the request BODY, and most errors are produced
// in the header phase where no body has been read. JSON-RPC explicitly permits null for a
// request whose id could not be determined, and guessing would be worse — a wrong id
// correlates the error to the wrong call.
//
// A policy that read the body is the exception, and the only thing that can lift the default:
// it has the real id, so a client with several calls in flight can tell which one failed.
func jsonRPCIDFor(e policy.FaultDetails) any {
	if e.JSONRPC == nil {
		return nil
	}
	return e.JSONRPC.ID
}

// jsonRPCEventStreamRenderer wraps the JSON-RPC error object in a single SSE event, for a
// caller that opened the exchange as an event stream.
//
// It composes rather than reimplements: the error object is identical, only the framing
// differs, and two copies of the object would drift the moment either gained a field.
//
// The framing is one `data:` line per line of the payload followed by a blank line, per the
// event-stream format. The payload is compact JSON with no newlines in it, so this is a single
// data line in practice — the split is there so it stays correct if that ever stops being true.
type jsonRPCEventStreamRenderer struct{}

func (jsonRPCEventStreamRenderer) ContentType() string { return "text/event-stream" }

func (r jsonRPCEventStreamRenderer) Render(in RenderInput) []byte {
	payload := jsonRPCRenderer{}.Render(in)
	var out bytes.Buffer
	for _, line := range bytes.Split(payload, []byte("\n")) {
		out.WriteString("data: ")
		out.Write(line)
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	return out.Bytes()
}

// ─── XML ─────────────────────────────────────────────────────────────────────

// xmlRenderer emits a plain XML error document for a REST caller that asked for XML.
//
// Deliberately NOT a SOAP fault: a SOAP fault is only correct inside a SOAP envelope
// with the right namespace for the version in play, and emitting a fault-shaped body to
// a plain-XML caller would be as wrong as sending JSON to a SOAP one.
type xmlRenderer struct{}

func (xmlRenderer) ContentType() string { return "application/xml" }

type xmlErrorBody struct {
	XMLName   xml.Name           `xml:"error"`
	Code      string             `xml:"code,omitempty"`
	Type      string             `xml:"type,omitempty"`
	Message   string             `xml:"message"`
	ErrorID   string             `xml:"errorId,omitempty"`
	Guardrail *xmlGuardrailBlock `xml:"guardrail,omitempty"`
}

// xmlGuardrailBlock mirrors guardrailBlock, with the assessments carried as a JSON string.
//
// encoding/xml cannot marshal an arbitrary map, and an assessment is whatever the guardrail
// put there. Dropping it for XML callers would silently withhold detail the operator opted
// into, so it travels as text — parseable by anyone who wants it, visible to everyone else.
type xmlGuardrailBlock struct {
	InterveningGuardrail string `xml:"interveningGuardrail,omitempty"`
	Action               string `xml:"action,omitempty"`
	ActionReason         string `xml:"actionReason,omitempty"`
	Assessments          string `xml:"assessments,omitempty"`
}

func (xmlRenderer) Render(in RenderInput) []byte {
	e := in.Err
	// xml.Marshal escapes text content, so a message containing markup cannot break
	// out of its element.
	body, err := xml.Marshal(xmlErrorBody{
		Code:      e.Code,
		Type:      e.Type,
		Message:   fallbackMessage(e),
		ErrorID:   in.ErrorID,
		Guardrail: xmlGuardrailFor(e),
	})
	if err != nil {
		return []byte(`<error><message>An unexpected error occurred.</message></error>`)
	}
	return append([]byte(xml.Header), body...)
}

// ─── guardrail ───────────────────────────────────────────────────────────────

// guardrailBlock is the rendered form of FaultDetails.Guardrail.
//
// Field names match the assessment envelope the shipped guardrails publish in their own
// bodies, so a client that already parses that envelope reads this one too.
type guardrailBlock struct {
	InterveningGuardrail string         `json:"interveningGuardrail,omitempty"`
	Action               string         `json:"action,omitempty"`
	ActionReason         string         `json:"actionReason,omitempty"`
	Assessments          map[string]any `json:"assessments,omitempty"`
}

// guardrailFor renders the block when the policy attached one, and nothing otherwise.
//
// A nil block means "no guardrail was involved", which is why the field is a pointer. It does
// NOT mean "the operator declined to show the assessment" — the shipped guardrails gate that
// per field, leaving Assessments empty while still attaching the block. Both nils are handled
// by the same omitempty, so the formatter makes no safety judgement either way.
func guardrailFor(e policy.FaultDetails) *guardrailBlock {
	if e.Guardrail == nil {
		return nil
	}
	return &guardrailBlock{
		InterveningGuardrail: e.Guardrail.InterveningGuardrail,
		Action:               e.Guardrail.Action,
		ActionReason:         e.Guardrail.ActionReason,
		Assessments:          e.Guardrail.Assessments,
	}
}

// jsonRPCDataFor fills the protocol's data slot, or leaves it absent when there is nothing
// returnable to put there. An empty object would be worse than no field: it says "there is
// detail" and then carries none.
func jsonRPCDataFor(in RenderInput) *jsonRPCData {
	g := guardrailFor(in.Err)
	if g == nil && in.ErrorID == "" && in.Err.Code == "" && in.Err.Type == "" {
		return nil
	}
	return &jsonRPCData{
		Code:      in.Err.Code,
		Type:      in.Err.Type,
		ErrorID:   in.ErrorID,
		Guardrail: g,
	}
}

// xmlGuardrailFor is guardrailFor for XML, flattening the assessments to JSON text.
func xmlGuardrailFor(e policy.FaultDetails) *xmlGuardrailBlock {
	g := guardrailFor(e)
	if g == nil {
		return nil
	}
	out := &xmlGuardrailBlock{
		InterveningGuardrail: g.InterveningGuardrail,
		Action:               g.Action,
		ActionReason:         g.ActionReason,
	}
	if len(g.Assessments) > 0 {
		if raw, err := json.Marshal(g.Assessments); err == nil {
			out.Assessments = string(raw)
		}
	}
	return out
}

// ─── shared ──────────────────────────────────────────────────────────────────

// fallbackMessage guarantees every rendered error carries a message.
//
// A renderer is frequently reached with a sparsely-populated error — derived from
// an Envoy local reply that carried no structure at all, for instance — and a body whose
// only human-readable field is empty is worse for the caller than a generic sentence.
func fallbackMessage(e policy.FaultDetails) string {
	if e.Message != "" {
		return e.Message
	}
	return "An unexpected error occurred."
}
