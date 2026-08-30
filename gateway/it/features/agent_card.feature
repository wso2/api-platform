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

  # D6: the protected (extended) card is not implemented by the gateway.
  # GetExtendedAgentCard is a route like any other operation and is proxied to the
  # agent. The upstream's extended card carries a skill its public card does not
  # ("book_trip"), which is what distinguishes "proxied to the agent" from
  # "answered locally with the public card".
  Scenario: GetExtendedAgentCard is proxied to the upstream on both bindings
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
