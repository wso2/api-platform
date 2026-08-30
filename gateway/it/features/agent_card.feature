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

# An Agent Card is served one of two ways. A managed card is validated and stored
# by the controller and answered by the gateway itself from the request-header
# phase, so the request never reaches the agent. A passthrough card is the
# upstream's own document, proxied unparsed.
#
# The managed cards below deliberately carry a name the upstream agent's own card
# does not ("Managed ..."), and the upstream's card carries a name no managed card
# uses. That is what makes "served locally" and "proxied from upstream" separable
# assertions rather than two ways of saying "a card came back".
#
# Card signing is NOT covered: Section 15 is deferred, and the gateway currently
# rejects signing.enabled outright (asserted in agent_deploy.feature). When
# signing lands, this file gains the signature and JWKS scenarios.
#
# Card <-> policy security consistency is also not covered, because it is not
# implemented: the bidirectional securitySchemes/securityRequirements checks were
# decided against. The rejections below are the ones the validator actually makes.

Feature: Agent Card serving
  As an A2A client
  I want to fetch an Agent's card from the gateway
  So that I can discover where and how to invoke the agent

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== MANAGED CARD ====================

  Scenario: A managed Agent Card is served locally with an ETag and answers a conditional GET
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-managed-card
      spec:
        displayName: Agent Managed Card
        version: v1.0
        context: /agent-managed-card
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
                name: Managed Trip Planner Card
                description: Served by the gateway, not by the agent
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: JSONRPC
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-managed-card
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-managed-card/v1
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
    And I save the Agent Card at "http://localhost:8080/agent-managed-card/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response Content-Type should be "application/json"
    And the response header "ETag" should exist
    And the response header "Cache-Control" should exist

    # The stored content is what came back — not the agent's own card. If the
    # request had been proxied, the upstream's card would name "Trip Planner"
    # and this managed name would be absent.
    And the response body should contain "Managed Trip Planner Card"
    And the response body should contain "Served by the gateway, not by the agent"

    # A card is fetched repeatedly by every client that talks to the agent and
    # changes only on redeploy, so the conditional GET is the case it exists for.
    When I fetch the Agent Card at "http://localhost:8080/agent-managed-card/.well-known/agent-card.json" with the saved ETag
    Then the response status code should be 304
    And the response body should be empty

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-managed-card"
    Then the response should be successful

  Scenario: A configured card path replaces the default well-known location
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-custom-card-path
      spec:
        displayName: Agent Custom Card Path
        version: v1.0
        context: /agent-custom-card-path
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
              path: /card.json
              content:
                name: Managed Card At A Custom Path
                description: Served somewhere other than the well-known location
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-custom-card-path/v1
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
    And I send an A2A "GET" request to "http://localhost:8080/agent-custom-card-path/card.json"
    Then the response status code should be 200
    And the response body should contain "Managed Card At A Custom Path"

    # Replaces rather than adds: the default location is not also served.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-custom-card-path/.well-known/agent-card.json"
    Then the response status code should be 404

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-custom-card-path"
    Then the response should be successful

  # The card's bytes are the contract: the gateway serves the document as
  # supplied and never rewrites it. Byte equality rather than JSON equality on
  # purpose — a re-encoding that is semantically equal still changes what a
  # future signature would be computed over.
  Scenario: Redeploying with changed card content changes the ETag
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-etag-change
      spec:
        displayName: Agent Card ETag Change
        version: v1.0
        context: /agent-card-etag-change
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
                name: Managed Card Before Change
                description: The first version of this card
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-etag-change/v1
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
    And I save the Agent Card at "http://localhost:8080/agent-card-etag-change/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Managed Card Before Change"

    Given I authenticate using basic auth as "admin"
    When I update the Agent "agent-card-etag-change" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-etag-change
      spec:
        displayName: Agent Card ETag Change
        version: v1.0
        context: /agent-card-etag-change
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
                name: Managed Card After Change
                description: The second version of this card
                version: 2.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-etag-change/v1
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
    And I send an A2A "GET" request to "http://localhost:8080/agent-card-etag-change/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Managed Card After Change"
    And the response ETag should differ from the saved ETag

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-card-etag-change"
    Then the response should be successful

  # ==================== PASSTHROUGH CARD ====================

  # In passthrough mode the card body is opaque to the gateway — it is fetched
  # from the upstream and proxied unparsed — so the assertion is byte-identity
  # against what the agent itself serves, fetched directly on its own port.
  Scenario: A passthrough Agent Card is proxied byte-identically from the upstream
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-passthrough-card
      spec:
        displayName: Agent Passthrough Card
        version: v1.0
        context: /agent-passthrough-card
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

    # The agent's own card, straight from the agent.
    When I clear all headers
    And I save the Agent Card at "http://localhost:9099/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Trip Planner"

    # The same bytes, through the gateway.
    When I clear all headers
    Then the Agent Card at "http://localhost:8080/agent-passthrough-card/.well-known/agent-card.json" should be byte-identical to the saved card

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-passthrough-card"
    Then the response should be successful

  # ==================== EXTENDED CARD ====================

  # `agentCard.protected` is optional, and omitting it is NOT the same as
  # configuring passthrough. An Agent written before protected cards existed keeps
  # the behaviour it shipped with — GetExtendedAgentCard is a route like any other
  # operation, proxied to the agent, with no gateway-added authentication guard —
  # so upgrading the controller cannot change what it does. That is what this
  # scenario pins, and it is why it still fetches the card with no credentials.
  #
  # The upstream's extended card carries a skill its public card does not
  # ("book_trip"), which is what distinguishes "proxied to the agent" from
  # "answered locally with the public card".
  Scenario: An omitted protected block leaves GetExtendedAgentCard proxied and unguarded
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-extended-card
      spec:
        displayName: Agent Extended Card
        version: v1.0
        context: /agent-extended-card
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
    And I send an A2A "GET" request to "http://localhost:8080/agent-extended-card/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "book_trip"

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-extended-card":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the response body should contain "book_trip"

    # The public card, by contrast, does not carry the extended skill.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-extended-card/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should not contain "book_trip"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-extended-card"
    Then the response should be successful

  # An explicit `protected` block opts into protected-card semantics. In
  # passthrough mode the gateway still proxies the upstream's own extended card,
  # but only for a request one of the Agent's policies authenticated — and the
  # response it proxies is byte-identical to what the upstream sent, because
  # passthrough adds no card-specific mutation of its own.
  Scenario: An explicit passthrough protected card is authenticated and then proxied unchanged
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-passthrough
      spec:
        displayName: Agent Protected Passthrough
        version: v1.0
        context: /agent-protected-passthrough
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
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # Unauthenticated on both bindings. The request never reaches the agent, so
    # the upstream's unguarded extended card is not what answers.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-passthrough/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-passthrough":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    # Authenticated: the upstream answers, and nothing rewrote what it said.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-passthrough/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "book_trip"
    And the response body should contain "Plans and books trips. Extended card."

    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-passthrough":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the response body should contain "book_trip"

    # The public card is unaffected: discovery stays reachable without
    # credentials, which is how a client learns how to authenticate at all.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-passthrough/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should not contain "book_trip"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-passthrough"
    Then the response should be successful

  # Managed mode answers locally: the configured card is served by the gateway and
  # the request never reaches the agent. The marker "gateway_managed_skill" appears
  # in neither the upstream's extended card nor either public card, so its presence
  # is proof of which document answered, and "book_trip"'s absence is proof the
  # upstream did not.
  #
  # The two bindings differ only in the wrapper: HTTP+JSON returns the bare card,
  # JSON-RPC returns it under `result` with the caller's own id echoed back as the
  # JSON value it arrived as.
  Scenario: A managed protected card is served locally and binding-correctly
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-managed
      spec:
        displayName: Agent Protected Managed
        version: v1.0
        context: /agent-protected-managed
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
              content: {
                "name": "Trip Planner",
                "description": "Plans trips. Public card.",
                "version": "1.0.0",
                "protocolVersion": "1.0",
                "supportedInterfaces": [
                  {"protocolBinding": "JSONRPC", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-managed"},
                  {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-managed/v1"}
                ],
                "capabilities": {"streaming": true, "extendedAgentCard": true},
                "defaultInputModes": ["text/plain"],
                "defaultOutputModes": ["text/plain"],
                "skills": [{"id": "plan_trip", "name": "Plan a trip"}]
              }
            protected:
              mode: managed
              content: {
                "name": "Trip Planner",
                "description": "Plans trips. Gateway-managed extended card.",
                "version": "1.0.0",
                "protocolVersion": "1.0",
                "supportedInterfaces": [
                  {"protocolBinding": "JSONRPC", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-managed"},
                  {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-managed/v1"}
                ],
                "capabilities": {"streaming": true, "extendedAgentCard": true},
                "defaultInputModes": ["text/plain"],
                "defaultOutputModes": ["text/plain"],
                "skills": [
                  {"id": "plan_trip", "name": "Plan a trip"},
                  {"id": "gateway_managed_skill", "name": "Only on the managed protected card"}
                ]
              }
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # Unauthenticated: refused before any card bytes exist, on both bindings.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-managed/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"
    And the response body should not contain "book_trip"

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-managed":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # HTTP+JSON: the bare card, uncached, and never the upstream's.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-managed/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response Content-Type should be "application/json"
    And the response header "cache-control" should be "no-store"
    And the response body should contain "gateway_managed_skill"
    And the response body should not contain "book_trip"

    # JSON-RPC with a numeric id: echoed as a number, not stringified.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-managed":
      """
      {"jsonrpc": "2.0", "id": 42, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "jsonrpc" should be "2.0"
    # The int form, not the quoted one: it fails if the id came back stringified,
    # which is the whole risk. A client matches responses to requests on this
    # value, so 42 arriving as "42" is a silently broken client.
    And the JSON response field "id" should be 42
    And the JSON response field "result.name" should be "Trip Planner"
    And the response body should contain "gateway_managed_skill"
    And the response body should not contain "book_trip"

    # And a string id: echoed as a string, with the quotes it arrived with.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-managed":
      """
      {"jsonrpc": "2.0", "id": "req-7", "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "id" should be "req-7"

    # The public card is served without credentials and is the other document.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-managed/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should not contain "gateway_managed_skill"
    And the response body should contain "extendedAgentCard"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-managed"
    Then the response should be successful

  # The fail-closed case, and the reason the guard is unconditional rather than
  # configurable: this Agent declares a protected card and attaches NO
  # authentication policy anywhere — not agent-wide, not on the operation.
  #
  # A guard the author had to remember to switch on would make this Agent's
  # extended card public, which is precisely the failure the feature exists to
  # prevent. So it answers 401 instead, on both bindings and in both modes, and
  # the operation is unreachable until an authentication policy is attached.
  #
  # Note what is NOT rejected: the Agent deploys. The gateway cannot tell an
  # author who forgot from one who intends to add the policy later, and refusing
  # to deploy would make the safe outcome unavailable rather than merely closed.
  Scenario: A protected card with no authentication policy attached is refused at runtime
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-no-auth
      spec:
        displayName: Agent Protected No Auth
        version: v1.0
        context: /agent-protected-no-auth
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
            protected:
              mode: managed
              content: {
                "name": "Trip Planner",
                "description": "Plans trips. Gateway-managed extended card.",
                "version": "1.0.0",
                "protocolVersion": "1.0",
                "supportedInterfaces": [
                  {"protocolBinding": "JSONRPC", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-no-auth"},
                  {"protocolBinding": "HTTP+JSON", "protocolVersion": "1.0",
                   "url": "https://localhost:8080/agent-protected-no-auth/v1"}
                ],
                "capabilities": {"streaming": true, "extendedAgentCard": true},
                "defaultInputModes": ["text/plain"],
                "defaultOutputModes": ["text/plain"],
                "skills": [{"id": "gateway_managed_skill", "name": "Only on the managed protected card"}]
              }
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # No credential is offered, and none is configured to validate one. The
    # request is still refused, and carries none of the card it was asking for.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-no-auth/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-no-auth":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # A credential the Agent has no policy to validate changes nothing: the guard
    # reads what the chain established, not what the caller sent.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I set the Authorization header to the JWT token
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-no-auth/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # The Agent's other operations are untouched — only the extended-card
    # operation is guarded, and only because this Agent asked for it to be.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-no-auth/v1/tasks"
    Then the response status code should be 200

    # Neither is the public card: discovery stays open.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-no-auth/.well-known/agent-card.json"
    Then the response status code should be 200

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-no-auth"
    Then the response should be successful

  # The same omission in passthrough mode. It matters more here, not less: there
  # is no local card to withhold, so a missing guard would have the gateway fetch
  # the upstream's extended card and hand it to an anonymous caller.
  Scenario: A passthrough protected card with no authentication policy never reaches the upstream
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-no-auth-passthrough
      spec:
        displayName: Agent Protected No Auth Passthrough
        version: v1.0
        context: /agent-protected-no-auth-passthrough
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
            protected:
              mode: passthrough
      """
    Then the response should be successful
    And I wait for policy snapshot sync

    # "book_trip" is on the upstream's extended card and nowhere else, so its
    # absence is what proves the request stopped at the gateway.
    When I clear all headers
    And I send an A2A "GET" request to "http://localhost:8080/agent-protected-no-auth-passthrough/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    When I clear all headers
    And I send an A2A JSON-RPC request to "http://localhost:8080/agent-protected-no-auth-passthrough":
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    Given I authenticate using basic auth as "admin"
    When I delete the Agent "agent-protected-no-auth-passthrough"
    Then the response should be successful

  # ==================== CARD VALIDATION REJECTIONS ====================

  # The match is bidirectional. This half is the quieter one: the JSONRPC route
  # exists and works, but no client discovers it, so the transport looks broken
  # rather than undeclared.
  Scenario: A managed card that does not advertise a configured transport is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-missing-binding
      spec:
        displayName: Agent Card Missing Binding
        version: v1.0
        context: /agent-card-missing-binding
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
                name: Card Missing A Binding
                description: Advertises only one of the two configured transports
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-missing-binding/v1
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
    Then the response should be a client error
    And the response body should contain "No Agent Card interface advertises protocolBinding 'JSONRPC'"

  Scenario: A managed card advertising an unconfigured transport is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-extra-binding
      spec:
        displayName: Agent Card Extra Binding
        version: v1.0
        context: /agent-card-extra-binding
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
                name: Card With An Extra Binding
                description: Advertises a transport the gateway does not serve
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-extra-binding/v1
                  - protocolBinding: JSONRPC
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-extra-binding
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
    Then the response should be a client error
    And the response body should contain "which is not exposed by spec.a2a.operationConfigs.transports"

  # L9: the host is not validated until Section 15 introduces gateway.external_url,
  # the only thing it could be compared against. The path IS validated, and a card
  # advertising the wrong path sends every client somewhere the gateway does not
  # serve.
  Scenario: A managed card interface URL with the wrong path is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-wrong-path
      spec:
        displayName: Agent Card Wrong Path
        version: v1.0
        context: /agent-card-wrong-path
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
                name: Card With The Wrong Path
                description: Advertises a path the gateway does not serve this transport at
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/somewhere-else/v1
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
    Then the response should be a client error
    And the response body should contain "but the gateway serves this transport at"

  Scenario: A managed card interface URL that is not https is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-plaintext-url
      spec:
        displayName: Agent Card Plaintext URL
        version: v1.0
        context: /agent-card-plaintext-url
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
                name: Card With A Plaintext URL
                description: Advertises http rather than https
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: http://agents.example.com/agent-card-plaintext-url/v1
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
    Then the response should be a client error
    And the response body should contain "must use https"

  # The gateway does not serve /{tenant}/... routes, so a card advertising a
  # tenant tells clients to send requests to paths that 404.
  Scenario: A managed card interface declaring a tenant is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-tenant
      spec:
        displayName: Agent Card Tenant
        version: v1.0
        context: /agent-card-tenant
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
                name: Card With A Tenant
                description: Advertises a tenant the gateway does not route
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-tenant/v1
                    tenant: acme
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
    Then the response should be a client error

  # The gateway owns the signature block. A card arriving with one already in it
  # is either signed by something else — which the gateway would then serve as
  # though it had vouched for it — or a leftover the gateway would silently
  # replace.
  Scenario: A managed card that arrives pre-signed is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-card-presigned
      spec:
        displayName: Agent Card Pre-signed
        version: v1.0
        context: /agent-card-presigned
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
                name: Pre-signed Card
                description: Arrives with a signature block already present
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-card-presigned/v1
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
                signatures:
                  - protected: eyJhbGciOiJFUzI1NiJ9
                    signature: not-a-real-signature
      """
    Then the response should be a client error

  # A passthrough card is fetched from the upstream, so anything that would
  # require the gateway to produce one is a contradiction rather than a harmless
  # extra.
  Scenario: A passthrough Agent Card with inline content is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-passthrough-with-content
      spec:
        displayName: Agent Passthrough With Content
        version: v1.0
        context: /agent-passthrough-with-content
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
              content:
                name: Content On A Passthrough Card
                version: 1.0.0
      """
    Then the response should be a client error
    And the response body should contain "remove content or set mode: managed"

  # ==================== PROTECTED CARD VALIDATION REJECTIONS ====================

  # Configuring a protected card is a promise that the extended-card operation
  # exists. A client reads capabilities.extendedAgentCard off the public card and
  # only then calls GetExtendedAgentCard, so a managed public card that does not
  # declare it produces an operation the gateway serves and no conformant client
  # ever asks for.
  Scenario: A protected card without the public capability declaration is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-no-capability
      spec:
        displayName: Agent Protected No Capability
        version: v1.0
        context: /agent-protected-no-capability
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
                name: Trip Planner
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-protected-no-capability/v1
                capabilities:
                  streaming: true
                defaultInputModes:
                  - text/plain
                defaultOutputModes:
                  - text/plain
                skills:
                  - id: plan_trip
                    name: Plan a trip
            protected:
              mode: passthrough
      """
    Then the response should be a client error
    And the response body should contain "capabilities.extendedAgentCard: true"

  # The value must be the boolean true. A quoted "true" is a different JSON value,
  # and it is the card's own bytes that reach clients: one deserializing the card
  # against the A2A model reads a type error, not a capability.
  Scenario: A public card declaring the extended-card capability as a string is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-string-capability
      spec:
        displayName: Agent Protected String Capability
        version: v1.0
        context: /agent-protected-string-capability
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
                name: Trip Planner
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-protected-string-capability/v1
                capabilities:
                  streaming: true
                  extendedAgentCard: "true"
                defaultInputModes:
                  - text/plain
                defaultOutputModes:
                  - text/plain
                skills:
                  - id: plan_trip
                    name: Plan a trip
            protected:
              mode: passthrough
      """
    Then the response should be a client error
    And the response body should contain "must be a boolean"

  Scenario: A managed protected Agent Card without content is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-no-content
      spec:
        displayName: Agent Protected No Content
        version: v1.0
        context: /agent-protected-no-content
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
            protected:
              mode: managed
      """
    Then the response should be a client error
    And the response body should contain "A managed protected Agent Card requires content"

  # The same contradiction as the public card's: the gateway forwards the
  # operation and never produces a document of its own, so content has nothing to
  # act on.
  Scenario: A passthrough protected Agent Card with inline content is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-passthrough-content
      spec:
        displayName: Agent Protected Passthrough Content
        version: v1.0
        context: /agent-protected-passthrough-content
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
            protected:
              mode: passthrough
              content:
                name: Content On A Passthrough Protected Card
                version: 1.0.0
      """
    Then the response should be a client error
    And the response body should contain "remove content or set mode: managed"

  # The managed-card checks run over the protected document too, and report it as
  # the protected one. Reported against the public content field they would send
  # an author to edit a document that is correct.
  Scenario: A managed protected card that disagrees with the configured transports is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-wrong-interface
      spec:
        displayName: Agent Protected Wrong Interface
        version: v1.0
        context: /agent-protected-wrong-interface
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
            protected:
              mode: managed
              content:
                name: Trip Planner
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-protected-wrong-interface/elsewhere
                capabilities:
                  streaming: true
                defaultInputModes:
                  - text/plain
                defaultOutputModes:
                  - text/plain
                skills:
                  - id: plan_trip
                    name: Plan a trip
      """
    Then the response should be a client error
    And the response body should contain "spec.a2a.agentCard.protected.content.supportedInterfaces[0].url"
    And the response body should contain "but the gateway serves this transport at"

  # The gateway writes signatures; a signature already in the document was
  # computed by someone else over a different document. The rule holds for both
  # representations.
  Scenario: A managed protected card that arrives pre-signed is rejected
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-presigned
      spec:
        displayName: Agent Protected Presigned
        version: v1.0
        context: /agent-protected-presigned
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
            protected:
              mode: managed
              content:
                name: Trip Planner
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-protected-presigned/v1
                capabilities:
                  streaming: true
                defaultInputModes:
                  - text/plain
                defaultOutputModes:
                  - text/plain
                skills:
                  - id: plan_trip
                    name: Plan a trip
                signatures:
                  - protected: eyJhbGciOiJFUzI1NiJ9
                    signature: not-a-real-signature
      """
    Then the response should be a client error
    And the response body should contain "spec.a2a.agentCard.protected.content.signatures"

  # Agent Card signing is not implemented yet, and stays fail-closed on BOTH
  # representations — they are signed independently, so a protected failure must
  # name the protected field, not the public one the author may not even have
  # written.
  Scenario: Requesting protected Agent Card signing is rejected while signing is unimplemented
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: agent-protected-signing
      spec:
        displayName: Agent Protected Signing
        version: v1.0
        context: /agent-protected-signing
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
            protected:
              mode: managed
              signing:
                enabled: true
              content:
                name: Trip Planner
                version: 1.0.0
                protocolVersion: "1.0"
                supportedInterfaces:
                  - protocolBinding: HTTP+JSON
                    protocolVersion: "1.0"
                    url: https://agents.example.com/agent-protected-signing/v1
                capabilities:
                  streaming: true
                defaultInputModes:
                  - text/plain
                defaultOutputModes:
                  - text/plain
                skills:
                  - id: plan_trip
                    name: Plan a trip
      """
    Then the response should be a client error
    And the response body should contain "spec.a2a.agentCard.protected.signing.enabled"
    And the response body should contain "Agent Card signing is not supported yet"
