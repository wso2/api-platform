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

# The upstream for every Agent here is a2a-trip-planner: a real A2A agent built
# on the official SDK, not a mock. Its JSON-RPC binding is mounted at "/" and its
# HTTP+JSON binding at "/v1" — the bindings' own defaults — so an Agent's
# pathPrefix values are the ones a real deployment would use. A transport's
# pathPrefix travels upstream with the request; only spec.context is stripped.
#
# Persistence across a controller restart is NOT covered here. Nothing in this
# file restarts anything, so an Agent that is stored but never restored would
# pass every scenario below. That property lives in startup-db-bootstrap.feature.
#
# The conformant calls below are made with the official Go A2A SDK, against an
# agent built on the official Python SDK: two independent implementations of the
# protocol with the gateway between them, rather than two halves of one test
# agreeing with each other. Requests that are deliberately NOT conformant — a
# missing or wrong protocol version, an unknown JSON-RPC method — stay on the raw
# HTTP steps, because a conformant client cannot produce them.
#
# Every SDK client is built from an explicit gateway endpoint. Resolving a
# passthrough card and following the URL inside it would reach the agent
# directly: that card advertises the upstream's own address, so such a test would
# pass having exercised no gateway route at all.

Feature: Agent deployment and A2A routing
  As an API developer
  I want to deploy an Agent and reach its A2A operations
  So that the gateway routes both protocol bindings to the agent behind it

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== CROSS-TRANSPORT INVOCATION ====================

  # One canonical operation, two bindings, one agent behind them. Each binding is
  # driven by its own SDK client so neither can borrow the other's transport, and
  # the two results are compared: reaching the agent over both is not the claim —
  # producing the same protocol result is.
  Scenario: One canonical operation is invoked over both bindings with the official A2A SDK
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-sdk-both
      spec:
        displayName: Agent SDK Both Bindings
        version: v1.0
        context: /agent-sdk-both
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
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding at "http://localhost:8080/agent-sdk-both"
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "http://localhost:8080/agent-sdk-both/v1"

    When the A2A client "rpc" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rpc" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rpc" should have received an artifact containing "Trip plan for Kandy: 3 days"

    When the A2A client "rest" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rest" should have received an artifact containing "Trip plan for Kandy: 3 days"

    # The itineraries are fully determined by the request, so equal artifacts mean
    # both bindings carried the same request to the same agent and brought back
    # the same answer.
    Then the A2A clients "rpc" and "rest" should have received the same artifact

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-sdk-both"
    Then the response should be successful

  # ==================== MANAGEMENT LIFECYCLE ====================

  # The management half of an Agent's life, with a conformant invocation in the
  # middle so "the resource exists" and "the resource is reachable" are both
  # asserted. The 404s at the end are what say a delete removed the routes rather
  # than only the record.
  Scenario: An Agent can be created, listed, fetched, updated, invoked and deleted
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-lifecycle
      spec:
        displayName: Agent Lifecycle
        version: v1.0
        context: /agent-lifecycle
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
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I list all Agents
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "count" should be 1

    When I get the Agent "agent-lifecycle"
    Then the response should be successful
    And the JSON response field "spec.displayName" should be "Agent Lifecycle"

    Given I authenticate using basic auth as "admin"
    When I update the Agent "agent-lifecycle" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-lifecycle
      spec:
        displayName: Agent Lifecycle Updated
        version: v1.0
        context: /agent-lifecycle
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
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I clear all headers
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "http://localhost:8080/agent-lifecycle/v1"
    And the A2A client "rest" sends the message "Plan a 2-day trip to Ella"
    Then the A2A client "rest" should have received a task in state "TASK_STATE_COMPLETED"
    And the A2A client "rest" should have received an artifact containing "Trip plan for Ella: 2 days"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-lifecycle"
    Then the response should be successful

    When I list all Agents
    Then the response should be successful
    And the JSON response field "count" should be 0

    # Both bindings, because a delete that removed one route set and left the
    # other would otherwise pass.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-lifecycle/v1/tasks"
    Then the response status code should be 404

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-lifecycle":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 404

  # ==================== ALL ELEVEN OPERATIONS, BOTH BINDINGS ====================

  # Enumerated rather than described as "the lifecycle", which reads as nine and
  # silently drops the two that do not fit a task's arc — SendStreamingMessage and
  # GetExtendedAgentCard. Both are here for reachability on both bindings only;
  # their semantics belong to agent_streaming.feature and agent_card.feature.
  #
  # The order is dictated by what each operation needs to act on: a completed task
  # for the first group, then a deliberately long-running one, because GetTask,
  # SubscribeToTask and CancelTask against an already-finished task exercise none
  # of the states they exist for.
  Scenario: All eleven A2A 1.0 operations are reachable over both bindings through the official SDK
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-sdk-operations
      spec:
        displayName: Agent SDK Operations
        version: v1.0
        context: /agent-sdk-operations
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
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # ---- HTTP+JSON ----
    When I clear all headers
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "http://localhost:8080/agent-sdk-operations/v1"

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
    When the A2A client "rest" creates a push notification config "rest-push-1" for the task
    Then the A2A client "rest" should have received a push config "rest-push-1"
    When the A2A client "rest" gets the push notification config "rest-push-1" for the task
    Then the A2A client "rest" should have received a push config "rest-push-1"
    When the A2A client "rest" lists push notification configs for the task
    Then the A2A client "rest" should have received 1 push config
    When the A2A client "rest" deletes the push notification config "rest-push-1" for the task
    Then the A2A client "rest" call should have succeeded
    When the A2A client "rest" lists push notification configs for the task
    Then the A2A client "rest" should have received 0 push configs

    # 9. SubscribeToTask — re-attaches to the running task and reads a live event.
    #    Over HTTP+JSON this is a POST, per the binding table's resolution of the
    #    upstream verb disagreement.
    When the A2A client "rest" subscribes to the task and reads 1 event
    Then the A2A client "rest" should have received at least 1 stream event

    # 10. CancelTask — real, because the task is still running.
    When the A2A client "rest" cancels the task
    Then the A2A client "rest" should have received a task in state "TASK_STATE_CANCELED"

    # 11. GetExtendedAgentCard — 200 and a card; which card, and under what
    #     conditions, is agent_card.feature's business.
    When the A2A client "rest" gets the extended Agent Card
    Then the A2A client "rest" call should have succeeded
    And the A2A client "rest" should have received an Agent Card named "Trip Planner"

    # ---- JSON-RPC: the same eleven ----
    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding at "http://localhost:8080/agent-sdk-operations"

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
    When the A2A client "rpc" creates a push notification config "rpc-push-1" for the task
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

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-sdk-operations"
    Then the response should be successful

  # ==================== ROUTES FOLLOW CONFIGURED TRANSPORTS ====================

  # D4: there is one canonical policy chain per operation regardless of which
  # transports are configured, but routes exist only for the transports that are.
  # A gateway that generated routes for both bindings whichever was asked for
  # would still pass every invocation scenario in this file.
  Scenario: An HTTP+JSON-only Agent does not serve the JSON-RPC endpoint
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-restonly
      spec:
        displayName: Agent REST Only
        version: v1.0
        context: /agent-restonly
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # The configured binding routes.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-restonly/v1/tasks"
    Then the response status code should be 200

    # The JSON-RPC endpoint was never generated.
    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-restonly":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-restonly"
    Then the response should be successful

  Scenario: A JSON-RPC-only Agent does not serve the HTTP+JSON routes
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-rpconly
      spec:
        displayName: Agent RPC Only
        version: v1.0
        context: /agent-rpconly
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

    # A JSON-RPC error rides an HTTP 200, so the status alone would pass on a
    # call that reached the agent and failed. The result field is what says the
    # operation actually ran.
    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-rpconly":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response should have field "result"
    And the JSON response field "error" should not exist

    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-rpconly/v1/tasks"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-rpconly"
    Then the response should be successful

  # ==================== RESOLUTION FAILURE ====================

  # A JSON-RPC method the protocol version does not define cannot resolve to a
  # canonical operation, so no policy chain is ever bound and the engine answers
  # with its own sterile response.
  #
  # Asserted as the sterile shape — a 404 with {"error":"Not Found","error_id":...}
  # and an x-error-id header — deliberately NOT as a JSON-RPC error object.
  # Protocol-shaped error rendering is a separate upcoming feature (limitation L2);
  # when it lands this scenario changes and nothing else does. Writing it as a
  # JSON-RPC error today would assert behaviour the gateway does not have.
  Scenario: An unknown JSON-RPC method fails resolution with the sterile response
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-unknown-method
      spec:
        displayName: Agent Unknown Method
        version: v1.0
        context: /agent-unknown-method
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

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-unknown-method":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "NotAnOperation", "params": {}}
      """
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "error" should be "Not Found"
    And the JSON response should have field "error_id"
    And the response header "x-error-id" should exist

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-unknown-method"
    Then the response should be successful

  # ==================== REQUEST PROTOCOL VERSION ====================

  # A2A 1.0 section 3.6.1 makes stating the protocol version a client obligation
  # on every operation request; 3.6.2 fixes what silence means — 0.3, not
  # "whatever the server serves". The gateway enforces that before it resolves an
  # operation, binds a chain, buffers a body or calls the agent, because the
  # Agent's configured version is what selects the operation table a request is
  # interpreted against.
  #
  # Asserted through the gateway rather than through the agent behind it. The
  # reference SDK also rejects a wrong version, so a scenario that only checked
  # "the request failed" would pass with the guard removed entirely. The sterile
  # 400 is the gateway's own answer and the agent's is not — its JSON-RPC binding
  # answers HTTP 200 with a JSON-RPC error object, and its HTTP+JSON binding
  # answers 400 with a FAILED_PRECONDITION body — so the shape is what tells them
  # apart.
  Scenario: An operation request is rejected unless it states the Agent's protocol version
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-version-guard
      spec:
        displayName: Agent Version Guard
        version: v1.0
        context: /agent-version-guard
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
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # The correct version, both bindings: unchanged behaviour, and the request
    # reaches the agent.
    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-version-guard":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-version-guard/v1/tasks"
    Then the response status code should be 200

    # No version at all means 0.3, which this Agent does not expose. The sterile
    # 400 says the gateway refused it — the agent would have answered 200 with a
    # JSON-RPC error on this binding.
    When I clear all headers
    And I send an A2A JSON-RPC request with no version to "http://localhost:8080/agent-version-guard":
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
    And I send an A2A JSON-RPC request with version " " to "http://localhost:8080/agent-version-guard":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400

    # A version the Agent does not expose. There is no range match and no
    # newest-version fallback: one Agent exposes exactly one version.
    When I clear all headers
    And I send an A2A JSON-RPC request with version "0.3" to "http://localhost:8080/agent-version-guard":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400

    # Not canonical Major.Minor. Refused rather than folded onto "1.0".
    When I clear all headers
    And I send an A2A JSON-RPC request with version "1.0.0" to "http://localhost:8080/agent-version-guard":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 400

    # The nine path-known HTTP+JSON operations resolve statically, so they are the
    # ones a misplaced guard would skip while every JSON-RPC assertion above still
    # passed.
    When I clear all headers
    And I send an A2A "GET" request with no version to "http://localhost:8080/agent-version-guard/v1/tasks"
    Then the response status code should be 400

    When I clear all headers
    And I send an A2A "GET" request with version "99.0" to "http://localhost:8080/agent-version-guard/v1/tasks"
    Then the response status code should be 400

    # The version may travel in the query instead, for a client that cannot set
    # headers. Same rules, same answers.
    When I clear all headers
    And I send an A2A "GET" request with no version to "http://localhost:8080/agent-version-guard/v1/tasks?A2A-Version=1.0"
    Then the response status code should be 200

    When I clear all headers
    And I send an A2A "GET" request with no version to "http://localhost:8080/agent-version-guard/v1/tasks?A2A-Version=0.3"
    Then the response status code should be 400

    # Both representations at once must agree. Contradicting themselves is
    # refused even though one of the two values is correct, because which one an
    # intermediary would have kept is not something a client gets to leave open.
    When I clear all headers
    And I send an A2A "GET" request with version "1.0" to "http://localhost:8080/agent-version-guard/v1/tasks?A2A-Version=0.3"
    Then the response status code should be 400

    When I clear all headers
    And I send an A2A "GET" request with version "1.0" to "http://localhost:8080/agent-version-guard/v1/tasks?A2A-Version=1.0"
    Then the response status code should be 200

    # The version is checked before the operation is, so a request that is wrong
    # about both is reported as the version problem: the gateway never reads the
    # body, and a 404 here would mean the guard ran too late.
    When I clear all headers
    And I send an A2A JSON-RPC request with no version to "http://localhost:8080/agent-version-guard":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "message/send", "params": {}}
      """
    Then the response status code should be 400

    # And once the version is right, the body's own failure is reported as
    # itself: an unknown 1.0 operation is still a 404.
    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-version-guard":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "message/send", "params": {}}
      """
    Then the response status code should be 404

    # Discovery is deliberately unversioned: a client commonly fetches the card
    # in order to learn which versions the Agent speaks, so requiring the answer
    # in the question would make the card unreachable to a new client.
    When I clear all headers
    And I send a GET request to "http://localhost:8080/agent-version-guard/.well-known/agent-card.json"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-version-guard"
    Then the response should be successful

  # ==================== DEPLOY-TIME REJECTIONS ====================

  # Two route collisions are possible and they are caught by different code, so
  # both are exercised. This is the exact-duplicate one: the card path resolves to
  # the same method and path as a generated operation route, so the two produce an
  # identical route key.
  #
  # Note that two transports sharing a pathPrefix is NOT a collision and is not
  # tested as one: the JSON-RPC binding is a single POST at the prefix itself,
  # while every HTTP+JSON route sits at least one segment below it, so the two
  # never produce the same path.
  Scenario: A card path identical to an operation route is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-route-duplicate
      spec:
        displayName: Agent Card Route Duplicate
        version: v1.0
        context: /agent-card-route-duplicate
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: passthrough
              # ListTasks is GET /v1/tasks, and the card is a GET too.
              path: /v1/tasks
      """
    Then the response should be a client error
    And the response body should contain "Route collision"

  # The card is served on the Agent's own context, so its path can land on top of
  # a templated operation route. "/tasks/agent-card.json" is not the string
  # "/tasks/{id}", so this is not a duplicate key — Envoy simply hands the request
  # to whichever route matched first, and the card becomes unreachable.
  Scenario: A card path already matched by an operation route is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-collision
      spec:
        displayName: Agent Card Collision
        version: v1.0
        context: /agent-card-collision
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /
          agentCard:
            public:
              mode: passthrough
              path: /tasks/agent-card.json
      """
    Then the response should be a client error
    And the response body should contain "Route collision"

  Scenario: An operation name the protocol version does not define is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-unknown-operation
      spec:
        displayName: Agent Unknown Operation
        version: v1.0
        context: /agent-unknown-operation
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: JSONRPC
                pathPrefix: /
            operations:
              - name: SendMessages
                policies:
                  - name: basic-ratelimit
                    version: v1
                    params:
                      limits:
                        - requests: 5
                          duration: "1h"
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be a client error
    And the response body should contain "is not an A2A 1.0 operation"

  # Rejected rather than defaulted: an Agent that silently fell back to a
  # different version would enforce an operation set its own card does not
  # advertise.
  Scenario: An unregistered protocol version is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-bad-version
      spec:
        displayName: Agent Bad Version
        version: v1.0
        context: /agent-bad-version
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "9.9"
          operationConfigs:
            transports:
              - protocolBinding: JSONRPC
                pathPrefix: /
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be a client error
    And the response body should contain "Unsupported A2A protocol version"

  # Section 15 (signing and JWKS) is deferred. Until it lands the gateway cannot
  # sign a card, and D15 requires every earlier section to fail closed rather than
  # accept the flag and quietly serve an unsigned card. When signing ships, this
  # scenario is replaced by one asserting a signature is produced.
  Scenario: Requesting Agent Card signing is rejected while signing is unimplemented
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-signing-requested
      spec:
        displayName: Agent Signing Requested
        version: v1.0
        context: /agent-signing-requested
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
          agentCard:
            public:
              mode: managed
              content:
                name: Signing Requested
                description: An Agent asking for a signature the gateway cannot produce
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-signing-requested/v1
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
              signing:
                enabled: true
      """
    Then the response should be a client error
    And the response body should contain "Agent Card signing is not supported yet"

  # A protected (extended) Agent Card is deployable, and adds no routes and no
  # chains of its own. It is the same GetExtendedAgentCard operation the Agent
  # already exposed, so the route topology must be identical to an Agent without
  # the block — the difference is entirely inside one chain.
  #
  # What the block *does* at runtime — the authentication guard, local serving, and
  # every rejection its own rules make — lives in agent_card.feature beside the
  # public card's.
  Scenario: A protected Agent Card block deploys without adding routes
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-card
      spec:
        displayName: Agent Protected Card
        version: v1.0
        context: /agent-protected-card
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
          agentCard:
            public:
              mode: passthrough
            protected:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # The extended-card operation is reachable on exactly the path its binding
    # table defines, and no new one appeared beside it.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-card/v1/extendedAgentCard"
    Then the response status code should be 200

    # The protected card has no path of its own — it is an operation, not a
    # document at a location — so nothing is served beside the public card route.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-card/.well-known/extended-agent-card.json"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-card"
    Then the response should be successful
