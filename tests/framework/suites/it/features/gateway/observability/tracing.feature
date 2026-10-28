# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@tracing
Feature: Request tracing
  As an operator
  I want each request through the gateway to produce one trace with spans for the
  policies it ran and the LLM, MCP and agent calls it made
  So that I can see where time went and what each AI call cost

  # The block exports over OTLP/gRPC with content capture on and analytics off, so the engine
  # reads AI bodies for the spans alone. Each scenario starts its own trace and reads back only
  # the spans of that trace.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A REST request forms one trace under the caller's, with policy spans nested in their phase
    Given I generate a unique resource name from "tracing-rest" and store it as "apiName"
    And I generate a unique API version from "tracing-rest" and store it as "apiVersion"
    And I generate a unique API context from "/tracing-rest" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                             |
      | name                   | ${CTX:apiName}                                                                                                                                        |
      | spec.displayName       | ${CTX:apiName}                                                                                                                                        |
      | spec.version           | ${CTX:apiVersion}                                                                                                                                     |
      | spec.context           | ${CTX:apiContext}/$version                                                                                                                            |
      | spec.upstream.main.url | http://testbench:3002                                                                                                                                 |
      | spec.operations        | [{"method":"GET","path":"/anything","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-traced","value":"yes"}]}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 200

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything"
    Then the response status code should be 200
    And the trace should have 1 span named "policy.request.set-headers"
    And the span "external_processing.process" should be a "SERVER" span
    And the span "external_processing.process" should be a child of a span from service "router"
    And the span "external_processing.process_request_headers" should be a child of the span "external_processing.process"
    And the span "policy.request.set-headers" should be a child of the span "external_processing.process_request_headers"
    And the span "policy.request.set-headers" should have attribute "wso2.policy.action" with value "modify"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An LLM provider call is a chat client span with its token usage
    Given I generate a unique resource name from "tracing-openai" and store it as "providerName"
    And I generate a unique value from "tracing-openai" and store it as "providerDisplayName"
    And I generate a unique API version from "tracing-openai" and store it as "providerVersion"
    And I generate a unique API context from "/tracing-openai" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerDisplayName}      |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello"}]}
      """

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"Hello, how are you?"}]}
      """
    Then the response status code should be 200
    And the trace should have 1 span named "chat gpt-4o"
    And the span "chat gpt-4o" should be a "CLIENT" span
    And the span "chat gpt-4o" should be a child of the span "external_processing.process"
    And the span "chat gpt-4o" should have attribute "gen_ai.operation.name" with value "chat"
    And the span "chat gpt-4o" should have attribute "gen_ai.provider.name" with value "openai"
    And the span "chat gpt-4o" should have attribute "gen_ai.request.model" with value "gpt-4o"
    And the span "chat gpt-4o" should have attribute "gen_ai.response.model" with value "gpt-4o"
    And the span "chat gpt-4o" should have attribute "gen_ai.response.id" with value "chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT"
    And the span "chat gpt-4o" attribute "gen_ai.response.finish_reasons" should contain "stop"
    And the span "chat gpt-4o" should have attribute "gen_ai.usage.input_tokens" with value "19"
    And the span "chat gpt-4o" should have attribute "gen_ai.usage.output_tokens" with value "10"
    And the span "chat gpt-4o" should have attribute "server.address" with value "testbench"
    And the span "chat gpt-4o" should have attribute "server.port" with value "3008"
    And the span "chat gpt-4o" should not have attribute "gen_ai.request.stream"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  # The mock writes each event on its own when the prompt carries STREAM_PACED, so the gateway
  # receives the stream as many body chunks.
  Scenario: A streamed LLM response streams through untouched and is one span per body
    Given I generate a unique resource name from "tracing-stream" and store it as "providerName"
    And I generate a unique value from "tracing-stream" and store it as "providerDisplayName"
    And I generate a unique API version from "tracing-stream" and store it as "providerVersion"
    And I generate a unique API context from "/tracing-stream" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerDisplayName}      |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}
      """

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4o-mini","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"Plan a trip STREAM_PACED"}]}
      """
    Then the response status code should be 200
    And the response body should contain "This "
    And the response body should contain "data: [DONE]"
    And the trace should have 1 span named "chat gpt-4o-mini"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.request.stream" with value "true"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.usage.input_tokens" with value "10"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.usage.output_tokens" with value "9"
    And the span "chat gpt-4o-mini" attribute "gen_ai.response.finish_reasons" should contain "stop"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.response.time_to_first_chunk"
    And the trace should have 1 span named "external_processing.process_response_body"
    And the span "external_processing.process_response_body" attribute "stream.chunk_count" should be greater than "1"
    # One await span per phase change (request headers to body, to response headers, to the first
    # chunk), none between the chunks of the stream.
    And the trace should have 3 spans named "external_processing.await_next_message"
    And the span "chat gpt-4o-mini" attribute "gen_ai.input.messages" should contain "Plan a trip"
    And the span "chat gpt-4o-mini" attribute "gen_ai.output.messages" should contain "This is a streamed reply from the mock model."

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: A request body over the buffer limit is still forwarded when only the span reads it
    Given I generate a unique resource name from "tracing-large" and store it as "providerName"
    And I generate a unique value from "tracing-large" and store it as "providerDisplayName"
    And I generate a unique API version from "tracing-large" and store it as "providerVersion"
    And I generate a unique API context from "/tracing-large" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerDisplayName}      |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}
      """

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with a chat body of 1572864 bytes
    Then the response status code should be 200
    # Envoy's buffer limit is a watermark: depending on how the body arrives, the engine gets all
    # of it or only the first part, and then cannot name the model. Either way the request is
    # forwarded and the call is traced.
    And a span with attribute "gen_ai.operation.name" with value "chat" should have attribute "gen_ai.usage.input_tokens" with value "19"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: A guardrail denial is recorded on its policy span and makes no LLM call
    Given I generate a unique resource name from "tracing-guarded" and store it as "providerName"
    And I generate a unique value from "tracing-guarded" and store it as "providerDisplayName"
    And I generate a unique API version from "tracing-guarded" and store it as "providerVersion"
    And I generate a unique API context from "/tracing-guarded" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                            |
      | name                   | ${CTX:providerName}                                                                                                                                  |
      | displayName            | ${CTX:providerDisplayName}                                                                                                                           |
      | version                | ${CTX:providerVersion}                                                                                                                               |
      | template               | openai                                                                                                                                               |
      | spec.context           | ${CTX:providerContext}                                                                                                                               |
      | spec.upstream.url      | http://testbench:3008/openai/v1                                                                                                                      |
      | accessControl.mode     | allow_all                                                                                                                                            |
      | spec.operationPolicies | [{"name":"content-length-guardrail","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"request":{"min":1,"max":20,"jsonPath":""}}}]}] |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 422 with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"this prompt is far longer than twenty bytes"}]}
      """

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"this prompt is far longer than twenty bytes"}]}
      """
    Then the response status code should be 422
    # The header phase also spans the policy, with no callback to run; the body phase denied.
    And the span "policy.request.content-length-guardrail" should have attribute "wso2.policy.action" with value "deny"
    And the span "policy.request.content-length-guardrail" should have attribute "wso2.guardrail.name" with value "content-length-guardrail"
    And the trace should have 0 spans named "chat gpt-4o-mini"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: A call through an LLM proxy records one inference span, on the provider route
    Given I generate a unique resource name from "tracing-proxied" and store it as "providerName"
    And I generate a unique value from "tracing-proxied" and store it as "providerDisplayName"
    And I generate a unique API version from "tracing-proxied" and store it as "providerVersion"
    And I generate a unique API context from "/tracing-proxied" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerDisplayName}      |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response should be successful

    Given I generate a unique resource name from "tracing-proxy" and store it as "proxyName"
    And I generate a unique value from "tracing-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "tracing-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/tracing-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:proxyName}          |
      | displayName | ${CTX:proxyDisplayName}   |
      | version     | ${CTX:proxyVersion}       |
      | context     | ${CTX:proxyContext}       |
      | provider.id | ${CTX:providerName}       |
    Then the response should be successful
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}
      """

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the trace should have 2 spans named "external_processing.process"
    And the trace should have 1 span named "chat gpt-4o-mini"
    And the span "external_processing.process" should have attribute "wso2.ai.proxy"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  Scenario: An MCP tool call is a tool span, and the server accepts the rewritten _meta
    Given I generate a unique resource name from "tracing-mcp" and store it as "mcpName"
    And I generate a unique value from "tracing-mcp" and store it as "mcpDisplayName"
    And I generate a unique API version from "tracing-mcp" and store it as "mcpVersion"
    And I generate a unique API context from "/tracing-mcp" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                          |
      | name              | ${CTX:mcpName}                                     |
      | displayName       | ${CTX:mcpDisplayName}                              |
      | version           | ${CTX:mcpVersion}                                  |
      | context           | ${CTX:mcpContext}                                  |
      | specVersion       | 2025-06-18                                         |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
    And the span "tools/call add" should be a "CLIENT" span
    And the span "tools/call add" should be a child of the span "external_processing.process"
    And the span "tools/call add" should have attribute "mcp.method.name" with value "tools/call"
    And the span "tools/call add" should have attribute "gen_ai.operation.name" with value "execute_tool"
    And the span "tools/call add" should have attribute "gen_ai.tool.name" with value "add"
    And the span "tools/call add" should have attribute "jsonrpc.request.id"
    And the span "tools/call add" attribute "gen_ai.tool.call.arguments" should contain "40"
    And no span in the trace should have attribute "mcp.body.capability.name"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: A message sent to an A2A agent is an invoke_agent span
    Given I generate a unique resource name from "tracing-agent" and store it as "agentName"
    And I generate a unique API context from "/tracing-agent" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                       |
      | name               | ${CTX:agentName}                                |
      | displayName        | Tracing Agent                                   |
      | context            | ${CTX:agentContext}                             |
      | upstreamUrl        | http://a2a-trip-planner:9099                    |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}               |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json" until status 200

    Given I start a new trace
    And I set header "traceparent" to "${CTX:traceparent}"
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "SendMessage", "params": {"message": {"messageId": "trace-msg-1", "contextId": "trace-ctx-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 2 day trip to Galle"}]}}}
      """
    Then the response status code should be 200
    And the span "invoke_agent Tracing Agent" should be a "CLIENT" span
    And the span "invoke_agent Tracing Agent" should have attribute "gen_ai.operation.name" with value "invoke_agent"
    And the span "invoke_agent Tracing Agent" should have attribute "gen_ai.provider.name" with value "a2a"
    And the span "invoke_agent Tracing Agent" should have attribute "gen_ai.conversation.id" with value "trace-ctx-1"
    And the span "invoke_agent Tracing Agent" should have attribute "a2a.message.id" with value "trace-msg-1"
    And the span "external_processing.process" should not have attribute "a2a.message.id"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
