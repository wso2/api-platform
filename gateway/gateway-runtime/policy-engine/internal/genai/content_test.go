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
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encoded(t *testing.T, v any) string {
	t.Helper()
	b, err := encode(v)
	require.NoError(t, err)
	return string(b)
}

func TestShape(t *testing.T) {
	assert.Equal(t, shapeOpenAI, Shape("azure-openai"))
	assert.Equal(t, shapeOpenAI, Shape("mistralai"))
	assert.Equal(t, shapeAnthropic, Shape("anthropic"))
	assert.Empty(t, Shape("gemini"), "other providers record no content")
	assert.Empty(t, Shape("awsbedrock"))
}

func TestRequestContent_OpenAI(t *testing.T) {
	body := `{"model":"gpt-4o","messages":[
		{"role":"system","content":"Be brief."},
		{"role":"user","content":[{"type":"text","text":"What is in"},{"type":"image_url","image_url":{"url":"https://x/cat.png"}},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]},
		{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Colombo\"}"}}]},
		{"role":"tool","tool_call_id":"call_1","content":"sunny"}
	]}`
	input, system, ok := RequestContent(shapeOpenAI, []byte(body))
	require.True(t, ok)
	assert.Nil(t, system, "OpenAI system messages stay in the conversation")
	assert.JSONEq(t, `[
		{"role":"system","parts":[{"type":"text","content":"Be brief."}]},
		{"role":"user","parts":[{"type":"text","content":"What is in"},{"type":"uri","modality":"image","uri":"https://x/cat.png"},{"type":"image"}]},
		{"role":"assistant","parts":[{"type":"tool_call","id":"call_1","name":"get_weather","arguments":{"city":"Colombo"}}]},
		{"role":"tool","parts":[{"type":"tool_call_response","id":"call_1","response":"sunny"}]}
	]`, encoded(t, input))
}

func TestRequestContent_Anthropic(t *testing.T) {
	body := `{"model":"claude","system":[{"type":"text","text":"Be brief."}],"messages":[
		{"role":"user","content":"Weather?"},
		{"role":"assistant","content":[{"type":"thinking","thinking":"use the tool"},{"type":"tool_use","id":"tu_1","name":"get_weather","input":{"city":"Colombo"}}]},
		{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu_1","content":[{"type":"text","text":"sunny"}]}]}
	]}`
	input, system, ok := RequestContent(shapeAnthropic, []byte(body))
	require.True(t, ok)
	assert.JSONEq(t, `[{"type":"text","content":"Be brief."}]`, encoded(t, system), "Anthropic's system prompt is the system instructions")
	assert.JSONEq(t, `[
		{"role":"user","parts":[{"type":"text","content":"Weather?"}]},
		{"role":"assistant","parts":[{"type":"reasoning","content":"use the tool"},{"type":"tool_call","id":"tu_1","name":"get_weather","arguments":{"city":"Colombo"}}]},
		{"role":"user","parts":[{"type":"tool_call_response","id":"tu_1","response":"sunny"}]}
	]`, encoded(t, input))
}

func TestResponseContent_OpenAI(t *testing.T) {
	out, ok := ResponseContent(shapeOpenAI, []byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"Hello"}}]}`))
	require.True(t, ok)
	assert.JSONEq(t, `[{"role":"assistant","parts":[{"type":"text","content":"Hello"}]}]`, encoded(t, out))

	stream := "data: {\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hel\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"f\",\"arguments\":\"{\\\"a\\\":\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"1}\"}}]}}]}\n\n" +
		"data: [DONE]\n\n"
	out, ok = ResponseContent(shapeOpenAI, []byte(stream))
	require.True(t, ok)
	assert.JSONEq(t, `[{"role":"assistant","parts":[{"type":"text","content":"Hello"},{"type":"tool_call","id":"call_1","name":"f","arguments":{"a":1}}]}]`, encoded(t, out))
}

func TestResponseContent_Anthropic(t *testing.T) {
	out, ok := ResponseContent(shapeAnthropic, []byte(`{"role":"assistant","content":[{"type":"text","text":"Hi"}],"stop_reason":"end_turn"}`))
	require.True(t, ok)
	assert.JSONEq(t, `[{"role":"assistant","parts":[{"type":"text","content":"Hi"}]}]`, encoded(t, out))

	stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"role\":\"assistant\"}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tu_1\",\"name\":\"f\",\"input\":{}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"a\\\":1}\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	out, ok = ResponseContent(shapeAnthropic, []byte(stream))
	require.True(t, ok)
	assert.JSONEq(t, `[{"role":"assistant","parts":[{"type":"text","content":"Hello"},{"type":"tool_call","id":"tu_1","name":"f","arguments":{"a":1}}]}]`, encoded(t, out))
}

func TestContent_UnconvertedShapes(t *testing.T) {
	_, _, ok := RequestContent("", []byte(`{"contents":[]}`))
	assert.False(t, ok)
	_, ok = ResponseContent(shapeOpenAI, []byte(`not json`))
	assert.False(t, ok)
}

// An attribute over its budget is cut at a character boundary and stays valid JSON.
func TestEncode_BudgetUTF8BoundaryAndValidJSON(t *testing.T) {
	messages := []Message{
		{Role: "user", Parts: []Part{textPart(strings.Repeat("é", 200))}},
		{Role: "assistant", Parts: []Part{toolCallPart("c", "f", `{"q":"`+strings.Repeat("x", 300)+`"}`)}},
	}
	got, cut := Encode(messages, 256)
	assert.True(t, cut)
	assert.LessOrEqual(t, len(got), 256)
	assert.True(t, json.Valid([]byte(got)), "cut content must leave valid JSON: %s", got)
	assert.True(t, utf8.ValidString(got))

	small, cut := Encode([]Message{{Role: "user", Parts: []Part{textPart("hi")}}}, 256)
	assert.False(t, cut)
	assert.JSONEq(t, `[{"role":"user","parts":[{"type":"text","content":"hi"}]}]`, small)

	tiny, cut := Encode(messages, 8)
	assert.True(t, cut)
	assert.LessOrEqual(t, len(tiny), 8)
	assert.True(t, json.Valid([]byte(tiny)))
}
