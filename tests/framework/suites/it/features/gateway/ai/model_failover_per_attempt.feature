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

@model-failover-per-attempt
Feature: Per-attempt policy chains
  A provider's own policies run on the attempts routed to THAT provider, and on no others.

  This is what makes a cross-vendor fallback work at all: the second provider needs its own
  credential, its own request format and its own headers, and none of those can be applied
  once at the start of the request because at the start nobody knows which provider will
  serve it.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: The rescuing attempt runs the rescuing provider's own policies
    # The primary resolves to nothing, so every attempt to it fails at the transport layer.
    Given I generate a unique resource name from "perattempt-dead" and store it as "deadName"
    And I generate a unique API version from "perattempt-dead" and store it as "deadVersion"
    And I generate a unique API context from "/perattempt-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                             |
      | name               | ${CTX:deadName}                                       |
      | displayName        | ${CTX:deadName}                                       |
      | version            | ${CTX:deadVersion}                                    |
      | template           | openai                                                |
      | spec.context       | ${CTX:deadContext}                                    |
      | spec.upstream.url  | http://perattempt-nonexistent:9080/dead-upstream      |
      | accessControl.mode | allow_all                                             |
    Then the response should be successful

    Given I generate a unique resource name from "perattempt-alive" and store it as "aliveName"
    And I generate a unique API version from "perattempt-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/perattempt-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                       |
      | name               | ${CTX:aliveName}                                |
      | displayName        | ${CTX:aliveName}                                |
      | version            | ${CTX:aliveVersion}                             |
      | template           | openai                                          |
      | spec.context       | ${CTX:aliveContext}                             |
      | spec.upstream.url  | http://testbench:3000/alive-upstream            |
      | accessControl.mode | allow_all                                       |
    Then the response should be successful

    # Only the FALLBACK carries a per-attempt policy. The primary carries none at all, which
    # is what makes the assertion below decisive: a header stamped on this response can only
    # have come from a chain that ran on the second attempt.
    #
    # suspendDuration is 0 so every request takes the same path. With suspension live the dead
    # target is withdrawn after its first failure, and a polling send can then observe a later
    # request that reached the fallback in ONE attempt - no longer the two-attempt rescue this
    # scenario is about.
    Given I generate a unique resource name from "perattempt-proxy" and store it as "proxyName"
    And I generate a unique API version from "perattempt-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/perattempt-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive-provider","policies":[{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"X-Attempt-Provider","value":"alive"}]}}}]}] |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/alive-upstream"
    # The decisive assertion: the second attempt ran the SECOND provider's chain. A chain built
    # once per request would have stamped nothing, because the primary carries no policies.
    And the response body should contain "X-Attempt-Provider"
    And the response body should contain "alive"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
