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

# The central claim of the Agent kind's design is that a canonical operation has
# ONE policy chain, and the two protocol bindings are two ways of reaching it —
# not two configurations that happen to be written the same way.
#
# Two scenarios here are what make that claim falsifiable. The operation-specific
# authorization scenario asserts the same operation is governed identically over
# both bindings, and the rate-limit scenario asserts requests arriving over the
# two bindings draw down ONE quota. If the transformer ever built a chain per
# transport, both would fail; nothing else in the suite would.
#
# Chain order is system policies first, then operationConfigs.policies, then the
# matching operations[].policies — plain concatenation, no dedup and no override,
# so a doubly-attached policy runs twice as two instances.

Feature: Agent policy attachment and enforcement
  As an API administrator
  I want policies attached to an Agent and its operations to be enforced
  So that access to A2A operations is controlled consistently across both bindings

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== AGENT-WIDE AUTHENTICATION ====================

  Scenario: Agent-wide authentication is enforced on both bindings but not on the card route
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-auth
      spec:
        displayName: Agent Auth
        version: v1.0
        context: /agent-auth
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
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
          agentCard:
            public:
              mode: managed
              content:
                name: Managed Auth Card
                description: Discovery is unauthenticated even when operations are not
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: JSONRPC
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-auth
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-auth/v1
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
    Then the response should be successful
    And I wait for policy snapshot sync

    # No credential: rejected on both bindings, before the agent is reached.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-auth/v1/tasks"
    Then the response status code should be 401

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-auth":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 401

    # A valid credential passes on both.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-auth/v1/tasks"
    Then the response status code should be 200

    When I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-auth":
      """
      {"jsonrpc": "2.0", "id": 2, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    # The card route carries no auth policy. A client that cannot yet
    # authenticate has to be able to read the card to discover HOW to — an
    # authenticated discovery document is a bootstrapping deadlock.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-auth/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Managed Auth Card"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-auth"
    Then the response should be successful

  # ==================== OPERATION-SPECIFIC AUTHORIZATION ====================

  # Authentication is Agent-wide; authorization differs per operation. CancelTask
  # carries a second jwt-auth instance requiring a scope that GetTask does not.
  #
  # Both bindings are exercised against BOTH operations. Testing one binding
  # would pass on a gateway that built a separate chain per transport and happened
  # to attach the operation policy to the one under test.
  Scenario: Operation-specific authorization differs between CancelTask and GetTask on both bindings
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-op-authz
      spec:
        displayName: Agent Operation Authz
        version: v1.0
        context: /agent-op-authz
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
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
            operations:
              - name: CancelTask
                policies:
                  - name: jwt-auth
                    version: v1
                    params:
                      issuers:
                        - mock-jwks
                      scopes:
                        allOf:
                          - "trip:cancel"
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # A token WITHOUT the cancel scope. Seed two long-running tasks with it, one
    # per binding, so each cancel attempt has a real task to act on.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read"
    And I set the Authorization header to the JWT token
    And I send an A2A "POST" request to "http://localhost:8080/agent-op-authz/v1/message:send" with:
      """
      {"message": {"messageId": "authz-seed-1", "role": "ROLE_USER", "parts": [{"text": "Plan a 4 day trip to Ella slowly"}]}, "configuration": {"returnImmediately": true}}
      """
    Then the response status code should be 200
    And I save the JSON response field "task.id" as "taskA"

    When I set the Authorization header to the JWT token
    And I send an A2A "POST" request to "http://localhost:8080/agent-op-authz/v1/message:send" with:
      """
      {"message": {"messageId": "authz-seed-2", "role": "ROLE_USER", "parts": [{"text": "Plan a 4 day trip to Ella slowly"}]}, "configuration": {"returnImmediately": true}}
      """
    Then the response status code should be 200
    And I save the JSON response field "task.id" as "taskB"

    # GetTask requires no extra scope — allowed on both bindings.
    When I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-op-authz/v1/tasks/{{taskA}}"
    Then the response status code should be 200

    When I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-op-authz":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetTask", "params": {"id": "{{taskA}}"}}
      """
    Then the response status code should be 200
    And the JSON response should have field "result"

    # CancelTask requires trip:cancel — denied on both bindings with the same token.
    When I set the Authorization header to the JWT token
    And I send an A2A "POST" request to "http://localhost:8080/agent-op-authz/v1/tasks/{{taskA}}:cancel"
    Then the response status code should be 401

    When I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-op-authz":
      """
      {"jsonrpc": "2.0", "id": 2, "method": "CancelTask", "params": {"id": "{{taskA}}"}}
      """
    Then the response status code should be 401

    # With the scope, the same two calls succeed — one task per binding, since
    # cancelling is not idempotent in a way this scenario should rely on.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read trip:cancel"
    And I set the Authorization header to the JWT token
    And I send an A2A "POST" request to "http://localhost:8080/agent-op-authz/v1/tasks/{{taskA}}:cancel"
    Then the response status code should be 200
    And the response body should contain "TASK_STATE_CANCELED"

    When I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-op-authz":
      """
      {"jsonrpc": "2.0", "id": 3, "method": "CancelTask", "params": {"id": "{{taskB}}"}}
      """
    Then the response status code should be 200
    And the response body should contain "TASK_STATE_CANCELED"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-op-authz"
    Then the response should be successful

  # ==================== CROSS-TRANSPORT QUOTA ====================

  # One canonical operation, one chain, one bucket. The limit is attached to
  # ListTasks alone, and the quota is drawn down by requests arriving over BOTH
  # bindings: two over JSON-RPC plus two over HTTP+JSON exhaust a limit of four,
  # and the fifth is rejected whichever binding it arrives on.
  #
  # A gateway that built one chain per transport would give each binding its own
  # bucket, and every request here would succeed.
  Scenario: The same canonical operation shares one rate-limit quota across both bindings
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-shared-quota
      spec:
        displayName: Agent Shared Quota
        version: v1.0
        context: /agent-shared-quota
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
            operations:
              - name: ListTasks
                policies:
                  - name: basic-ratelimit
                    version: v1
                    params:
                      limits:
                        - requests: 4
                          duration: "1h"
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # Two over JSON-RPC.
    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-shared-quota":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200

    When I send an A2A JSON-RPC request to "http://localhost:8080/agent-shared-quota":
      """
      {"jsonrpc": "2.0", "id": 2, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200

    # Two over HTTP+JSON — the same canonical operation, the same bucket.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-shared-quota/v1/tasks"
    Then the response status code should be 200

    When I send an A2A "GET" request to "http://localhost:8080/agent-shared-quota/v1/tasks"
    Then the response status code should be 200

    # The quota is now spent. The fifth request is rejected on either binding.
    When I send an A2A "GET" request to "http://localhost:8080/agent-shared-quota/v1/tasks"
    Then the response status code should be 429

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-shared-quota":
      """
      {"jsonrpc": "2.0", "id": 3, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 429

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-shared-quota"
    Then the response should be successful

  # ==================== CHAIN COMPOSITION ====================

  # D5: operationConfigs.policies then operations[].policies, plain concatenation
  # with no dedup and no override, so a doubly-attached policy runs twice as two
  # instances.
  #
  # Asserted through RESPONSE headers rather than request headers, because the
  # upstream here is a real A2A agent and does not echo what it received — an
  # assertion on request headers would have nothing to read and would reduce to
  # "the call returned 200", which passes whether or not either policy ran.
  #
  # Each instance sets a header only it sets, proving both ran; both also set
  # X-Chain-Order, so its final value names whichever ran last. Two separate
  # facts: that neither instance was dropped, and what the order was.
  Scenario: Agent-wide and operation-level policies both run, in that order
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-chain-order
      spec:
        displayName: Agent Chain Order
        version: v1.0
        context: /agent-chain-order
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: set-headers
                version: v1
                params:
                  response:
                    headers:
                      - name: X-Agent-Wide
                        value: agent-level
                      - name: X-Chain-Order
                        value: agent-level
            operations:
              - name: ListTasks
                policies:
                  - name: set-headers
                    version: v1
                    params:
                      response:
                        headers:
                          - name: X-Operation-Level
                            value: operation-level
                          - name: X-Chain-Order
                            value: operation-level
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-chain-order/v1/tasks"
    Then the response status code should be 200
    # Both instances ran: neither was deduplicated away.
    And the response header "X-Agent-Wide" should be "agent-level"
    And the response header "X-Operation-Level" should be "operation-level"
    # And in that order. set-headers appends rather than replaces, so a header
    # both instances set carries both values in the order they were written, and
    # the assertion reads the first. "agent-level" here means the Agent-wide
    # instance wrote first — which is D5's order. If the operation-level instance
    # ran first this would read "operation-level".
    And the response header "X-Chain-Order" should be "agent-level"

    # An operation with no operation-level policies gets only the Agent-wide
    # instance — the operation policy is scoped to ListTasks, not to the Agent.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-chain-order/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response header "X-Agent-Wide" should be "agent-level"
    And the response header "X-Operation-Level" should not exist
    # Only one instance wrote it here, so this is the sole value rather than the
    # first of two.
    And the response header "X-Chain-Order" should be "agent-level"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-chain-order"
    Then the response should be successful

  Scenario: An Agent referencing a policy that does not exist fails to deploy
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-missing-policy
      spec:
        displayName: Agent Missing Policy
        version: v1.0
        context: /agent-missing-policy
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: no-such-policy
                version: v1
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be a client error

  # ==================== CORS PREFLIGHT ====================

  # Envoy needs a route to match a preflight against before any policy — the cors
  # policy that answers it included — ever runs, so the transformer generates
  # OPTIONS routes alongside the real ones. That includes the card path, which is
  # the one a browser-based client hits first during discovery.
  Scenario: CORS preflight is answered on operation routes and on the card route
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-cors
      spec:
        displayName: Agent CORS
        version: v1.0
        context: /agent-cors
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
              - name: cors
                version: v1
                params:
                  allowedOrigins:
                    - "https://client.example.com"
                  allowedMethods:
                    - GET
                    - POST
                    - DELETE
                    - OPTIONS
                  allowedHeaders:
                    - Content-Type
                    - Authorization
                    - A2A-Version
          agentCard:
            public:
              mode: passthrough
              policies:
                - name: cors
                  version: v1
                  params:
                    allowedOrigins:
                      - "https://client.example.com"
                    allowedMethods:
                      - GET
                      - OPTIONS
                    allowedHeaders:
                      - Content-Type
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # HTTP+JSON operation route.
    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/agent-cors/v1/tasks"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "https://client.example.com"

    # JSON-RPC endpoint.
    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/agent-cors"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "https://client.example.com"

    # Card route.
    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/agent-cors/.well-known/agent-card.json"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "https://client.example.com"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-cors"
    Then the response should be successful

  # The protected (extended) Agent Card is an operation like any other, so the
  # policies attached to it run before it is answered — including in managed mode,
  # where the gateway holds the card and could otherwise be tempted to answer
  # first. The card instance sits at the tail of the chain for exactly this
  # reason: authentication establishes who the caller is, the operation's own
  # policy decides whether they may have this document, and only then are card
  # bytes produced.
  #
  # "gateway_managed_skill" is on the managed protected card and nowhere else, so
  # its absence from a denial is proof no bytes escaped.
  Scenario: Operation-specific authorization runs before a managed protected card is served
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-scope
      spec:
        displayName: Agent Protected Scope
        version: v1.0
        context: /agent-protected-scope
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
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
            operations:
              - name: GetExtendedAgentCard
                policies:
                  - name: jwt-auth
                    version: v1
                    params:
                      issuers:
                        - mock-jwks
                      scopes:
                        allOf:
                          - "trip:extended"
          agentCard:
            public:
              mode: passthrough
            protected:
              mode: managed
              content: {
                "name": "Trip Planner",
                "description": "Plans trips. Gateway-managed extended card.",
                "version": "1.0.0",
                "protocolVersion": "1.0",
                "supportedInterfaces": [
                  {"protocolBinding": "JSONRPC", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-scope"},
                  {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-scope/v1"}
                ],
                "capabilities": {"streaming": true, "extendedAgentCard": true},
                "defaultInputModes": ["text/plain"],
                "defaultOutputModes": ["text/plain"],
                "skills": [{"id": "gateway_managed_skill", "name": "Only on the managed protected card"}]
              }
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # Authenticated, but without the scope the operation requires. Both bindings.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-scope/v1/extendedAgentCard"
    Then the response should be a client error
    And the response body should not contain "gateway_managed_skill"

    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read"
    And I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-scope":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response should be a client error
    And the response body should not contain "gateway_managed_skill"

    # With the scope, the same caller gets the card on both bindings.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read trip:extended"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-scope/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "gateway_managed_skill"

    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read trip:extended"
    And I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-scope":
      """
      {"jsonrpc": "2.0", "id": 9, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "result.name" should be "Trip Planner"
    And the response body should contain "gateway_managed_skill"

    # Another operation is unaffected by the extended card's scope requirement.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "trip:read"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-scope/v1/tasks"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-scope"
    Then the response should be successful

  # A browser sends a preflight with no credentials, by specification. The
  # extended-card path must therefore answer one — otherwise no browser client can
  # reach the operation at all — while the preflight itself carries no card
  # content and grants nothing: the GET that follows still meets the guard.
  #
  # This works because the preflight builder borrows only the resolved `cors`
  # instances from a chain, never the protected-card instance.
  Scenario: A CORS preflight on the extended-card path succeeds without granting the card
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-cors
      spec:
        displayName: Agent Protected CORS
        version: v1.0
        context: /agent-protected-cors
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            policies:
              - name: cors
                version: v1
                params:
                  allowedOrigins:
                    - "https://client.example.com"
                  allowedMethods:
                    - GET
                    - POST
                    - OPTIONS
                  allowedHeaders:
                    - Content-Type
                    - Authorization
                    - A2A-Version
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
          agentCard:
            public:
              mode: passthrough
            protected:
              mode: managed
              content: {
                "name": "Trip Planner",
                "description": "Plans trips. Gateway-managed extended card.",
                "version": "1.0.0",
                "protocolVersion": "1.0",
                "supportedInterfaces": [
                  {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-cors/v1"}
                ],
                "capabilities": {"streaming": true, "extendedAgentCard": true},
                "defaultInputModes": ["text/plain"],
                "defaultOutputModes": ["text/plain"],
                "skills": [{"id": "gateway_managed_skill", "name": "Only on the managed protected card"}]
              }
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # The preflight is answered, carries no credentials, and returns no card.
    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Authorization"
    And I send an OPTIONS request to "http://localhost:8080/agent-protected-cors/v1/extendedAgentCard"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "https://client.example.com"
    And the response body should not contain "gateway_managed_skill"

    # The preflight granted nothing: the real request still meets the guard.
    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-cors/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-cors"
    Then the response should be successful
