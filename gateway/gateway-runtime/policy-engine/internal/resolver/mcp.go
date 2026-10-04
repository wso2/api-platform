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

package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wso2/api-platform/common/chainkey"
)

// MCPResolverName is the registered name of the MCP resolver, matching the
// resolver_name the controller emits on an MCP proxy's POST route.
const MCPResolverName = "mcp"

// mcpEnrichOnlyOperation is the operation component every MCP chain key carries today.
//
// It is a constant because this resolver is *enrich-only*: one policy chain per route,
// exactly as MCP ships now, with the resolver contributing facts rather than selecting
// between chains. A composed key is still required — validateResolvedKey rejects a
// protocol resolver that returns the route key — so the key is composed from the
// captured partition with a fixed operation.
//
// Per-operation chains ("tools/call:get_weather") are the natural next step and are the
// only thing that changes here when they are taken up.
const mcpEnrichOnlyOperation = "mcp"

// Attribute keys this resolver publishes. Every key is prefixed mcp.body. because MCP
// 2026-07-28 carries the same facts in both headers and the body, and the two
// disagreeing is a security-relevant condition (JSON-RPC -32020) rather than a detail.
// A consumer comparing the two sides must never be in doubt which side it holds.
//
// There is deliberately no mcp.header.* group: headers already reach every policy
// through RequestHeaderContext, and copying them here would create a second source of
// truth for a value that already has one.
const (
	AttrMCPBodyMethod           = "mcp.body.method"
	AttrMCPBodyCapabilityType   = "mcp.body.capability.type"
	AttrMCPBodyCapabilityAction = "mcp.body.capability.action"
	AttrMCPBodyCapabilityName   = "mcp.body.capability.name"
	AttrMCPBodyProtocolVersion  = "mcp.body.protocol.version"

	// AttrMCPBodyTaskID is the task a tasks/* operation addresses, from params.taskId.
	// The Tasks extension has a modern client mirror it into Mcp-Name, so a policy needs the
	// body value to check that header against.
	//
	// Kept apart from AttrMCPBodyCapabilityName deliberately: a task id is a handle to one
	// in-flight operation, not a capability an operator writes rules against, and the policies
	// that match on a capability name must not start matching on task ids.
	AttrMCPBodyTaskID = "mcp.body.task.id"
	// AttrMCPBodyJSONRPCID carries the id as a JSON *token*, not a display string: 7 for a
	// number and "7" with its quotes for a string. A consumer echoes it into an error
	// envelope verbatim; unwrapping it would lose the type a client correlates on.
	AttrMCPBodyJSONRPCID = "mcp.body.jsonrpc.id"

	// The client that sent the request, which MCP states in a different place in each era:
	// params.clientInfo on a legacy initialize, params._meta on every modern request. Both
	// publish here, so a consumer never branches on era to learn who is calling.
	//
	// Telemetry only. The spec is explicit that clientInfo is self-reported, unverified, and
	// "intended for display, logging, and debugging" — implementations SHOULD NOT use it to
	// change behaviour or rely on it for security decisions. No policy may govern on these.
	AttrMCPBodyClientName    = "mcp.body.client.name"
	AttrMCPBodyClientVersion = "mcp.body.client.version"

	// AttrMCPBodyRequestStateHash fingerprints the opaque requestState a client echoes back
	// when it retries a request the server answered with input_required. It correlates the
	// retry with that answer, which nothing else can: the spec requires the JSON-RPC id to
	// DIFFER between the two, since they are independent requests.
	//
	// Hashed, not published raw, for two reasons. The value is a capability a log reader
	// could replay, and the spec has servers encode a principal, a TTL and a request digest
	// into it. And a realistic AEAD blob exceeds the kernel's 256-character attribute limit,
	// so the raw value would be dropped for length and the fact lost without a trace.
	AttrMCPBodyRequestStateHash = "mcp.body.request.state.hash"

	// AttrMCPBodyPresent reports that bytes arrived. Its value is always "true" — the key's
	// presence is the fact.
	//
	// Without it two states both publish nothing: a JSON-RPC response that names no
	// operation, which is authoritative, and a request with no body, where a consumer must
	// fail closed. Published on every path where bytes arrived, AttrMCPBodyUnusable
	// included, so "was there anything to go on" needs no check of the reason first.
	//
	//	absent                     no body
	//	present                    a body, read fine — facts follow if it named any
	//	present + unusable         a body, and this resolver could not read it
	AttrMCPBodyPresent = "mcp.body.present"

	// AttrMCPBodyUnusable reports that no JSON-RPC envelope could be read from the body, and
	// why. Values are the MCPBodyUnusable* constants below.
	//
	// It means the body could not be READ, not that it held little: a bodyless request, or
	// an object naming no method, reads fine and publishes no reason. Published instead of
	// the facts, never alongside them, since a method taken from a body that reads two ways
	// is the problem this guards. AttrMCPBodyPresent is still published.
	AttrMCPBodyUnusable = "mcp.body.unusable"
)

// Reasons for AttrMCPBodyUnusable. A closed set, so the attribute stays span-safe.
//
// They are split along the same line the released MCP policies split their JSON-RPC error
// codes: broken syntax was a parse error (-32700), everything else an invalid request
// (-32600). Reporting one undifferentiated "malformed" would force a consumer to answer
// -32700 for a request that merely used the wrong type, which is not what clients saw
// before.
const (
	// MCPBodySyntaxError is a body whose JSON does not parse — almost always truncation,
	// e.g. `{"id":1,"method":"tools/call"` with no closing brace, or a trailing comma.
	// Maps to JSON-RPC -32700.
	MCPBodySyntaxError = "syntax-error"

	// MCPBodyInvalidMemberType is well-formed JSON in which a member this resolver reads
	// carries the wrong type: `{"method":42}`, `{"method":["a"]}`, or a non-object
	// `_meta`. The document is fine; one of its values is not what it must be.
	//
	// Only members that are modelled can cause this. `params` and `id` are held raw
	// precisely so their shape cannot fail the envelope — JSON-RPC permits `"params":[]`,
	// and rejecting it would discard a perfectly readable method over an unrelated field.
	// Maps to JSON-RPC -32600.
	MCPBodyInvalidMemberType = "invalid-member-type"

	// MCPBodyNotAnObject is valid JSON that is not a request object at all — a JSON-RPC
	// batch, which names several operations and therefore identifies none, or a bare
	// scalar. Maps to JSON-RPC -32600.
	MCPBodyNotAnObject = "not-an-object"

	// MCPBodyAmbiguous is an object naming a member this resolver reads in more than one
	// way. Unlike the others, a backend may not reject it — it may simply resolve a
	// different value than the gateway did, which is the confused-deputy vector. Maps to
	// JSON-RPC -32600.
	MCPBodyAmbiguous = "ambiguous"
)

// spanSafeAttributes is the subset of published attributes that may be stamped on a
// trace span. The rule is closed sets only: a span attribute is indexed by the tracing
// backend, so a caller-chosen value mints one index entry per distinct value.
//
// mcp.body.capability.name is excluded because a tool name is an open set, and
// mcp.body.jsonrpc.id because it is unique per request — the worst possible span
// attribute. Both remain available to policies, which can bound them for their own use.
//
// A future resolver publishing attributes adds its own span-safe keys here.
var spanSafeAttributes = map[string]bool{
	AttrMCPBodyMethod:           true,
	AttrMCPBodyCapabilityType:   true,
	AttrMCPBodyCapabilityAction: true,
	AttrMCPBodyProtocolVersion:  true,
	// A closed set of four reasons, and the attribute an operator would most want to
	// alert on: a rising count means someone is probing the parser.
	AttrMCPBodyUnusable: true,

	// AttrMCPBodyPresent is deliberately absent. It is a closed set and so would pass the
	// rule above, but its value is always "true" — a span attribute with one possible
	// value carries no information and only costs an index entry.
	//
	// AttrMCPBodyRequestStateHash is absent: one value per exchange is unbounded cardinality,
	// and a correlation token is not a span facet.
	//
	// AttrMCPBodyClientName and AttrMCPBodyClientVersion are absent on the spec's own terms:
	// clientInfo is self-reported and unverified, so indexing it would invite exactly the
	// reliance the spec warns against. Unbounded cardinality is the second reason, not the
	// first, and is why AttrMCPBodyCapabilityName is absent too.
}

// IsSpanSafeAttribute reports whether an attribute key may be recorded on a span.
// Unknown keys are not span-safe: a new attribute must be reasoned about before it is
// indexed, rather than inheriting permission by default.
func IsSpanSafeAttribute(key string) bool { return spanSafeAttributes[key] }

// mcpMetaProtocolVersionKey is where a modern request states its protocol version
// inside _meta. MCP namespaces its reserved _meta members, so the slash is part of the
// key rather than a path separator.
const mcpMetaProtocolVersionKey = "io.modelcontextprotocol/protocolVersion"

// mcpMetaClientInfoKey is the same for the client's identity, which 2026-07-28 moved out of
// the initialize handshake and into per-request _meta.
const mcpMetaClientInfoKey = "io.modelcontextprotocol/clientInfo"

// mcpCapabilityTypeResource is named because it is the one family MCP identifies by uri
// rather than by name — see capabilityName.
const mcpCapabilityTypeResource = "resource"

// mcpCapabilityTypeTask is named because the Tasks extension identifies its operations by
// params.taskId, which is published on its own key rather than as a capability name.
const mcpCapabilityTypeTask = "task"

// mcpCapabilityTypes maps a method's family segment to the singular capability type.
//
// Singular is chosen deliberately: the MCP policies are split between a plural and a
// singular form in SharedContext.Metadata, and this canonical set has to pick one rather
// than inherit the split.
var mcpCapabilityTypes = map[string]string{
	"tools":         "tool",
	"resources":     mcpCapabilityTypeResource,
	"prompts":       "prompt",
	"tasks":         mcpCapabilityTypeTask,
	"completion":    "completion",
	"logging":       "logging",
	"notifications": "notification",
	"subscriptions": "subscription",
	"sampling":      "sampling",
	"elicitation":   "elicitation",
	"roots":         "roots",
	"server":        "server",
}

// MCPResolver is the resolver factory for MCP proxy routes. It parses the JSON-RPC body
// once before the chain runs, replacing up to six unmarshals of the same bytes by the MCP
// policies and the analytics policy.
//
// Both eras prepare identically: the body is always the side facts come from, so a policy
// governing on a modern request's mirrored headers has something to check them against.
//
// resolver_config is ignored, so a field a newer controller emits cannot fail a route here.
type MCPResolver struct{}

// Name returns the wire value the controller emits for MCP routes.
func (*MCPResolver) Name() string { return MCPResolverName }

// Prepare builds one MCP route's resolver. The chain key is composed once here, so a
// request costs only a parse and a map build.
func (*MCPResolver) Prepare(cfg ResolverRouteConfig) (PreparedResolver, error) {
	if !chainkey.ValidComponent(cfg.APIID) {
		// Composing a key from an empty or separator-bearing API id would produce a key
		// that either fails partition validation or, worse, reaches another partition.
		// Reject the route at ingest instead of per request.
		return nil, fmt.Errorf("mcp resolver requires a valid API id, got %q", cfg.APIID)
	}

	return &preparedMCP{chainKey: ChainKeyFor(cfg.APIID, cfg.Vhost, mcpEnrichOnlyOperation)}, nil
}

// preparedMCP is an MCP route whose request body is parsed once, before the chain runs,
// and whose findings are published for policies to read.
//
// Every MCP route prepares this way, legacy or modern. A modern request mirrors its method
// and capability name into headers, but this resolver reads only the body: two sources for
// one value leave a consumer unable to tell which it holds, and their disagreement is a
// security condition (-32020). Policies already read headers via RequestHeaderContext.
//
// So an empty ResolutionAttributes means no MCP resolver ran, not that the body was skipped.
// A body read but not interpretable is reported under AttrMCPBodyUnusable.
type preparedMCP struct {
	chainKey string
}

// Requirements asks for the buffered body, so the kernel defers chain selection to the
// body callback. Header-phase policies run there too, after the body is read.
func (*preparedMCP) Requirements() RequestRequirements {
	return RequestRequirements{Body: BodyBuffered}
}

// Resolve reads the MCP request body and publishes its facts. The chain key is fixed for
// this route; the body supplies only the method, capability name, id and _meta.
//
// A body that cannot be read yields AttrMCPBodyUnusable with an MCPBodyUnusable* reason
// rather than a resolver error, so the chain still runs and an MCP policy can answer in
// the JSON-RPC error shape the client expects.
//
// A nil body is not a fault: a request whose headers end the stream gets no body callback,
// so a BodyBuffered resolver is called with Body nil (extproc.go).
func (p *preparedMCP) Resolve(_ context.Context, view RequestView) (Resolution, error) {
	res := Resolution{ChainKey: p.chainKey}

	if len(view.Body) == 0 {
		// Nothing to read, which is not the same as failing to read. Legitimate for a
		// bodyless request; consumers fail closed on the missing method.
		return res, nil
	}

	// Bytes arrived. Set before any verdict so every path below reports it, the unusable
	// ones included — see AttrMCPBodyPresent for why presence and readability stay
	// orthogonal. Whitespace counts as bytes: it is a body, just not a readable one.
	res.Attributes = map[string]string{AttrMCPBodyPresent: "true"}

	body := trimLeadingSpace(view.Body)
	if len(body) == 0 {
		res.Attributes[AttrMCPBodyUnusable] = MCPBodySyntaxError
		return res, nil
	}

	if body[0] != '{' {
		// Two different failures share this branch, and the codes the consuming policies
		// answer with differ: a batch or a scalar is valid JSON of the wrong kind, while
		// anything that does not parse is a syntax error. A batch names several operations
		// and so identifies none — picking one to enforce on while the server runs them
		// all would be the same divergence ambiguity creates.
		reason := MCPBodyNotAnObject
		if !json.Valid(body) {
			reason = MCPBodySyntaxError
		}
		res.Attributes[AttrMCPBodyUnusable] = reason
		return res, nil
	}

	// Before unmarshalling: see the ordering note above. Report it and publish none of the
	// facts — a method taken from a body that reads two ways is precisely what must not
	// reach a policy.
	if hasAmbiguousMembers(body) {
		res.Attributes[AttrMCPBodyUnusable] = MCPBodyAmbiguous
		return res, nil
	}

	var env mcpEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		// Distinguish the two failures the released policies distinguished: broken syntax
		// was a parse error, a wrong type an invalid request. Collapsing them would make a
		// consumer answer -32700 for a request that merely used the wrong type.
		reason := MCPBodyInvalidMemberType
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			reason = MCPBodySyntaxError
		}
		res.Attributes[AttrMCPBodyUnusable] = reason
		return res, nil
	}

	if reason := env.addAttributes(res.Attributes); reason != "" {
		res.Attributes[AttrMCPBodyUnusable] = reason
	}
	return res, nil
}

// mcpEnvelope is the subset of a JSON-RPC request this resolver reads. Everything else
// in the payload is deliberately not modelled: this is a fixed vocabulary, not a
// projection of whatever the caller sent.
type mcpEnvelope struct {
	Method string          `json:"method"`
	ID     json.RawMessage `json:"id"`

	// Params is held raw and decoded separately. JSON-RPC 2.0 permits params to be an
	// array as well as an object, so declaring it as a struct would make the whole
	// unmarshal fail on a legitimate `"params": []` — discarding a perfectly readable
	// method because an unrelated member has an unexpected shape.
	Params json.RawMessage `json:"params"`
}

// mcpParams is the subset of an object-shaped params this resolver reads.
type mcpParams struct {
	Name string `json:"name"`
	URI  string `json:"uri"`

	// The task a tasks/* operation addresses. Governed, since Mcp-Name is checked against it.
	TaskID string `json:"taskId"`

	// Where a legacy initialize states what modern requests put in _meta.
	ClientInfo      *mcpClientInfo `json:"clientInfo"`
	ProtocolVersion string         `json:"protocolVersion"`

	RequestState string `json:"requestState"`

	Meta map[string]json.RawMessage `json:"_meta"`
}

// mcpClientInfo is the client identity, shaped the same in both eras — only its location moves.
type mcpClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// governedParams holds only the members the published facts are read from. A wrong type in
// one of these means the capability or the era could not be read, which is a body this
// resolver could not read rather than one that carried little — and the difference decides
// how a policy acts, since "named no capability" reads as nothing to govern.
//
// They are decoded on their own because encoding/json reports only the first type error.
// Decoded as part of mcpParams, a wrong-typed telemetry member earlier in the object would
// hide one here: {"clientInfo":42,"name":5} reported clientInfo and published no name.
type governedParams struct {
	Name            string `json:"name"`
	URI             string `json:"uri"`
	TaskID          string `json:"taskId"`
	ProtocolVersion string `json:"protocolVersion"`
}

// params decodes the params member when it is an object. A non-object params is valid
// JSON-RPC and simply names no capability, so it yields the zero value rather than a reason.
//
// It returns the reason the body is unusable, or "". encoding/json keeps decoding past a
// type error, so a partly decoded value carries some members and silently drops others:
// {"name":5,"uri":"file:///x"} yields an empty Name beside a populated URI. Publishing that
// would hand a policy a body that read one way here and may read another at the server,
// which is the divergence an unusable reason exists to report.
//
// Only a governed member triggers it. A telemetry member that will not decode - clientInfo,
// requestState, _meta - leaves every published fact readable, and rejecting on it would make
// this gateway refuse requests the server accepts.
func (e *mcpEnvelope) params() (mcpParams, string) {
	var p mcpParams
	if len(e.Params) == 0 || !isJSONObject(e.Params) {
		return p, ""
	}

	var governed governedParams
	var typeErr *json.UnmarshalTypeError
	if err := json.Unmarshal(e.Params, &governed); errors.As(err, &typeErr) {
		// Nothing partial escapes: the zero value goes back with the reason, and the
		// caller publishes the reason instead of any fact.
		return mcpParams{}, MCPBodyInvalidMemberType
	}

	// The governed members decode cleanly here too, so an error can only come from a
	// telemetry member, which is dropped rather than fatal.
	_ = json.Unmarshal(e.Params, &p)
	return p, ""
}

// addAttributes adds what the body actually said, omitting anything it did not carry.
//
// The set splits in two. An id and a protocol version stand on their own, so they are read
// from any body that carried them. The rest describe an operation and are published only
// when the body named one — inventing a capability for a body that invoked nothing is how
// a policy ends up enforcing on a fact no request asserted.
//
// Naming no operation is not the same as being unreadable, so no unusable reason is
// published here. The common case is a JSON-RPC response: an id and a result, no method.
// Caps on what this resolver publishes, tighter than the framework backstop.
// capability.name carries params.uri, which is a URI rather than an identifier; every
// other value is a method name, a revision date, a hash or a client name.
const (
	maxMCPAttributeValueBytes = 256
	maxMCPCapabilityNameBytes = MaxResolutionAttributeValueBytes
)

// It reports the reason the body is unusable, or "", and publishes nothing when it does:
// an unusable reason replaces the facts rather than joining them.
func (e *mcpEnvelope) addAttributes(attrs map[string]string) string {
	params, reason := e.params()
	if reason != "" {
		return reason
	}

	// Every value below comes out of the request body. Bind rejects a resolution carrying an
	// over-long one outright, so capping here is what keeps a caller from failing its own
	// request with a long tool name; dropped rather than truncated, since a shortened name
	// still looks valid and a policy cannot tell it was altered.
	add := func(name, value string) {
		limit := maxMCPAttributeValueBytes
		if name == AttrMCPBodyCapabilityName {
			limit = maxMCPCapabilityNameBytes
		}
		if value == "" || len(value) > limit {
			return
		}
		attrs[name] = value
	}

	// ─── Facts that do not depend on the method ──────────────────────────────

	// No id means a notification, which takes no response. Read that off this key's
	// absence rather than a separate fact, so it cannot disagree with the id itself.
	if id, ok := renderJSONRPCID(e.ID); ok {
		add(AttrMCPBodyJSONRPCID, id)
	}

	// Published even when the body names no operation: a JSON-RPC response still states
	// the era, and a policy needs it either way.
	if v := mcpProtocolVersion(params); v != "" {
		add(AttrMCPBodyProtocolVersion, v)
	}

	if params.RequestState != "" {
		add(AttrMCPBodyRequestStateHash, hashRequestState(params.RequestState))
	}

	// Describes the client, not the operation, so it is published without a method too.
	if info := mcpClientInfoOf(params); info.Name != "" || info.Version != "" {
		if info.Name != "" {
			add(AttrMCPBodyClientName, info.Name)
		}
		if info.Version != "" {
			add(AttrMCPBodyClientVersion, info.Version)
		}
	}

	// ─── Facts that describe the operation ───────────────────────────────────

	if e.Method != "" {
		add(AttrMCPBodyMethod, e.Method)

		// Guarded by the family, though params is readable regardless: a method addressing
		// none, such as ping, names no capability, and a stray params.name published as one
		// is a value an ACL could match on for a body that invoked nothing.
		if capType, action, ok := splitMCPMethod(e.Method); ok {
			add(AttrMCPBodyCapabilityType, capType)
			add(AttrMCPBodyCapabilityAction, action)

			if name := capabilityName(capType, params); name != "" {
				add(AttrMCPBodyCapabilityName, name)
			}

			// Only for the family that defines it. params.taskId elsewhere addresses
			// nothing, and publishing it would invent a task for an operation that
			// names none.
			if capType == mcpCapabilityTypeTask {
				add(AttrMCPBodyTaskID, params.TaskID)
			}
		}
	}

	return ""
}

// capabilityName picks the member that identifies the capability this operation addresses:
// params.uri for a resource, params.name for everything else.
//
// Keyed on the family, never on whichever member happens to be populated. MCP identifies a
// resource by uri and defines no name for resources/*, so a params.name there is not the
// capability and a client sets it freely. Taking the first non-empty member let a decoy name
// mask the resource the server actually reads: measured, a body naming a protected uri
// alongside an unrelated name escaped a rule written against that uri, while the same body
// parsed on a resolver-less gateway did not. Same class of divergence as the mirrored request
// headers, and the reason mcp-spec-validation exists.
//
// Publishing nothing is the right answer when the family's own member is absent. A
// resources/read without a uri addresses no resource, and naming one from a member the server
// will not read is a value an ACL could match on for a capability never invoked.
func capabilityName(capType string, params mcpParams) string {
	if capType == mcpCapabilityTypeResource {
		return params.URI
	}
	return params.Name
}

// splitMCPMethod maps "tools/call" to the singular capability type and its action.
// A method with no family segment ("initialize") has neither, and reports false rather
// than inventing one.
func splitMCPMethod(method string) (capType, action string, ok bool) {
	family, action, found := strings.Cut(method, "/")
	if !found || family == "" || action == "" {
		return "", "", false
	}
	capType, known := mcpCapabilityTypes[family]
	if !known {
		// An unknown family is still a real segment; report it as-is rather than
		// dropping the fact. Policies match on the method anyway.
		capType = family
	}
	return capType, action, true
}

// mcpProtocolVersion reads the per-request protocol version out of params._meta, where
// 2026-07-28 places it, then falls back to params.protocolVersion, where a legacy initialize
// states one.
//
// The two are not quite the same thing, and folding them is deliberate, for the simpler key
// set: modern _meta declares the version this request uses, while a legacy initialize proposes
// the newest version its client supports and lets the server pick. A consumer that must tell
// them apart can, since only initialize carries the legacy form. The one reader today,
// mcp-spec-validation, compares this against the mirrored header behind an era gate that a
// legacy request never passes.
func mcpProtocolVersion(params mcpParams) string {
	if raw, ok := params.Meta[mcpMetaProtocolVersionKey]; ok {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil && v != "" {
			return v
		}
	}
	return params.ProtocolVersion
}

// hashRequestState fingerprints an opaque requestState so two events can be matched without the
// value itself travelling. Plain SHA-256 hex: a consumer hashing the other half of the exchange
// must use exactly this, so the two sides cannot be matched if either changes.
func hashRequestState(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

// mcpClientInfoOf reads the calling client's identity, with the same precedence:
// params._meta first, then the legacy params.clientInfo.
//
// A member of the wrong shape is left to the zero value rather than failing the parse. It is
// one fact among several, and a body that states it oddly is still readable for the rest.
func mcpClientInfoOf(params mcpParams) mcpClientInfo {
	if raw, ok := params.Meta[mcpMetaClientInfoKey]; ok {
		var info mcpClientInfo
		if err := json.Unmarshal(raw, &info); err == nil && (info.Name != "" || info.Version != "") {
			return info
		}
	}
	if params.ClientInfo != nil {
		return *params.ClientInfo
	}
	return mcpClientInfo{}
}

// renderJSONRPCID renders the id as the JSON token it arrived as — 7 for a number, "7" with
// its quotes for a string. JSON-RPC allows either and a client correlates by matching the
// value, so a consumer echoes the token verbatim rather than re-parsing it.
//
// Reports false only when the member was absent. Null and "" are members that are present, so
// both publish and both are requests: a notification is a method with no id, both halves.
func renderJSONRPCID(raw json.RawMessage) (string, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

// trimLeadingSpace skips leading JSON whitespace so the first meaningful byte can be
// inspected without allocating.
func trimLeadingSpace(b []byte) []byte {
	for i, c := range b {
		switch c {
		case ' ', '\t', '\n', '\r':
			continue
		default:
			return b[i:]
		}
	}
	return nil
}

func init() {
	// Registered unconditionally, like route-key. It stays inert until a controller
	// emits resolver_name "mcp" on a route, so registering it cannot change the
	// behaviour of any MCP proxy deployed today.
	RegisterDefault(&MCPResolver{})
}
