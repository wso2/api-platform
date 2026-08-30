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

Feature: Startup DB Bootstrap
  As a gateway operator
  I want the gateway-controller to restore persisted configs from the database on startup
  So that control-plane sync failures do not prevent already stored configs from becoming active

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # mock-platform-api accepts the WebSocket connection in IT, but its startup sync
  # endpoints are intentionally not implemented. Restarting gateway-controller here
  # exercises the DB bootstrap path while control-plane startup sync is failing.
  Scenario: Restarted gateway-controller restores persisted resources from the database
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: startup-db-llm-provider
      spec:
        displayName: Startup DB LLM Provider
        version: v1.0
        template: openai
        context: /startup-db-llm
        upstream:
          url: http://mock-openapi:4010/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-startup-db-test
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: startup-db-llm-proxy
      spec:
        displayName: Startup DB LLM Proxy
        version: v1.0
        context: /startup-db-proxy
        provider:
          id: startup-db-llm-provider
      """
    Then the response status should be 201

    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: startup-db-mcp-v1.0
      spec:
        displayName: Startup DB MCP
        version: v1.0
        context: /startup-db-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001/mcp
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: startup-db-rest-api
      spec:
        displayName: Startup DB Rest API
        version: v1.0
        context: /startup-db-rest/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v2
        operations:
          - method: GET
            path: /{country_code}/{city}
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    # An Agent joins the other kinds here rather than getting a scenario of its
    # own, so it goes through the same single restart under the same
    # control-plane-sync-is-failing conditions.
    #
    # This is the only place Agent persistence is exercised. agent_deploy.feature's
    # CRUD scenario never restarts anything, so an Agent that is stored but never
    # restored passes every assertion in it. Dropping models.KindAgent from
    # supportsRuntimeBootstrapKind leaves storage completely intact and the
    # management API answering correctly while every Agent route disappears —
    # nothing else in the suite would fail.
    #
    # A managed card and an auth policy are both attached because restoring the
    # route is not the same as restoring what hangs off it: a route that comes
    # back with an empty policy chain still answers, having silently dropped
    # authentication and the locally-served card.
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: startup-db-agent
      spec:
        displayName: Startup DB Agent
        version: v1.0
        context: /startup-db-agent
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: JSONRPC
                pathPrefix: /
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: api-key-auth
                version: v1
                params:
                  key: API-Key
                  in: header
          agentCard:
            public:
              mode: managed
              content:
                name: Startup DB Agent
                description: Persisted Agent restored from the database on restart
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: JSONRPC
                    protocolVersion: "1.0"
                    url: https://agents.example.com/startup-db-agent
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/startup-db-agent/v1
                capabilities:
                  streaming: true
                defaultInputModes:
                  - text/plain
                defaultOutputModes:
                  - text/plain
                skills:
                  - id: plan_trip
                    name: Plan a trip
                    description: Plans a trip itinerary
                    tags:
                      - travel
      """
    # Agent create answers 201 with the k8s-style resource envelope, whose
    # `status` is an object — not the {"status": "success"} wrapper the older
    # kinds' handlers return.
    Then the response status should be 201
    And the response should be valid JSON
    And the JSON response should have field "status.id"

    When I clear all headers
    And I wait for the endpoint "http://localhost:8080/startup-db-llm/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"warmup"}]}'
    And I wait for the endpoint "http://localhost:8080/startup-db-proxy/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"proxy warmup"}]}'
    And I wait for the endpoint "http://localhost:8080/startup-db-rest/v1.0/us/seattle" to be ready
    And I wait for 2 seconds

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/startup-db-llm/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "before restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/startup-db-proxy/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy before restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I clear all headers
    And I send a GET request to "http://localhost:8080/startup-db-rest/v1.0/us/seattle"
    Then the response status code should be 200
    And the response body should contain "/api/v2/us/seattle"

    When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/startup-db-mcp/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/startup-db-mcp/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # --- Agent, before the restart ---
    Given I authenticate using basic auth as "admin"
    When I send a POST request to the "gateway-controller" service at "/agents/startup-db-agent/api-keys" with body:
      """
      {
        "name": "startup-db-agent-key",
        "apiKey": "startup-db-agent-secret"
      }
      """
    Then the response status should be 201
    And I wait for 2 seconds

    When I clear all headers
    And I set header "API-Key" to "startup-db-agent-secret"
    And I send an A2A "GET" request to "http://localhost:8080/startup-db-agent/v1/tasks"
    Then the response status code should be 200

    When I clear all headers
    And I set header "API-Key" to "startup-db-agent-secret"
    And I send an A2A JSON-RPC request to "http://localhost:8080/startup-db-agent":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    # The card route carries no auth policy, so it is fetched without a key.
    When I clear all headers
    And I save the Agent Card at "http://localhost:8080/startup-db-agent/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Startup DB Agent"

    When I restart the "gateway-controller" service
    And I wait for the endpoint "http://localhost:8080/startup-db-llm/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"after restart warmup"}]}'
    And I wait for the endpoint "http://localhost:8080/startup-db-proxy/chat/completions" to be ready with method "POST" and body '{"model":"gpt-4","messages":[{"role":"user","content":"proxy after restart warmup"}]}'
    And I wait for the endpoint "http://localhost:8080/startup-db-rest/v1.0/us/seattle" to be ready
    And I wait for 2 seconds

    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller-admin" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "startup-db-llm-provider"
    And the response body should contain "startup-db-llm-proxy"
    And the response body should contain "startup-db-mcp-v1.0"
    And the response body should contain "startup-db-rest-api"
    And the response body should contain "startup-db-agent"
    And the response body should contain "startup-db-llm"
    And the response body should contain "startup-db-proxy"
    And the response body should contain "startup-db-mcp"
    And the response body should contain "startup-db-rest"

    When I clear all headers
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/startup-db-llm/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "after restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/startup-db-proxy/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "proxy after restart"}]
      }
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "object" should be "chat.completion"

    When I clear all headers
    And I send a GET request to "http://localhost:8080/startup-db-rest/v1.0/us/seattle"
    Then the response status code should be 200
    And the response body should contain "/api/v2/us/seattle"

    When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/startup-db-mcp/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/startup-db-mcp/mcp"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # --- Agent, after the restart ---
    # The management API still knows the Agent.
    Given I authenticate using basic auth as "admin"
    When I get the Agent "startup-db-agent"
    Then the response status should be 200
    And the response body should contain "startup-db-agent"

    # Both transports still route. Asserting only one would pass on a restore
    # that brought back a single transport's routes.
    When I clear all headers
    And I set header "API-Key" to "startup-db-agent-secret"
    And I send an A2A "GET" request to "http://localhost:8080/startup-db-agent/v1/tasks"
    Then the response status code should be 200

    When I clear all headers
    And I set header "API-Key" to "startup-db-agent-secret"
    And I send an A2A JSON-RPC request to "http://localhost:8080/startup-db-agent":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    # The policy chain came back with the route. A 401 here rather than a 404 is
    # the point: the route exists and its auth policy is still enforcing.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/startup-db-agent/v1/tasks"
    Then the response status code should be 401

    # The managed card is still served locally, byte-for-byte as stored, under
    # the same tag. A changed tag on unchanged bytes would tell every client
    # holding the old one that its cached copy is stale.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/startup-db-agent/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response ETag should match the saved ETag
    And the Agent Card at "http://localhost:8080/startup-db-agent/.well-known/agent-card.json" should be byte-identical to the saved card

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "startup-db-agent"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/startup-db-llm-proxy"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "startup-db-mcp-v1.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

    Given I authenticate using basic auth as "admin"
    When I delete the API "startup-db-rest-api"
    Then the response should be successful

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "startup-db-llm-provider"
    Then the response status code should be 200
