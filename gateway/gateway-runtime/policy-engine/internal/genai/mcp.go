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
	"bytes"
	"encoding/json"
	"strconv"

	"go.opentelemetry.io/otel/propagation"
)

// MCP carries trace context in a request's params._meta, under the unprefixed W3C keys
// MCP SEP-414 reserves for it, as well as in HTTP headers: a client whose transport
// does not propagate headers end to end still links its tool calls to its trace.
const (
	metaTraceparent = "traceparent"
	metaTracestate  = "tracestate"
)

// MCPMetaContext returns the trace context a JSON-RPC request carries in params._meta,
// empty when it carries none.
func MCPMetaContext(body []byte) propagation.MapCarrier {
	carrier := propagation.MapCarrier{}
	var req struct {
		Params struct {
			Meta map[string]any `json:"_meta"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &req) != nil {
		return carrier
	}
	for _, key := range []string{metaTraceparent, metaTracestate} {
		if v, ok := req.Params.Meta[key].(string); ok && v != "" {
			carrier[key] = v
		}
	}
	return carrier
}

// RewriteMCPMeta returns body with params._meta's traceparent and tracestate set from
// carrier, adding _meta when params has none. Every other member is kept as it was,
// baggage included; members are re-emitted in key order and HTML characters are not
// escaped, so tool arguments reach the server as the client wrote them.
//
// ok is false, and the body must be forwarded unchanged, when it is not a JSON object,
// params is absent or not an object, or _meta is not an object.
//
// shortcut: a request without params (a ping) is left alone rather than given a params
// object only to carry _meta; add one if servers need to trace such calls.
func RewriteMCPMeta(body []byte, carrier propagation.MapCarrier) ([]byte, bool) {
	var top map[string]json.RawMessage
	if json.Unmarshal(body, &top) != nil || top == nil {
		return nil, false
	}
	var params map[string]json.RawMessage
	if raw, ok := top["params"]; !ok || json.Unmarshal(raw, &params) != nil || params == nil {
		return nil, false
	}
	meta := map[string]json.RawMessage{}
	if raw, ok := params["_meta"]; ok {
		if json.Unmarshal(raw, &meta) != nil || meta == nil {
			return nil, false
		}
	}
	for _, key := range []string{metaTraceparent, metaTracestate} {
		if v := carrier.Get(key); v != "" {
			meta[key] = json.RawMessage(strconv.Quote(v))
		} else {
			delete(meta, key)
		}
	}
	var err error
	if params["_meta"], err = encode(meta); err != nil {
		return nil, false
	}
	if top["params"], err = encode(params); err != nil {
		return nil, false
	}
	out, err := encode(top)
	if err != nil {
		return nil, false
	}
	return out, true
}

func encode(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// JSONRPCOutcome is how a JSON-RPC server (an MCP server or an A2A agent) answered a
// request.
type JSONRPCOutcome struct {
	// ErrorCode is the JSON-RPC error code, "" when the server returned a result.
	ErrorCode string
	// ToolError reports an MCP tool call whose result says isError: the tool ran and
	// failed.
	ToolError bool
}

// ParseJSONRPCResponse reads the JSON-RPC response from a JSON body or, for a streamed
// response, the first event carrying a result or an error. ok is false when no
// response is found, as for a notification the server accepted with no body.
func ParseJSONRPCResponse(body []byte) (JSONRPCOutcome, bool) {
	for _, event := range jsonEvents(body) {
		var resp struct {
			Result *struct {
				IsError bool `json:"isError"`
			} `json:"result"`
			Error *struct {
				Code json.Number `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(event, &resp) != nil {
			continue
		}
		switch {
		case resp.Error != nil:
			return JSONRPCOutcome{ErrorCode: resp.Error.Code.String()}, true
		case resp.Result != nil:
			return JSONRPCOutcome{ToolError: resp.Result.IsError}, true
		}
	}
	return JSONRPCOutcome{}, false
}
