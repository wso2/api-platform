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
	"net/http"
	"strings"

	policy "github.com/wso2/api-platform/sdk/core/policy/v1alpha2"
)

// Decision is what the caller should do with an error response.
type Decision struct {
	// Format is true when the caller must replace the body with Body/ContentType.
	Format bool
	// Body is the rendered error body, set only when Format is true.
	Body []byte
	// ContentType describes Body, set only when Format is true.
	ContentType string
	// Reason explains the outcome, for logging. Always set.
	Reason string
}

// knownAPIKinds is every kind the engine can see on a route, used to tell a misspelled name
// from a kind that simply is not enabled. Kept here rather than in the kernel because this
// package already owns what a kind means for formatting.
var knownAPIKinds = []policy.APIKind{
	policy.APIKindRestApi,
	policy.APIKindLlmProvider,
	policy.APIKindLlmProxy,
	policy.APIKindMCP,
	policy.APIKindAgent,
	policy.APIKindWebSubApi,
}

// supportedKinds is the set of API kinds the gateway synthesizes an error body for.
//
// In code rather than configuration because formatting is a protocol concern, not an operator
// preference.
//
// Agent only. Every other kind returns the errors it always has, and an Agent API has none to
// preserve — it is new in this release, so there is no body any client has already been
// written against.
//
// It is also the kind that needs this most. MCP fixes a protocol too, but its policies write
// their own JSON-RPC envelope, so an MCP rejection is already the right shape before this
// package is consulted. A2A has no such policies: a rejection on an Agent route comes from an
// ordinary policy like api-key-auth, which writes ordinary JSON — unparseable to the JSON-RPC
// client on the other end. Nothing else fills that gap.
var supportedKinds = []policy.APIKind{
	policy.APIKindAgent,
}

// SupportedKinds returns the kinds error-body synthesis is enabled for.
//
// Empty today; see supportedKinds.
func SupportedKinds() KindSet {
	set, _ := NewKindSet(kindNames(supportedKinds))
	return set
}

func kindNames(kinds []policy.APIKind) []string {
	names := make([]string, 0, len(kinds))
	for _, k := range kinds {
		names = append(names, string(k))
	}
	return names
}

// KindSet answers "may the formatter synthesize a body for this API kind?".
//
// A nil set says no to everything, which is what supportedKinds yields today.
type KindSet map[string]bool

// NewKindSet builds a KindSet from kind names, returning any entry that is not a known API
// kind.
//
// Unknown entries are REPORTED rather than rejected. An unknown name cannot match a real
// route, so it can only ever leave formatting off — the safe direction — and refusing to
// start over one would take a gateway down for something that changes no behaviour. The
// caller logs what came back so a mistake is visible instead of silently doing nothing.
func NewKindSet(kinds []string) (KindSet, []string) {
	if len(kinds) == 0 {
		return nil, nil
	}
	known := make(map[string]bool, len(knownAPIKinds))
	for _, k := range knownAPIKinds {
		known[strings.ToLower(string(k))] = true
	}

	set := make(KindSet, len(kinds))
	var unknown []string
	for _, raw := range kinds {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if !known[name] {
			unknown = append(unknown, raw)
			continue
		}
		set[name] = true
	}
	if len(set) == 0 {
		return nil, unknown
	}
	return set, unknown
}

// Enabled reports whether this kind is in the set. Case-insensitive, so a name spelled
// "restapi" matches APIKindRestApi.
//
// An empty kind — a request that matched no route, so there is no API to have a kind — is
// never enabled: there is nothing that could have named it.
func (s KindSet) Enabled(kind policy.APIKind) bool {
	if len(s) == 0 || kind == "" {
		return false
	}
	return s[strings.ToLower(string(kind))]
}

// Input is everything the decision depends on.
type Input struct {
	// FormatterEnabled reports whether synthesis is enabled for this API kind — see
	// supportedKinds. False for every kind shipping today, which is the whole of the
	// shipped default.
	FormatterEnabled bool
	// BodyAuthored reports whether something decided the FINAL body the client receives —
	// a policy that wrote one without describing an error, or any fault entry. See
	// ShouldFormat for why this is neither "the body has bytes in it" nor "a policy set a
	// body".
	BodyAuthored bool
	// Err is the producing policy's account of the failure. A zero value is normal and
	// still renderable — see Render.
	Err policy.FaultDetails
	// PolicyDescribed reports that a POLICY described this failure: the producing policy
	// declared a Fault, or a fault entry re-described it. False for a failure only the
	// engine can account for — a router error, the engine's own 500, a resolution failure —
	// even though Err may carry the engine's description of it.
	//
	// Formatting is opt-in through this, and that is a compatibility decision: gateways
	// released before the fault contract sent those errors unformatted, and a policy that
	// predates the contract describes nothing. Rendering only what a policy described keeps
	// every such response byte-for-byte what it was.
	PolicyDescribed bool
	// Status is the response status, used only to pick a JSON-RPC code.
	Status int
	// ErrorID correlates a client-visible error with the engine log entry that explains it.
	// Set only for the engine's own failures — see RenderInput.ErrorID.
	ErrorID string
	// RequestMethod is the request's method. Only HEAD matters: a HEAD response carries no
	// body by definition, so synthesizing one would contradict the method.
	RequestMethod string
	// APIKind, ContentType and Accept are the request-side shape signals.
	APIKind     policy.APIKind
	ContentType string
	Accept      string
	// Transport is the wire protocol an Agent route serves, "JSONRPC" or "HTTP+JSON".
	//
	// Needed because an Agent API, unlike an MCP one, does not fix its protocol by kind:
	// the two transports carry the same content type and are told apart only by how the
	// route was configured. Empty for every other kind, and empty for an Agent request
	// that failed before its operation resolved.
	Transport string
}

// ShouldFormat decides whether to synthesize a body, and renders it if so.
//
// Nothing is synthesized for a kind outside supportedKinds, and that gate comes first.
//
// For an enabled kind, authorship decides the rest:
//
//	Not described by a policy -> leave alone (see Input.PolicyDescribed)
//	Fault described   -> render, whatever the body holds
//	no Fault          -> the body stands, described or empty
//
// A body accompanying a described fault is a FALLBACK for a gateway that cannot render, not a
// decision — so omitting Fault is how a policy keeps its own body. A body written by a fault
// ENTRY is authorship unconditionally, since it ran after the description.
//
// The gate is BodyAuthored, not len(body) == 0: a router local reply carries a non-empty body
// no MCP or SOAP client can parse, and an explicitly empty body ([]byte{}) is a decision that
// the client gets nothing. The caller tracks authorship and this function trusts it.
func ShouldFormat(r *Registry, in Input) Decision {
	if strings.EqualFold(in.RequestMethod, http.MethodHead) {
		// A HEAD response has no body. Envoy recalculates content-length from an ext_proc
		// body mutation, so writing one here would produce a response that contradicts the
		// method — the kind of thing that passes a status assertion and breaks a client.
		return Decision{Reason: "HEAD response carries no body"}
	}

	if !in.FormatterEnabled {
		// This kind is not in supportedKinds. Checked before authorship so the log says which
		// of the two stopped it — someone debugging "why is my error not a SOAP fault" needs
		// to tell "this kind does not format" apart from "a policy wrote a body".
		return Decision{Reason: "error formatting not enabled for API kind " + string(in.APIKind)}
	}

	if in.BodyAuthored {
		// Something already decided the final body. This is the customer-override path: a
		// policy that writes a body and describes no Error switches this package off for that
		// route, with no ordering requirement between the two.
		return Decision{Reason: "body already authored by a policy"}
	}

	if !in.PolicyDescribed {
		// Checked after authorship only so the log keeps naming an authored body as the
		// reason when both apply; the outcome is the same either way.
		return Decision{Reason: "no policy described the failure"}
	}

	shape := Negotiate(Request{
		APIKind:     in.APIKind,
		Transport:   in.Transport,
		ContentType: in.ContentType,
		Accept:      in.Accept,
	})

	body, contentType, ok := r.Render(shape, RenderInput{
		Err:     in.Err,
		Status:  in.Status,
		ErrorID: in.ErrorID,
	})
	if !ok {
		// Passthrough, or a declared shape with no renderer yet. Both mean "leave it
		// alone", which is the pre-existing behaviour rather than a new wrong one.
		return Decision{Reason: "no formatting for shape " + string(shape)}
	}

	return Decision{
		Format:      true,
		Body:        body,
		ContentType: contentType,
		Reason:      "formatted as " + string(shape),
	}
}

// DescribedError reports whether an action supplied an FaultDetails.
//
// Paired with BodyAuthored to tell a fallback body apart from an authored one: an action with
// both set is describing a failure and providing something for a gateway that cannot render
// it, not deciding what this gateway sends.
func DescribedError(action policy.ResponseAction) bool {
	switch a := action.(type) {
	case policy.ImmediateResponse:
		return a.Fault != nil
	case policy.DownstreamResponseModifications:
		return a.Fault != nil
	default:
		return false
	}
}

// BodyAuthored reports whether an action explicitly decided the client's body.
//
// nil means the action expressed no opinion; a non-nil slice — including an empty one — is a
// decision. That mirrors the SDK's own documented meaning for these fields, where nil is
// passthrough and []byte{} clears the body.
func BodyAuthored(action policy.ResponseAction) bool {
	switch a := action.(type) {
	case policy.ImmediateResponse:
		return a.Body != nil
	case policy.DownstreamResponseModifications:
		return a.Body != nil
	default:
		return false
	}
}
