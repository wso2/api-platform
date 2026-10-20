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
//
// Gemini's native API names the method after a colon on the model
// (/models/{model}:generateContent). Bedrock's Converse API (/model/{id}/converse,
// /converse-stream) is chat; its /invoke serves chat and embedding models alike, so
// it is left unclassified. OpenAI's Responses API (/responses) is chat. Every endpoint
// is matched on the route's suffix, so the API context, sub-resources
// (/messages/count_tokens), reads of a stored result (/responses/{id}) and lookalike
// routes are not classified. Trailing slashes are ignored.
func OperationName(route string) string {
	r := strings.TrimRight(strings.ToLower(route), "/")
	hasSuffix := func(suffixes ...string) bool {
		for _, s := range suffixes {
			if strings.HasSuffix(r, s) {
				return true
			}
		}
		return false
	}
	switch {
	case hasSuffix("/chat/completions", "/messages", "/converse", "/converse-stream", "/responses"):
		return OperationChat
	case hasSuffix("/embeddings", ":embedcontent", ":batchembedcontents"):
		return OperationEmbeddings
	case hasSuffix("/generatecontent", ":generatecontent", ":streamgeneratecontent"):
		return OperationGenerateContent
	case hasSuffix("/completions"):
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
