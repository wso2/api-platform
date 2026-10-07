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


# The gateway emits event dimensions; counts, distinct-consumer rollups and success rates are
# computed downstream. So what is asserted here is that each dimension a downstream A2A dashboard
# needs is present and correct — not that anything was aggregated.
#
# The A2A block is a first-class field of the published event — Moesif's own A2A schema, a sibling
# of metadata rather than a key inside it — and the request and response facts sit in their own
# objects within it. That is why these assertions need their own step rather than the flat
# metadata-field one, and why a field is named by its published path: "response.task_state", not
# "taskState".
#
# Two dimensions carry most of the weight. `operation` is stamped by the kernel from the bound
# chain key rather than by anything that re-parsed the request, so it names the operation whose
# policies actually ran. `outcome` is derived from the A2A result rather than the HTTP status,
# because a JSON-RPC error rides a 200 and a status-only reading would report a failed invocation
# as a success.
#
# Every event is selected by the URI the gateway publishes for it. A request proxied to the agent
# is reported by its path without the query string; traffic the gateway answers or refuses itself
# — a card fetch, a CORS preflight, a policy denial, a locally served protected card — is reported
# by the Agent's context alone.
#
# Readiness polling makes analytics events of its own, so every scenario lets the collector
# settle and resets it after the Agent is reachable and before the requests it asserts on.

@agent-analytics
Feature: Agent analytics event dimensions
  As an operator
  I want A2A invocations to emit correct analytics dimensions
  So that agent traffic can be measured without misreporting what happened

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # The clearest statement of D4 the suite can make: one canonical operation reached over two
  # bindings aggregates to the same operation while remaining distinguishable by transport. If the
  # two bindings ever stopped sharing a canonical operation, the two events would disagree here.
  Scenario: The same operation over both bindings shares an operation dimension and differs by transport
    Given I generate a unique resource name from "agent-analytics-transport" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-transport" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
      | displayName        | Agent Analytics Transport                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Managed Analytics Card","description":"Card fetches must not be counted as invocations","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 200

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200

    # The HTTP+JSON call.
    Then the latest analytics event for path "${CTX:agentContext}/v1/tasks" should have A2A field "request_type" with value "operation"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks" should have A2A field "operation" with value "ListTasks"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks" should have A2A field "transport" with value "HTTP+JSON"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks" should have A2A field "protocol_version" with value "1.0"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks" should have A2A field "outcome" with value "SUCCESS"

    # The JSON-RPC call: same operation, different transport. Its endpoint is the Agent's context
    # itself, which the path selector matches exactly, so it never selects the HTTP+JSON event
    # whatever order the two invocations are made in.
    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "request_type" with value "operation"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "ListTasks"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "transport" with value "JSONRPC"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "outcome" with value "SUCCESS"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # An Agent serves three shapes of traffic on one context: its operations, its card, and the
  # preflights for both. Only the first is an invocation. A card fetch that arrived carrying an
  # operation and an outcome would let a downstream rollup count a client's card polling as agent
  # traffic, and card polling is frequent — a client re-reads the card to discover how to
  # authenticate before it can invoke anything.
  Scenario: Card fetches and preflights are reported but not shaped like invocations
    Given I generate a unique resource name from "agent-analytics-card" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-card" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | name                               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | displayName                        | Agent Analytics Card                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | context                            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | spec.a2a.operationConfigs.policies | [{"name":"cors","version":"v1","params":{"allowedOrigins":["https://client.example.com"],"allowedMethods":["GET","POST","OPTIONS"],"allowedHeaders":["Content-Type","A2A-Version"]}}]                                                                                                                                                                                                                                                                                                                         |
      | spec.a2a.agentCard                 | {"public":{"mode":"managed","content":{"name":"Managed Analytics Card Only","description":"A card fetch is discovery, not an invocation","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    # Exactly one request, so the event the assertions read is the one it made. The gateway answers
    # the card itself, so its event is reported by the Agent's context alone.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200

    # The card fetch: visible as traffic, but naming no operation it resolved to and no outcome, so
    # nothing downstream can roll it in with an invocation.
    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "request_type" with value "agentCard"
    # request_type is the only dimension such an event determines. operation and transport are
    # present because the schema requires them on every event, and the step asserts they hold
    # their catch-alls (Unknown/UNKNOWN) rather than a real value; no outcome and none of the
    # request or response dimensions appear.
    And the latest analytics event for path "${CTX:agentContext}" should carry only A2A field "request_type"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # A preflight is the third shape of traffic on an Agent's context, and the gateway answers it
  # itself. Its own scenario, again so the assertions read the single event this makes.
  Scenario: A CORS preflight is reported as a preflight, not an invocation
    Given I generate a unique resource name from "agent-analytics-preflight" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-preflight" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                                                                                                          |
      | name                               | ${CTX:agentName}                                                                                                                                                                   |
      | displayName                        | Agent Analytics Preflight                                                                                                                                                          |
      | context                            | ${CTX:agentContext}                                                                                                                                                                |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                                                                                                       |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                               |
      | spec.a2a.operationConfigs.policies | [{"name":"cors","version":"v1","params":{"allowedOrigins":["https://client.example.com"],"allowedMethods":["GET","POST","OPTIONS"],"allowedHeaders":["Content-Type","A2A-Version"]}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"}}                                                                                                                                                  |
    Then the response should be successful
    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:agentContext}/v1/tasks" until status 204
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 204

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "request_type" with value "preflight"
    And the latest analytics event for path "${CTX:agentContext}" should carry only A2A field "request_type"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # A JSON-RPC error travels inside an HTTP 200, so an outcome read from the status alone reports a
  # failed invocation as a success. This is the half of the outcome rule that a status-only reading
  # gets wrong in the optimistic direction; the next scenario is the pessimistic one.
  Scenario: A JSON-RPC error inside a 200 is reported as a failed invocation
    Given I generate a unique resource name from "agent-analytics-outcome" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-outcome" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                           | ${CTX:gatewaySpecVersion}                                                                                         |
      | name                                 | ${CTX:agentName}                                                                                                  |
      | displayName                          | Agent Analytics Outcome                                                                                           |
      | context                              | ${CTX:agentContext}                                                                                               |
      | upstreamUrl                          | http://a2a-trip-planner:9099                                                                                      |
      | transports                           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]               |
      | spec.a2a.operationConfigs.operations | [{"name":"GetTask","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}]           |
      | spec.a2a.agentCard                   | {"public":{"mode":"passthrough"}}                                                                                 |
    Then the response should be successful
    # GetTask answering 401 proves its operation chain is bound.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks/readiness-probe" until status 401
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    # One request, so the assertions read the single event it makes.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "CancelTask", "params": {"id": "no-such-task"}}
      """
    Then the response status code should be 200
    And the JSON response should have field "error"

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "CancelTask"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "response.is_error" with value "true"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "outcome" with value "FAILURE"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The pessimistic half: a policy denial arrives as a 401, and a status-only reading blames the
  # agent for a request the gateway refused before the agent ever saw it.
  Scenario: A policy denial is attributed to the gateway rather than to the agent
    Given I generate a unique resource name from "agent-analytics-denial" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-denial" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                           | ${CTX:gatewaySpecVersion}                                                                               |
      | name                                 | ${CTX:agentName}                                                                                        |
      | displayName                          | Agent Analytics Denial                                                                                  |
      | context                              | ${CTX:agentContext}                                                                                     |
      | upstreamUrl                          | http://a2a-trip-planner:9099                                                                            |
      | transports                           | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                    |
      | spec.a2a.operationConfigs.operations | [{"name":"GetTask","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
      | spec.a2a.agentCard                   | {"public":{"mode":"passthrough"}}                                                                       |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks/some-task" until status 401
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks/some-task"
    Then the response status code should be 401

    # The operation is still named even though the request never reached the agent — it comes
    # from the bound chain, not from a response. The gateway refused it, so its event is reported by
    # the Agent's context alone.
    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "GetTask"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "outcome" with value "FAILURE"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "failure_origin" with value "POLICY"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # A managed protected Agent Card is answered by the gateway, exactly as a managed public card is
  # — but it must NOT be reported the same way. The public card is discovery: `requestType:
  # agentCard`, no operation. The protected card is the GetExtendedAgentCard *operation*, reached
  # over a transport, by an authenticated consumer. Reporting it as `agentCard` would move
  # authenticated invocations into the discovery bucket and lose the consumer dimension with them.
  #
  # That the gateway rather than the agent produced the response is not something the event should
  # reflect: the operation ran, its policies ran, and a client received a result.
  Scenario: A locally served protected Agent Card is reported as an operation, not as discovery
    Given I generate a unique resource name from "agent-analytics-protected" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-protected" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | name                               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | displayName                        | Agent Analytics Protected                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | context                            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                               |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","content":{"name":"Trip Planner","description":"Plans trips. Gateway-managed extended card.","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"gateway_managed_skill","name":"Only on the managed protected card"}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "gateway_managed_skill"

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "request_type" with value "operation"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "GetExtendedAgentCard"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "transport" with value "HTTP+JSON"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "outcome" with value "SUCCESS"

    # The same operation over the other binding: same operation dimension, and still not discovery.
    # Every event in this scenario is reported by the Agent's context, so the collector is reset
    # before each request to keep the previous request's event from being the one read.
    When I wait for the analytics collector to settle
    And I reset the analytics collector
    And I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "request_type" with value "operation"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "GetExtendedAgentCard"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "transport" with value "JSONRPC"
    And I wait for the analytics collector to settle

    # A refusal at the guard is still the operation, and still attributed to the gateway rather
    # than to the agent — the request never reached it.
    When I reset the analytics collector
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "request_type" with value "operation"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "GetExtendedAgentCard"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "outcome" with value "FAILURE"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "failure_origin" with value "POLICY"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # Three of an A2A event's dimensions describe what the caller asked for and four describe what
  # came back, and none of them is reachable from the resolver's output: the summaries are in the
  # request body or the query string, and the observed facts exist only after the agent has
  # answered.
  #
  # The observed identifiers are asserted as present rather than by value. They are the agent's to
  # generate — which is the point of having them: a client that sends a bare message gets back a
  # task id it never supplied, and without this dimension that invocation cannot be correlated to
  # anything at all.
  Scenario: A send request's own summaries and the agent's observed task both reach the event
    Given I generate a unique resource name from "agent-analytics-properties" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-properties" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Analytics Properties                                                                          |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    # A send with no configuration block at all. returnImmediately still reports a value, because
    # its absence is defined by the protocol as false — what the request will be treated as having
    # asked for, which is what a dashboard splitting on it needs.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "SendMessage", "params": {"message": {"messageId": "props-rpc-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 3 day trip to Kandy"}, {"text": "budget travel"}]}}}
      """
    Then the response status code should be 200
    And I store the JSON response field "result.task.id" as "propsTask"

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "SendMessage"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "request.input_part_count" with value "2"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "request.return_immediately" with value "false"
    And the latest analytics event for path "${CTX:agentContext}" should not have A2A field "request.history_length"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "response.payload_type" with value "task"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "response.task_state" with value "TASK_STATE_COMPLETED"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "terminal" with value "true"
    And the latest analytics event for path "${CTX:agentContext}" should have a non-empty A2A field "response.task_id"
    And the latest analytics event for path "${CTX:agentContext}" should have a non-empty A2A field "response.context_id"

    # GetTask is a GET on the HTTP+JSON binding, so its history length is in the query string and
    # nowhere else. A body-phase extraction would have dropped it — along with the same field on
    # six other operations that carry no body.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks/${CTX:propsTask}?historyLength=2"
    Then the response status code should be 200

    Then the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should have A2A field "operation" with value "GetTask"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should have A2A field "transport" with value "HTTP+JSON"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should have A2A field "request.history_length" with value "2"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should have A2A field "response.payload_type" with value "task"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should have A2A field "response.task_state" with value "TASK_STATE_COMPLETED"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should have A2A field "terminal" with value "true"
    # An operation whose request shape has no message reports neither send-only summary: emitting
    # returnImmediately here would state that the caller chose the default on a field its request
    # does not have.
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should not have A2A field "request.input_part_count"
    And the latest analytics event for path "${CTX:agentContext}/v1/tasks/${CTX:propsTask}" should not have A2A field "request.return_immediately"

    # The same three summaries over the other binding, where they are at the top level of the body
    # rather than under params.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}/v1/message:send" with body:
      """
      {"message": {"messageId": "props-rest-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 5 day trip to Galle slowly"}]}, "configuration": {"returnImmediately": true, "historyLength": 4}}
      """
    Then the response status code should be 200

    Then the latest analytics event for path "${CTX:agentContext}/v1/message:send" should have A2A field "operation" with value "SendMessage"
    And the latest analytics event for path "${CTX:agentContext}/v1/message:send" should have A2A field "transport" with value "HTTP+JSON"
    And the latest analytics event for path "${CTX:agentContext}/v1/message:send" should have A2A field "request.input_part_count" with value "1"
    And the latest analytics event for path "${CTX:agentContext}/v1/message:send" should have A2A field "request.return_immediately" with value "true"
    And the latest analytics event for path "${CTX:agentContext}/v1/message:send" should have A2A field "request.history_length" with value "4"
    And the latest analytics event for path "${CTX:agentContext}/v1/message:send" should have A2A field "response.payload_type" with value "task"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # A streamed invocation observes its facts across many events rather than one document, and the
  # two rules differ: the payload type is whatever the last event was, while the task state is the
  # last state any event reported — the state the invocation actually reached. A stream ending in
  # an artifact would otherwise report no state at all.
  Scenario: A streamed invocation reports the state its stream reached
    Given I generate a unique resource name from "agent-analytics-stream-props" and store it as "agentName"
    And I generate a unique API context from "/agent-analytics-stream-props" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Analytics Stream Properties                                                                   |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.resilience    | {"idleTimeout":"30s"}                                                                               |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I open an A2A stream with a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "SendStreamingMessage", "params": {"message": {"messageId": "props-stream-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 3 day trip to Kandy"}]}}}
      """
    Then the response status code should be 200
    And the A2A stream's last event should contain "TASK_STATE_COMPLETED"

    Then the latest analytics event for path "${CTX:agentContext}" should have A2A field "operation" with value "SendStreamingMessage"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "response.is_streaming" with value "true"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "request.input_part_count" with value "1"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "response.payload_type" with value "status_update"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "response.task_state" with value "TASK_STATE_COMPLETED"
    And the latest analytics event for path "${CTX:agentContext}" should have A2A field "terminal" with value "true"
    And the latest analytics event for path "${CTX:agentContext}" should have a non-empty A2A field "response.task_id"
    And the latest analytics event for path "${CTX:agentContext}" should have a non-empty A2A field "response.context_id"
    And I wait for the analytics collector to settle

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
