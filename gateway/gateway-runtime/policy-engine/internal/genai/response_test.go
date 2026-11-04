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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseResponse(t *testing.T) {
	cases := []struct {
		name     string
		template string
		body     string
		want     Response
	}{
		{
			name:     "openai json",
			template: "openai",
			body:     `{"id":"chatcmpl-1","model":"gpt-4o-2024-08-06","choices":[{"index":0,"finish_reason":"stop"},{"index":1,"finish_reason":"length"}]}`,
			want:     Response{ID: "chatcmpl-1", Model: "gpt-4o-2024-08-06", FinishReasons: []string{"stop", "length"}},
		},
		{
			name:     "openai sse",
			template: "openai",
			body: "data: {\"id\":\"chatcmpl-2\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hi\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"id\":\"chatcmpl-2\",\"model\":\"gpt-4o-mini\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: {\"id\":\"chatcmpl-2\",\"choices\":[],\"usage\":{\"prompt_tokens\":10}}\n\n" +
				"data: [DONE]\n\n",
			want: Response{ID: "chatcmpl-2", Model: "gpt-4o-mini", FinishReasons: []string{"stop"}},
		},
		{
			name:     "mistral shares the openai shape",
			template: "mistralai",
			body:     `{"id":"m-1","model":"mistral-large","choices":[{"index":0,"finish_reason":"tool_calls"}]}`,
			want:     Response{ID: "m-1", Model: "mistral-large", FinishReasons: []string{"tool_calls"}},
		},
		{
			name:     "openai responses api",
			template: "openai",
			body:     `{"id":"resp_1","object":"response","model":"gpt-4.1","status":"completed"}`,
			want:     Response{ID: "resp_1", Model: "gpt-4.1", FinishReasons: []string{"completed"}},
		},
		{
			name:     "anthropic json",
			template: "anthropic",
			body:     `{"id":"msg_01","type":"message","model":"claude-sonnet-4-5","stop_reason":"end_turn"}`,
			want:     Response{ID: "msg_01", Model: "claude-sonnet-4-5", FinishReasons: []string{"end_turn"}},
		},
		{
			name:     "anthropic sse",
			template: "anthropic",
			body: "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_02\",\"model\":\"claude-haiku-4-5\",\"stop_reason\":null}}\n\n" +
				"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hi\"}}\n\n" +
				"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"},\"usage\":{\"output_tokens\":5}}\n\n" +
				"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n",
			want: Response{ID: "msg_02", Model: "claude-haiku-4-5", FinishReasons: []string{"max_tokens"}},
		},
		{
			name:     "gemini json",
			template: "gemini",
			body:     `{"responseId":"g-1","modelVersion":"gemini-2.0-flash-001","candidates":[{"finishReason":"STOP"}]}`,
			want:     Response{ID: "g-1", Model: "gemini-2.0-flash-001", FinishReasons: []string{"STOP"}},
		},
		{
			name:     "bedrock converse",
			template: "awsbedrock",
			body:     `{"output":{"message":{"role":"assistant"}},"stopReason":"end_turn"}`,
			want:     Response{FinishReasons: []string{"end_turn"}},
		},
		{
			name:     "not json",
			template: "openai",
			body:     `<html>bad gateway</html>`,
			want:     Response{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ParseResponse(tc.template, []byte(tc.body)))
		})
	}
}

// An event split over several data fields is joined before it is parsed.
func TestParseResponse_MultiLineEvent(t *testing.T) {
	body := "data: {\"id\":\"x\",\ndata: \"model\":\"m\"}\n\n"
	assert.Equal(t, Response{ID: "x", Model: "m"}, ParseResponse("openai", []byte(body)))
}
