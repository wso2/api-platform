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

package kernel

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/wso2/api-platform/gateway/common/agentproto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/genai"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/resolver"
)

// startAgentSpan starts "invoke_agent {agent}" for a message sent to an A2A agent. Only
// the sending operations invoke the agent; reading or cancelling a task does not.
//
// The agent is the API the route serves: its name is the agent's display name and its
// version the agent's, as the A2A ingest maps them. The A2A context id is the
// conversation, and the message and task ids, kept off the ext_proc spans (#3737), are
// recorded here, on the one span of the call.
func (ec *PolicyExecutionContext) startAgentSpan() {
	switch agentproto.Operation(ec.operation) {
	case agentproto.SendMessage, agentproto.SendStreamingMessage:
	default:
		return
	}
	facts := ec.resolutionAttributes
	agent := ec.sharedCtx.APIName
	attrs := []attribute.KeyValue{
		attribute.String(genai.AttrOperationName, genai.OperationInvokeAgent),
		attribute.String(genai.AttrProviderName, genai.ProviderA2A),
	}
	if agent != "" {
		attrs = append(attrs, attribute.String(genai.AttrAgentName, agent))
	}
	if v := ec.sharedCtx.APIVersion; v != "" {
		attrs = append(attrs, attribute.String(genai.AttrAgentVersion, v))
	}
	if v := facts[resolver.AttrA2AContextID]; v != "" {
		attrs = append(attrs, attribute.String(genai.AttrConversationID, v))
	}
	for _, key := range []string{
		resolver.AttrA2AOperation, resolver.AttrA2ATransport, resolver.AttrA2AProtocolVersion,
		resolver.AttrA2AMessageID, resolver.AttrA2ATaskID,
	} {
		if v := facts[key]; v != "" {
			attrs = append(attrs, attribute.String(key, v))
		}
	}
	attrs = append(attrs, ec.serverAttributes()...)

	name := genai.OperationInvokeAgent
	if agent != "" {
		name += " " + agent
	}
	ec.startClientSpan(name, attrs...)
}

// finishAgentSpan records a JSON-RPC error the agent answered with, when a policy had the
// response read; the engine does not read an agent's response for the span itself.
func (ec *PolicyExecutionContext) finishAgentSpan(span trace.Span) {
	if ec.genai.response == nil {
		return
	}
	if outcome, ok := genai.ParseJSONRPCResponse(ec.genai.response.Bytes()); ok && outcome.ErrorCode != "" {
		span.SetAttributes(
			attribute.String(genai.AttrErrorType, outcome.ErrorCode),
			attribute.String(genai.AttrRPCStatusCode, outcome.ErrorCode),
		)
		span.SetStatus(codes.Error, "")
	}
}
