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

Feature: Agent deployment and A2A routing
  As an API developer
  I want to deploy an Agent and reach its A2A operations
  So that the gateway routes both protocol bindings to the agent behind it

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

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
