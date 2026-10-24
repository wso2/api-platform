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
	"net/url"
	"regexp"
	"strings"
)

// ProviderName maps a WSO2 LLM provider template name onto the gen_ai.provider.name
// enum. An unrecognised provider yields "", so the attribute is left unset rather
// than carrying a value outside the enum.
func ProviderName(templateName string) string {
	switch strings.ToLower(templateName) {
	case "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	case "awsbedrock":
		return "aws.bedrock"
	case "azure-openai":
		return "azure.ai.openai"
	case "azureai-foundry":
		return "azure.ai.inference"
	case "gemini":
		return "gcp.gemini"
	case "mistralai":
		return "mistral_ai"
	default:
		return ""
	}
}

// OperationName derives gen_ai.operation.name from a provider route's resource path:
// "" when the path is not a recognised inference endpoint (a model listing, say), for
// which no inference span is recorded.
func OperationName(route string) string {
	r := strings.ToLower(route)
	switch {
	case strings.Contains(r, "/chat/completions"), strings.Contains(r, "/messages"),
		strings.Contains(r, "/converse"), strings.HasSuffix(r, "/responses"):
		// /converse covers Bedrock's Converse and ConverseStream; /responses is the
		// OpenAI Responses API, a chat interface.
		return OperationChat
	case strings.Contains(r, "/embeddings"):
		return OperationEmbeddings
	case strings.Contains(r, "generatecontent"):
		// generateContent and streamGenerateContent.
		return OperationGenerateContent
	case strings.Contains(r, "/completions"):
		return OperationTextCompletion
	default:
		return ""
	}
}

// geminiPathModel and bedrockPathModel match the model segment of the providers that
// name the model in the path rather than the body: Gemini (models/{model}:generateContent)
// and Bedrock (model/{model}/converse, where the model id itself may hold a colon).
var (
	geminiPathModel  = regexp.MustCompile(`(?:^|/)models/([^/:]+)`)
	bedrockPathModel = regexp.MustCompile(`(?:^|/)model/([^/]+)/`)
)

// ModelFromPath returns the model a request path names, or "".
func ModelFromPath(path string) string {
	for _, re := range []*regexp.Regexp{geminiPathModel, bedrockPathModel} {
		if m := re.FindStringSubmatch(path); m != nil {
			if decoded, err := url.PathUnescape(m[1]); err == nil {
				return decoded
			}
			return m[1]
		}
	}
	return ""
}
