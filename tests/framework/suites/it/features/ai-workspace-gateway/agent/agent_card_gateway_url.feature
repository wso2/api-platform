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



# The interface URLs of a rewritten passthrough Agent Card are normally built from the request
# that fetched the card and the Agent's vhost, which is the right address only when the gateway
# is the edge. This gateway is configured with a public base URL
# ([policy_configurations.a2a_v1] gateway_url, agent-card-gateway-url.toml), the setting for a
# gateway behind a load balancer or ingress, and that URL must be what the card advertises.
#
# The setting is gateway-wide, so this feature has a block of its own: agent_card.feature
# asserts the request-derived URLs on a gateway that leaves it unset.
#
# The upstream is a2a-trip-planner, whose own card advertises http://a2a-trip-planner:9099.

@agent-card-gateway-url @AIW-AGENT
Feature: Agent Card rewriting with a configured gateway URL
  As a gateway operator running the gateway behind another entry point
  I want rewritten Agent Card URLs to use the public address I configure
  So that clients configured from the card reach the gateway through that entry point

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # The request below arrives over plaintext http on an authority of its own, and the configured
  # URL differs from it in scheme, host, port and path prefix. The card advertising the configured
  # URL, and neither the request's authority nor the agent's address, is the assertion: the
  # request-derived URL would name agent-card-internal.example.test:8080.
  #
  # The Agent exposes one of the two bindings the upstream advertises, so the interface on the
  # other binding is covered as well: it takes the exposed transport's URL, on the configured
  # base like any other.
  @AIW-AGC-001
  Scenario: A configured gateway URL replaces the request's address in a rewritten Agent Card
    Given I generate a unique resource name from "agent-card-gateway-url" and store it as "agentName"
    And I generate a unique API context from "/agent-card-gateway-url" and store it as "agentContext"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:agentName}                                     |
      | displayName        | Agent Card Gateway URL                               |
      | context            | ${CTX:agentContext}                                  |
      | upstreamUrl        | http://a2a-trip-planner:9099                         |
      | transports         | [{"protocolBinding":"HTTP+JSON","pathPrefix":"/v1"}] |
      | spec.a2a.agentCard | {"public":{"mode":"passthrough","rewriteUrls":true}} |
    Then the response should be successful
    When I clear all headers
    And I set header "A2A-Version" to "1.0"
    And I send a "GET" request to "${CTX:agentContext}/v1/tasks" until status 200

    When I set request host to "agent-card-internal.example.test:8080"
    And I send a "GET" request to "${CTX:agentContext}/.well-known/agent-card.json"
    Then the response status code should be 200
    # The upstream's own card came back, so the absences below are not an empty document.
    And the response body should contain "Trip Planner"
    # The configured base URL, with its scheme, port and path prefix, ahead of the gateway path.
    And the response body should contain "https://agents-public.example.test:9443/edge${CTX:agentContext}/v1"
    # Not the address the request arrived on, and not the agent's own.
    And the response body should not contain "agent-card-internal.example.test"
    And the response body should not contain "a2a-trip-planner:9099"

    Given I reset the request
    And I clear all headers
    And I authenticate using basic auth as "admin"
    When I delete the Agent "${CTX:agentName}"
    Then the response should be successful
