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
	"sort"
)

// Response is what an LLM response says about itself beyond its token usage, which
// llmusage reads from the provider template.
type Response struct {
	ID            string
	Model         string
	FinishReasons []string
}

// ParseResponse reads the response id, model and finish reasons from a JSON body or a
// server-sent event stream, in the shape of the provider the template names. The id
// and model come from the first event carrying them and the finish reasons from the
// last, which is where streaming providers put them.
//
// shortcut: a Bedrock ConverseStream body is a binary event stream and yields nothing;
// parse application/vnd.amazon.eventstream when Bedrock streaming spans need it.
func ParseResponse(templateName string, body []byte) Response {
	extract := extractorFor(templateName)
	var out Response
	reasons := map[int]string{}
	for _, event := range jsonEvents(body) {
		var doc map[string]any
		if json.Unmarshal(event, &doc) != nil {
			continue
		}
		id, model := extract(doc, reasons)
		if out.ID == "" {
			out.ID = id
		}
		if out.Model == "" {
			out.Model = model
		}
	}
	out.FinishReasons = orderedReasons(reasons)
	return out
}

// extractor reads one JSON document's id and model and records its finish reasons by
// choice index, later documents overwriting earlier ones.
type extractor func(doc map[string]any, reasons map[int]string) (id, model string)

func extractorFor(templateName string) extractor {
	switch templateName {
	case "anthropic":
		return anthropicResponse
	case "gemini":
		return geminiResponse
	case "awsbedrock":
		return bedrockResponse
	default:
		// OpenAI, Azure OpenAI, Azure AI Foundry, Mistral and OpenAI-compatible
		// templates share the chat-completions shape.
		return openAIResponse
	}
}

func openAIResponse(doc map[string]any, reasons map[int]string) (string, string) {
	for i, c := range list(doc["choices"]) {
		choice, _ := c.(map[string]any)
		idx := i
		if n, ok := choice["index"].(float64); ok {
			idx = int(n)
		}
		if r := str(choice["finish_reason"]); r != "" {
			reasons[idx] = r
		}
	}
	// The Responses API reports an object with a status rather than choices.
	if r := str(doc["status"]); r != "" && doc["object"] == "response" {
		reasons[0] = r
	}
	return str(doc["id"]), str(doc["model"])
}

func anthropicResponse(doc map[string]any, reasons map[int]string) (string, string) {
	id, model := str(doc["id"]), str(doc["model"])
	// Streaming: message_start carries the message, message_delta the stop reason.
	if msg, ok := doc["message"].(map[string]any); ok {
		id, model = str(msg["id"]), str(msg["model"])
	}
	if r := str(doc["stop_reason"]); r != "" {
		reasons[0] = r
	}
	if delta, ok := doc["delta"].(map[string]any); ok {
		if r := str(delta["stop_reason"]); r != "" {
			reasons[0] = r
		}
	}
	return id, model
}

func geminiResponse(doc map[string]any, reasons map[int]string) (string, string) {
	for i, c := range list(doc["candidates"]) {
		candidate, _ := c.(map[string]any)
		idx := i
		if n, ok := candidate["index"].(float64); ok {
			idx = int(n)
		}
		if r := str(candidate["finishReason"]); r != "" {
			reasons[idx] = r
		}
	}
	return str(doc["responseId"]), str(doc["modelVersion"])
}

func bedrockResponse(doc map[string]any, reasons map[int]string) (string, string) {
	if r := str(doc["stopReason"]); r != "" {
		reasons[0] = r
	}
	return "", ""
}

// jsonEvents splits a body into its JSON documents: the payload of every server-sent
// event, or the body itself when it is not a stream.
func jsonEvents(body []byte) [][]byte {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte("data:")) && !bytes.HasPrefix(trimmed, []byte("event:")) {
		return [][]byte{body}
	}
	var events [][]byte
	var data [][]byte
	flush := func() {
		if len(data) > 0 {
			events = append(events, bytes.Join(data, []byte("\n")))
			data = nil
		}
	}
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			flush()
			continue
		}
		if payload, ok := bytes.CutPrefix(line, []byte("data:")); ok {
			payload = bytes.TrimSpace(payload)
			if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
				data = append(data, payload)
			}
		}
	}
	flush()
	return events
}

func orderedReasons(byIndex map[int]string) []string {
	if len(byIndex) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(byIndex))
	for i := range byIndex {
		indexes = append(indexes, i)
	}
	sort.Ints(indexes)
	out := make([]string, len(indexes))
	for i, idx := range indexes {
		out[i] = byIndex[idx]
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func list(v any) []any {
	l, _ := v.([]any)
	return l
}
