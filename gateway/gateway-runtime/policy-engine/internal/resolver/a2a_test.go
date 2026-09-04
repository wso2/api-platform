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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/common/agentproto"
	"github.com/wso2/api-platform/common/chainkey"
)

const (
	a2aAPIID = "agent-1"
	a2aVhost = "agents.example.com"
)

// a2aConfig encodes a route's resolver_config exactly as the controller does, through
// the shared wire type — so a rename on either side breaks this test rather than
// silently producing a route the engine cannot prepare.
func a2aConfig(t *testing.T, version agentproto.ProtocolVersion, transport agentproto.Transport, operation agentproto.Operation) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(agentproto.ResolverConfig{
		ProtocolVersion: version,
		Transport:       transport,
		Operation:       operation,
	})
	require.NoError(t, err)
	return raw
}

// a2aRoute builds the route config ingest would hand Prepare for an Agent route.
func a2aRoute(routeKey string, cfg json.RawMessage) ResolverRouteConfig {
	return ResolverRouteConfig{
		RouteKey:          routeKey,
		CanonicalChainKey: routeKey,
		ResolverName:      agentproto.ResolverName,
		APIID:             a2aAPIID,
		Vhost:             a2aVhost,
		APIContext:        "/weather",
		ResolverConfig:    cfg,
	}
}

// jsonRPCRoute prepares the single JSON-RPC endpoint of an Agent on version.
func jsonRPCRoute(t *testing.T, version agentproto.ProtocolVersion) *PreparedRoute {
	t.Helper()
	return prepareWith(t, DefaultRegistry(), a2aRoute("POST|/weather/rpc|"+a2aVhost,
		a2aConfig(t, version, agentproto.TransportJSONRPC, "")))
}

// sendMessageRoute prepares the HTTP+JSON POST /message:send route — one of the two
// bindings whose identifiers live only in the request body.
func sendMessageRoute(t *testing.T) *PreparedRoute {
	t.Helper()
	return prepareWith(t, DefaultRegistry(), a2aRoute("POST|/weather/http/message:send|"+a2aVhost,
		a2aConfig(t, agentproto.V1_0, agentproto.TransportHTTPJSON, agentproto.SendMessage)))
}

// jsonRPCBody is a minimal well-formed A2A JSON-RPC envelope.
func jsonRPCBody(method string) []byte {
	return fmt.Appendf(nil, `{"jsonrpc":"2.0","id":1,"method":%q,"params":{}}`, method)
}

// resolveJSONRPC runs one request against a prepared JSON-RPC route.
func resolveJSONRPC(t *testing.T, pr *PreparedRoute, body []byte) (Resolution, error) {
	t.Helper()
	return pr.Resolver.Resolve(context.Background(), RequestView{
		RouteKey: "POST|/weather/rpc|" + a2aVhost,
		Method:   "POST",
		Path:     "/rpc",
		Body:     body,
	})
}

// requireFailure asserts a resolution failed with a specific classification and bound
// nothing. The classification is what the kernel picks a status and a metric label
// from, so it is the part worth pinning.
func requireFailure(t *testing.T, res Resolution, err error, want FailureKind) {
	t.Helper()
	var re *ResolutionError
	require.ErrorAs(t, err, &re)
	assert.Equal(t, want, re.Kind)
	assert.Empty(t, res.ChainKey, "a failed resolution must never name a chain")
}

// ─── Registration ────────────────────────────────────────────────────────────

func TestA2AResolver_IsRegisteredUnderTheSharedWireName(t *testing.T) {
	r, ok := DefaultRegistry().Get(agentproto.ResolverName)
	require.True(t, ok, "the controller writes this name into every Agent route")
	assert.Equal(t, agentproto.ResolverName, r.Name())
	assert.IsType(t, &A2AResolver{}, r)
}

// ─── Prepare: protocol version selection ─────────────────────────────────────

// The version is the whole of what selects an operation set, so there is no default
// and no fallback: a route naming a version this binary does not know is dropped, not
// served against the only table it happens to have.
func TestA2APrepare_RejectsAnUnknownOrMissingVersion(t *testing.T) {
	tests := []struct {
		name string
		cfg  string
		want string
	}{
		{"no config at all", "", "no resolver_config"},
		{"empty version", `{"transport":"JSONRPC"}`, "no protocolVersion"},
		{"a version that does not exist", `{"protocolVersion":"9.9","transport":"JSONRPC"}`, "unsupported A2A protocol version"},
		{"a 0.x version", `{"protocolVersion":"0.3","transport":"JSONRPC"}`, "unsupported A2A protocol version"},
		{"not JSON", `{`, "not valid JSON"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PrepareRoute(DefaultRegistry(),
				a2aRoute("POST|/weather/rpc|"+a2aVhost, json.RawMessage(tt.cfg)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// Selecting one version must not be able to reach another version's table. With one
// version registered the strongest available statement is that every unregistered
// version is refused and the registered one is not — which is the property that keeps
// holding when a second version lands.
func TestA2APrepare_UsesOnlyTheSelectedVersionsTable(t *testing.T) {
	for _, version := range agentproto.Versions() {
		t.Run(string(version), func(t *testing.T) {
			operations, ok := agentproto.Operations(version)
			require.True(t, ok)

			pr := jsonRPCRoute(t, version)
			for _, operation := range operations {
				res, err := resolveJSONRPC(t, pr, jsonRPCBody(string(operation)))
				require.NoError(t, err, "every operation of the selected version must resolve")
				assert.Equal(t, ChainKeyFor(a2aAPIID, a2aVhost, string(operation)), res.ChainKey)
			}
		})
	}

	assert.False(t, agentproto.IsSupportedVersion("9.9"),
		"the negative case above depends on this version being unregistered")
}

// ─── Prepare: transport wiring ───────────────────────────────────────────────

// One factory produces three shapes, and which one a route gets is decided entirely by
// its config. That is the whole reason requirements are a property of the prepared
// route rather than the factory.
func TestA2APrepare_RouteConfigDecidesTheResolverShape(t *testing.T) {
	// Many operations on one endpoint: the operation is in the body, so the body is
	// mandatory and nothing can be known at ingest.
	jsonRPC := jsonRPCRoute(t, agentproto.V1_0)
	assert.False(t, jsonRPC.IsStatic(), "the operation is in the body, so it cannot be known at ingest")
	assert.Equal(t, RequestRequirements{Body: BodyBuffered}, jsonRPC.Requirements)

	// One operation, identifiers in the path: nothing about the request is needed, so
	// the request path does no work at all.
	inPath := prepareWith(t, DefaultRegistry(), a2aRoute("GET|/weather/http/tasks/{id}|"+a2aVhost,
		a2aConfig(t, agentproto.V1_0, agentproto.TransportHTTPJSON, agentproto.GetTask)))
	require.True(t, inPath.IsStatic(), "a route whose identifiers are in the path must not buffer")
	assert.Equal(t, RequestRequirements{}, inPath.Requirements,
		"a static resolver must declare the zero value or PrepareRoute refuses it")
	assert.Equal(t, ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.GetTask)),
		inPath.StaticResolution.ChainKey)

	// One operation, identifiers in the body: the chain is fixed but the attributes
	// are not, so it buffers and therefore cannot be static.
	inBody := sendMessageRoute(t)
	assert.False(t, inBody.IsStatic(),
		"a route that reads the body cannot also declare a static resolution")
	assert.Equal(t, RequestRequirements{Body: BodyBuffered}, inBody.Requirements)
}

func TestA2APrepare_RejectsAMisconfiguredRoute(t *testing.T) {
	tests := []struct {
		name      string
		transport agentproto.Transport
		operation agentproto.Operation
		want      string
	}{
		{
			// The route would resolve to whatever the caller named, silently ignoring
			// the operation the config asked for.
			name:      "JSON-RPC naming an operation",
			transport: agentproto.TransportJSONRPC,
			operation: agentproto.SendMessage,
			want:      "must not name an operation",
		},
		{
			name:      "HTTP+JSON naming none",
			transport: agentproto.TransportHTTPJSON,
			want:      "names no operation",
		},
		{
			name:      "HTTP+JSON naming a non-operation",
			transport: agentproto.TransportHTTPJSON,
			operation: "SendMessages",
			want:      "not an A2A 1.0 operation",
		},
		{
			// Case matters: A2A operation names are case-sensitive, and normalising a
			// near-miss into a real operation would attach the wrong chain to the route.
			name:      "HTTP+JSON naming a case variant",
			transport: agentproto.TransportHTTPJSON,
			operation: "sendmessage",
			want:      "not an A2A 1.0 operation",
		},
		{
			name:      "an unknown transport",
			transport: "GRPC",
			want:      "unsupported transport",
		},
		{
			name: "no transport at all",
			want: "unsupported transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PrepareRoute(DefaultRegistry(), a2aRoute("POST|/weather/rpc|"+a2aVhost,
				a2aConfig(t, agentproto.V1_0, tt.transport, tt.operation)))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// A key component that cannot be one would compose the same key as a different
// (API, vhost, operation) triple, so the route is refused rather than served.
func TestA2APrepare_RejectsAnUnusablePartition(t *testing.T) {
	tests := []struct {
		name  string
		apiID string
		vhost string
		want  string
	}{
		{"no API id", "", a2aVhost, "unusable API id"},
		{"separator in the API id", "agent" + chainkey.Separator + "1", a2aVhost, "unusable API id"},
		{"separator in the vhost", a2aAPIID, "a" + chainkey.Separator + "b", "unusable vhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := a2aRoute("POST|/weather/rpc|"+a2aVhost,
				a2aConfig(t, agentproto.V1_0, agentproto.TransportJSONRPC, ""))
			cfg.APIID, cfg.Vhost = tt.apiID, tt.vhost

			_, err := PrepareRoute(DefaultRegistry(), cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// An empty vhost is the default vhost, not a missing one, so it must prepare.
func TestA2APrepare_AcceptsTheDefaultVhost(t *testing.T) {
	cfg := a2aRoute("POST|/weather/rpc|", a2aConfig(t, agentproto.V1_0, agentproto.TransportJSONRPC, ""))
	cfg.Vhost = ""

	pr, err := PrepareRoute(DefaultRegistry(), cfg)
	require.NoError(t, err)

	res, err := resolveJSONRPC(t, pr, jsonRPCBody(string(agentproto.GetTask)))
	require.NoError(t, err)
	assert.Equal(t, ChainKeyFor(a2aAPIID, "", string(agentproto.GetTask)), res.ChainKey)
}

// A field a newer controller adds must not drop a route this version can still serve
// correctly — the version check is what guards the incompatible case.
func TestA2APrepare_ToleratesUnknownConfigFields(t *testing.T) {
	pr, err := PrepareRoute(DefaultRegistry(), a2aRoute("POST|/weather/rpc|"+a2aVhost,
		json.RawMessage(`{"protocolVersion":"1.0","transport":"JSONRPC","somethingNew":true}`)))
	require.NoError(t, err)
	assert.True(t, pr.Requirements.BuffersBody())
}

// ─── Resolve: JSON-RPC ───────────────────────────────────────────────────────

func TestA2AResolve_JSONRPCResolvesEveryOperation(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)
	require.Len(t, operations, 11, "A2A 1.0 defines eleven canonical operations")

	for _, operation := range operations {
		t.Run(string(operation), func(t *testing.T) {
			res, err := resolveJSONRPC(t, pr, jsonRPCBody(string(operation)))
			require.NoError(t, err)
			assert.Equal(t, ChainKeyFor(a2aAPIID, a2aVhost, string(operation)), res.ChainKey)
		})
	}
}

func TestA2AResolve_JSONRPCClassifiesEveryFailure(t *testing.T) {
	tests := []struct {
		name string
		body string
		want FailureKind
	}{
		{"malformed JSON", `{"jsonrpc":"2.0","method":`, FailureParse},
		{"not an object", `"SendMessage"`, FailureParse},
		{"a batch", `[{"jsonrpc":"2.0","id":1,"method":"SendMessage"}]`, FailureMultiOperation},
		{"a batch behind whitespace", "  \n\t[{\"jsonrpc\":\"2.0\",\"method\":\"SendMessage\"}]", FailureMultiOperation},
		{"an empty body", ``, FailureInvalidRequest},
		{"whitespace only", "  \n ", FailureInvalidRequest},
		{"no jsonrpc field", `{"id":1,"method":"SendMessage"}`, FailureInvalidRequest},
		{"the wrong jsonrpc version", `{"jsonrpc":"1.0","id":1,"method":"SendMessage"}`, FailureInvalidRequest},
		{"no method", `{"jsonrpc":"2.0","id":1}`, FailureInvalidRequest},
		{"an empty method", `{"jsonrpc":"2.0","id":1,"method":""}`, FailureInvalidRequest},
		{"an unknown method", `{"jsonrpc":"2.0","id":1,"method":"DeleteEverything"}`, FailureUnknownOperation},
		{"a 0.x-style method name", `{"jsonrpc":"2.0","id":1,"method":"message/send"}`, FailureUnknownOperation},
		{"a case variant of a real method", `{"jsonrpc":"2.0","id":1,"method":"sendmessage"}`, FailureUnknownOperation},
	}

	pr := jsonRPCRoute(t, agentproto.V1_0)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := resolveJSONRPC(t, pr, []byte(tt.body))
			requireFailure(t, res, err, tt.want)
		})
	}
}

// params is not the gateway's to police. Chain selection needs jsonrpc and method
// and nothing else, so every params shape a client can legally send — JSON-RPC
// permits a positional array, and null or absent are legal envelopes — must reach
// the Agent, which is the component that can tell the client what was wrong with
// it. Decoding params into a fixed struct here made each of these a FailureParse
// answered by the engine's sterile response instead.
//
// The identifiers are best-effort: unreadable params costs attributes, never the
// chain.
func TestA2AResolve_JSONRPCAcceptsAnyParamsShape(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)
	wantKey := ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.SendMessage))

	for name, params := range map[string]string{
		"a positional array":          `[{"message":{"messageId":"msg-1"}}]`,
		"an empty array":              `[]`,
		"a string":                    `"send it"`,
		"a number":                    `7`,
		"null":                        `null`,
		"an object with no message":   `{"configuration":{"blocking":true}}`,
		"a message of the wrong type": `{"message":"msg-1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":` + params + `}`)

			res, err := resolveJSONRPC(t, pr, body)
			require.NoError(t, err, "params shape must not decide whether a chain binds")
			assert.Equal(t, wantKey, res.ChainKey)
			assert.Equal(t, string(agentproto.SendMessage), res.Attributes[AttrA2AOperation])
			for _, absent := range []string{AttrA2AMessageID, AttrA2AContextID, AttrA2ATaskID} {
				assert.NotContains(t, res.Attributes, absent,
					"an identifier the gateway could not read must be absent, not guessed")
			}
		})
	}

	// Absent entirely — the shape every operation that takes no arguments sends.
	t.Run("absent", func(t *testing.T) {
		res, err := resolveJSONRPC(t, pr, []byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage"}`))
		require.NoError(t, err)
		assert.Equal(t, wantKey, res.ChainKey)
	})
}

// A request whose headers are end-of-stream never produces a body callback, so a
// BodyBuffered route is resolved at the header phase with no body at all. It must
// classify, not panic — and nil and empty must behave alike.
func TestA2AResolve_JSONRPCToleratesAMissingBody(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	for _, body := range [][]byte{nil, {}} {
		require.NotPanics(t, func() {
			res, err := resolveJSONRPC(t, pr, body)
			requireFailure(t, res, err, FailureInvalidRequest)
		})
	}
}

// The failure cause is for the internal log; what the client sees is the kernel's
// sterile response. This asserts the resolver never returns a bare error, because an
// unclassified one would be reported as an engine fault rather than a bad request.
func TestA2AResolve_JSONRPCFailuresAreAlwaysClassified(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	_, err := resolveJSONRPC(t, pr, []byte(`{"jsonrpc":"2.0","method":"Nope"}`))
	require.Error(t, err)
	assert.Equal(t, FailureUnknownOperation, NormalizeResolutionError(err).Kind,
		"an unclassified error would normalise to FailureInternal and read as an engine fault")
}

// ─── Resolve: HTTP+JSON ──────────────────────────────────────────────────────

// Every HTTP+JSON route resolves to its own operation, whichever shape it took. Only
// the two message-sending operations read the body; the other nine stay on the static
// fast path, because their identifiers are in the path and cost no buffering.
func TestA2AResolve_HTTPJSONResolvesEveryBinding(t *testing.T) {
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)

	var static, buffered int
	for _, operation := range operations {
		bindings, ok := agentproto.HTTPJSONBindings(agentproto.V1_0, operation)
		require.True(t, ok)
		require.NotEmpty(t, bindings)

		for _, binding := range bindings {
			t.Run(string(operation)+" "+binding.Method+binding.PathTemplate, func(t *testing.T) {
				routeKey := binding.Method + "|/weather/http" + binding.PathTemplate + "|" + a2aVhost
				pr := prepareWith(t, DefaultRegistry(), a2aRoute(routeKey,
					a2aConfig(t, agentproto.V1_0, agentproto.TransportHTTPJSON, operation)))

				want := ChainKeyFor(a2aAPIID, a2aVhost, string(operation))

				if carriesMessageInBody(operation) {
					buffered++
					assert.False(t, pr.IsStatic(),
						"a route that reads the body cannot declare a static resolution")
					assert.True(t, pr.Requirements.BuffersBody())
				} else {
					static++
					require.True(t, pr.IsStatic(),
						"this operation's identifiers are in the path, so it must not buffer")
					assert.Equal(t, want, pr.StaticResolution.ChainKey)
					assert.Equal(t, RequestRequirements{}, pr.Requirements)
				}

				// Whichever shape, the route resolves to its own operation — with no
				// body at all, which is what a bodyless or unreadable request gives it.
				res, err := pr.Resolver.Resolve(context.Background(), RequestView{})
				require.NoError(t, err)
				assert.Equal(t, want, res.ChainKey)
			})
		}
	}

	assert.Equal(t, 2, buffered, "only message:send and message:stream may buffer")
	assert.Equal(t, 9, static)
}

// The binder's fast path for a static route is exactly one chain lookup — no
// validation, no view, no Resolve call.
func TestA2ABind_HTTPJSONCostsOneLookup(t *testing.T) {
	pr := prepareWith(t, DefaultRegistry(), a2aRoute("GET|/weather/http/tasks/{id}|"+a2aVhost,
		a2aConfig(t, agentproto.V1_0, agentproto.TransportHTTPJSON, agentproto.GetTask)))

	key := ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.GetTask))
	store := chainsPresent(key)
	bound, chain, err := BindStatic(pr, store.get)
	require.NoError(t, err)
	require.NotNil(t, chain)
	assert.Equal(t, key, bound.ChainKey)
	assert.Equal(t, string(agentproto.GetTask), bound.Operation)
	assert.Equal(t, []string{key}, store.lookedUp)
}

// ─── Request attributes ──────────────────────────────────────────────────────

// The identifiers come out of the same parse that selected the chain, and the
// protocol facts come from the route — so a consumer gets the whole set without
// reading the body again, on either transport.
func TestA2AResolve_CarriesTheFullAttributeSet(t *testing.T) {
	message := `{"messageId":"msg-1","contextId":"ctx-1","taskId":"task-1","role":"ROLE_USER"}`

	t.Run("JSONRPC", func(t *testing.T) {
		pr := jsonRPCRoute(t, agentproto.V1_0)
		body := []byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":` + message + `}}`)

		res, err := resolveJSONRPC(t, pr, body)
		require.NoError(t, err)
		assert.Equal(t, ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.SendMessage)), res.ChainKey)
		assert.Equal(t, map[string]string{
			AttrA2AMessageID:       "msg-1",
			AttrA2AContextID:       "ctx-1",
			AttrA2ATaskID:          "task-1",
			AttrA2AOperation:       string(agentproto.SendMessage),
			AttrA2ATransport:       string(agentproto.TransportJSONRPC),
			AttrA2AProtocolVersion: string(agentproto.V1_0),
		}, res.Attributes)
	})

	t.Run("HTTP+JSON", func(t *testing.T) {
		pr := sendMessageRoute(t)
		res, err := pr.Resolver.Resolve(context.Background(), RequestView{
			Body: []byte(`{"message":` + message + `}`),
		})
		require.NoError(t, err)
		assert.Equal(t, ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.SendMessage)), res.ChainKey)
		assert.Equal(t, map[string]string{
			AttrA2AMessageID:       "msg-1",
			AttrA2AContextID:       "ctx-1",
			AttrA2ATaskID:          "task-1",
			AttrA2AOperation:       string(agentproto.SendMessage),
			AttrA2ATransport:       string(agentproto.TransportHTTPJSON),
			AttrA2AProtocolVersion: string(agentproto.V1_0),
		}, res.Attributes)
	})
}

// Two system policies read these attributes out of SharedContext from their own
// Go modules, which cannot import this internal package and so mirror the
// literals instead: the analytics policy reports them as event dimensions, and
// the A2A policy decides from a2a.transport which binding a protected Agent Card
// request arrived on. A rename here alone is silent on both sides — a dimension
// simply stops appearing, and a managed protected card starts answering 500 —
// so the spelling is pinned here as well as there.
//
// The matching assertions are TestA2AKeySpellingsMatchThePolicyEngine in
// system-policies/analytics and TestResolutionAttributeSpellingsMatchThePolicyEngine
// in system-policies/a2a.
func TestA2AAttributeSpellingsArePinned(t *testing.T) {
	for _, testCase := range []struct{ got, want string }{
		{AttrA2AMessageID, "a2a.message.id"},
		{AttrA2AContextID, "a2a.context.id"},
		{AttrA2ATaskID, "a2a.task.id"},
		{AttrA2AOperation, "a2a.operation"},
		{AttrA2ATransport, "a2a.transport"},
		{AttrA2AProtocolVersion, "a2a.protocol.version"},
		// The values of a2a.transport, which the A2A system policy compares
		// against by literal for the same module-boundary reason.
		{string(agentproto.TransportJSONRPC), "JSONRPC"},
		{string(agentproto.TransportHTTPJSON), "HTTP+JSON"},
	} {
		assert.Equal(t, testCase.want, testCase.got)
	}
}

// The attribute set is closed: exactly these six names and nothing else, so a
// consumer can rely on it and no unbounded value can appear by accident.
func TestA2AResolve_AttributeSetIsClosed(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)
	res, err := resolveJSONRPC(t, pr,
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":`+
			`{"messageId":"m","contextId":"c","taskId":"t","role":"ROLE_USER","parts":[{"text":"hi"}]}}}`))
	require.NoError(t, err)

	names := make([]string, 0, len(res.Attributes))
	for name := range res.Attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	assert.Equal(t, []string{
		"a2a.context.id", "a2a.message.id", "a2a.operation",
		"a2a.protocol.version", "a2a.task.id", "a2a.transport",
	}, names)
	assert.LessOrEqual(t, len(names), MaxResolutionAttributes)
}

// The protocol facts are known from the route, so every resolved request carries
// them — including operations whose payload has no message at all.
func TestA2AResolve_ProtocolFactsAreAlwaysPresent(t *testing.T) {
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)

	jsonRPC := jsonRPCRoute(t, agentproto.V1_0)
	for _, operation := range operations {
		t.Run(string(operation), func(t *testing.T) {
			res, err := resolveJSONRPC(t, jsonRPC, jsonRPCBody(string(operation)))
			require.NoError(t, err)
			assert.Equal(t, string(operation), res.Attributes[AttrA2AOperation])
			assert.Equal(t, string(agentproto.TransportJSONRPC), res.Attributes[AttrA2ATransport])
			assert.Equal(t, string(agentproto.V1_0), res.Attributes[AttrA2AProtocolVersion])
		})
	}
}

// The reported operation and the chain that runs come from one map entry, built in
// one loop iteration from one operation name — so they cannot name different
// operations, the same guarantee BoundResolution.Operation gives by derivation.
func TestA2AResolve_ReportedOperationMatchesTheChainKey(t *testing.T) {
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)

	pr := jsonRPCRoute(t, agentproto.V1_0)
	for _, operation := range operations {
		res, err := resolveJSONRPC(t, pr, jsonRPCBody(string(operation)))
		require.NoError(t, err)

		_, _, fromKey, ok := chainkey.Split(res.ChainKey)
		require.True(t, ok)
		assert.Equal(t, fromKey, res.Attributes[AttrA2AOperation],
			"the attribute must name the operation whose chain the key selects")
	}
}

// lowerCamelCase is the only spelling A2A JSON uses, so the proto's snake_case field
// names are not a second accepted form — they simply yield no identifiers.
func TestA2AResolve_ReadsCamelCaseOnly(t *testing.T) {
	pr := sendMessageRoute(t)

	res, err := pr.Resolver.Resolve(context.Background(), RequestView{
		Body: []byte(`{"message":{"message_id":"msg-1","context_id":"ctx-1","task_id":"task-1"}}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, res.Attributes, AttrA2AMessageID)
	assert.NotContains(t, res.Attributes, AttrA2AContextID)
	assert.NotContains(t, res.Attributes, AttrA2ATaskID)
	assert.Equal(t, string(agentproto.SendMessage), res.Attributes[AttrA2AOperation],
		"the protocol facts are unaffected — they come from the route")
}

// Absent identifiers produce no attributes at all, not empty strings — an operation
// whose payload has no message is the common case.
func TestA2AResolve_OmitsAbsentIdentifiers(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	res, err := resolveJSONRPC(t, pr, jsonRPCBody(string(agentproto.ListTasks)))
	require.NoError(t, err)
	assert.NotContains(t, res.Attributes, AttrA2AMessageID)
	assert.NotContains(t, res.Attributes, AttrA2AContextID)
	assert.NotContains(t, res.Attributes, AttrA2ATaskID)

	// A partial message carries only what it had.
	res, err = resolveJSONRPC(t, pr,
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"contextId":"ctx-1"}}}`))
	require.NoError(t, err)
	assert.Equal(t, "ctx-1", res.Attributes[AttrA2AContextID])
	assert.NotContains(t, res.Attributes, AttrA2AMessageID)
	assert.NotContains(t, res.Attributes, AttrA2ATaskID)
}

// These are caller-controlled strings read before authentication. An over-long one is
// dropped, not truncated: a truncated opaque identifier is a *different* identifier,
// and correlating on it would silently group unrelated requests.
func TestA2AResolve_DropsOversizedIdentifiers(t *testing.T) {
	oversized := strings.Repeat("x", MaxResolutionAttributeValueBytes+1)
	pr := sendMessageRoute(t)

	res, err := pr.Resolver.Resolve(context.Background(), RequestView{
		Body: []byte(`{"message":{"messageId":"` + oversized + `","contextId":"ctx-1"}}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, res.Attributes, AttrA2AMessageID, "the oversized value must be dropped whole")
	assert.Equal(t, "ctx-1", res.Attributes[AttrA2AContextID], "its neighbours are unaffected")

	// And whatever a resolver produces, the generic backstop still holds.
	require.Nil(t, pr.ValidateResolution(res))
}

// On an HTTP+JSON route the route already fixed the operation, so an unreadable body
// costs only the attributes — the chain still runs and the Agent gets to reject the
// payload itself. This is the opposite of the JSON-RPC route, where the body names the
// operation and an unreadable one means nothing can be selected.
func TestA2AResolve_HTTPJSONStillResolvesWithAnUnusableBody(t *testing.T) {
	pr := sendMessageRoute(t)
	want := ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.SendMessage))

	for _, body := range []string{``, `   `, `not json at all`, `[1,2,3]`, `{"message":"not an object"}`, `{}`} {
		t.Run(body, func(t *testing.T) {
			res, err := pr.Resolver.Resolve(context.Background(), RequestView{Body: []byte(body)})
			require.NoError(t, err, "the operation is known from the route, so this must not fail")
			assert.Equal(t, want, res.ChainKey)
			assert.NotContains(t, res.Attributes, AttrA2AMessageID, "no identifiers could be read")
			assert.Equal(t, string(agentproto.SendMessage), res.Attributes[AttrA2AOperation],
				"the protocol facts come from the route, so they survive an unreadable body")
		})
	}

	require.NotPanics(t, func() {
		res, err := pr.Resolver.Resolve(context.Background(), RequestView{Body: nil})
		require.NoError(t, err)
		assert.Equal(t, want, res.ChainKey)
	})
}

// The prepared resolution is shared by every request on the route, so enriching one
// request must not leak into the next.
func TestA2AResolve_HTTPJSONDoesNotMutateItsPreparedResolution(t *testing.T) {
	pr := sendMessageRoute(t)

	withAttrs, err := pr.Resolver.Resolve(context.Background(), RequestView{
		Body: []byte(`{"message":{"messageId":"msg-1"}}`),
	})
	require.NoError(t, err)
	require.Equal(t, "msg-1", withAttrs.Attributes[AttrA2AMessageID])

	plain, err := pr.Resolver.Resolve(context.Background(), RequestView{})
	require.NoError(t, err)
	assert.NotContains(t, plain.Attributes, AttrA2AMessageID,
		"the next request must not inherit the previous one's identifiers")
	assert.Equal(t, withAttrs.ChainKey, plain.ChainKey)

	// And the first request's map must not have been reused for the second.
	assert.Equal(t, "msg-1", withAttrs.Attributes[AttrA2AMessageID],
		"the earlier resolution's attributes must be unaffected by a later request")
}

// Attributes ride through binding untouched, and change nothing about which chain runs.
func TestA2ABind_CarriesAttributesWithoutAffectingSelection(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)
	key := ChainKeyFor(a2aAPIID, a2aVhost, string(agentproto.SendMessage))

	res, err := resolveJSONRPC(t, pr,
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"SendMessage","params":{"message":{"taskId":"task-1"}}}`))
	require.NoError(t, err)

	bound, chain, err := Bind(pr, res, chainsPresent(key).get)
	require.NoError(t, err)
	require.NotNil(t, chain)
	assert.Equal(t, key, bound.ChainKey)
	assert.Equal(t, string(agentproto.SendMessage), bound.Operation,
		"the operation is still derived from the key, never from an attribute")
	assert.Equal(t, "task-1", bound.Attributes[AttrA2ATaskID])
	assert.Equal(t, string(agentproto.SendMessage), bound.Attributes[AttrA2AOperation])
}

// ─── Convergence ─────────────────────────────────────────────────────────────

// The property the whole design exists for: one logical operation reached over two
// transports selects one chain. They converge because both routes compose from the
// same canonical operation name with the same helper — not because either was pointed
// at the other's key.
func TestA2A_TransportsConvergeOnOneChain(t *testing.T) {
	operations, ok := agentproto.Operations(agentproto.V1_0)
	require.True(t, ok)

	jsonRPC := jsonRPCRoute(t, agentproto.V1_0)

	for _, operation := range operations {
		t.Run(string(operation), func(t *testing.T) {
			bindings, ok := agentproto.HTTPJSONBindings(agentproto.V1_0, operation)
			require.True(t, ok)
			binding := bindings[0]

			httpJSON := prepareWith(t, DefaultRegistry(), a2aRoute(
				binding.Method+"|/weather/http"+binding.PathTemplate+"|"+a2aVhost,
				a2aConfig(t, agentproto.V1_0, agentproto.TransportHTTPJSON, operation)))

			// Only this operation's chain exists, so a resolver that converged on the
			// wrong key would fail to bind rather than quietly select a neighbour.
			key := ChainKeyFor(a2aAPIID, a2aVhost, string(operation))
			store := chainsPresent(key)

			fromBody, err := resolveJSONRPC(t, jsonRPC, jsonRPCBody(string(operation)))
			require.NoError(t, err)
			boundFromBody, chainFromBody, err := Bind(jsonRPC, fromBody, store.get)
			require.NoError(t, err)
			require.NotNil(t, chainFromBody)

			// The two message-sending operations read the body for attributes, so
			// they bind through Resolve; the other nine bind from their stored
			// static resolution. Convergence must hold across that difference.
			var boundFromRoute BoundResolution
			var chainFromRoute *fakeChain
			if httpJSON.IsStatic() {
				boundFromRoute, chainFromRoute, err = BindStatic(httpJSON, store.get)
			} else {
				var fromRoute Resolution
				fromRoute, err = httpJSON.Resolver.Resolve(context.Background(), RequestView{})
				require.NoError(t, err)
				boundFromRoute, chainFromRoute, err = Bind(httpJSON, fromRoute, store.get)
			}
			require.NoError(t, err)
			require.NotNil(t, chainFromRoute)

			assert.Equal(t, boundFromBody.ChainKey, boundFromRoute.ChainKey)
			assert.Equal(t, key, boundFromBody.ChainKey)
			assert.Equal(t, string(operation), boundFromBody.Operation,
				"telemetry names the chain that ran, on both transports")
			assert.Equal(t, boundFromBody.Operation, boundFromRoute.Operation)
		})
	}
}

// A resolver may not reach outside its own route's partition, whichever transport it
// serves. The a2a resolver composes keys from the partition captured at ingest, so
// this asserts the captured value is the one used.
func TestA2A_KeysStayInsideTheRoutesPartition(t *testing.T) {
	pr := jsonRPCRoute(t, agentproto.V1_0)

	res, err := resolveJSONRPC(t, pr, jsonRPCBody(string(agentproto.SendMessage)))
	require.NoError(t, err)

	apiID, vhost, operation, ok := chainkey.Split(res.ChainKey)
	require.True(t, ok)
	assert.Equal(t, a2aAPIID, apiID)
	assert.Equal(t, a2aVhost, vhost)
	assert.Equal(t, string(agentproto.SendMessage), operation)

	// And the binder agrees: another partition's chain is not reachable from here.
	other := ChainKeyFor("other-agent", a2aVhost, string(agentproto.SendMessage))
	_, chain, err := Bind(pr, Resolution{ChainKey: other}, chainsPresent(other).get)
	require.Error(t, err)
	assert.Nil(t, chain)
}
