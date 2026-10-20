# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Runs only with tracing on: cd gateway/it && make test-tracing (see
# docker-compose.test.tracing.override.yaml). Analytics is off in that run, so the bodies
# the policy engine reads for GenAI spans are read for the spans alone.
Feature: Request tracing
  As an operator
  I want each request through the gateway to produce one trace with spans for the
  policies it ran and the LLM, MCP and agent calls it made
  So that I can see where time went and what each AI call cost

  Background:
    Given the gateway services are running
    And I reset the trace collector

  Scenario: A REST request forms one trace under the caller's, with policy spans nested in their phase
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: tracing-rest-api
      spec:
        displayName: Tracing REST API
        version: v1.0
        context: /tracing-rest/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
            policies:
              - name: set-headers
                version: v1
                params:
                  request:
                    headers:
                      - name: x-traced
                        value: "yes"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/tracing-rest/v1.0/health" to be ready
    And I reset the trace collector

    When I send the following requests in a new trace
    And I send a GET request to "http://localhost:8080/tracing-rest/v1.0/health"
    Then the response status code should be 200
    And I wait for the span "policy.request.set-headers"
    And the trace should be the one the requests sent
    And the span "external_processing.process" should be a "SERVER" span
    And the span "external_processing.process" should be a child of a span from service "router"
    And the span "external_processing.process_request_headers" should be a child of the span "external_processing.process"
    And the span "policy.request.set-headers" should be a child of the span "external_processing.process_request_headers"
    And the span "policy.request.set-headers" should have attribute "wso2.policy.action" with value "modify"

    Given I authenticate using basic auth as "admin"
    When I delete the API "tracing-rest-api"
    Then the response should be successful

  Scenario: An LLM provider call is a chat client span with its token usage
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tracing-openai
      spec:
        displayName: Tracing OpenAI
        version: v1.0
        template: openai
        context: /tracing-openai
        upstream:
          url: http://mock-openapi:4010/openai/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    And I reset the trace collector

    When I send the following requests in a new trace
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tracing-openai/chat/completions" with body:
      """
      {"model": "gpt-4", "messages": [{"role": "user", "content": "Hello, how are you?"}]}
      """
    Then the response status code should be 200
    And I wait for the span "chat gpt-4"
    And the span "chat gpt-4" should be a "CLIENT" span
    And the span "chat gpt-4" should be a child of the span "external_processing.process"
    And the span "chat gpt-4" should have attribute "gen_ai.operation.name" with value "chat"
    And the span "chat gpt-4" should have attribute "gen_ai.provider.name" with value "openai"
    And the span "chat gpt-4" should have attribute "gen_ai.request.model" with value "gpt-4"
    And the span "chat gpt-4" should have attribute "gen_ai.response.model" with value "gpt-4.1-2025-04-14"
    And the span "chat gpt-4" should have attribute "gen_ai.response.id" with value "chatcmpl-B9MBs8CjcvOU2jLn4n570S5qMJKcT"
    And the span "chat gpt-4" attribute "gen_ai.response.finish_reasons" should contain "stop"
    And the span "chat gpt-4" should have attribute "gen_ai.usage.input_tokens" with value "19"
    And the span "chat gpt-4" should have attribute "gen_ai.usage.output_tokens" with value "10"
    And the span "chat gpt-4" should have attribute "server.address" with value "mock-openapi"
    And the span "chat gpt-4" should have attribute "server.port" with value "4010"
    And the span "chat gpt-4" should not have attribute "gen_ai.request.stream"

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "tracing-openai"
    Then the response status code should be 200

  Scenario: A streamed LLM response streams through untouched and is one span per body
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tracing-stream
      spec:
        displayName: Tracing Stream
        version: v1.0
        template: openai
        context: /tracing-stream
        upstream:
          url: http://mock-llm-stream:8080/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    And I reset the trace collector

    When I send the following requests in a new trace
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tracing-stream/chat/completions" with body:
      """
      {"model": "gpt-4o-mini", "stream": true, "messages": [{"role": "user", "content": "Plan a trip"}]}
      """
    Then the response status code should be 200
    And the response body should contain "Hello"
    And the response body should contain "data: [DONE]"
    And I wait for the span "chat gpt-4o-mini"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.request.stream" with value "true"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.usage.input_tokens" with value "10"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.usage.output_tokens" with value "7"
    And the span "chat gpt-4o-mini" attribute "gen_ai.response.finish_reasons" should contain "stop"
    And the span "chat gpt-4o-mini" should have attribute "gen_ai.response.time_to_first_chunk"
    And the trace should have 1 span named "external_processing.process_response_body"
    And the span "external_processing.process_response_body" attribute "stream.chunk_count" should be greater than 1
    # Content capture is on in this run.
    And the span "chat gpt-4o-mini" attribute "gen_ai.input.messages" should contain "Plan a trip"
    And the span "chat gpt-4o-mini" attribute "gen_ai.output.messages" should contain "Hello! How can I help?"

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "tracing-stream"
    Then the response status code should be 200

  Scenario: A request body over the buffer limit is still forwarded when only the span reads it
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tracing-large
      spec:
        displayName: Tracing Large
        version: v1.0
        template: openai
        context: /tracing-large
        upstream:
          url: http://mock-llm-stream:8080/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    And I reset the trace collector

    When I send the following requests in a new trace
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tracing-large/chat/completions" with a chat body of 1572864 bytes
    Then the response status code should be 200
    # The engine read only the first part of the body, so it cannot name the model.
    And I wait for the span "chat"
    And the span "chat" should have attribute "gen_ai.usage.input_tokens" with value "10"

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "tracing-large"
    Then the response status code should be 200

  Scenario: A guardrail denial is recorded on its policy span and makes no LLM call
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tracing-guarded
      spec:
        displayName: Tracing Guarded
        version: v1.0
        template: openai
        context: /tracing-guarded
        upstream:
          url: http://mock-llm-stream:8080/v1
        accessControl:
          mode: allow_all
        policies:
          - name: content-length-guardrail
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  request:
                    min: 1
                    max: 20
                    jsonPath: ""
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    And I reset the trace collector

    When I send the following requests in a new trace
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tracing-guarded/chat/completions" with body:
      """
      {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "this prompt is far longer than twenty bytes"}]}
      """
    Then the response status code should be 422
    And I wait for the span "external_processing.process"
    # The header phase also spans the policy, with no callback to run; the body phase denied.
    And some span "policy.request.content-length-guardrail" should have attribute "wso2.policy.action" with value "deny"
    And some span "policy.request.content-length-guardrail" should have attribute "wso2.guardrail.name" with value "content-length-guardrail"
    And the trace should have 0 spans named "chat gpt-4o-mini"

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "tracing-guarded"
    Then the response status code should be 200

  Scenario: A call through an LLM proxy records one inference span, on the provider route
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: tracing-proxied-provider
      spec:
        displayName: Tracing Proxied Provider
        version: v1.0
        template: openai
        context: /tracing-proxied-provider
        upstream:
          url: http://mock-llm-stream:8080/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: tracing-proxy
      spec:
        displayName: Tracing Proxy
        version: v1.0
        context: /tracing-proxy
        provider:
          id: tracing-proxied-provider
      """
    Then the response status should be 201
    And I wait for the endpoint "http://localhost:8080/tracing-proxy/chat/completions" to be ready with method "POST" and body '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}'
    And I reset the trace collector

    When I send the following requests in a new trace
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/tracing-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And I wait for 2 spans named "external_processing.process"
    And the trace should have 1 span named "chat gpt-4o-mini"
    And some span "external_processing.process" should have attribute "wso2.ai.proxy"

    Given I authenticate using basic auth as "admin"
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/tracing-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "tracing-proxied-provider"
    Then the response status code should be 200

  Scenario: An MCP tool call is a tool span, and the server accepts the rewritten _meta
    Given I authenticate using basic auth as "admin"
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: tracing-mcp
      spec:
        displayName: Tracing MCP
        version: v1.0
        context: /tracing-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001/mcp
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    And I reset the trace collector

    When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/tracing-mcp/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/tracing-mcp/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
    And I wait for the span "tools/call add"
    And the span "tools/call add" should be a "CLIENT" span
    And the span "tools/call add" should be a child of the span "external_processing.process"
    And the span "tools/call add" should have attribute "mcp.method.name" with value "tools/call"
    And the span "tools/call add" should have attribute "gen_ai.operation.name" with value "execute_tool"
    And the span "tools/call add" should have attribute "gen_ai.tool.name" with value "add"
    And the span "tools/call add" should have attribute "jsonrpc.request.id"
    And the span "tools/call add" attribute "gen_ai.tool.call.arguments" should contain "40"
    And no span in the trace should have attribute "mcp.body.capability.name"

    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "tracing-mcp"
    Then the response should be successful

  Scenario: A message sent to an A2A agent is an invoke_agent span
    Given I authenticate using basic auth as "admin"
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: tracing-agent
      spec:
        displayName: Tracing Agent
        version: v1.0
        context: /tracing-agent
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: JSONRPC
                pathPrefix: /
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    And I reset the trace collector

    When I send an A2A JSON-RPC request to "http://localhost:8080/tracing-agent":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "SendMessage", "params": {"message": {"messageId": "trace-msg-1", "contextId": "trace-ctx-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 2 day trip to Galle"}]}}}
      """
    Then the response status code should be 200
    And I wait for the span "invoke_agent Tracing Agent"
    And the span "invoke_agent Tracing Agent" should be a "CLIENT" span
    And the span "invoke_agent Tracing Agent" should have attribute "gen_ai.operation.name" with value "invoke_agent"
    And the span "invoke_agent Tracing Agent" should have attribute "gen_ai.provider.name" with value "a2a"
    And the span "invoke_agent Tracing Agent" should have attribute "gen_ai.conversation.id" with value "trace-ctx-1"
    And the span "invoke_agent Tracing Agent" should have attribute "a2a.message.id" with value "trace-msg-1"
    And the span "external_processing.process" should not have attribute "a2a.message.id"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "tracing-agent"
    Then the response should be successful
