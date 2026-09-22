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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/common/chainkey"
)

// prepareMCP prepares a route with no resolver config, which is what the controller
// emits. resolverConfig is accepted so a test can prove config is ignored rather than
// read; every other caller passes nil.
func prepareMCP(t *testing.T, resolverConfig json.RawMessage) PreparedResolver {
	t.Helper()
	prepared, err := (&MCPResolver{}).Prepare(ResolverRouteConfig{
		ResolverName:   MCPResolverName,
		APIID:          "api-1",
		Vhost:          "gw.example.com",
		Path:           "/weather/mcp",
		Method:         "POST",
		ResolverConfig: resolverConfig,
	})
	require.NoError(t, err)
	return prepared
}

func resolveMCP(t *testing.T, prepared PreparedResolver, body string) Resolution {
	t.Helper()
	res, err := prepared.Resolve(context.Background(), RequestView{
		RouteKey: "POST|/weather/mcp|gw.example.com",
		Method:   "POST",
		Path:     "/weather/mcp",
		Body:     []byte(body),
	})
	require.NoError(t, err)
	return res
}

// ─── Preparation: every route reads the body ─────────────────────────────────

// The central invariant of the design: one factory, one shape, and no route configuration
// can talk this resolver out of reading the body. A route that resolved statically would
// publish no facts at all, because the kernel binds a static route from its stored result
// without ever calling Resolve — so the negative asserted here is the one that matters.
func TestMCPPrepare_EveryRouteReadsTheBody(t *testing.T) {
	cases := []struct {
		name   string
		config json.RawMessage
	}{
		{"no resolver config, as the controller emits", nil},
		{"empty object", json.RawMessage(`{}`)},
		{"a modern spec version does not opt out", json.RawMessage(`{"specVersion":"2026-07-28"}`)},
		{"a legacy spec version", json.RawMessage(`{"specVersion":"2025-06-18"}`)},
		{"the retired validateHeaderBody flag is inert", json.RawMessage(`{"specVersion":"2026-07-28","validateHeaderBody":false}`)},
		{"an unknown future field", json.RawMessage(`{"somethingNew":["a"]}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prepared := prepareMCP(t, tc.config)

			assert.True(t, prepared.Requirements().BuffersBody(),
				"a route that does not buffer publishes no facts, and every MCP policy reads facts")

			_, isStatic := prepared.(StaticPreparedResolver)
			assert.False(t, isStatic,
				"a static route is bound from its stored result without calling Resolve, so it would publish nothing")
		})
	}
}

// A task id is published on its own key, never as the capability name: the policies that match
// a capability name against operator rules must not start matching one task's handle. It is
// published only for the family that defines it, so params.taskId elsewhere names nothing.
func TestMCPResolve_ATaskIDIsPublishedApartFromTheCapabilityName(t *testing.T) {
	prepared := prepareMCP(t, nil)

	for _, method := range []string{"tasks/get", "tasks/update", "tasks/cancel"} {
		t.Run(method, func(t *testing.T) {
			res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"`+method+
				`","params":{"taskId":"786512e2-9e0d-44bd-8f29-789f320fe840"}}`)
			assert.Equal(t, "786512e2-9e0d-44bd-8f29-789f320fe840", res.Attributes[AttrMCPBodyTaskID])
			assert.NotContains(t, res.Attributes, AttrMCPBodyCapabilityName,
				"a task id is a handle to one operation, not a capability an operator writes rules against")
			assert.Equal(t, "task", res.Attributes[AttrMCPBodyCapabilityType])
		})
	}

	t.Run("params.taskId on another family names nothing", func(t *testing.T) {
		res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"tools/call",`+
			`"params":{"name":"get_weather","taskId":"t-1"}}`)
		assert.NotContains(t, res.Attributes, AttrMCPBodyTaskID)
		assert.Equal(t, "get_weather", res.Attributes[AttrMCPBodyCapabilityName])
	})

	t.Run("a task call naming no task publishes no id", func(t *testing.T) {
		res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{}}`)
		assert.NotContains(t, res.Attributes, AttrMCPBodyTaskID)
	})

	t.Run("a wrong-typed task id makes the body unusable", func(t *testing.T) {
		res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"taskId":5}}`)
		assert.Equal(t, MCPBodyInvalidMemberType, res.Attributes[AttrMCPBodyUnusable])
	})

	t.Run("a task id named twice is ambiguous", func(t *testing.T) {
		res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"tasks/get",`+
			`"params":{"taskId":"t-1","TaskId":"t-2"}}`)
		assert.Equal(t, MCPBodyAmbiguous, res.Attributes[AttrMCPBodyUnusable])
	})

	// A span attribute is indexed per distinct value, and a task id is unique per task.
	assert.False(t, IsSpanSafeAttribute(AttrMCPBodyTaskID))
}

// The other half of the unusable rule. A wrong type in a member no published fact is read
// from leaves every fact readable, so the body is not unusable: rejecting it would refuse
// requests the server itself accepts, and the telemetry member is simply dropped.
func TestMCPResolve_AWrongTypedTelemetryMemberLeavesTheFactsReadable(t *testing.T) {
	prepared := prepareMCP(t, nil)
	for _, tc := range []struct{ name, body string }{
		{
			name: "requestState",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
				`"params":{"name":"get_weather","requestState":123}}`,
		},
		{
			name: "clientInfo",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
				`"params":{"name":"get_weather","clientInfo":"nope"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			assert.NotContains(t, res.Attributes, AttrMCPBodyUnusable,
				"a member no fact is read from cannot make the body unreadable")
			assert.Equal(t, "tools/call", res.Attributes[AttrMCPBodyMethod])
			assert.Equal(t, "get_weather", res.Attributes[AttrMCPBodyCapabilityName])
		})
	}
}

// Telemetry that disagrees with itself is still telemetry. An ambiguous member suppresses every
// fact, so reserving that answer for the members a decision is read from is what keeps a governable
// request governable - the duplicate-member twin of the wrong-type rule above.
func TestMCPResolve_ADuplicateClientInfoLeavesTheFactsReadable(t *testing.T) {
	prepared := prepareMCP(t, nil)
	for _, tc := range []struct{ name, body string }{
		{
			name: "params.clientInfo in two letter cases",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_weather",` +
				`"clientInfo":{"name":"a"},"ClientInfo":{"name":"b"}}}`,
		},
		{
			name: "the _meta clientInfo key in two letter cases",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_weather",` +
				`"_meta":{"io.modelcontextprotocol/clientInfo":{"name":"a"},` +
				`"IO.MODELCONTEXTPROTOCOL/CLIENTINFO":{"name":"b"}}}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			assert.NotContains(t, res.Attributes, AttrMCPBodyUnusable,
				"a member no decision is read from cannot make the body ungovernable")
			assert.Equal(t, "tools/call", res.Attributes[AttrMCPBodyMethod])
			assert.Equal(t, "get_weather", res.Attributes[AttrMCPBodyCapabilityName])
		})
	}

	// The era is read from _meta, so two spellings of it still suppress the facts.
	t.Run("the _meta protocol version is still ambiguous", func(t *testing.T) {
		res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x",`+
			`"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",`+
			`"IO.MODELCONTEXTPROTOCOL/PROTOCOLVERSION":"2025-06-18"}}}`)
		assert.Equal(t, MCPBodyAmbiguous, res.Attributes[AttrMCPBodyUnusable])
	})
}

// A modern request must publish the same facts as a legacy one. The mirrored headers are
// an additional source for a policy, never a reason for this resolver to skip the body.
func TestMCPResolve_ModernRequestPublishesTheSameFactsAsLegacy(t *testing.T) {
	const body = `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_forecast",` +
		`"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`

	modern := resolveMCP(t, prepareMCP(t, json.RawMessage(`{"specVersion":"2026-07-28"}`)), body)
	legacy := resolveMCP(t, prepareMCP(t, nil), body)

	assert.Equal(t, "tools/call", modern.Attributes[AttrMCPBodyMethod])
	assert.Equal(t, "get_forecast", modern.Attributes[AttrMCPBodyCapabilityName])
	assert.Equal(t, "2026-07-28", modern.Attributes[AttrMCPBodyProtocolVersion])
	assert.Equal(t, legacy.Attributes, modern.Attributes,
		"the declared era must not change what the resolver publishes")
}

func TestMCPPrepare_RejectsUnusablePartition(t *testing.T) {
	_, err := (&MCPResolver{}).Prepare(ResolverRouteConfig{ResolverName: MCPResolverName, APIID: ""})
	assert.Error(t, err, "an empty API id would compose a key that escapes its partition")

	_, err = (&MCPResolver{}).Prepare(ResolverRouteConfig{
		ResolverName: MCPResolverName, APIID: "api" + chainkey.Separator + "evil",
	})
	assert.Error(t, err, "a separator in the API id is a partition-crossing attempt")
}

// This resolver reads no route configuration, so malformed configuration cannot fail a
// route. That is deliberate: a controller newer than the engine may emit a field this
// build does not know, and dropping the route over it would take an MCP proxy offline on
// a version skew that has no effect on behaviour.
func TestMCPPrepare_IgnoresRouteConfigEntirely(t *testing.T) {
	prepared, err := (&MCPResolver{}).Prepare(ResolverRouteConfig{
		ResolverName:   MCPResolverName,
		APIID:          "api-1",
		ResolverConfig: json.RawMessage(`{"specVersion":`), // truncated, unparseable
	})
	require.NoError(t, err, "config is never decoded, so it cannot be malformed")
	assert.True(t, prepared.Requirements().BuffersBody())
}

// The key must be composed and inside the route's own partition — validateResolvedKey
// rejects a protocol resolver that returns the route key.
func TestMCPResolve_ComposesKeyInsideItsPartition(t *testing.T) {
	res := resolveMCP(t, prepareMCP(t, nil),
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)

	apiID, vhost, operation, ok := chainkey.Split(res.ChainKey)
	require.True(t, ok, "a protocol resolver must return a composed key")
	assert.Equal(t, "api-1", apiID)
	assert.Equal(t, "gw.example.com", vhost)
	assert.Equal(t, mcpEnrichOnlyOperation, operation)
}

// ─── Resolution: what a policy would otherwise have parsed for itself ────────

func TestMCPResolve_PublishesBodyFacts(t *testing.T) {
	cases := []struct {
		name string
		body string
		want map[string]string
	}{
		{
			name: "tools/call names its capability with params.name",
			body: `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"get_forecast"}}`,
			want: map[string]string{
				AttrMCPBodyPresent:          "true",
				AttrMCPBodyMethod:           "tools/call",
				AttrMCPBodyCapabilityType:   "tool",
				AttrMCPBodyCapabilityAction: "call",
				AttrMCPBodyCapabilityName:   "get_forecast",
				AttrMCPBodyJSONRPCID:        "7",
			},
		},
		{
			name: "resources/read names its capability with params.uri",
			body: `{"jsonrpc":"2.0","id":"abc","method":"resources/read","params":{"uri":"file:///a.txt"}}`,
			want: map[string]string{
				AttrMCPBodyPresent:          "true",
				AttrMCPBodyMethod:           "resources/read",
				AttrMCPBodyCapabilityType:   "resource",
				AttrMCPBodyCapabilityAction: "read",
				AttrMCPBodyCapabilityName:   "file:///a.txt",
				AttrMCPBodyJSONRPCID:        `"abc"`,
			},
		},
		{
			name: "a modern request states its protocol version in params._meta",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
				`"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
			want: map[string]string{
				AttrMCPBodyPresent:          "true",
				AttrMCPBodyMethod:           "tools/list",
				AttrMCPBodyCapabilityType:   "tool",
				AttrMCPBodyCapabilityAction: "list",
				AttrMCPBodyProtocolVersion:  "2026-07-28",
				AttrMCPBodyJSONRPCID:        "1",
			},
		},
		{
			// A notification is not published as its own fact: a consumer reads it off
			// the absence of jsonrpc.id on a body that named a method, which is the same
			// test the resolver would have applied.
			name: "a request with no id publishes no id",
			body: `{"jsonrpc":"2.0","method":"notifications/initialized"}`,
			want: map[string]string{
				AttrMCPBodyPresent:          "true",
				AttrMCPBodyMethod:           "notifications/initialized",
				AttrMCPBodyCapabilityType:   "notification",
				AttrMCPBodyCapabilityAction: "initialized",
			},
		},
		{
			name: "a method with no family segment has no capability",
			body: `{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
			want: map[string]string{
				AttrMCPBodyPresent:   "true",
				AttrMCPBodyMethod:    "initialize",
				AttrMCPBodyJSONRPCID: "1",
			},
		},
		{
			name: "server/discover is the modern entry point",
			body: `{"jsonrpc":"2.0","id":1,"method":"server/discover"}`,
			want: map[string]string{
				AttrMCPBodyPresent:          "true",
				AttrMCPBodyMethod:           "server/discover",
				AttrMCPBodyCapabilityType:   "server",
				AttrMCPBodyCapabilityAction: "discover",
				AttrMCPBodyJSONRPCID:        "1",
			},
		},
	}

	prepared := prepareMCP(t, nil)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveMCP(t, prepared, tc.body).Attributes)
		})
	}
}

// Leniency is a deliberate choice, not an omission: rejecting here would replace MCP's
// own JSON-RPC error shapes with the engine's sterile response, which no policy can see
// or customise. Publishing nothing is safe because policies fail closed on missing facts.
func TestMCPResolve_IsLenientAndStillBinds(t *testing.T) {
	prepared := prepareMCP(t, nil)
	wantKey := ChainKeyFor("api-1", "gw.example.com", mcpEnrichOnlyOperation)

	// The resolver reports when it could not READ the body, and publishes only
	// mcp.body.present when it read one and found nothing to take. Three states, not two —
	// nothing at all, present alone, or a reason — so which side of the line each case
	// falls on is asserted per case rather than "no attributes" for all.
	cases := []struct {
		name        string
		body        string
		wantReason  string // "" means no reason published
		wantPresent bool   // a body was there and was read
	}{
		{
			// Broken syntax — almost always truncation. -32700 territory.
			name: "truncated JSON does not parse",
			body: `{"jsonrpc":`, wantReason: MCPBodySyntaxError,
		},
		{
			// Parses fine, but a modelled member has the wrong shape. The released
			// policies answered -32600 for this, not -32700, which is why it is a
			// separate reason.
			name: "a modelled member with the wrong type",
			body: `{"method":42}`, wantReason: MCPBodyInvalidMemberType,
		},
		{
			// encoding/json keeps decoding past a type error, so the capability would
			// otherwise be published as absent while the method reads fine - and "named no
			// capability" is what tells a restricting policy there is nothing to govern.
			name:       "a wrong-typed capability name is unreadable, not absent",
			body:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":5}}`,
			wantReason: MCPBodyInvalidMemberType,
		},
		{
			// The same, reached through the member a resource family is keyed on.
			name:       "a wrong-typed resource uri is unreadable",
			body:       `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":5}}`,
			wantReason: MCPBodyInvalidMemberType,
		},
		{
			// A decoy: the governed member is wrong-typed while another member of the same
			// object is fine, so a partial decode would publish the readable half.
			name:       "a wrong-typed name beside a readable uri publishes neither",
			body:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":5,"uri":"file:///x"}}`,
			wantReason: MCPBodyInvalidMemberType,
		},
		{
			// encoding/json reports only the first type error, so a wrong-typed telemetry
			// member ahead of the governed one used to hide it and publish no name at all.
			name:       "a wrong-typed telemetry member first does not hide a wrong-typed name",
			body:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"clientInfo":42,"name":5}}`,
			wantReason: MCPBodyInvalidMemberType,
		},
		{
			name:       "a wrong-typed telemetry member first does not hide a wrong-typed uri",
			body:       `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"requestState":1,"uri":5}}`,
			wantReason: MCPBodyInvalidMemberType,
		},
		{
			name: "a batch names several operations and therefore none",
			body: `[{"method":"tools/list"}]`, wantReason: MCPBodyNotAnObject,
		},
		{
			name: "a JSON scalar is not a request envelope",
			body: `"hello"`, wantReason: MCPBodyNotAnObject,
		},
		{
			// Shares the non-object branch with the two above, but does not parse at all,
			// so the consuming policies must answer -32700 rather than -32600.
			name: "bytes that are not JSON are a syntax error, not a wrong kind",
			body: `x`, wantReason: MCPBodySyntaxError,
		},
		{
			// The duplicate check reads the object before the unmarshal does, so it has to
			// read to the end of the input: bytes after a complete object make this a
			// syntax error, whatever the object itself said.
			name: "trailing bytes after a duplicate member are a syntax error",
			body: `{"method":"a","method":"b"}x`, wantReason: MCPBodySyntaxError,
		},
		{
			// Bytes arrived, so this is a body; whitespace alone is not valid JSON.
			// Reading it as "no body" would hide it behind the bodyless case, which every
			// consumer treats as legitimate.
			name: "a whitespace-only body is a body that cannot be read",
			body: "  \n\t ", wantReason: MCPBodySyntaxError,
		},
		{
			// Read successfully; there was simply nothing there. Not a failure to read, so
			// no reason — a bodyless request on a buffered route produces this legitimately.
			name: "an empty body is not a failure to read",
			body: ``, wantReason: "",
		},
		{
			// Parsed fine, named no operation. Nothing usable to take, but nothing wrong
			// with the reading either.
			name: "an empty object carries no facts and no fault",
			body: `{}`, wantReason: "", wantPresent: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			assert.Equal(t, wantKey, res.ChainKey, "the chain must still bind so a policy can answer")

			// Never a method, whichever side of the line: consumers keep failing closed.
			assert.NotContains(t, res.Attributes, AttrMCPBodyMethod)

			if tc.wantReason == "" {
				if tc.wantPresent {
					assert.Equal(t, map[string]string{AttrMCPBodyPresent: "true"}, res.Attributes,
						"read fine and found nothing, which is not the same as never having a body to read")
				} else {
					assert.Empty(t, res.Attributes)
				}
				return
			}
			assert.Equal(t, map[string]string{AttrMCPBodyPresent: "true", AttrMCPBodyUnusable: tc.wantReason}, res.Attributes)
		})
	}
}

// A body that names no operation is still a body that was read, and the facts that do not
// depend on the method survive it. The commonest such body is not malformed at all: MCP
// streamable HTTP has the client POST JSON-RPC responses to server-initiated requests, and
// those carry an id and a result but never a method.
func TestMCPResolve_MethodlessBodyKeepsWhatDoesNotDependOnTheMethod(t *testing.T) {
	prepared := prepareMCP(t, nil)
	wantKey := ChainKeyFor("api-1", "gw.example.com", mcpEnrichOnlyOperation)

	cases := []struct {
		name string
		body string
		want map[string]string
	}{
		{
			// The motivating case. Without the id a policy rejecting this request emits
			// "id": null against a request that plainly stated one.
			name: "a client-posted response keeps its id",
			body: `{"jsonrpc":"2.0","id":7,"result":{"ok":true}}`,
			want: map[string]string{AttrMCPBodyPresent: "true", AttrMCPBodyJSONRPCID: "7"},
		},
		{
			name: "a string id keeps its quotes, which carry its type",
			body: `{"jsonrpc":"2.0","id":"req-7","result":{}}`,
			want: map[string]string{AttrMCPBodyPresent: "true", AttrMCPBodyJSONRPCID: `"req-7"`},
		},
		{
			// _meta is read the same way whatever the body went on to name.
			name: "the protocol version survives from params._meta",
			body: `{"jsonrpc":"2.0","id":7,"params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
			want: map[string]string{
				AttrMCPBodyPresent:         "true",
				AttrMCPBodyJSONRPCID:       "7",
				AttrMCPBodyProtocolVersion: "2026-07-28",
			},
		},
		{
			// Beside method rather than inside params, so the server never reads it and
			// neither does this resolver: the body states nothing.
			name: "but not from a top-level _meta, which no revision defines",
			body: `{"jsonrpc":"2.0","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`,
			want: map[string]string{AttrMCPBodyPresent: "true"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			assert.Equal(t, wantKey, res.ChainKey, "the chain must still bind")
			assert.Equal(t, tc.want, res.Attributes)

			// Never inferred from a body that named no operation.
			assert.NotContains(t, res.Attributes, AttrMCPBodyMethod)
			assert.NotContains(t, res.Attributes, AttrMCPBodyUnusable,
				"the body was read successfully; it simply named no operation")
		})
	}
}

// The id is published as the JSON token it arrived as, because JSON-RPC allows a String or
// a Number and a client correlates its pending request by matching the value. Unwrapping a
// string here would answer `"id":"7"` with `"id":7`, which no client would match.
func TestMCPResolve_JSONRPCIDKeepsItsType(t *testing.T) {
	prepared := prepareMCP(t, nil)

	cases := []struct {
		name string
		id   string // as written in the body
		want string // the published attribute, "" meaning not published
	}{
		{"a number stays a number", `7`, `7`},
		{"a string keeps its quotes", `"7"`, `"7"`},
		{"a non-numeric string likewise", `"req-7"`, `"req-7"`},
		{"zero is a number, not an absence", `0`, `0`},
		// A notification is a request object WITHOUT an id member. An empty string and a
		// null are both members that are present, so both are ids and neither is a
		// notification.
		{"an empty string is an id, not a notification", `""`, `""`},
		{"a null id is a member that is present", `null`, `null`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":`+tc.id+`,"method":"tools/list"}`)
			got, published := res.Attributes[AttrMCPBodyJSONRPCID]
			if tc.want == "" {
				assert.False(t, published, "an absent id must publish nothing")
				return
			}
			assert.True(t, published, "the id must be published")
			assert.Equal(t, tc.want, got)
		})
	}

	// The case the whole change is for: these two must not collide.
	number := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`)
	str := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":"7","method":"tools/list"}`)
	assert.NotEqual(t, number.Attributes[AttrMCPBodyJSONRPCID], str.Attributes[AttrMCPBodyJSONRPCID],
		"a number id and a string id that looks numeric must be distinguishable")
}

// A notification is "method present, id member absent" — both halves, and presence rather
// than value. JSON-RPC defines a notification as a request object *without* an id, so a null
// or empty-string id is a request, and a body with an id but no method is neither: it is a
// response the client POSTed to answer a server-initiated call.
func TestMCPResolve_NotificationIsAnAbsentIDMember(t *testing.T) {
	prepared := prepareMCP(t, nil)

	for _, tc := range []struct {
		name           string
		body           string
		wantMethod     bool
		wantID         bool
		isNotification bool
	}{
		{"a request with a number id", `{"jsonrpc":"2.0","id":7,"method":"tools/list"}`, true, true, false},
		{"a request with a null id", `{"jsonrpc":"2.0","id":null,"method":"tools/list"}`, true, true, false},
		{"a request with an empty string id", `{"jsonrpc":"2.0","id":"","method":"tools/list"}`, true, true, false},
		{"a notification", `{"jsonrpc":"2.0","method":"notifications/initialized"}`, true, false, true},
		{"a client-posted response is neither", `{"jsonrpc":"2.0","id":7,"result":{}}`, false, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := resolveMCP(t, prepared, tc.body).Attributes
			_, hasMethod := a[AttrMCPBodyMethod]
			_, hasID := a[AttrMCPBodyJSONRPCID]
			assert.Equal(t, tc.wantMethod, hasMethod, "method presence")
			assert.Equal(t, tc.wantID, hasID, "id presence")
			assert.Equal(t, tc.isNotification, hasMethod && !hasID, "the notification test")
		})
	}
}

// A string id costs two more characters than it used to, for its quotes. The kernel drops an
// attribute over maxResolutionAttributeValueLen rather than truncating it, so an id near the
// limit now falls off the end — which is correct, since a truncated id correlates with nothing,
// but is worth pinning rather than discovering.
func TestMCPResolve_LongStringIDCostsItsQuotes(t *testing.T) {
	prepared := prepareMCP(t, nil)
	long := strings.Repeat("a", 250)
	res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":"`+long+`","method":"tools/list"}`)
	assert.Equal(t, len(long)+2, len(res.Attributes[AttrMCPBodyJSONRPCID]),
		"the published token is the id plus its two quotes")
}

// "Notification" is read off an absent jsonrpc.id, and that inference holds only for a
// body that named an operation. These bodies name none, so a consumer testing the id
// alone would call every one of them a notification — which is why the test is "method
// present AND id absent", and why the resolver publishes no notification fact of its own
// that could assert it here.
func TestMCPResolve_MethodlessBodyAssertsNoNotification(t *testing.T) {
	prepared := prepareMCP(t, nil)

	for _, body := range []string{
		`{}`,
		`{"jsonrpc":"2.0"}`,
		`{"jsonrpc":"2.0","id":7,"result":{}}`,
		`{"jsonrpc":"2.0","result":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			res := resolveMCP(t, prepared, body)
			// No method, so the precondition for the notification test is never met.
			assert.NotContains(t, res.Attributes, AttrMCPBodyMethod)
		})
	}
}

// The counterpart: a body carrying nothing method-independent either yields no attributes
// at all, so "the resolver found nothing in it" stays one observable state rather than
// splitting into nil and an empty map.
// A body that was read but yielded nothing publishes mcp.body.present and nothing else, while
// a request that carried no body publishes nothing at all. The two are different facts and must
// not share one representation: the first says authoritatively that no operation was named — the
// answer a resolver-less gateway reaches by parsing — and the second says there is nothing to go
// on, where a consumer has to fail closed.
func TestMCPResolve_ReadingNothingIsNotTheSameAsHavingNothingToRead(t *testing.T) {
	prepared := prepareMCP(t, nil)

	t.Run("no body at all", func(t *testing.T) {
		res := resolveMCP(t, prepared, ``)
		assert.Nil(t, res.Attributes, "nil, not an empty map")
	})

	// The third is the motivating case: a client-posted JSON-RPC response answering a
	// server-initiated sampling call. It names no operation and, lacking an id, carries no
	// other fact either — so before this key it was indistinguishable from no body.
	for _, body := range []string{
		`{}`,
		`{"jsonrpc":"2.0"}`,
		`{"jsonrpc":"2.0","result":{"role":"assistant","content":{"type":"text","text":"warm"}}}`,
	} {
		t.Run(body, func(t *testing.T) {
			res := resolveMCP(t, prepared, body)
			assert.Equal(t, map[string]string{AttrMCPBodyPresent: "true"}, res.Attributes)
		})
	}
}

// An unreadable body still reports that a body was there. Presence and readability are
// separate questions: "instead of the facts" governs values read FROM the body, because a
// method taken from a body that reads two ways is the confused-deputy vector — whereas saying
// bytes existed reveals nothing about what they said. Keeping them orthogonal means a consumer
// asking "was there anything to go on" never has to check the reason first.
func TestMCPResolve_UnusableStillReportsTheBodyWasThere(t *testing.T) {
	prepared := prepareMCP(t, nil)

	for _, body := range []string{`{"method":`, `{"method":42}`, `[{"jsonrpc":"2.0"}]`, `{"method":"a","method":"b"}`} {
		t.Run(body, func(t *testing.T) {
			res := resolveMCP(t, prepared, body)
			assert.Equal(t, "true", res.Attributes[AttrMCPBodyPresent], "bytes arrived, whatever they turned out to be")
			assert.Contains(t, res.Attributes, AttrMCPBodyUnusable)
			// Still no values read FROM the body — that part of the rule is unchanged.
			assert.NotContains(t, res.Attributes, AttrMCPBodyMethod)
			assert.NotContains(t, res.Attributes, AttrMCPBodyCapabilityName)
			assert.NotContains(t, res.Attributes, AttrMCPBodyJSONRPCID)
		})
	}
}

// Publishing more on the methodless path must not weaken the unusable rule: a body that
// could not be READ still reports only its reason, with no facts alongside it.
func TestMCPResolve_UnusableStillPublishesNothingElse(t *testing.T) {
	prepared := prepareMCP(t, nil)

	// Each of these carries an id and a _meta version that would be readable on their own.
	cases := map[string]string{
		MCPBodySyntaxError:       `{"jsonrpc":"2.0","id":7,"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}`,
		MCPBodyInvalidMemberType: `{"jsonrpc":"2.0","id":7,"method":42,"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`,
		MCPBodyNotAnObject:       `[{"jsonrpc":"2.0","id":7}]`,
		MCPBodyAmbiguous:         `{"jsonrpc":"2.0","id":7,"method":"tools/list","method":"tools/call"}`,
	}
	for reason, body := range cases {
		t.Run(reason, func(t *testing.T) {
			res := resolveMCP(t, prepared, body)
			assert.Equal(t, map[string]string{AttrMCPBodyPresent: "true", AttrMCPBodyUnusable: reason}, res.Attributes,
				"the reason is published INSTEAD of the facts, never alongside")
		})
	}
}

// A BodyBuffered resolver is called with a nil body whenever the request headers are
// end-of-stream, because Envoy then sends no body callback at all.
func TestMCPResolve_ToleratesNilBodyOnABufferedRoute(t *testing.T) {
	prepared := prepareMCP(t, nil)
	require.True(t, prepared.Requirements().BuffersBody())

	res, err := prepared.Resolve(context.Background(), RequestView{Body: nil})
	require.NoError(t, err, "a bodyless request must not fail resolution")
	assert.Equal(t, ChainKeyFor("api-1", "gw.example.com", mcpEnrichOnlyOperation), res.ChainKey)
	assert.Empty(t, res.Attributes)
}

// ─── The span allow-list ─────────────────────────────────────────────────────

func TestMCPSpanSafeAttributes_ExcludeOpenSets(t *testing.T) {
	assert.True(t, IsSpanSafeAttribute(AttrMCPBodyMethod), "a closed set of ~10 values")
	assert.True(t, IsSpanSafeAttribute(AttrMCPBodyCapabilityType))

	assert.False(t, IsSpanSafeAttribute(AttrMCPBodyCapabilityName),
		"a tool name is caller-chosen and would mint one index entry per value")
	assert.False(t, IsSpanSafeAttribute(AttrMCPBodyJSONRPCID),
		"unique per request — the worst possible span attribute")
	assert.False(t, IsSpanSafeAttribute("mcp.body.something.new"),
		"an unknown key must not inherit permission by default")
}

// ─── Registration ────────────────────────────────────────────────────────────

func TestMCPResolver_IsRegisteredButInert(t *testing.T) {
	res, ok := DefaultRegistry().Get(MCPResolverName)
	require.True(t, ok, "the controller can only reach a registered factory")
	assert.Equal(t, MCPResolverName, res.Name())
}

// ─── Ambiguous members ───────────────────────────────────────────────────────

// The confused-deputy vector this closes: encoding/json folds duplicate and
// case-variant names onto one field and keeps the LAST. A backend matching exactly, or
// keeping the first, reads a different request from the same bytes — the gateway
// exempts tools/list from authentication while the server executes tools/call.
//
// Publishing nothing is the safe outcome: consumers fail closed on missing facts, so an
// ambiguous request cannot match an exception list.
func TestMCPResolve_AmbiguousMembersPublishNothing(t *testing.T) {
	prepared := prepareMCP(t, nil)
	wantKey := ChainKeyFor("api-1", "gw.example.com", mcpEnrichOnlyOperation)

	ambiguous := []struct {
		name string
		body string
	}{
		{
			name: "duplicate method — the gateway would read the last, a strict server the first",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","method":"tools/call"}`,
		},
		{
			name: "case-variant method, which encoding/json folds onto the canonical name",
			body: `{"jsonrpc":"2.0","id":1,"Method":"tools/call"}`,
		},
		{
			name: "both spellings present",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/list","Method":"tools/call"}`,
		},
		{
			name: "duplicate params",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
				`"params":{"name":"safe"},"params":{"name":"delete_everything"}}`,
		},
		{
			name: "duplicate capability name inside params",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
				`"params":{"name":"safe","name":"delete_everything"}}`,
		},
		{
			name: "case-variant capability name inside params",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"Name":"delete_everything"}}`,
		},
		{
			// _meta decodes into a map, so a repeat keeps the last value while a parser
			// taking the first reads another version for the same request.
			name: "duplicate protocol version inside params._meta",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","_meta":` +
				`{"io.modelcontextprotocol/protocolVersion":"2025-06-18",` +
				`"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`,
		},
		{
			name: "case-variant protocol version inside params._meta",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","_meta":` +
				`{"io.modelcontextprotocol/ProtocolVersion":"2026-07-28"}}}`,
		},
		{
			// Unicode simple folding: "ſ" (U+017F LATIN SMALL LETTER LONG S) folds to "s",
			// so encoding/json accepts "paramſ" as "params". A byte-comparing backend does
			// not — the same divergence as a case variant, but far harder to spot.
			//
			// It must be a member that actually contains an "s"; an earlier version of
			// this case used "meſthod", which is not a fold of "method" at all (the rune
			// is inserted, not substituted, and "method" has no "s"). That payload passed
			// only because the assertion was "publishes nothing" and an unrecognised
			// member also publishes nothing.
			name: "unicode fold equivalent of params",
			body: "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"," +
				"\"paramſ\":{\"name\":\"delete_everything\"}}",
		},
	}

	for _, tc := range ambiguous {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			assert.Equal(t, wantKey, res.ChainKey, "the chain must still bind so a policy can answer")

			// The marker and nothing else. Publishing a method alongside it would defeat
			// the point — a value read from a body that reads two ways is exactly what
			// must not reach a policy.
			assert.Equal(t, map[string]string{AttrMCPBodyPresent: "true", AttrMCPBodyUnusable: MCPBodyAmbiguous}, res.Attributes)
			assert.NotContains(t, res.Attributes, AttrMCPBodyMethod,
				"consumers must still see no method, so they keep failing closed")
		})
	}
}

// The guard must not fire on ordinary requests — a false positive would silently make
// every consumer fail closed and reject valid traffic.
func TestMCPResolve_UnambiguousMembersStillResolve(t *testing.T) {
	prepared := prepareMCP(t, nil)

	unambiguous := []struct {
		name       string
		body       string
		wantMethod string
	}{
		{
			name:       "an ordinary request",
			wantMethod: "tools/call",
			body:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_forecast"}}`,
		},
		{
			// Only members the resolver reads are policed. A tool's own arguments are
			// the backend's business, and may legitimately contain anything.
			name:       "duplicates in members the resolver does not read",
			wantMethod: "tools/call",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call",` +
				`"params":{"name":"get_forecast","arguments":{"a":1,"a":2}}}`,
		},
		{
			name:       "a capability name that merely resembles a reserved member",
			wantMethod: "tools/call",
			body:       `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"method"}}`,
		},
		{
			// A vendor member is nobody's business but the backend's, and _meta is the
			// extension point the spec reserves for them.
			name:       "duplicates in a vendor _meta member",
			wantMethod: "tools/call",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","_meta":` +
				`{"com.example/trace":{"a":1,"a":2},"com.example/Trace":"y"}}}`,
		},
		{
			name:       "the reserved _meta members named once",
			wantMethod: "tools/call",
			body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","_meta":` +
				`{"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
				`"io.modelcontextprotocol/clientInfo":{"name":"a","version":"1"}}}}`,
		},
		{
			name:       "params is not an object",
			body:       `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":[]}`,
			wantMethod: "tools/list",
		},
	}

	for _, tc := range unambiguous {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			assert.NotEmpty(t, res.Attributes, "a well-formed request must still publish facts")
			assert.Equal(t, tc.wantMethod, res.Attributes[AttrMCPBodyMethod])
			assert.NotContains(t, res.Attributes, AttrMCPBodyUnusable,
				"a clean body must not be flagged; a false positive here fails every request closed")
		})
	}
}

// The marker is safe to index: it is "true" or absent, unlike a tool name. It is also the
// attribute an operator would most want to alert on, since a rising count means someone is
// probing the parser rather than using the API.
func TestMCPSpanSafeAttributes_IncludeTheUnusableReason(t *testing.T) {
	assert.True(t, IsSpanSafeAttribute(AttrMCPBodyUnusable))
}

// params and id are held raw so their shape cannot fail the envelope. JSON-RPC permits
// "params": [], and an earlier version modelled params as a struct — which failed the whole
// decode and discarded a perfectly readable method over an unrelated field.
func TestMCPResolve_RawMembersCannotFailTheEnvelope(t *testing.T) {
	prepared := prepareMCP(t, nil)

	for _, body := range []string{
		`{"method":"tools/list","params":"a string"}`,
		`{"method":"tools/list","params":[]}`,
		`{"method":"tools/list","id":{"nested":true}}`,
	} {
		res := resolveMCP(t, prepared, body)
		assert.NotContains(t, res.Attributes, AttrMCPBodyUnusable, "body: %s", body)
		assert.Equal(t, "tools/list", res.Attributes[AttrMCPBodyMethod],
			"the method must survive an odd shape in an unrelated member: %s", body)
	}
}

// The capability name is keyed on the family, not on whichever params member is populated.
// MCP identifies a resource by uri and defines no name for resources/*, so a params.name there
// is client-controlled noise — taking it let a decoy mask the resource the server actually
// reads, and a rule written against that uri stopped matching.
func TestMCPResolve_CapabilityNameIsKeyedOnTheFamily(t *testing.T) {
	prepared := prepareMCP(t, nil)

	tests := []struct {
		name string
		body string
		want string // "" means no capability name published
	}{
		{
			// The bypass. Before this rule the decoy name won and a rule naming the uri
			// was never consulted.
			name: "a decoy name does not mask the uri a resource read addresses",
			body: `{"method":"resources/read","params":{"name":"display-name","uri":"file:///secret"}}`,
			want: "file:///secret",
		},
		{
			name: "a resource read is its uri",
			body: `{"method":"resources/read","params":{"uri":"file:///logs"}}`,
			want: "file:///logs",
		},
		{
			// No uri, so no resource is addressed. Naming one from a member the server will
			// not read would give an ACL something to match for a capability never invoked.
			name: "a resource read with no uri names nothing",
			body: `{"method":"resources/read","params":{"name":"display-name"}}`,
			want: "",
		},
		{
			name: "a tool call is its name",
			body: `{"method":"tools/call","params":{"name":"get_weather"}}`,
			want: "get_weather",
		},
		{
			// The mirror of the first case: a uri on a tools/call is not the tool.
			name: "a stray uri does not name a tool",
			body: `{"method":"tools/call","params":{"uri":"file:///secret"}}`,
			want: "",
		},
		{
			name: "a prompt get is its name",
			body: `{"method":"prompts/get","params":{"name":"greet"}}`,
			want: "greet",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := resolveMCP(t, prepared, tc.body)
			got, published := res.Attributes[AttrMCPBodyCapabilityName]
			if tc.want == "" {
				assert.False(t, published, "expected no capability name, got %q", got)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// MCP states the calling client in a different place in each era, so both have to publish the
// same two keys or a consumer would have to know which revision it is holding.
func TestMCPResolve_PublishesClientInfoFromEitherEra(t *testing.T) {
	prepared := prepareMCP(t, nil)

	cases := []struct {
		name        string
		body        string
		wantName    string
		wantVersion string
	}{
		{
			name:        "modern states it in params._meta on an ordinary call",
			body:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","_meta":{"io.modelcontextprotocol/clientInfo":{"name":"ExampleClient","version":"1.0.0"}}}}`,
			wantName:    "ExampleClient",
			wantVersion: "1.0.0",
		},
		{
			// Outside params, so the server ignores it and so must this resolver.
			name:        "a top-level _meta is not a place MCP defines clientInfo",
			body:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t"},"_meta":{"io.modelcontextprotocol/clientInfo":{"name":"TopLevel","version":"2.0"}}}`,
			wantName:    "",
			wantVersion: "",
		},
		{
			name:        "legacy states it in the initialize params",
			body:        `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"LegacyClient","version":"0.9"}}}`,
			wantName:    "LegacyClient",
			wantVersion: "0.9",
		},
		{
			name:        "params._meta wins over the legacy member",
			body:        `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"clientInfo":{"name":"Legacy","version":"0.9"},"_meta":{"io.modelcontextprotocol/clientInfo":{"name":"Modern","version":"2.0"}}}}`,
			wantName:    "Modern",
			wantVersion: "2.0",
		},
		{
			name:        "a partial identity publishes only what it stated",
			body:        `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","_meta":{"io.modelcontextprotocol/clientInfo":{"name":"NameOnly"}}}}`,
			wantName:    "NameOnly",
			wantVersion: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := resolveMCP(t, prepared, tc.body).Attributes
			if tc.wantName == "" {
				assert.NotContains(t, attrs, AttrMCPBodyClientName)
			} else {
				assert.Equal(t, tc.wantName, attrs[AttrMCPBodyClientName])
			}
			if tc.wantVersion == "" {
				assert.NotContains(t, attrs, AttrMCPBodyClientVersion)
			} else {
				assert.Equal(t, tc.wantVersion, attrs[AttrMCPBodyClientVersion])
			}
		})
	}
}

// The identity describes the client, not the operation, so it survives a body that names no
// method — which on 2026-07-28 is a client-posted JSON-RPC response, still carrying _meta.
func TestMCPResolve_MethodlessBodyStillPublishesClientInfo(t *testing.T) {
	prepared := prepareMCP(t, nil)
	attrs := resolveMCP(t, prepared,
		`{"jsonrpc":"2.0","id":7,"result":{},"params":{"_meta":{"io.modelcontextprotocol/clientInfo":{"name":"Responder","version":"3.1"}}}}`).Attributes

	assert.Equal(t, "Responder", attrs[AttrMCPBodyClientName])
	assert.Equal(t, "3.1", attrs[AttrMCPBodyClientVersion])
	assert.NotContains(t, attrs, AttrMCPBodyMethod, "a response names no operation")
}

// A legacy initialize proposes a version in params rather than _meta. Folding it into the same
// key was decided deliberately; see mcpProtocolVersion.
func TestMCPResolve_LegacyInitializeProtocolVersion(t *testing.T) {
	prepared := prepareMCP(t, nil)

	t.Run("params.protocolVersion is read", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`).Attributes
		assert.Equal(t, "2025-06-18", attrs[AttrMCPBodyProtocolVersion])
	})

	t.Run("_meta still wins when both are stated", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`).Attributes
		assert.Equal(t, "2026-07-28", attrs[AttrMCPBodyProtocolVersion])
	})
}

// Every member a decision is read from has to be in the ambiguity list, or the gateway and the
// backend can disagree about which spelling won. The legacy protocolVersion is one: it states the
// era. The client's identity is not, and is covered by
// TestMCPResolve_ADuplicateClientInfoLeavesTheFactsReadable.
func TestMCPResolve_AnAmbiguousLegacyProtocolVersionIsUnusable(t *testing.T) {
	prepared := prepareMCP(t, nil)

	attrs := resolveMCP(t, prepared,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","ProtocolVersion":"2026-07-28"}}`).Attributes
	assert.Equal(t, MCPBodyAmbiguous, attrs[AttrMCPBodyUnusable])
}

// A member of the wrong shape is one bad fact, not a bad body: the rest still reads.
func TestMCPResolve_MalformedClientInfoDoesNotSpoilTheBody(t *testing.T) {
	prepared := prepareMCP(t, nil)

	for _, body := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","clientInfo":42}}`,
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t","_meta":{"io.modelcontextprotocol/clientInfo":"not-an-object"}}}`,
	} {
		attrs := resolveMCP(t, prepared, body).Attributes
		assert.NotContains(t, attrs, AttrMCPBodyClientName, "body: %s", body)
		assert.NotContains(t, attrs, AttrMCPBodyClientVersion, "body: %s", body)
		assert.Equal(t, "tools/call", attrs[AttrMCPBodyMethod], "the operation is still readable")
		assert.NotContains(t, attrs, AttrMCPBodyUnusable, "a malformed fact is not an unreadable body")
	}
}

// Caller-controlled free text of unbounded cardinality, so not indexed - the same reason
// AttrMCPBodyCapabilityName is absent.
func TestMCPClientInfoAttributesAreNotSpanSafe(t *testing.T) {
	assert.False(t, IsSpanSafeAttribute(AttrMCPBodyClientName))
	assert.False(t, IsSpanSafeAttribute(AttrMCPBodyClientVersion))
}

// Both revisions define a request as {jsonrpc, id, method, params?} and place _meta inside
// params, so a _meta beside method is a member the server ignores. Reading it would let the
// gateway derive a fact from bytes the backend never looks at, and its shape must not fail a
// parse that is otherwise fine.
func TestMCPResolve_EnvelopeLevelMetaIsNotAMember(t *testing.T) {
	prepared := prepareMCP(t, nil)

	t.Run("its shape cannot fail the envelope", func(t *testing.T) {
		attrs := resolveMCP(t, prepared, `{"method":"tools/call","_meta":"not-an-object"}`).Attributes
		assert.NotContains(t, attrs, AttrMCPBodyUnusable, "an unmodelled member is not a bad body")
		assert.Equal(t, "tools/call", attrs[AttrMCPBodyMethod], "the operation still reads")
	})

	t.Run("its contents are not read", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t"},`+
				`"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",`+
				`"io.modelcontextprotocol/clientInfo":{"name":"Ghost","version":"1.0"}}}`).Attributes
		assert.NotContains(t, attrs, AttrMCPBodyProtocolVersion)
		assert.NotContains(t, attrs, AttrMCPBodyClientName)
		assert.NotContains(t, attrs, AttrMCPBodyClientVersion)
	})

	t.Run("duplicating it is not ambiguous, since nothing reads it", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"t"},"_meta":{"a":1},"_Meta":{"a":2}}`).Attributes
		assert.NotContains(t, attrs, AttrMCPBodyUnusable)
		assert.Equal(t, "tools/call", attrs[AttrMCPBodyMethod])
	})
}

// The requestState a client echoes when retrying is the only thing that ties the retry to the
// input_required answer it responds to: the spec requires the JSON-RPC id to differ between them.
func TestMCPResolve_PublishesTheRequestStateFingerprint(t *testing.T) {
	prepared := prepareMCP(t, nil)
	const state = "eyJsb2NhdGlvbiI6Ik5ldyBZb3JrIn0-AEAD-protected-blob"

	t.Run("a retry publishes a hash, never the value", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_forecast",`+
				`"inputResponses":{"github_login":{"action":"accept"}},`+
				`"requestState":"`+state+`"}}`).Attributes

		got := attrs[AttrMCPBodyRequestStateHash]
		assert.Equal(t, hashRequestState(state), got)
		assert.NotContains(t, got, "AEAD", "the blob itself must not travel")
		assert.Len(t, got, 64, "sha256 hex, comfortably inside the 256-char attribute limit")
	})

	t.Run("the same state fingerprints identically, a different one does not", func(t *testing.T) {
		body := func(s string) string {
			return `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"t","requestState":"` + s + `"}}`
		}
		a := resolveMCP(t, prepared, body(state)).Attributes[AttrMCPBodyRequestStateHash]
		again := resolveMCP(t, prepared, body(state)).Attributes[AttrMCPBodyRequestStateHash]
		other := resolveMCP(t, prepared, body("a-different-blob")).Attributes[AttrMCPBodyRequestStateHash]

		assert.Equal(t, a, again, "two halves of one exchange must match")
		assert.NotEqual(t, a, other, "two different exchanges must not")
	})

	t.Run("an ordinary request publishes none", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_forecast"}}`).Attributes
		assert.NotContains(t, attrs, AttrMCPBodyRequestStateHash)
	})

	// Every member this resolver reads must be in the ambiguity list, or the gateway and the
	// backend can disagree about which spelling won.
	t.Run("a duplicated requestState is unusable", func(t *testing.T) {
		attrs := resolveMCP(t, prepared,
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"requestState":"a","RequestState":"b"}}`).Attributes
		assert.Equal(t, MCPBodyAmbiguous, attrs[AttrMCPBodyUnusable])
	})

	// A correlation token is one value per exchange, so indexing it would be unbounded.
	t.Run("it is not span-safe", func(t *testing.T) {
		assert.False(t, IsSpanSafeAttribute(AttrMCPBodyRequestStateHash))
	})
}

// Bind rejects a resolution carrying more attributes than the framework permits, so the
// count a fully populated body publishes is a bound this resolver has to stay inside.
// Asserted against the most attribute-bearing body there is: a modern tools/call that
// also carries an id, a client identity and a request state.
func TestMCPResolver_FullBodyStaysInsideTheAttributeBound(t *testing.T) {
	prepared := prepareMCP(t, nil)
	res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{
		"name":"get_weather","requestState":"state-token",
		"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",
		         "io.modelcontextprotocol/clientInfo":{"name":"acme-client","version":"1.4.0"}}}}`)

	assert.LessOrEqual(t, len(res.Attributes), MaxResolutionAttributes)
	assert.NoError(t, validateResolutionAttributes(res.Attributes))
}

// An over-long value is dropped rather than published, because Bind fails the whole
// resolution on one — a caller could otherwise fail its own request with a long resource
// URI, and the rest of the resolution would be discarded with it.
func TestMCPResolver_DropsOverLongAttributeValues(t *testing.T) {
	prepared := prepareMCP(t, nil)
	longURI := "file:///" + strings.Repeat("a", maxMCPCapabilityNameBytes)
	res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"`+longURI+`"}}`)

	_, published := res.Attributes[AttrMCPBodyCapabilityName]
	assert.False(t, published, "an over-long capability name must not be published")
	assert.Equal(t, "resources/read", res.Attributes[AttrMCPBodyMethod],
		"dropping one value must not discard the rest of the resolution")
	assert.NoError(t, validateResolutionAttributes(res.Attributes))
}

// A resource URI is a URI, not an identifier, so one longer than the 256 bytes that bounds
// every other attribute must still be published — otherwise the governing policies lose the
// name they match a URI rule on and fall open for exactly the resources most worth naming.
func TestMCPResolver_PublishesARealisticallyLongResourceURI(t *testing.T) {
	prepared := prepareMCP(t, nil)
	uri := "file:///" + strings.Repeat("a/", 250) + "report.json"
	require.Greater(t, len(uri), maxMCPAttributeValueBytes)
	require.LessOrEqual(t, len(uri), maxMCPCapabilityNameBytes)

	res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"`+uri+`"}}`)

	assert.Equal(t, uri, res.Attributes[AttrMCPBodyCapabilityName])
	assert.NoError(t, validateResolutionAttributes(res.Attributes))
}

// A family outside the known vocabulary is still published, as the family segment itself.
// Dropping it would hide the operation from a policy that matches on the method.
func TestMCPResolve_UnknownCapabilityFamilyKeepsItsSegment(t *testing.T) {
	prepared := prepareMCP(t, nil)
	res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"vendor/act"}`)

	assert.Equal(t, "vendor/act", res.Attributes[AttrMCPBodyMethod])
	assert.Equal(t, "vendor", res.Attributes[AttrMCPBodyCapabilityType])
	assert.Equal(t, "act", res.Attributes[AttrMCPBodyCapabilityAction])
}

// jsonObjectMembers reads the members before the unmarshal, so it sees bytes no other
// caller does. Anything it cannot read is reported as "not an object", which leaves the
// ambiguity check answering false and the unmarshal below it to produce the reason.
func TestJSONObjectMembers(t *testing.T) {
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"an object", `{"a":1,"b":2}`, true},
		{"duplicates are kept, which is the whole point", `{"a":1,"a":2}`, true},
		{"an empty object", `{}`, true},
		{"an array is not an object", `[1,2]`, false},
		{"a scalar is not an object", `"a"`, false},
		{"empty input", ``, false},
		{"truncated after a member", `{"a":1,`, false},
		{"a member with no value", `{"a":}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			members, ok := jsonObjectMembers([]byte(tc.body))
			assert.Equal(t, tc.ok, ok)
			if !tc.ok {
				assert.Nil(t, members)
			}
		})
	}
}

// A method addressing no capability family names no capability either, whatever params holds.
// Publishing a stray params.name as one would give an ACL a value to match on for a body that
// invoked nothing.
func TestMCPResolve_MethodWithoutAFamilyPublishesNoCapabilityName(t *testing.T) {
	prepared := prepareMCP(t, nil)
	res := resolveMCP(t, prepared, `{"jsonrpc":"2.0","id":1,"method":"ping","params":{"name":"allowed-tool"}}`)

	assert.Equal(t, "ping", res.Attributes[AttrMCPBodyMethod])
	assert.NotContains(t, res.Attributes, AttrMCPBodyCapabilityName)
	assert.NotContains(t, res.Attributes, AttrMCPBodyCapabilityType)
	assert.NotContains(t, res.Attributes, AttrMCPBodyCapabilityAction)
}
