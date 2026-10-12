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


# An Agent Card is served one of two ways. A managed card is validated and stored by the
# controller and answered by the gateway itself from the request-header phase, so the request
# never reaches the agent. A passthrough card is the upstream's own document, proxied unparsed.
#
# The managed cards below deliberately carry a name the upstream agent's own card does not
# ("Managed ..."), and the upstream's card carries a name no managed card uses. That is what
# makes "served locally" and "proxied from upstream" separable assertions rather than two ways
# of saying "a card came back".
#
# The upstream for every Agent here is a2a-trip-planner, whose own card advertises its own
# address, http://a2a-trip-planner:9099. That address surviving in, or vanishing from, a
# document the gateway served is what the rewriting scenarios assert on.
#
# A rewritten interface URL names the authority of the request that fetched the card. Where a
# scenario asserts that authority, it fixes it with a request Host so the expected URL can be
# written out exactly; the gateway reads the request's authority either way.
#
# Card signing is NOT covered: signing is deferred, and the gateway currently rejects
# signing.enabled outright (asserted in agent_deploy.feature). When signing lands, this file
# gains the signature and JWKS scenarios.
#
# Card <-> policy security consistency is also not covered, because it is not implemented: the
# bidirectional securitySchemes/securityRequirements checks were decided against. The rejections
# below are the ones the validator actually makes.
#
# Data-plane readiness is polled on an operation route rather than assumed after the management
# call returns, and an operation route behind an authentication policy answering 401 proves its
# chain is bound.

@agent-card
Feature: Agent Card serving
  As an A2A client
  I want to fetch an Agent's card from the gateway
  So that I can discover where and how to invoke the agent

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== MANAGED CARD ====================

  Scenario: A managed Agent Card is served locally with an ETag and answers a conditional GET
    Given I generate a unique resource name from "agent-managed-card" and store it as "agentName"
    And I generate a unique API context from "/agent-managed-card" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | displayName        | Agent Managed Card                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Managed Trip Planner Card","description":"Served by the gateway, not by the agent","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And I store the response header "ETag" as "etag"
    And the response header "Content-Type" should be "application/json"
    And the response header "ETag" should exist
    And the response header "Cache-Control" should exist

    # The stored content is what came back — not the agent's own card. If the request had been
    # proxied, the upstream's card would name "Trip Planner" and this managed name would be
    # absent.
    And the response body should contain "Managed Trip Planner Card"
    And the response body should contain "Served by the gateway, not by the agent"

    # A card is fetched repeatedly by every client that talks to the agent and changes only on
    # redeploy, so the conditional GET is the case it exists for.
    When I set header "If-None-Match" to "${CTX:etag}"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 304
    And the response body should be empty

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  Scenario: A configured card path replaces the default well-known location
    Given I generate a unique resource name from "agent-custom-card-path" and store it as "agentName"
    And I generate a unique API context from "/agent-custom-card-path" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | displayName        | Agent Custom Card Path                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | spec.a2a.agentCard | {"public":{"mode":"managed","path":"/card.json","content":{"name":"Managed Card At A Custom Path","description":"Served somewhere other than the well-known location","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I send a "GET" request to "${CTX:agentContext}/card.json"
    Then the response status code should be 200
    And the response body should contain "Managed Card At A Custom Path"

    # Replaces rather than adds: the default location is not also served.
    When I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 404

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The card's bytes are the contract: the gateway serves the document as supplied and never
  # rewrites it. Byte equality rather than JSON equality on purpose — a re-encoding that is
  # semantically equal still changes what a future signature would be computed over.
  Scenario: Redeploying with changed card content changes the ETag
    Given I generate a unique resource name from "agent-card-etag-change" and store it as "agentName"
    And I generate a unique API context from "/agent-card-etag-change" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | displayName        | Agent Card ETag Change                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Managed Card Before Change","description":"The first version of this card","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Managed Card Before Change"
    And I store the response header "ETag" as "etag"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I update Agent "${CTX:agentName}" from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | displayName        | Agent Card ETag Change                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Managed Card After Change","description":"The second version of this card","version":"2.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be successful

    # Only the updated card names itself "After Change", so its arrival is what says the
    # redeploy reached the gateway.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json" until the response body contains "Managed Card After Change" with body:
      """
      """
    Then the response status code should be 200
    And the response body should contain "Managed Card After Change"
    And the response header "ETag" should exist
    And the response header "ETag" should not be "${CTX:etag}"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== PASSTHROUGH CARD ====================

  # With `rewriteUrls: false` the card body is opaque to the gateway — it is fetched from the
  # upstream and proxied unparsed — so the assertion is byte-identity against what the agent
  # itself serves, fetched directly from the agent's own service.
  #
  # The flag has to be written out: rewriting is the default, because a proxied card advertises
  # the agent's own address and would send every client past the gateway. Opting out is how an
  # author keeps the upstream's exact bytes, signatures included, and accepts that consequence.
  Scenario: A passthrough Agent Card that opts out of rewriting is proxied byte-identically
    Given I generate a unique resource name from "agent-passthrough-card" and store it as "agentName"
    And I generate a unique API context from "/agent-passthrough-card" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                             |
      | name               | ${CTX:agentName}                                      |
      | displayName        | Agent Passthrough Card                                |
      | context            | ${CTX:agentContext}                                   |
      | upstreamUrl        | http://a2a-trip-planner:9099                          |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]  |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","rewriteUrls":false}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # The agent's own card, straight from the agent.
    When I clear all headers
    And I send a "GET" request to the "a2a-trip-planner" service at "/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Trip Planner"
    And I store the response body as "card"

    # The same bytes, through the gateway.
    When I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should be:
      """
      ${CTX:card}
      """

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # `agentCard` is optional, and so is its `public` block. Both omissions resolve to the same
  # configuration an explicit passthrough card resolves to: the upstream's own card, proxied at
  # the well-known discovery path, with its interface URLs pointed at the gateway.
  #
  # This is the shape most Agents have — an agent that is content to publish its own discovery
  # document writes none of this — so it is the configuration whose defaults would be missed most
  # quietly, in both halves. An Agent that generated no card route at all would 404 an A2A
  # client's very first request; one that generated the route but published the agent's own
  # address would send every client that read the card straight past the gateway.
  Scenario: An Agent that configures no Agent Card serves the default rewritten passthrough route
    Given I generate a unique resource name from "agent-no-card-block" and store it as "agentName"
    And I generate a unique API context from "/agent-no-card-block" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                           |
      | name        | ${CTX:agentName}                                                                                    |
      | displayName | Agent No Card Block                                                                                 |
      | context     | ${CTX:agentContext}                                                                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                                                                        |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I set request host to "agent-card-gateway.example.test:8080"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "Trip Planner"
    # Both transports are exposed, so every advertised interface is rewritten and the agent's own
    # address (a2a-trip-planner:9099) is gone from the document.
    And the response body should contain "http://agent-card-gateway.example.test:8080${CTX:agentContext}/v1"
    And the response body should not contain "a2a-trip-planner:9099"

    Given I reset the request
    And I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The same default, reached the other way: an `agentCard` block that configures only the
  # protected representation. The public block is absent, so it defaults, and the protected block
  # being explicit must not change that — the two representations default independently, and an
  # author who configured one has said nothing about the other.
  Scenario: An omitted public block defaults while an explicit protected block stands
    Given I generate a unique resource name from "agent-no-public-block" and store it as "agentName"
    And I generate a unique API context from "/agent-no-public-block" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                           |
      | name                               | ${CTX:agentName}                                                                                    |
      | displayName                        | Agent No Public Block                                                                               |
      | context                            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                        |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                             |
      | spec.a2a.agentCard                 | {"protected":{"mode":"passthrough"}}                                                                |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # The public card is proxied at the default path with its URLs rewritten, and — because the
    # public card policies are its own scope — the Agent-wide auth policy does not apply to it.
    # Discovery has to stay reachable for a client to learn how to authenticate at all.
    When I set request host to "agent-card-gateway.example.test:8080"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "http://agent-card-gateway.example.test:8080${CTX:agentContext}/v1"
    And the response body should not contain "a2a-trip-planner:9099"

    # The explicit protected block still stands: the extended card is guarded.
    When I reset the request
    And I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== PASSTHROUGH URL REWRITING ====================

  # Rewriting is what makes the gateway touch a passthrough card's body, and it is on by default:
  # the upstream agent's own card advertises the agent's own address, so a client configured from
  # an unrewritten card talks to the agent directly — past the gateway, its policies, and its
  # analytics. Each advertised interface URL is replaced by the gateway endpoint serving that
  # protocol binding. The flag is written out here to state what is under test.
  #
  # The scheme is the *original request's*, not the gateway's connection to the agent: here the
  # gateway reaches the agent over plaintext http, so an https request that came back advertising
  # http:// URLs would be a card no TLS client could use. Both schemes are exercised against the
  # same Agent for that reason.
  #
  # Selection is by binding rather than by position: the upstream card lists HTTP+JSON before
  # JSON-RPC while this Agent configures them the other way round, so a rewrite keyed on array
  # position would advertise each endpoint under the wrong protocol — two URLs that both resolve,
  # both wrong.
  Scenario: Passthrough interface URLs are rewritten to the gateway on both schemes
    Given I generate a unique resource name from "agent-card-rewrite" and store it as "agentName"
    And I generate a unique API context from "/agent-card-rewrite" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Card Rewrite                                                                                  |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","rewriteUrls":true}}                                                |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # Over http: both interfaces point at this gateway, on the authority the client named, and
    # the agent's own address is gone from the document.
    When I set request host to "agent-card-gateway.example.test:8080"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "http://agent-card-gateway.example.test:8080${CTX:agentContext}/v1"
    # The agent advertises its own address (a2a-trip-planner:9099) in the card it serves, so its
    # absence is what makes the assertion above cover both interfaces: one unrewritten entry
    # would leave that address in the document.
    And the response body should not contain "a2a-trip-planner:9099"
    And the response body should contain "Trip Planner"

    # A rewritten card depends on the scheme and authority of the request that fetched it, so it
    # must not be stored by a shared cache and must carry no validator identifying the upstream's
    # bytes.
    And the response header "cache-control" should contain "no-store"
    And the response header "etag" should not exist

    # Over https, through the same Agent, with the upstream leg still plaintext.
    When I set request host to "agent-card-gateway.example.test:8443"
    And I send a "GET" request over HTTPS to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "https://agent-card-gateway.example.test:8443${CTX:agentContext}/v1"
    And the response body should not contain "http://agent-card-gateway.example.test:8443"
    And the response body should not contain "a2a-trip-planner:9099"

    Given I reset the request
    And I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # Rewriting covers the transports the Agent exposes, and only those.
  #
  # The upstream agent advertises both bindings; this Agent fronts one of them. The exposed
  # interface is pointed at the gateway, and the other keeps the agent's own address, because the
  # gateway has no endpoint to name for a transport it was not configured to carry. That address
  # surviving in the document is the assertion — and it is also the consequence an operator is
  # accepting: a client that selects that binding talks to the agent directly, outside every
  # policy on this gateway.
  Scenario: Rewriting leaves an interface no configured transport serves alone
    Given I generate a unique resource name from "agent-card-rewrite-partial" and store it as "agentName"
    And I generate a unique API context from "/agent-card-rewrite-partial" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent Card Rewrite Partial                           |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","rewriteUrls":true}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I set request host to "agent-card-gateway.example.test:8080"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    # The exposed transport, pointed at this gateway.
    And the response body should contain "http://agent-card-gateway.example.test:8080${CTX:agentContext}/v1"
    # The unexposed one, exactly as the agent wrote it.
    And the response body should contain "a2a-trip-planner:9099"

    Given I reset the request
    And I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The Agent's configured vhost decides the host a rewritten URL advertises, ahead of the
  # address the client happened to reach the gateway on.
  #
  # This is the one assertion the policy's own tests cannot make: they can fix a vhost on a
  # synthetic request context, but not prove that `spec.vhost` travels from the management API
  # through the controller into the route metadata the policy engine hands the policy. Here the
  # request is dialled at the gateway's own address and carries the vhost as its Host, so the
  # two differ — and the card naming the vhost rather than the dialled address is what shows
  # which one was used.
  #
  # No port is advertised because the Host header names none. A port is taken from the request
  # when it carries one, which is what every other rewriting scenario here exercises.
  Scenario: A configured vhost outranks the address the client dialled
    Given I generate a unique resource name from "agent-card-rewrite-vhost" and store it as "agentName"
    And I generate a unique API context from "/agent-card-rewrite-vhost" and store it as "agentContext"
    And I generate a unique value from "agent-card-vhost" and store it as "vhostName"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion}                                                                           |
      | name        | ${CTX:agentName}                                                                                    |
      | displayName | Agent Card Rewrite Vhost                                                                            |
      | context     | ${CTX:agentContext}                                                                                 |
      | upstreamUrl | http://a2a-trip-planner:9099                                                                        |
      | transports  | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.vhost  | ${CTX:vhostName}.example.com                                                                        |
    Then the response should be successful
    When I clear all headers
    And I set request host to "${CTX:vhostName}.example.com"
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should contain "http://${CTX:vhostName}.example.com${CTX:agentContext}/v1"
    # Neither the agent's own address nor the one this client dialled.
    And the response body should not contain "a2a-trip-planner:9099"
    And the response body should not contain "localhost"
    And the response body should not contain "127.0.0.1"

    Given I reset the request
    And I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The point of rewriting, asserted the way a client experiences it: a client that bootstraps
  # from the card reaches the gateway.
  #
  # This is the only scenario that builds an SDK client by following a card, and it is written so
  # that following an *unrewritten* card fails rather than passes. The Agent carries an
  # authentication policy the upstream agent does not, so a client that ended up at the agent's
  # own address would succeed without a token — and the URL assertion names the address it
  # reached, so the failure says which document sent it there.
  Scenario: A client bootstrapped from a rewritten card reaches the gateway's policies
    Given I generate a unique resource name from "agent-card-rewrite-sdk" and store it as "agentName"
    And I generate a unique API context from "/agent-card-rewrite-sdk" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                           |
      | name                               | ${CTX:agentName}                                                                                    |
      | displayName                        | Agent Card Rewrite SDK                                                                              |
      | context                            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                        |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                             |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough","rewriteUrls":true}}                                                |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding from the Agent Card at "${CTX:agentContext}/.well-known/agent-card.json"
    And I create an A2A client "rest" for the "HTTP+JSON" binding from the Agent Card at "${CTX:agentContext}/.well-known/agent-card.json"
    Then the A2A client "rpc" should be talking to "${CTX:agentContext}"
    And the A2A client "rest" should be talking to "${CTX:agentContext}/v1"

    # Reaching the gateway means reaching its policies. Without a token the gateway refuses; the
    # agent behind it would have answered.
    When I clear all headers
    And the A2A client "rest" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" call should have failed

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And the A2A client "rest" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" call should have succeeded

    When the A2A client "rpc" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rpc" call should have succeeded

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== EXTENDED CARD ====================

  # `agentCard.protected` is optional, and omitting it reads as passthrough — not as
  # "unprotected". The extended card is the more privileged of the two representations by A2A
  # convention, so leaving it open is never the safer reading of an author's silence: requiring
  # the block to be written out before the guard applies would make protection depend on the
  # author knowing the field exists. That is what this scenario pins, and it is why it configures
  # no `protected` block and no authentication policy — with nothing able to authenticate the
  # request, the unconditional guard refuses it, exactly as it does for an explicit passthrough
  # block with no auth policy.
  #
  # "book_trip" is on the upstream's extended card and nowhere else, so its absence is what
  # proves the request stopped at the gateway rather than being proxied to the agent.
  Scenario: An omitted protected block is passthrough, so GetExtendedAgentCard is guarded
    Given I generate a unique resource name from "agent-extended-card" and store it as "agentName"
    And I generate a unique API context from "/agent-extended-card" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Extended Card                                                                                 |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"}}                                                                   |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    # The public card is unaffected by the extended card's guard, and does not carry the extended
    # skill.
    When I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should not contain "book_trip"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # An explicit `protected` block opts into protected-card semantics. In passthrough mode the
  # gateway still proxies the upstream's own extended card, but only for a request one of the
  # Agent's policies authenticated.
  #
  # Both representations opt rewriting out here, so what the upstream sent is what the client
  # gets and the scenario is about the guard alone. The rewriting default is exercised in the
  # scenarios above and below.
  Scenario: An explicit passthrough protected card is authenticated and then proxied unchanged
    Given I generate a unique resource name from "agent-protected-passthrough" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-passthrough" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                                    |
      | name                               | ${CTX:agentName}                                                                                             |
      | displayName                        | Agent Protected Passthrough                                                                                  |
      | context                            | ${CTX:agentContext}                                                                                          |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                                 |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]          |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                                      |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough","rewriteUrls":false},"protected":{"mode":"passthrough","rewriteUrls":false}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # Unauthenticated on both bindings. The request never reaches the agent, so the upstream's
    # unguarded extended card is not what answers.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    # Authenticated: the upstream answers, and nothing rewrote what it said.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "book_trip"
    And the response body should contain "Plans and books trips. Extended card."

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the response body should contain "book_trip"

    # The public card is unaffected: discovery stays reachable without credentials, which is how
    # a client learns how to authenticate at all.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should not contain "book_trip"

    # Client-visible interoperability: the assertions above are about bytes, and a proxied
    # response can be byte-perfect and still unusable if the gateway answered on a binding the
    # client did not ask for or wrapped it wrongly. The official SDK decodes what came back into a
    # typed AgentCard on both bindings, and is refused outright without credentials.
    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding at "${CTX:agentContext}"
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"
    And the A2A client "rest" gets the extended Agent Card
    Then the A2A client "rest" call should have failed

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And the A2A client "rest" gets the extended Agent Card
    Then the A2A client "rest" should have received an Agent Card with the skill "book_trip"

    When the A2A client "rpc" gets the extended Agent Card
    Then the A2A client "rpc" should have received an Agent Card with the skill "book_trip"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # Rewriting on the protected representation. It is a different response with a different
  # shape — the bare card on HTTP+JSON, the card under `result` on JSON-RPC — and it is reached
  # only after the authentication guard, so rewriting must happen on both bindings without
  # weakening the guard.
  #
  # The public card opts out here, which is what makes this a test of independence rather than of
  # the shared default: rewriting is resolved per representation, so turning it off on one must
  # not turn it off on the other.
  #
  # The JSON-RPC half also pins the envelope: the caller's id comes back as the JSON value it
  # sent, because that is what a client correlates responses on.
  Scenario: A protected passthrough card rewrites its interface URLs independently of the public card
    Given I generate a unique resource name from "agent-protected-rewrite" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-rewrite" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                                   |
      | name                               | ${CTX:agentName}                                                                                            |
      | displayName                        | Agent Protected Rewrite                                                                                     |
      | context                            | ${CTX:agentContext}                                                                                         |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                                |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]         |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                                     |
      | spec.a2a.agentCard                 | {"public":{"mode":"passthrough","rewriteUrls":false},"protected":{"mode":"passthrough","rewriteUrls":true}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # The guard is unaffected by rewriting: without credentials nothing is fetched, so there is
    # nothing to rewrite and no card bytes in the answer.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    # Authenticated, HTTP+JSON: the bare extended card, with the agent's own addresses replaced
    # by this gateway's.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I set request host to "agent-card-gateway.example.test:8080"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response body should contain "book_trip"
    And the response body should contain "http://agent-card-gateway.example.test:8080${CTX:agentContext}/v1"
    And the response body should not contain "a2a-trip-planner:9099"
    And the response header "cache-control" should contain "no-store"

    # Authenticated, JSON-RPC: the same card under `result`, with the id echoed back as the JSON
    # value it arrived as.
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 77, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "id" should be greater than 76
    And the response body should contain "book_trip"
    And the response body should contain "http://agent-card-gateway.example.test:8080${CTX:agentContext}/v1"
    And the response body should not contain "a2a-trip-planner:9099"

    # The public card opted rewriting out, so it is still proxied untouched — rewriting is per
    # representation, and the protected card's is not what decides the public one's.
    When I reset the request
    And I clear all headers
    And I send a "GET" request to the "a2a-trip-planner" service at "/.well-known/agent-card.json"
    Then the response status code should be 200
    And I store the response body as "card"

    When I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should be:
      """
      ${CTX:card}
      """

    # Other operations are unaffected: only the GetExtendedAgentCard chain carries the rewrite,
    # so an ordinary invocation is neither buffered differently nor altered.
    When I clear all headers
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And the A2A client "rest" sends the message "Plan a 3-day trip to Kandy"
    Then the A2A client "rest" call should have succeeded
    And the A2A client "rest" should have received an artifact containing "Kandy"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # Managed mode answers locally: the configured card is served by the gateway and the request
  # never reaches the agent. The marker "gateway_managed_skill" appears in neither the upstream's
  # extended card nor either public card, so its presence is proof of which document answered,
  # and "book_trip"'s absence is proof the upstream did not.
  #
  # The two bindings differ only in the wrapper: HTTP+JSON returns the bare card, JSON-RPC
  # returns it under `result` with the caller's own id echoed back as the JSON value it arrived
  # as.
  Scenario: A managed protected card is served locally and binding-correctly
    Given I generate a unique resource name from "agent-protected-managed" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-managed" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion                         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | name                               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | displayName                        | Agent Protected Managed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
      | context                            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | upstreamUrl                        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | transports                         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | spec.a2a.operationConfigs.policies | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
      | spec.a2a.agentCard                 | {"public":{"mode":"managed","content":{"name":"Trip Planner","description":"Plans trips. Public card.","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}]}},"protected":{"mode":"managed","content":{"name":"Trip Planner","description":"Plans trips. Gateway-managed extended card.","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"},{"id":"gateway_managed_skill","name":"Only on the managed protected card"}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 401

    # Unauthenticated: refused before any card bytes exist, on both bindings.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"
    And the response body should not contain "book_trip"

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # HTTP+JSON: the bare card, uncached, and never the upstream's.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 200
    And the response header "Content-Type" should be "application/json"
    And the response header "cache-control" should be "no-store"
    And the response body should contain "gateway_managed_skill"
    And the response body should not contain "book_trip"

    # JSON-RPC with a numeric id: echoed as a number, not stringified.
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 42, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "jsonrpc" should be "2.0"
    # The int form, not the quoted one: it fails if the id came back stringified, which is the
    # whole risk. A client matches responses to requests on this value, so 42 arriving as "42"
    # is a silently broken client.
    And the JSON response field "id" should be 42
    And the JSON response field "result.name" should be "Trip Planner"
    And the response body should contain "gateway_managed_skill"
    And the response body should not contain "book_trip"

    # And a string id: echoed as a string, with the quotes it arrived with.
    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": "req-7", "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 200
    And the JSON response field "id" should be "req-7"

    # The public card is served without credentials and is the other document.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    And the response body should not contain "gateway_managed_skill"
    And the response body should contain "extendedAgentCard"

    # Client-visible interoperability: a locally served card has to satisfy the same client the
    # proxied one does. The gateway builds this response itself — the bare document on HTTP+JSON,
    # the JSON-RPC envelope on the other binding — so it is the response most likely to be shaped
    # subtly wrong, and the official SDK decoding it into a typed AgentCard on both bindings is
    # what says it is not. Refused without credentials, exactly as the raw probes above found.
    When I clear all headers
    And I create an A2A client "rpc" for the "JSONRPC" binding at "${CTX:agentContext}"
    And I create an A2A client "rest" for the "HTTP+JSON" binding at "${CTX:agentContext}/v1"
    And the A2A client "rest" gets the extended Agent Card
    Then the A2A client "rest" call should have failed

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And the A2A client "rest" gets the extended Agent Card
    Then the A2A client "rest" should have received an Agent Card named "Trip Planner"
    And the A2A client "rest" should have received an Agent Card with the skill "gateway_managed_skill"
    And the A2A client "rest" should have received an Agent Card without the skill "book_trip"

    When the A2A client "rpc" gets the extended Agent Card
    Then the A2A client "rpc" should have received an Agent Card named "Trip Planner"
    And the A2A client "rpc" should have received an Agent Card with the skill "gateway_managed_skill"
    And the A2A client "rpc" should have received an Agent Card without the skill "book_trip"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The fail-closed case, and the reason the guard is unconditional rather than configurable:
  # this Agent declares a protected card and attaches NO authentication policy anywhere — not
  # agent-wide, not on the operation.
  #
  # A guard the author had to remember to switch on would make this Agent's extended card public,
  # which is precisely the failure the feature exists to prevent. So it answers 401 instead, on
  # both bindings and in both modes, and the operation is unreachable until an authentication
  # policy is attached.
  #
  # Note what is NOT rejected: the Agent deploys. The gateway cannot tell an author who forgot
  # from one who intends to add the policy later, and refusing to deploy would make the safe
  # outcome unavailable rather than merely closed.
  @type:negative
  Scenario: A protected card with no authentication policy attached is refused at runtime
    Given I generate a unique resource name from "agent-protected-no-auth" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-no-auth" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | displayName        | Agent Protected No Auth                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","content":{"name":"Trip Planner","description":"Plans trips. Gateway-managed extended card.","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}"},{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"gateway_managed_skill","name":"Only on the managed protected card"}]}}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # No credential is offered, and none is configured to validate one. The request is still
    # refused, and carries none of the card it was asking for.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # A credential the Agent has no policy to validate changes nothing: the guard reads what the
    # chain established, not what the caller sent.
    When I clear all headers
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "jwt"
    And I set header "Authorization" to "Bearer ${CTX:jwt}"
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "gateway_managed_skill"

    # The Agent's other operations are untouched — only the extended-card operation is guarded,
    # and only because this Agent asked for it to be.
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks"
    Then the response status code should be 200

    # Neither is the public card: discovery stays open.
    When I clear all headers
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # The same omission in passthrough mode. It matters more here, not less: there is no local
  # card to withhold, so a missing guard would have the gateway fetch the upstream's extended
  # card and hand it to an anonymous caller.
  @type:negative
  Scenario: A passthrough protected card with no authentication policy never reaches the upstream
    Given I generate a unique resource name from "agent-protected-no-auth-pt" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-no-auth-pt" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                           |
      | name               | ${CTX:agentName}                                                                                    |
      | displayName        | Agent Protected No Auth Passthrough                                                                 |
      | context            | ${CTX:agentContext}                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                        |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"passthrough"}}                                |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    # "book_trip" is on the upstream's extended card and nowhere else, so its absence is what
    # proves the request stopped at the gateway.
    When I send a "GET" request to "${CTX:agentContext}/v1/extendedAgentCard"
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    When I send a "POST" request to "${CTX:agentContext}" with body:
      """
      {"jsonrpc": "2.0", "id": 1, "method": "GetExtendedAgentCard", "params": {}}
      """
    Then the response status code should be 401
    And the response body should not contain "book_trip"

    Given I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful

  # ==================== CARD VALIDATION REJECTIONS ====================

  # The match is bidirectional. This half is the quieter one: the JSONRPC route exists and
  # works, but no client discovers it, so the transport looks broken rather than undeclared.
  @type:negative
  Scenario: A managed card that does not advertise a configured transport is rejected
    Given I generate a unique resource name from "agent-card-missing-binding" and store it as "agentName"
    And I generate a unique API context from "/agent-card-missing-binding" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
      | displayName        | Agent Card Missing Binding                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | transports         | [{"protocolBinding":"JSONRPC","pathPrefix":"/"},{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Card Missing A Binding","description":"Advertises only one of the two configured transports","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be a client error
    And the response body should contain "No Agent Card interface advertises protocolBinding 'JSONRPC'"

  @type:negative
  Scenario: A managed card advertising an unconfigured transport is rejected
    Given I generate a unique resource name from "agent-card-extra-binding" and store it as "agentName"
    And I generate a unique API context from "/agent-card-extra-binding" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | displayName        | Agent Card Extra Binding                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Card With An Extra Binding","description":"Advertises a transport the gateway does not serve","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"},{"protocolBinding":"JSONRPC","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be a client error
    And the response body should contain "which is not exposed by spec.a2a.operationConfigs.transports"

  # The host is not validated until the gateway has a configured external URL, the only thing it
  # could be compared against. The path IS validated, and a card advertising the wrong path sends
  # every client somewhere the gateway does not serve.
  @type:negative
  Scenario: A managed card interface URL with the wrong path is rejected
    Given I generate a unique resource name from "agent-card-wrong-path" and store it as "agentName"
    And I generate a unique API context from "/agent-card-wrong-path" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | displayName        | Agent Card Wrong Path                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Card With The Wrong Path","description":"Advertises a path the gateway does not serve this transport at","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com/somewhere-else/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be a client error
    And the response body should contain "but the gateway serves this transport at"

  @type:negative
  Scenario: A managed card interface URL that is not http or https is rejected
    Given I generate a unique resource name from "agent-card-plaintext-url" and store it as "agentName"
    And I generate a unique API context from "/agent-card-plaintext-url" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | displayName        | Agent Card Plaintext URL                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Card With A Non-HTTP URL","description":"Advertises ws rather than http or https","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"ws://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be a client error
    And the response body should contain "must use http or https"

  # The gateway does not serve /{tenant}/... routes, so a card advertising a tenant tells clients
  # to send requests to paths that 404.
  @type:negative
  Scenario: A managed card interface declaring a tenant is rejected
    Given I generate a unique resource name from "agent-card-tenant" and store it as "agentName"
    And I generate a unique API context from "/agent-card-tenant" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | displayName        | Agent Card Tenant                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Card With A Tenant","description":"Advertises a tenant the gateway does not route","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1","tenant":"acme"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}]}}} |
    Then the response should be a client error

  # The gateway owns the signature block. A card arriving with one already in it is either signed
  # by something else — which the gateway would then serve as though it had vouched for it — or a
  # leftover the gateway would silently replace.
  @type:negative
  Scenario: A managed card that arrives pre-signed is rejected
    Given I generate a unique resource name from "agent-card-presigned" and store it as "agentName"
    And I generate a unique API context from "/agent-card-presigned" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | displayName        | Agent Card Pre-signed                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Pre-signed Card","description":"Arrives with a signature block already present","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip","description":"Plans a trip itinerary","tags":["travel"]}],"signatures":[{"protected":"eyJhbGciOiJFUzI1NiJ9","signature":"not-a-real-signature"}]}}} |
    Then the response should be a client error

  # A passthrough card is fetched from the upstream, so anything that would require the gateway
  # to produce one is a contradiction rather than a harmless extra.
  @type:negative
  Scenario: A passthrough Agent Card with inline content is rejected
    Given I generate a unique resource name from "agent-passthrough-with-content" and store it as "agentName"
    And I generate a unique API context from "/agent-passthrough-with-content" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                              |
      | name               | ${CTX:agentName}                                                                                       |
      | displayName        | Agent Passthrough With Content                                                                         |
      | context            | ${CTX:agentContext}                                                                                    |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                           |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                   |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","content":{"name":"Content On A Passthrough Card","version":"1.0.0"}}} |
    Then the response should be a client error
    And the response body should contain "remove content or set mode: managed"

  # rewriteUrls rewrites a document the gateway did not author, so it belongs to a passthrough
  # card and to nothing else. A managed card is authored here and its interfaces are validated
  # against the configured transports, so there is nothing for a rewrite to correct — an accepted
  # flag would silently do nothing.
  #
  # A stated `false` is rejected too, because the flag means nothing in managed mode either way:
  # an author who wrote it believes something about their configuration that is not the case, and
  # they would only find out if they ever flipped it and found it ignored.
  @type:negative
  Scenario Outline: rewriteUrls on a managed Agent Card is rejected: <name>
    Given I generate a unique resource name from "agent-managed-rewrite-<name>" and store it as "agentName"
    And I generate a unique API context from "/agent-managed-rewrite-<name>" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                       |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                |
      | displayName        | Agent Managed Rewrite <name>                                                                                                                                                                                                                                    |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                             |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                    |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                            |
      | spec.a2a.agentCard | {"public":{"mode":"managed","rewriteUrls":<flag>,"content":{"name":"Managed Card With A Rewrite Flag","version":"1.0.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://localhost:8080${CTX:agentContext}/v1"}]}}} |
    Then the response should be a client error
    And the response body should contain "rewriteUrls applies only to a passthrough Agent Card"

    Examples:
      | name     | flag  |
      | enabled  | true  |
      | disabled | false |

  # ==================== PROTECTED CARD VALIDATION REJECTIONS ====================

  # Configuring a protected card is a promise that the extended-card operation exists. A client
  # reads capabilities.extendedAgentCard off the public card and only then calls
  # GetExtendedAgentCard, so a managed public card that does not declare it produces an operation
  # the gateway serves and no conformant client ever asks for.
  @type:negative
  Scenario: A protected card without the public capability declaration is rejected
    Given I generate a unique resource name from "agent-protected-no-capability" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-no-capability" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                  |
      | displayName        | Agent Protected No Capability                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                               |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                              |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Trip Planner","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}]}},"protected":{"mode":"passthrough"}} |
    Then the response should be a client error
    And the response body should contain "capabilities.extendedAgentCard: true"

  # The value must be the boolean true. A quoted "true" is a different JSON value, and it is the
  # card's own bytes that reach clients: one deserializing the card against the A2A model reads a
  # type error, not a capability.
  @type:negative
  Scenario: A public card declaring the extended-card capability as a string is rejected
    Given I generate a unique resource name from "agent-protected-string-capability" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-string-capability" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | displayName        | Agent Protected String Capability                                                                                                                                                                                                                                                                                                                                                                                                                                            |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | spec.a2a.agentCard | {"public":{"mode":"managed","content":{"name":"Trip Planner","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true,"extendedAgentCard":"true"},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}]}},"protected":{"mode":"passthrough"}} |
    Then the response should be a client error
    And the response body should contain "must be a boolean"

  @type:negative
  Scenario: A managed protected Agent Card without content is rejected
    Given I generate a unique resource name from "agent-protected-no-content" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-no-content" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:agentName}                                                 |
      | displayName        | Agent Protected No Content                                       |
      | context            | ${CTX:agentContext}                                              |
      | upstreamUrl        | http://a2a-trip-planner:9099                                     |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]             |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"managed"}} |
    Then the response should be a client error
    And the response body should contain "A managed protected Agent Card requires content"

  # The same contradiction as the public card's: the gateway forwards the operation and never
  # produces a document of its own, so content has nothing to act on.
  @type:negative
  Scenario: A passthrough protected Agent Card with inline content is rejected
    Given I generate a unique resource name from "agent-protected-passthrough-content" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-passthrough-content" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                           |
      | name               | ${CTX:agentName}                                                                                                                                    |
      | displayName        | Agent Protected Passthrough Content                                                                                                                 |
      | context            | ${CTX:agentContext}                                                                                                                                 |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                        |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"passthrough","content":{"name":"Content On A Passthrough Protected Card","version":"1.0.0"}}} |
    Then the response should be a client error
    And the response body should contain "remove content or set mode: managed"

  # The managed-card checks run over the protected document too, and report it as the protected
  # one. Reported against the public content field they would send an author to edit a document
  # that is correct.
  @type:negative
  Scenario: A managed protected card that disagrees with the configured transports is rejected
    Given I generate a unique resource name from "agent-protected-wrong-interface" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-wrong-interface" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | displayName        | Agent Protected Wrong Interface                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                     |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","content":{"name":"Trip Planner","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/elsewhere"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}]}}} |
    Then the response should be a client error
    And the response body should contain "spec.a2a.agentCard.protected.content.supportedInterfaces[0].url"
    And the response body should contain "but the gateway serves this transport at"

  # The gateway writes signatures; a signature already in the document was computed by someone
  # else over a different document. The rule holds for both representations.
  @type:negative
  Scenario: A managed protected card that arrives pre-signed is rejected
    Given I generate a unique resource name from "agent-protected-presigned" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-presigned" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | displayName        | Agent Protected Presigned                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","content":{"name":"Trip Planner","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}],"signatures":[{"protected":"eyJhbGciOiJFUzI1NiJ9","signature":"not-a-real-signature"}]}}} |
    Then the response should be a client error
    And the response body should contain "spec.a2a.agentCard.protected.content.signatures"

  # Agent Card signing is not implemented yet, and stays fail-closed on BOTH representations —
  # they are signed independently, so a protected failure must name the protected field, not the
  # public one the author may not even have written.
  @type:negative
  Scenario: Requesting protected Agent Card signing is rejected while signing is unimplemented
    Given I generate a unique resource name from "agent-protected-signing" and store it as "agentName"
    And I generate a unique API context from "/agent-protected-signing" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
      | name               | ${CTX:agentName}                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
      | displayName        | Agent Protected Signing                                                                                                                                                                                                                                                                                                                                                                                                                                                      |
      | context            | ${CTX:agentContext}                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
      | upstreamUrl        | http://a2a-trip-planner:9099                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}]                                                                                                                                                                                                                                                                                                                                                                                                                         |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough"},"protected":{"mode":"managed","signing":{"enabled":true},"content":{"name":"Trip Planner","version":"1.0.0","protocolVersion":"1.0","supportedInterfaces":[{"protocolBinding":"HTTP+JSON","protocolVersion":"1.0","url":"https://agents.example.com${CTX:agentContext}/v1"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"plan_trip","name":"Plan a trip"}]}}} |
    Then the response should be a client error
    And the response body should contain "spec.a2a.agentCard.protected.signing.enabled"
    And the response body should contain "Agent Card signing is not supported yet"
