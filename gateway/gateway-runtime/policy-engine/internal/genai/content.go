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
	"sort"
	"strings"
	"unicode/utf8"
)

// Content capture converts provider payloads into the OpenTelemetry message schema
// (model/gen-ai/gen-ai-{input,output}-messages.json and gen-ai-system-instructions.json
// at the pinned commit): a message is a role and a list of typed parts. Two payload
// shapes are converted, which between them cover most traffic: the OpenAI chat
// completions shape (OpenAI, Azure OpenAI, Azure AI Foundry, Mistral and the
// OpenAI-compatible APIs) and Anthropic Messages. Other providers record no content.

// Message is one message of the conversation.
type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
	Name  string `json:"name,omitempty"`
}

// Part is one typed part of a message: text, tool_call, tool_call_response, reasoning,
// uri, or a generic part naming only its type.
type Part map[string]any

// Shape names the payload shape a provider template speaks, "" for one not converted.
func Shape(templateName string) string {
	switch templateName {
	case "openai", "azure-openai", "azureai-foundry", "mistralai":
		return shapeOpenAI
	case "anthropic":
		return shapeAnthropic
	}
	return ""
}

const (
	shapeOpenAI    = "openai"
	shapeAnthropic = "anthropic"
)

// RequestContent converts a chat request into its input messages and system
// instructions. ok is false for a shape that is not converted or a body that does not
// parse.
func RequestContent(shape string, body []byte) (input []Message, system []Part, ok bool) {
	switch shape {
	case shapeOpenAI:
		var req struct {
			Messages []openAIMessage `json:"messages"`
		}
		if json.Unmarshal(body, &req) != nil {
			return nil, nil, false
		}
		for _, m := range req.Messages {
			input = append(input, m.toMessage())
		}
		return input, nil, true
	case shapeAnthropic:
		var req struct {
			System   json.RawMessage    `json:"system"`
			Messages []anthropicMessage `json:"messages"`
		}
		if json.Unmarshal(body, &req) != nil {
			return nil, nil, false
		}
		system = anthropicBlocks(req.System)
		for _, m := range req.Messages {
			input = append(input, Message{Role: m.Role, Parts: anthropicBlocks(m.Content)})
		}
		return input, system, true
	}
	return nil, nil, false
}

// ResponseContent converts a chat response, JSON or a server-sent event stream, into
// its output messages.
func ResponseContent(shape string, body []byte) ([]Message, bool) {
	events := jsonEvents(body)
	streamed := isEventStream(body)
	switch shape {
	case shapeOpenAI:
		if streamed {
			return openAIStream(events), true
		}
		var resp struct {
			Choices []struct {
				Message openAIMessage `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(body, &resp) != nil {
			return nil, false
		}
		out := make([]Message, 0, len(resp.Choices))
		for _, c := range resp.Choices {
			out = append(out, c.Message.toMessage())
		}
		return out, true
	case shapeAnthropic:
		if streamed {
			return anthropicStream(events), true
		}
		var resp anthropicMessage
		if json.Unmarshal(body, &resp) != nil {
			return nil, false
		}
		return []Message{{Role: resp.Role, Parts: anthropicBlocks(resp.Content)}}, true
	}
	return nil, false
}

// ─── OpenAI chat completions ─────────────────────────────────────────────────

type openAIMessage struct {
	Role       string           `json:"role"`
	Name       string           `json:"name"`
	Content    json.RawMessage  `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls"`
	ToolCallID string           `json:"tool_call_id"`
}

type openAIToolCall struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (m openAIMessage) toMessage() Message {
	msg := Message{Role: m.Role, Name: m.Name}
	if m.Role == "tool" {
		msg.Parts = []Part{{"type": "tool_call_response", "id": m.ToolCallID, "response": textOf(m.Content)}}
		return msg
	}
	msg.Parts = openAIContent(m.Content)
	for _, tc := range m.ToolCalls {
		msg.Parts = append(msg.Parts, toolCallPart(tc.ID, tc.Function.Name, tc.Function.Arguments))
	}
	if msg.Parts == nil {
		msg.Parts = []Part{}
	}
	return msg
}

// openAIContent converts message content: a string, or a list of typed parts.
func openAIContent(raw json.RawMessage) []Part {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s == "" {
			return nil
		}
		return []Part{textPart(s)}
	}
	var items []map[string]any
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	var parts []Part
	for _, item := range items {
		switch t := str(item["type"]); t {
		case "text":
			parts = append(parts, textPart(str(item["text"])))
		case "image_url":
			url := ""
			if img, ok := item["image_url"].(map[string]any); ok {
				url = str(img["url"])
			}
			parts = append(parts, mediaPart("image", url))
		default:
			parts = append(parts, Part{"type": t})
		}
	}
	return parts
}

// openAIStream assembles each choice's message from its deltas: text concatenated,
// tool calls assembled by their index.
func openAIStream(events [][]byte) []Message {
	type choice struct {
		role  string
		text  strings.Builder
		calls map[int]*openAIToolCall
	}
	choices := map[int]*choice{}
	for _, event := range events {
		var chunk struct {
			Choices []struct {
				Index int           `json:"index"`
				Delta openAIMessage `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal(event, &chunk) != nil {
			continue
		}
		for _, c := range chunk.Choices {
			ch := choices[c.Index]
			if ch == nil {
				ch = &choice{calls: map[int]*openAIToolCall{}}
				choices[c.Index] = ch
			}
			if c.Delta.Role != "" {
				ch.role = c.Delta.Role
			}
			ch.text.WriteString(textOf(c.Delta.Content))
			for _, tc := range c.Delta.ToolCalls {
				call := ch.calls[tc.Index]
				if call == nil {
					call = &openAIToolCall{Index: tc.Index}
					ch.calls[tc.Index] = call
				}
				if tc.ID != "" {
					call.ID = tc.ID
				}
				if tc.Function.Name != "" {
					call.Function.Name = tc.Function.Name
				}
				call.Function.Arguments += tc.Function.Arguments
			}
		}
	}
	out := make([]Message, 0, len(choices))
	for _, idx := range sortedKeys(choices) {
		ch := choices[idx]
		role := ch.role
		if role == "" {
			role = "assistant"
		}
		parts := []Part{}
		if ch.text.Len() > 0 {
			parts = append(parts, textPart(ch.text.String()))
		}
		for _, i := range sortedKeys(ch.calls) {
			c := ch.calls[i]
			parts = append(parts, toolCallPart(c.ID, c.Function.Name, c.Function.Arguments))
		}
		out = append(out, Message{Role: role, Parts: parts})
	}
	return out
}

// ─── Anthropic Messages ──────────────────────────────────────────────────────

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// anthropicBlocks converts content: a string, or a list of content blocks.
func anthropicBlocks(raw json.RawMessage) []Part {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		if s == "" {
			return []Part{}
		}
		return []Part{textPart(s)}
	}
	var blocks []map[string]any
	if json.Unmarshal(raw, &blocks) != nil {
		return []Part{}
	}
	parts := []Part{}
	for _, b := range blocks {
		parts = append(parts, anthropicBlock(b))
	}
	return parts
}

func anthropicBlock(b map[string]any) Part {
	switch t := str(b["type"]); t {
	case "text":
		return textPart(str(b["text"]))
	case "thinking":
		return Part{"type": "reasoning", "content": str(b["thinking"])}
	case "tool_use":
		return Part{"type": "tool_call", "id": str(b["id"]), "name": str(b["name"]), "arguments": b["input"]}
	case "tool_result":
		var response any = b["content"]
		if blocks, ok := b["content"].([]any); ok {
			var text strings.Builder
			for _, x := range blocks {
				if m, ok := x.(map[string]any); ok && str(m["type"]) == "text" {
					text.WriteString(str(m["text"]))
				}
			}
			response = text.String()
		}
		return Part{"type": "tool_call_response", "id": str(b["tool_use_id"]), "response": response}
	case "image":
		url := ""
		if src, ok := b["source"].(map[string]any); ok && str(src["type"]) == "url" {
			url = str(src["url"])
		}
		return mediaPart("image", url)
	default:
		return Part{"type": t}
	}
}

// anthropicStream assembles the message from content_block_start and the deltas that
// follow it, block by block.
func anthropicStream(events [][]byte) []Message {
	type block struct {
		part Part
		text strings.Builder
		json strings.Builder
	}
	blocks := map[int]*block{}
	for _, event := range events {
		var e struct {
			Type         string         `json:"type"`
			Index        int            `json:"index"`
			ContentBlock map[string]any `json:"content_block"`
			Delta        map[string]any `json:"delta"`
		}
		if json.Unmarshal(event, &e) != nil {
			continue
		}
		switch e.Type {
		case "content_block_start":
			blocks[e.Index] = &block{part: anthropicBlock(e.ContentBlock)}
		case "content_block_delta":
			b := blocks[e.Index]
			if b == nil {
				continue
			}
			switch str(e.Delta["type"]) {
			case "text_delta":
				b.text.WriteString(str(e.Delta["text"]))
			case "thinking_delta":
				b.text.WriteString(str(e.Delta["thinking"]))
			case "input_json_delta":
				b.json.WriteString(str(e.Delta["partial_json"]))
			}
		}
	}
	parts := []Part{}
	for _, idx := range sortedKeys(blocks) {
		b := blocks[idx]
		if b.text.Len() > 0 {
			b.part["content"] = str(b.part["content"]) + b.text.String()
		}
		if b.json.Len() > 0 {
			b.part["arguments"] = jsonOrString(b.json.String())
		}
		parts = append(parts, b.part)
	}
	return []Message{{Role: "assistant", Parts: parts}}
}

// ─── Parts ───────────────────────────────────────────────────────────────────

func textPart(s string) Part { return Part{"type": "text", "content": s} }

// toolCallPart records a tool call; its arguments arrive as a JSON string and are kept
// as JSON when they parse.
func toolCallPart(id, name, arguments string) Part {
	p := Part{"type": "tool_call", "name": name}
	if id != "" {
		p["id"] = id
	}
	if arguments != "" {
		p["arguments"] = jsonOrString(arguments)
	}
	return p
}

// mediaPart records media by its URL. Inline data (a data: URL or base64) is not
// copied onto the span; the part only says media was there.
func mediaPart(modality, url string) Part {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return Part{"type": "uri", "modality": modality, "uri": url}
	}
	return Part{"type": modality}
}

func jsonOrString(s string) any {
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	return s
}

// textOf reads content that is a string, or the text parts of a list.
func textOf(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var b strings.Builder
	for _, p := range openAIContent(raw) {
		if p["type"] == "text" {
			b.WriteString(str(p["content"]))
		}
	}
	return b.String()
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// ─── Size budget ─────────────────────────────────────────────────────────────

// cuttable are the part members holding caller content, cut first when an attribute is
// over its budget.
var cuttable = []string{"content", "arguments", "response"}

// Encode returns v as JSON of at most limit bytes, and whether anything was cut. The
// result is always valid JSON: content members are cut at a UTF-8 boundary, in order,
// and parts and messages are dropped from the end once nothing is left to cut.
func Encode[T Message | Part](items []T, limit int) (string, bool) {
	out, err := encode(items)
	if err != nil {
		return "[]", true
	}
	if len(out) <= limit {
		return string(out), false
	}
	parts := allParts(items)
	for budget := limit; budget > 0; {
		cutContent(parts, budget)
		out, _ = encode(items)
		if len(out) <= limit {
			return string(out), true
		}
		budget -= len(out) - limit
	}
	// Structure alone is over the limit: drop parts, then messages, from the end.
	for len(items) > 0 {
		items = items[:len(items)-1]
		if out, _ = encode(items); len(out) <= limit {
			return string(out), true
		}
	}
	return "[]", true
}

func allParts[T Message | Part](items []T) []Part {
	var parts []Part
	for _, it := range items {
		switch v := any(it).(type) {
		case Message:
			parts = append(parts, v.Parts...)
		case Part:
			parts = append(parts, v)
		}
	}
	return parts
}

// cutContent spends budget bytes on the parts' content in order, cutting what does not
// fit and emptying what comes after.
func cutContent(parts []Part, budget int) {
	for _, p := range parts {
		for _, key := range cuttable {
			v, ok := p[key]
			if !ok {
				continue
			}
			s, isString := v.(string)
			if !isString {
				b, _ := encode(v)
				s = string(b)
			}
			if len(s) <= budget {
				budget -= len(s)
				continue
			}
			p[key] = truncateUTF8(s, budget)
			budget = 0
		}
	}
}

// truncateUTF8 cuts s to at most n bytes without splitting a character.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
