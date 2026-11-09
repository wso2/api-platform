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

@model-failover-core
Feature: Model failover core behaviour
  When an AI model target fails, the gateway re-attempts the SAME client request against
  the next target in the chain, so the consuming application receives a successful response
  instead of an upstream failure and never writes retry logic of its own.

  The re-attempt happens inside the proxy: the downstream filter chain runs once, and only
  the upstream filters run again per attempt. "x-envoy-attempt-count" on the response is the
  observable proof of how many upstream attempts the request actually took - one means no
  failover occurred, two means the first target failed and the second answered.

  A dead target is produced by pointing a provider's upstream at a host that does not
  resolve. That is a transport failure: deterministic, and independent of the request -
  unlike a forced status code, which would apply identically to every attempt and so could
  never demonstrate a rescue.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A request to a failing target succeeds on the next target
    # The primary resolves to nothing, so its attempt always fails at the transport layer.
    Given I generate a unique resource name from "failover-dead" and store it as "deadName"
    And I generate a unique value from "failover-dead" and store it as "deadDisplayName"
    And I generate a unique API version from "failover-dead" and store it as "deadVersion"
    And I generate a unique API context from "/failover-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                           |
      | name               | ${CTX:deadName}                                     |
      | displayName        | ${CTX:deadDisplayName}                              |
      | version            | ${CTX:deadVersion}                                  |
      | template           | openai                                              |
      | spec.context       | ${CTX:deadContext}                                  |
      | spec.upstream.url  | http://failover-nonexistent-host:9080/dead-upstream |
      | accessControl.mode | allow_all                                           |
    Then the response should be successful

    # The fallback is healthy and echoes the path it was called on, which is how the
    # assertion below can name WHICH target served the request.
    Given I generate a unique resource name from "failover-alive" and store it as "aliveName"
    And I generate a unique value from "failover-alive" and store it as "aliveDisplayName"
    And I generate a unique API version from "failover-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/failover-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}             |
      | name               | ${CTX:aliveName}                      |
      | displayName        | ${CTX:aliveDisplayName}               |
      | version            | ${CTX:aliveVersion}                   |
      | template           | openai                                |
      | spec.context       | ${CTX:aliveContext}                   |
      | spec.upstream.url  | http://testbench:3000/alive-upstream  |
      | accessControl.mode | allow_all                             |
    Then the response should be successful

    Given I generate a unique resource name from "failover-rescue" and store it as "proxyName"
    And I generate a unique value from "failover-rescue" and store it as "proxyDisplayName"
    And I generate a unique API version from "failover-rescue" and store it as "proxyVersion"
    And I generate a unique API context from "/failover-rescue" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                    | ${CTX:proxyName}                                                                                                                                                       |
      | displayName             | ${CTX:proxyDisplayName}                                                                                                                                                |
      | version                 | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                 | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id             | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive-provider"}]                                                                                                                     |
      | spec.policies           | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}]}}]}] |
    Then the response should be successful

    # The client sees a 200 it never had to retry for, served by the SECOND target.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # TWO attempts is the whole scenario: one would mean no failover happened, and the
    # status code alone cannot tell those apart.
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
