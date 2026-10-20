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

// Package genai holds what the policy engine needs to record GenAI, MCP and A2A client
// spans: the attribute keys, and the parsing of provider and protocol payloads.
//
// Every key follows the OpenTelemetry GenAI and MCP semantic conventions pinned at
// open-telemetry/semantic-conventions-genai@06ec68e (schema gen-ai-dev/1.42.0-dev).
// All of them are at Development stability and no Go package publishes them after
// semconv/v1.41.0, so they live here, in one place, to move together when the
// conventions do.
package genai

// GenAI attributes.
const (
	AttrOperationName         = "gen_ai.operation.name"
	AttrProviderName          = "gen_ai.provider.name"
	AttrRequestModel          = "gen_ai.request.model"
	AttrRequestStream         = "gen_ai.request.stream"
	AttrResponseModel         = "gen_ai.response.model"
	AttrResponseID            = "gen_ai.response.id"
	AttrResponseFinishReasons = "gen_ai.response.finish_reasons"
	AttrResponseTTFC          = "gen_ai.response.time_to_first_chunk"
	AttrUsageInputTokens      = "gen_ai.usage.input_tokens"
	AttrUsageOutputTokens     = "gen_ai.usage.output_tokens"
	AttrUsageCacheReadTokens  = "gen_ai.usage.cache_read.input_tokens"
	AttrUsageCacheWriteTokens = "gen_ai.usage.cache_write.input_tokens"
	AttrUsageReasoningTokens  = "gen_ai.usage.reasoning.output_tokens"
	AttrInputMessages         = "gen_ai.input.messages"
	AttrOutputMessages        = "gen_ai.output.messages"
	AttrSystemInstructions    = "gen_ai.system_instructions"
	AttrToolName              = "gen_ai.tool.name"
	AttrToolCallArguments     = "gen_ai.tool.call.arguments"
	AttrToolCallResult        = "gen_ai.tool.call.result"
	AttrPromptName            = "gen_ai.prompt.name"
	AttrAgentName             = "gen_ai.agent.name"
	AttrAgentVersion          = "gen_ai.agent.version"
	AttrConversationID        = "gen_ai.conversation.id"
)

// MCP and JSON-RPC attributes.
const (
	AttrMCPMethodName      = "mcp.method.name"
	AttrMCPProtocolVersion = "mcp.protocol.version"
	AttrMCPSessionID       = "mcp.session.id"
	AttrMCPResourceURI     = "mcp.resource.uri"
	AttrJSONRPCRequestID   = "jsonrpc.request.id"
	AttrRPCStatusCode      = "rpc.response.status_code"
)

// General attributes the GenAI and MCP spans share with the HTTP conventions.
const (
	AttrErrorType           = "error.type"
	AttrServerAddress       = "server.address"
	AttrServerPort          = "server.port"
	AttrNetworkTransport    = "network.transport"
	AttrNetworkProtocolName = "network.protocol.name"
)

// Gateway facts no convention covers.
const (
	// AttrAIProxy names the LLM proxy a request came through, on the proxy route's
	// root span. The proxy route records no inference span itself: its upstream is
	// the provider route, which does, so tokens are counted once.
	AttrAIProxy = "wso2.ai.proxy"
	// AttrResponseTruncated marks a span whose response body was larger than the
	// engine reads, so usage and metadata may be missing.
	AttrResponseTruncated = "wso2.gen_ai.response.truncated"
	// AttrContentTruncated marks captured content cut to max_content_bytes.
	AttrContentTruncated = "wso2.gen_ai.content.truncated"
)

// Operation names (gen_ai.operation.name).
const (
	OperationChat            = "chat"
	OperationTextCompletion  = "text_completion"
	OperationEmbeddings      = "embeddings"
	OperationGenerateContent = "generate_content"
	OperationExecuteTool     = "execute_tool"
	OperationInvokeAgent     = "invoke_agent"
)

// ProviderA2A is gen_ai.provider.name for a remote A2A agent. The attribute is
// required, the agent's own provider is unknown to the gateway, and the conventions
// allow a custom value.
const ProviderA2A = "a2a"

// ErrorTypeOther is error.type when the failure has no more specific class.
const ErrorTypeOther = "_OTHER"

// ErrorTypeToolError is error.type for an MCP tool call whose result says isError.
const ErrorTypeToolError = "tool_error"
