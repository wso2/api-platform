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

func TestOperationName(t *testing.T) {
	for path, want := range map[string]string{
		"/chat/completions":                                     OperationChat,
		"/openai/deployments/gpt/chat/completions":              OperationChat,
		"/v1/messages":                                          OperationChat,
		"/model/anthropic.claude-v2:1/converse":                 OperationChat,
		"/model/anthropic.claude-v2:1/converse-stream":          OperationChat,
		"/v1/responses":                                         OperationChat,
		"/embeddings":                                           OperationEmbeddings,
		"/v1beta/models/gemini-2.0-flash:generateContent":       OperationGenerateContent,
		"/v1beta/models/gemini-2.0-flash:streamGenerateContent": OperationGenerateContent,
		"/completions":                                          OperationTextCompletion,
		"/models":                                               "",
		"/v1/responses/resp_1/input_items":                      "",
	} {
		assert.Equal(t, want, OperationName(path), "path %s", path)
	}
}

func TestProviderName(t *testing.T) {
	assert.Equal(t, "aws.bedrock", ProviderName("awsbedrock"))
	assert.Equal(t, "gcp.gemini", ProviderName("Gemini"))
	assert.Empty(t, ProviderName("my-custom-template"), "an unknown provider leaves the enum unset")
}

func TestModelFromPath(t *testing.T) {
	assert.Equal(t, "gemini-2.0-flash", ModelFromPath("/v1beta/models/gemini-2.0-flash:generateContent"))
	assert.Equal(t, "anthropic.claude-3-sonnet-20240229-v1:0", ModelFromPath("/model/anthropic.claude-3-sonnet-20240229-v1:0/converse"))
	assert.Equal(t, "anthropic.claude-3-sonnet-20240229-v1:0", ModelFromPath("/model/anthropic.claude-3-sonnet-20240229-v1%3A0/converse"))
	assert.Empty(t, ModelFromPath("/chat/completions"))
}
