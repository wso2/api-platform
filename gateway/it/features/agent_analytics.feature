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

# The gateway emits event dimensions; counts, distinct-consumer rollups and
# success rates are computed downstream. So what is asserted here is that each
# dimension a downstream A2A dashboard needs is present and correct — not that
# anything was aggregated.
#
# The A2A block is nested under metadata.a2aAnalytics on the published event,
# which is why it needs its own assertion steps rather than the flat
# metadata-field one.
#
# Two dimensions carry most of the weight. `operation` is stamped by the kernel
# from the bound chain key rather than by anything that re-parsed the request, so
# it names the operation whose policies actually ran. `outcome` is derived from
# the A2A result rather than the HTTP status, because a JSON-RPC error rides a
# 200 and a status-only reading would report a failed invocation as a success.

Feature: Agent analytics event dimensions
  As an operator
  I want A2A invocations to emit correct analytics dimensions
  So that agent traffic can be measured without misreporting what happened

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # The clearest statement of D4 the suite can make: one canonical operation
  # reached over two bindings aggregates to the same operation while remaining
  # distinguishable by transport. If the two bindings ever stopped sharing a
  # canonical operation, the two events would disagree here.
  Scenario: The same operation over both bindings shares an operation dimension and differs by transport
    Given I reset the analytics collector
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-analytics-transport
      spec:
        displayName: Agent Analytics Transport
        version: v1.0
        context: /agent-analytics-transport
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
              mode: managed
              content:
                name: Managed Analytics Card
                description: Card fetches must not be counted as invocations
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: JSONRPC
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-analytics-transport
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-analytics-transport/v1
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

    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-analytics-transport/v1/tasks"
    Then the response status code should be 200

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-analytics-transport":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "ListTasks", "params": {}}
      """
    Then the response status code should be 200

    When I wait 5 seconds for analytics to be published

    # The HTTP+JSON call.
    Then the latest analytics event should have request URI "/agent-analytics-transport/v1/tasks"
    And the latest analytics event should have A2A field "requestType" with value "operation"
    And the latest analytics event should have A2A field "operation" with value "ListTasks"
    And the latest analytics event should have A2A field "transport" with value "HTTP+JSON"
    And the latest analytics event should have A2A field "protocolVersion" with value "1.0"
    And the latest analytics event should have A2A field "outcome" with value "SUCCESS"

    # The JSON-RPC call: same operation, different transport.
    #
    # The URI filter is a substring match, and the JSON-RPC endpoint is the
    # Agent's context itself — which is also a prefix of the HTTP+JSON path. It
    # resolves correctly because the filter takes the most recent match and the
    # JSON-RPC call was made second. Do not reorder the two invocations above
    # without changing this: the filter would then select the HTTP+JSON event
    # and the transport assertion would fail confusingly.
    Then the latest analytics event should have request URI "/agent-analytics-transport"
    And the latest analytics event should have A2A field "requestType" with value "operation"
    And the latest analytics event should have A2A field "operation" with value "ListTasks"
    And the latest analytics event should have A2A field "transport" with value "JSONRPC"
    And the latest analytics event should have A2A field "outcome" with value "SUCCESS"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-analytics-transport"
    Then the response should be successful

  # An Agent serves three shapes of traffic on one context: its operations, its
  # card, and the preflights for both. Only the first is an invocation. A card
  # fetch that arrived carrying an operation and an outcome would let a
  # downstream rollup count a client's card polling as agent traffic, and card
  # polling is frequent — a client re-reads the card to discover how to
  # authenticate before it can invoke anything.
  Scenario: Card fetches and preflights are reported but not shaped like invocations
    Given I reset the analytics collector
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-analytics-card
      spec:
        displayName: Agent Analytics Card
        version: v1.0
        context: /agent-analytics-card
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
                    - A2A-Version
          agentCard:
            public:
              mode: managed
              content:
                name: Managed Analytics Card Only
                description: A card fetch is discovery, not an invocation
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-analytics-card/v1
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

    # Exactly one request, so the assertions below can read "the latest event"
    # without a URI filter.
    #
    # The filter would not work here anyway: the published event's URI is the
    # matched route's TEMPLATE, not the request path — a request to
    # /v1/tasks/abc is reported as /v1/tasks/{id} — and a route with no
    # operation template is reported by context alone. Filtering by the path a
    # scenario sent therefore misses exactly the routes this scenario is about.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-analytics-card/.well-known/agent-card.json"
    Then the response status code should be 200

    When I wait 5 seconds for analytics to be published

    # The card fetch: visible as traffic, but carrying neither an operation nor
    # an outcome, so nothing downstream can roll it in with an invocation.
    Then the latest analytics event should have A2A field "requestType" with value "agentCard"
    And the latest analytics event should not have A2A field "operation"
    And the latest analytics event should not have A2A field "outcome"
    And the latest analytics event should not have A2A field "transport"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-analytics-card"
    Then the response should be successful

  # A preflight is the third shape of traffic on an Agent's context, and the
  # gateway answers it itself. Its own scenario, again so the assertions read the
  # single event this makes.
  Scenario: A CORS preflight is reported as a preflight, not an invocation
    Given I reset the analytics collector
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-analytics-preflight
      spec:
        displayName: Agent Analytics Preflight
        version: v1.0
        context: /agent-analytics-preflight
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
                    - A2A-Version
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I clear all headers
    And I set header "Origin" to "https://client.example.com"
    And I set header "Access-Control-Request-Method" to "GET"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send an OPTIONS request to "http://localhost:8080/agent-analytics-preflight/v1/tasks"
    Then the response status code should be 204

    When I wait 5 seconds for analytics to be published
    Then the latest analytics event should have A2A field "requestType" with value "preflight"
    And the latest analytics event should not have A2A field "operation"
    And the latest analytics event should not have A2A field "outcome"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-analytics-preflight"
    Then the response should be successful

  # A JSON-RPC error travels inside an HTTP 200, so an outcome read from the
  # status alone reports a failed invocation as a success. This is the half of
  # the outcome rule that a status-only reading gets wrong in the optimistic
  # direction; the next scenario is the pessimistic one.
  Scenario: A JSON-RPC error inside a 200 is reported as a failed invocation
    Given I reset the analytics collector
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-analytics-outcome
      spec:
        displayName: Agent Analytics Outcome
        version: v1.0
        context: /agent-analytics-outcome
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
              - name: GetTask
                policies:
                  - name: jwt-auth
                    version: v1
                    params:
                      issuers:
                        - mock-jwks
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # One request, so the assertions read the single event it makes.
    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-analytics-outcome":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "CancelTask", "params": {"id": "no-such-task"}}
      """
    Then the response status code should be 200
    And the JSON response should have field "error"

    When I wait 5 seconds for analytics to be published
    Then the latest analytics event should have A2A field "operation" with value "CancelTask"
    And the latest analytics event should have A2A field "isError" with value "true"
    And the latest analytics event should have A2A field "outcome" with value "FAILURE"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-analytics-outcome"
    Then the response should be successful

  # The pessimistic half: a policy denial arrives as a 401, and a status-only
  # reading blames the agent for a request the gateway refused before the agent
  # ever saw it.
  Scenario: A policy denial is attributed to the gateway rather than to the agent
    Given I reset the analytics collector
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-analytics-denial
      spec:
        displayName: Agent Analytics Denial
        version: v1.0
        context: /agent-analytics-denial
        upstream:
          url: http://a2a-trip-planner:9099
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: HTTP+JSON
                pathPrefix: /v1
            operations:
              - name: GetTask
                policies:
                  - name: jwt-auth
                    version: v1
                    params:
                      issuers:
                        - mock-jwks
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-analytics-denial/v1/tasks/some-task"
    Then the response status code should be 401

    # The operation is still named even though the request never reached the
    # agent — it comes from the bound chain, not from a response.
    When I wait 5 seconds for analytics to be published
    Then the latest analytics event should have A2A field "operation" with value "GetTask"
    And the latest analytics event should have A2A field "outcome" with value "FAILURE"
    And the latest analytics event should have A2A field "failureOrigin" with value "policy"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-analytics-denial"
    Then the response should be successful

  # A managed protected Agent Card is answered by the gateway, exactly as a
  # managed public card is — but it must NOT be reported the same way. The public
  # card is discovery: `requestType: agentCard`, no operation. The protected card
  # is the GetExtendedAgentCard *operation*, reached over a transport, by an
  # authenticated consumer. Reporting it as `agentCard` would move authenticated
  # invocations into the discovery bucket and lose the consumer dimension with
  # them.
  #
  # That the gateway rather than the agent produced the response is not something
  # the event should reflect: the operation ran, its policies ran, and a client
  # received a result.
  Scenario: A locally served protected Agent Card is reported as an operation, not as discovery
    Given I reset the analytics collector
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-analytics-protected
      spec:
        displayName: Agent Analytics Protected
        version: v1.0
        context: /agent-analytics-protected
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
                   "url": "https://localhost:8080/agent-analytics-protected"},
                  {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-analytics-protected/v1"}
                ],
                "capabilities": {"streaming": true, "extendedAgentCard": true},
                "defaultInputModes": ["text/plain"],
                "defaultOutputModes": ["text/plain"],
                "skills": [{"id": "gateway_managed_skill", "name": "Only on the managed protected card"}]
              }
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-analytics-protected/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "gateway_managed_skill"

    When I wait 5 seconds for analytics to be published
    Then the latest analytics event should have A2A field "requestType" with value "operation"
    And the latest analytics event should have A2A field "operation" with value "GetExtendedAgentCard"
    And the latest analytics event should have A2A field "transport" with value "HTTP+JSON"
    And the latest analytics event should have A2A field "outcome" with value "SUCCESS"

    # The same operation over the other binding: same operation dimension, and
    # still not discovery.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-analytics-protected":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200

    When I wait 5 seconds for analytics to be published
    Then the latest analytics event should have A2A field "requestType" with value "operation"
    And the latest analytics event should have A2A field "operation" with value "GetExtendedAgentCard"
    And the latest analytics event should have A2A field "transport" with value "JSONRPC"

    # A refusal at the guard is still the operation, and still attributed to the
    # gateway rather than to the agent — the request never reached it.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-analytics-protected/v1/extendedAgentCard"
    Then the response status code should be 401

    When I wait 5 seconds for analytics to be published
    Then the latest analytics event should have A2A field "requestType" with value "operation"
    And the latest analytics event should have A2A field "operation" with value "GetExtendedAgentCard"
    And the latest analytics event should have A2A field "outcome" with value "FAILURE"
    And the latest analytics event should have A2A field "failureOrigin" with value "policy"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-analytics-protected"
    Then the response should be successful
