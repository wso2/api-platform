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


# The upstream for every Agent here is a2a-trip-planner: a real A2A agent built on the official
# Python SDK. Its JSON-RPC binding is mounted at "/" and its HTTP+JSON binding at "/v1", so an
# Agent's pathPrefix values are the ones a real deployment would use. A transport's pathPrefix
# travels upstream with the request; only spec.context is stripped.
#
# Persistence across a controller restart is not covered here: nothing in this file restarts
# anything, so an Agent that is stored but never restored would pass every scenario below.
#
# The conformant calls are made with the official Go A2A SDK against the agent on the official
# Python SDK: two independent implementations of the protocol with the gateway between them.
# Requests that are deliberately not conformant — a missing or wrong protocol version, an unknown
# JSON-RPC method — go through the ordinary data-plane request steps with an explicit A2A-Version
# header, because a conformant client cannot produce them.
#
# Every SDK client is built from an explicit gateway path. A passthrough card advertises the
# upstream's own address, so a client that followed it would pass having exercised no gateway
# route at all.
#
# Data-plane readiness is polled on an operation route rather than assumed after the management
# call returns: a route answering is what proves the deployment reached the router and the policy
# engine, and an operation route behind an authentication policy answering 401 proves its chain
# is bound.

@agent-deploy
Feature: Agent deployment and A2A routing
  As an API developer
  I want to deploy an Agent and reach its A2A operations
  So that the gateway routes both protocol bindings to the agent behind it

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== CROSS-TRANSPORT INVOCATION ====================

  # One canonical operation, two bindings, one agent behind them. Each binding is driven by its
  # own SDK client so neither can borrow the other's transport, and the two results are compared:
  # reaching the agent over both is not the claim — producing the same protocol result is.
  @type:smoke
  Scenario: One canonical operation is invoked over both bindings with the official A2A SDK
    Given I generate a unique resource name from "agent-sdk-both" and store it as "agentName"
    And I generate a unique API context from "/agent-sdk-both" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent SDK Both Bindings                                                                             |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding at "${CTX:agentContext}"
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"

    When the A2A client "rpc" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rpc" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rpc" should have received an artifact containing "Trip plan for Kandy: 3 days"

    When the A2A client "rest" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rest" should have received an artifact containing "Trip plan for Kandy: 3 days"

    # The itineraries are fully determined by the request, so equal artifacts mean both bindings
    # carried the same request to the same agent and brought back the same answer.
    Then the A2A clients "rpc" and "rest" should have received the same artifact

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== MANAGEMENT LIFECYCLE ====================

  # The management half of an Agent's life, with a conformant invocation in the middle so "the
  # resource exists" and "the resource is reachable" are both asserted. The 404s at the end are
  # what say a delete removed the routes rather than only the record. The listing is filtered to
  # this Agent's context, because other runners deploy Agents into the same gateway.
  Scenario: An Agent can be created, listed, fetched, updated, invoked and deleted
    Given I generate a unique resource name from "agent-lifecycle" and store it as "agentName"
    And I generate a unique API context from "/agent-lifecycle" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Lifecycle                                                                                     |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/agents?context=${CTX:agentContext}"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "count" should be 1
    And the JSON response array "agents" should contain an item with "metadata.name" equal to "${CTX:agentName}"

    When I get the Agent "${CTX:agentName}"
    Then the response should be successful
    And the JSON response field "spec.displayName" should be "Agent Lifecycle"

    When I update Agent "${CTX:agentName}" from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Lifecycle Updated                                                                             |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I get the Agent "${CTX:agentName}"
    Then the JSON response field "spec.displayName" should be "Agent Lifecycle Updated"

    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    When I clear all headers
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"
    And the A2A client "rest" sends the message "Plan a 2-day trip to Ella"
    Then the A2A client "rest" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rest" should have received an artifact containing "Trip plan for Ella: 2 days"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

    When I send a "GET" request to the "gateway-controller" service at "/agents?context=${CTX:agentContext}"
    Then the response should be successful
    And the JSON response field "count" should be 0

    # Both bindings, because a delete that removed one route set and left the other would
    # otherwise pass.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 404
    Then the response status code should be 404

    When I send a "POST" request to "${CTX:agentContext}" until status 404 with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 404

  # ==================== ALL ELEVEN OPERATIONS, BOTH BINDINGS ====================

  # Enumerated rather than described as "the lifecycle", which reads as nine and silently drops
  # the two that do not fit a task's arc — SendStreamingMessage and GetExtendedAgentCard. Both are
  # here for reachability on both bindings only; their semantics belong to agent_streaming.feature
  # and agent_card.feature.
  #
  # The order is dictated by what each operation needs to act on: a completed task for the first
  # group, then a deliberately long-running one, because GetTask, SubscribeToTask and CancelTask
  # against an already-finished task exercise none of the states they exist for.
  #
  # The jwt-auth policy is required for GetExtendedAgentCard to be reachable at all: the extended
  # card is guarded on every Agent, so without an authentication policy no caller can ever reach
  # that one operation. An Agent-wide policy is what the guard checks against, so every operation
  # below authenticates.
  @type:smoke
  Scenario: All eleven A2A 1.0 operations are reachable over both bindings through the official SDK
    Given I generate a unique resource name from "agent-sdk-operations" and store it as "agentName"
    And I generate a unique API context from "/agent-sdk-operations" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                           |
      | name                               | ${CTX:agentName}                                                                                    |
      | displayName                        | Agent SDK Operations                                                                                |
      | context                            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                        |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                             |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # ---- HTTP+JSON ----
    # The client reads the Authorization header at call time, so setting it once here covers
    # every operation it issues below.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"

    # 1. SendMessage
    When the A2A client "rest" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rest" should have received an artifact containing "Trip plan for Kandy: 3 days"

    # 2. SendStreamingMessage — drained for reachability; event semantics live in
    #    agent_streaming.feature.
    When the A2A client "rest" streams the message "Plan a 2-day trip to Ella"
    Then the A2A client "rest" should have received at least 2 stream events
    And the A2A client "rest" stream should end in state "TASK_STATE_COMPLETED"

    # A task that is genuinely still running, for the six operations below.
    When the A2A client "rest" sends the message "Plan a trip to Galle slowly" and returns immediately
    Then the A2A client "rest" should have received a task that is still running

    # 3. GetTask
    When the A2A client "rest" gets the task
    Then the A2A client "rest" should have received a task that is still running

    # 4. ListTasks
    When the A2A client "rest" lists tasks
    Then the A2A client "rest" should have received a task list containing the task

    # 5-8. The four push-notification-config operations.
    When the A2A client "rest" creates the push notification config "rest-push-1" for the task
    Then the A2A client "rest" should have received a push config "rest-push-1"
    When the A2A client "rest" gets the push notification config "rest-push-1" for the task
    Then the A2A client "rest" should have received a push config "rest-push-1"
    When the A2A client "rest" lists push notification configs for the task
    Then the A2A client "rest" should have received 1 push config
    When the A2A client "rest" deletes the push notification config "rest-push-1" for the task
    Then the A2A client "rest" call should have succeeded
    When the A2A client "rest" lists push notification configs for the task
    Then the A2A client "rest" should have received 0 push configs

    # 9. SubscribeToTask — re-attaches to the running task and reads a live event. Over
    #    HTTP+JSON this is a POST, per the binding table's resolution of the upstream verb
    #    disagreement.
    When the A2A client "rest" subscribes to the task and reads 1 event
    Then the A2A client "rest" should have received at least 1 stream event

    # 10. CancelTask — real, because the task is still running.
    When the A2A client "rest" cancels the task
    Then the A2A client "rest" should have received a task in state "TASK_STATE_CANCELED"

    # 11. GetExtendedAgentCard — 200 and a card; which card, and under what conditions, is
    #     agent_card.feature's business.
    When the A2A client "rest" gets the extended Agent Card
    Then the A2A client "rest" call should have succeeded
    And the A2A client "rest" should have received an Agent Card named "Trip Planner"

    # ---- JSON-RPC: the same eleven ----
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I create an A2A client "rpc" for the "JSONRPC" binding at "${CTX:agentContext}"

    # 1. SendMessage
    When the A2A client "rpc" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rpc" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rpc" should have received an artifact containing "Trip plan for Kandy: 3 days"

    # 2. SendStreamingMessage
    When the A2A client "rpc" streams the message "Plan a 2-day trip to Ella"
    Then the A2A client "rpc" should have received at least 2 stream events
    And the A2A client "rpc" stream should end in state "TASK_STATE_COMPLETED"

    When the A2A client "rpc" sends the message "Plan a trip to Galle slowly" and returns immediately
    Then the A2A client "rpc" should have received a task that is still running

    # 3. GetTask
    When the A2A client "rpc" gets the task
    Then the A2A client "rpc" should have received a task that is still running

    # 4. ListTasks
    When the A2A client "rpc" lists tasks
    Then the A2A client "rpc" should have received a task list containing the task

    # 5-8. Push notification configs.
    When the A2A client "rpc" creates the push notification config "rpc-push-1" for the task
    Then the A2A client "rpc" should have received a push config "rpc-push-1"
    When the A2A client "rpc" gets the push notification config "rpc-push-1" for the task
    Then the A2A client "rpc" should have received a push config "rpc-push-1"
    When the A2A client "rpc" lists push notification configs for the task
    Then the A2A client "rpc" should have received 1 push config
    When the A2A client "rpc" deletes the push notification config "rpc-push-1" for the task
    Then the A2A client "rpc" call should have succeeded
    When the A2A client "rpc" lists push notification configs for the task
    Then the A2A client "rpc" should have received 0 push configs

    # 9. SubscribeToTask
    When the A2A client "rpc" subscribes to the task and reads 1 event
    Then the A2A client "rpc" should have received at least 1 stream event

    # 10. CancelTask
    When the A2A client "rpc" cancels the task
    Then the A2A client "rpc" should have received a task in state "TASK_STATE_CANCELED"

    # 11. GetExtendedAgentCard
    When the A2A client "rpc" gets the extended Agent Card
    Then the A2A client "rpc" call should have succeeded
    And the A2A client "rpc" should have received an Agent Card named "Trip Planner"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== ROUTES FOLLOW CONFIGURED TRANSPORTS ====================

  # There is one canonical policy chain per operation regardless of which transports are
  # configured, but routes exist only for the transports that are. A gateway that generated
  # routes for both bindings whichever was asked for would still pass every invocation scenario
  # in this file.
  Scenario: An HTTP+JSON-only Agent does not serve the JSON-RPC endpoint
    Given I generate a unique resource name from "agent-restonly" and store it as "agentName"
    And I generate a unique API context from "/agent-restonly" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                             |
      | name               | ${CTX:agentName}                                      |
      | displayName        | Agent REST Only                                       |
      | context            | ${CTX:agentContext}                                   |
      | upstreamUrl        | http://a2a-trip-planner:9099                          |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                     |
    Then the response should be successful

    # The configured binding routes.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200
    Then the response status code should be 200

    # The JSON-RPC endpoint was never generated.
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 404

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  Scenario: A JSON-RPC-only Agent does not serve the HTTP+JSON routes
    Given I generate a unique resource name from "agent-rpconly" and store it as "agentName"
    And I generate a unique API context from "/agent-rpconly" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                        |
      | name               | ${CTX:agentName}                                 |
      | displayName        | Agent RPC Only                                   |
      | context            | ${CTX:agentContext}                              |
      | upstreamUrl        | http://a2a-trip-planner:9099                     |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                |
    Then the response should be successful

    # A JSON-RPC error rides an HTTP 200, so the status alone would pass on a call that reached
    # the agent and failed. The result field is what says the operation actually ran.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" until status 200 with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response should have field "result"
    And the JSON response field "error" should not exist

    When I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 404

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== RESOLUTION FAILURE ====================

  # A JSON-RPC method the protocol version does not define cannot resolve to a canonical
  # operation, so no policy chain is ever bound and the engine answers with its own sterile
  # response.
  #
  # Asserted as the sterile shape — a 404 with {"error":"Not Found","error_id":...} and an
  # x-error-id header — deliberately not as a JSON-RPC error object. Protocol-shaped error
  # rendering is a separate feature; when it lands this scenario changes and nothing else does.
  @type:negative
  Scenario: An unknown JSON-RPC method fails resolution with the sterile response
    Given I generate a unique resource name from "agent-unknown-method" and store it as "agentName"
    And I generate a unique API context from "/agent-unknown-method" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                        |
      | name               | ${CTX:agentName}                                 |
      | displayName        | Agent Unknown Method                             |
      | context            | ${CTX:agentContext}                              |
      | upstreamUrl        | http://a2a-trip-planner:9099                     |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" until status 200 with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "NotAnOperation", "params": {}}
      """
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "error" should be "Not Found"
    And the JSON response should have field "error_id"
    And the response header "x-error-id" should exist

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== REQUEST PROTOCOL VERSION ====================

  # A2A 1.0 section 3.6.1 makes stating the protocol version a client obligation on every
  # operation request; 3.6.2 fixes what silence means — 0.3, not "whatever the server serves".
  # The gateway enforces that before it resolves an operation, binds a chain, buffers a body or
  # calls the agent, because the Agent's configured version selects the operation table a request
  # is interpreted against.
  #
  # Asserted through the gateway rather than through the agent behind it. The reference SDK also
  # rejects a wrong version, so a scenario that only checked "the request failed" would pass with
  # the guard removed. The sterile 400 is the gateway's own answer and the agent's is not — its
  # JSON-RPC binding answers HTTP 200 with a JSON-RPC error object — so the shape is what tells
  # them apart.
  @type:negative
  Scenario: An operation request is rejected unless it states the Agent's protocol version
    Given I generate a unique resource name from "agent-version-guard" and store it as "agentName"
    And I generate a unique API context from "/agent-version-guard" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Version Guard                                                                                 |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful

    # The correct version, both bindings: the request reaches the agent.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" until status 200 with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    When I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 200

    # No version at all means 0.3, which this Agent does not expose. The sterile 400 says the
    # gateway refused it — the agent would have answered 200 with a JSON-RPC error.
    When I clear all headers
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400
    And the response should be valid JSON
    And the JSON response field "error" should be "Bad Request"
    And the JSON response should have field "error_id"
    And the response header "x-error-id" should exist

    # An empty value is the same case as an absent one.
    When I clear all headers
    And I set header "A2A-Version" to " "
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400

    # A version the Agent does not expose. There is no range match and no newest-version
    # fallback: one Agent exposes exactly one version.
    When I clear all headers
    And I set header "A2A-Version" to "0.3"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400

    # Not canonical Major.Minor. Refused rather than folded onto "1.0".
    When I clear all headers
    And I set header "A2A-Version" to "1.0.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400

    # The nine path-known HTTP+JSON operations resolve statically, so they are the ones a
    # misplaced guard would skip while every JSON-RPC assertion above still passed.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 400

    When I clear all headers
    And I set header "A2A-Version" to "99.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 400

    # The version may travel in the query instead, for a client that cannot set headers. Same
    # rules, same answers.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks?A2A-Version=1.0"
    Then the response status code should be 200

    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks?A2A-Version=0.3"
    Then the response status code should be 400

    # Both representations at once must agree. Contradicting themselves is refused even though
    # one of the two values is correct, because which one an intermediary would have kept is not
    # something a client gets to leave open.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks?A2A-Version=0.3"
    Then the response status code should be 400

    When I send a "GET" request to "${CTX:agentContext}/v1/tasks?A2A-Version=1.0"
    Then the response status code should be 200

    # The version is checked before the operation is, so a request that is wrong about both is
    # reported as the version problem: the gateway never reads the body, and a 404 here would
    # mean the guard ran too late.
    When I clear all headers
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "message/send", "params": {}}
      """
    Then the response status code should be 400

    # And once the version is right, the body's own failure is reported as itself: an unknown
    # 1.0 operation is still a 404.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "message/send", "params": {}}
      """
    Then the response status code should be 404

    # Discovery is deliberately unversioned: a client commonly fetches the card in order to learn
    # which versions the Agent speaks, so requiring the answer in the question would make the
    # card unreachable to a new client.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== DEPLOY-TIME REJECTIONS ====================

  # Two route collisions are possible and they are caught by different code, so both are
  # exercised. This is the exact-duplicate one: the card path resolves to the same method and
  # path as a generated operation route, so the two produce an identical route key.
  #
  # Two transports sharing a pathPrefix is not a collision and is not tested as one: the JSON-RPC
  # binding is a single POST at the prefix itself, while every HTTP+JSON route sits at least one
  # segment below it.
  @type:negative
  Scenario: A card path identical to an operation route is rejected
    Given I generate a unique resource name from "agent-card-route-duplicate" and store it as "agentName"
    And I generate a unique API context from "/agent-card-route-duplicate" and store it as "agentContext"
    # ListTasks is GET /v1/tasks, and the card is a GET too.
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                             |
      | name               | ${CTX:agentName}                                      |
      | displayName        | Agent Card Route Duplicate                            |
      | context            | ${CTX:agentContext}                                   |
      | upstreamUrl        | http://a2a-trip-planner:9099                          |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","path":"/v1/tasks"}}  |
    Then the response should be a client error
    And the response body should contain "Route collision"

  # The card is served on the Agent's own context, so its path can land on top of a templated
  # operation route. "/tasks/agent-card.json" is not the string "/tasks/{id}", so this is not a
  # duplicate key — Envoy simply hands the request to whichever route matched first, and the
  # card becomes unreachable.
  @type:negative
  Scenario: A card path already matched by an operation route is rejected
    Given I generate a unique resource name from "agent-card-collision" and store it as "agentName"
    And I generate a unique API context from "/agent-card-collision" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                         |
      | name               | ${CTX:agentName}                                                  |
      | displayName        | Agent Card Collision                                              |
      | context            | ${CTX:agentContext}                                               |
      | upstreamUrl        | http://a2a-trip-planner:9099                                      |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/"}]               |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","path":"/tasks/agent-card.json"}} |
    Then the response should be a client error
    And the response body should contain "Route collision"

  @type:negative
  Scenario: An operation name the protocol version does not define is rejected
    Given I generate a unique resource name from "agent-unknown-operation" and store it as "agentName"
    And I generate a unique API context from "/agent-unknown-operation" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                           | ${CTX:gatewaySpecVersion}                                                                                                  |
      | name                                 | ${CTX:agentName}                                                                                                           |
      | displayName                          | Agent Unknown Operation                                                                                                    |
      | context                              | ${CTX:agentContext}                                                                                                        |
      | upstreamUrl                          | http://a2a-trip-planner:9099                                                                                               |
      | transports                           | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}]                                                                           |
      | spec.a2a.operationConfigs.operations | [{"name":"SendMessages","policies":[{"name":"basic-ratelimit","version":"v1","params":{"limits":[{"requests":5,"duration":"1h"}]}}]}] |
      | spec.a2a.agentCard                   | {"public":{"mode":"passthrough"}}                                                                                          |
    Then the response should be a client error
    And the response body should contain "is not an A2A 1.0 operation"

  # Rejected rather than defaulted: an Agent that silently fell back to a different version would
  # enforce an operation set its own card does not advertise.
  @type:negative
  Scenario: An unregistered protocol version is rejected
    Given I generate a unique resource name from "agent-bad-version" and store it as "agentName"
    And I generate a unique API context from "/agent-bad-version" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                        |
      | name                     | ${CTX:agentName}                                 |
      | displayName              | Agent Bad Version                                |
      | context                  | ${CTX:agentContext}                              |
      | upstreamUrl              | http://a2a-trip-planner:9099                     |
      | transports               | [{"protocolBinding":"JSONRPC","pathPrefix":"/"}] |
      | spec.a2a.protocolVersion | "9.9"                                            |
      | spec.a2a.agentCard       | {"public":{"mode":"passthrough"}}                |
    Then the response should be a client error
    And the response body should contain "Unsupported A2A protocol version"

  # Agent Card signing is not implemented yet, so the gateway cannot sign a card, and every
  # section that precedes it must fail closed rather than accept the flag and quietly serve an
  # unsigned card. When signing ships, this scenario is replaced by one asserting a signature is
  # produced.
  @type:negative
  Scenario: Requesting Agent Card signing is rejected while signing is unimplemented
    Given I generate a unique resource name from "agent-signing-requested" and store it as "agentName"
    And I generate a unique API context from "/agent-signing-requested" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | displayName        | Agent Signing Requested                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Signing Requested","description":"An Agent asking for a signature the gateway cannot produce","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]},"signing":{"enabled":true}}} |
    Then the response should be a client error
    And the response body should contain "Agent Card signing is not supported yet"

  # A protected (extended) Agent Card is deployable, and adds no routes and no chains of its own.
  # It is the same GetExtendedAgentCard operation the Agent already exposed, so the route
  # topology must be identical to an Agent without the block — the difference is entirely inside
  # one chain. What the block does at runtime lives in agent_card.feature.
  Scenario: A protected Agent Card block deploys without adding routes
    Given I generate a unique resource name from "agent-protected-card" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-card" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                               |
      | name                               | ${CTX:agentName}                                                        |
      | displayName                        | Agent Protected Card                                                    |
      | context                            | ${CTX:agentContext}                                                     |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                            |
      | transports                         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                   |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough"},"protected":{"mode":"passthrough"}}    |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # The extended-card operation is reachable on exactly the path its binding table defines,
    # and no new one appeared beside it.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200

    # The protected card has no path of its own — it is an operation, not a document at a
    # location — so nothing is served beside the public card route.
    When I send a "GET" request to "${CTX:agentContext}/.well-known/extended-agent-card.json"
    Then the response status code should be 404

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
