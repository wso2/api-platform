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

package genai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
)

const testTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func TestMCPMetaContext(t *testing.T) {
	carrier := MCPMetaContext([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"x","_meta":{"traceparent":"` + testTraceparent + `","tracestate":"k=v"}}}`))
	assert.Equal(t, testTraceparent, carrier.Get("traceparent"))
	assert.Equal(t, "k=v", carrier.Get("tracestate"))
	assert.Empty(t, MCPMetaContext([]byte(`{"params":{}}`)).Keys())
	assert.Empty(t, MCPMetaContext([]byte(`not json`)).Keys())
}

func meta(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var req struct {
		Params map[string]any `json:"params"`
	}
	require.NoError(t, json.Unmarshal(body, &req))
	m, _ := req.Params["_meta"].(map[string]any)
	return m
}

func TestRewriteMCPMeta(t *testing.T) {
	carrier := propagation.MapCarrier{"traceparent": testTraceparent}

	t.Run("adds _meta", func(t *testing.T) {
		out, ok := RewriteMCPMeta([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_invoice","arguments":{"id":"7"}}}`), carrier)
		require.True(t, ok)
		assert.Equal(t, testTraceparent, meta(t, out)["traceparent"])
		var req map[string]any
		require.NoError(t, json.Unmarshal(out, &req))
		assert.Equal(t, "get_invoice", req["params"].(map[string]any)["name"])
		assert.Equal(t, "tools/call", req["method"])
	})

	t.Run("replaces the caller's context and drops its tracestate", func(t *testing.T) {
		out, ok := RewriteMCPMeta([]byte(`{"params":{"_meta":{"traceparent":"00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bbbbbbbbbbbbbbbb-01","tracestate":"old=1","baggage":"user=x","io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`), carrier)
		require.True(t, ok)
		m := meta(t, out)
		assert.Equal(t, testTraceparent, m["traceparent"])
		assert.NotContains(t, m, "tracestate", "a tracestate belongs with the traceparent it came with")
		assert.Equal(t, "user=x", m["baggage"], "baggage is the caller's, not the gateway's")
		assert.Equal(t, "2026-07-28", m["io.modelcontextprotocol/protocolVersion"])
	})

	t.Run("keeps tool arguments as written", func(t *testing.T) {
		out, ok := RewriteMCPMeta([]byte(`{"params":{"arguments":{"q":"a<b && c>d","n":1.50}}}`), carrier)
		require.True(t, ok)
		assert.Contains(t, string(out), `"q":"a<b && c>d"`, "HTML characters must not be escaped")
		assert.Contains(t, string(out), `"n":1.50`, "numbers keep their source form")
	})

	for name, body := range map[string]string{
		"no params":           `{"jsonrpc":"2.0","id":1,"method":"ping"}`,
		"params not object":   `{"params":[1,2]}`,
		"_meta not an object": `{"params":{"_meta":"x"}}`,
		"not json":            `{"params":`,
		"batch":               `[{"params":{}}]`,
	} {
		t.Run("skips "+name, func(t *testing.T) {
			_, ok := RewriteMCPMeta([]byte(body), carrier)
			assert.False(t, ok)
		})
	}
}

func TestParseMCPResponse(t *testing.T) {
	cases := map[string]struct {
		body string
		want MCPOutcome
		ok   bool
	}{
		"result":         {`{"jsonrpc":"2.0","id":1,"result":{"content":[]}}`, MCPOutcome{}, true},
		"tool error":     {`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[]}}`, MCPOutcome{ToolError: true}, true},
		"jsonrpc error":  {`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"bad"}}`, MCPOutcome{ErrorCode: "-32602"}, true},
		"sse after note": {"event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-32601}}\n\n", MCPOutcome{ErrorCode: "-32601"}, true},
		"empty":          {``, MCPOutcome{}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, ok := ParseMCPResponse([]byte(tc.body))
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
