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

package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ScriptedPathPrefix starts every path whose reply the request itself scripts. A request on any
// path below it gets exactly the reply its body names in ScriptedReplyField, so a scenario
// controls the model, usage and stream shape a gateway policy sees. Paths outside the prefix are
// unaffected.
const ScriptedPathPrefix = "/scripted/"

// ScriptedReplyField is the request body field that scripts the reply. Its value is an object
// with exactly one of:
//
//	"body":   any JSON value, written as an application/json body
//	"raw":    a string written verbatim, for malformed or empty bodies
//	"events": a server-sent event stream, one element per event: a JSON object or array is
//	          sent as "data: <compact JSON>", and a string is sent as the event's raw lines,
//	          such as "data: [DONE]" or "event: message_start\ndata: {...}"
//
// "contentType" optionally replaces the default content type of the chosen form. The service
// flushes after every event, so the gateway receives the stream as it is written.
const ScriptedReplyField = "x_testbench_reply"

// ScriptedHeader is set on every scripted reply, so a scenario can show that the gateway
// passed the upstream's headers through unchanged.
const ScriptedHeader = "X-Testbench-Scripted"

type scriptedReply struct {
	Body        json.RawMessage   `json:"body"`
	Raw         *string           `json:"raw"`
	Events      []json.RawMessage `json:"events"`
	ContentType string            `json:"contentType"`
}

func isScriptedPath(path string) bool {
	return strings.HasPrefix(path, ScriptedPathPrefix)
}

// parseScriptedReply reads ScriptedReplyField from a request body and checks that it names
// exactly one reply form.
func parseScriptedReply(body []byte) (scriptedReply, error) {
	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return scriptedReply{}, fmt.Errorf("a scripted request needs a JSON object body: %w", err)
	}
	field, ok := request[ScriptedReplyField]
	if !ok {
		return scriptedReply{}, fmt.Errorf("a scripted request needs the %q field", ScriptedReplyField)
	}
	var reply scriptedReply
	if err := json.Unmarshal(field, &reply); err != nil {
		return scriptedReply{}, fmt.Errorf("%q must be an object: %w", ScriptedReplyField, err)
	}
	// A JSON null body scripts nothing, like an absent one.
	if string(reply.Body) == "null" {
		reply.Body = nil
	}
	forms := 0
	if len(reply.Body) > 0 {
		forms++
	}
	if reply.Raw != nil {
		forms++
	}
	if reply.Events != nil {
		forms++
	}
	if forms != 1 {
		return scriptedReply{}, fmt.Errorf("%q must name exactly one of body, raw or events", ScriptedReplyField)
	}
	return reply, nil
}

func (s *Service) scripted(w http.ResponseWriter, body []byte) {
	reply, err := parseScriptedReply(body)
	if err != nil {
		http.Error(w, "openai: "+err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set(ScriptedHeader, "true")
	switch {
	case reply.Events != nil:
		writeScriptedEvents(w, reply)
	case reply.Raw != nil:
		w.Header().Set("Content-Type", orElse(reply.ContentType, "application/json"))
		_, _ = w.Write([]byte(*reply.Raw))
	default:
		var compact bytes.Buffer
		if err := json.Compact(&compact, reply.Body); err != nil {
			http.Error(w, "openai: scripted body is not valid JSON", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", orElse(reply.ContentType, "application/json"))
		_, _ = w.Write(compact.Bytes())
	}
}

// scriptedEvent renders one element of a scripted stream as its SSE lines, ending with the
// blank line that closes an event.
func scriptedEvent(element json.RawMessage) (string, error) {
	var raw string
	if json.Unmarshal(element, &raw) == nil {
		return raw + "\n\n", nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, element); err != nil {
		return "", err
	}
	return "data: " + compact.String() + "\n\n", nil
}

func writeScriptedEvents(w http.ResponseWriter, reply scriptedReply) {
	rendered := make([]string, 0, len(reply.Events))
	for i, element := range reply.Events {
		event, err := scriptedEvent(element)
		if err != nil {
			http.Error(w, fmt.Sprintf("openai: scripted event %d is not valid JSON", i), http.StatusBadRequest)
			return
		}
		rendered = append(rendered, event)
	}
	w.Header().Set("Content-Type", orElse(reply.ContentType, "text/event-stream"))
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	for _, event := range rendered {
		_, _ = w.Write([]byte(event))
		if flusher != nil {
			flusher.Flush()
		}
	}
}
