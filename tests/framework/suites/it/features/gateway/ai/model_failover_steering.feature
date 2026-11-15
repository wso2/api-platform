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

@model-failover-steering
Feature: A steering policy that publishes only a provider still leads the chain
  Most steering policies in the repo publish a single selection and nothing more. A consumer
  reading one policy's private key sees every other policy's decision as ABSENT and silently
  discards it, publishing its own configured order instead. Nothing fails and nothing is
  logged, because absent is a legitimate state - which is how that stays invisible.

  llm-header-router is the subject because it is the purest case: it writes only the shared
  single-selection key, and it selects from a request header, so this controls the decision
  exactly rather than inferring it from a cost table or a rotation cursor.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A steering policy that publishes only a provider still leads the failover chain
    Given I generate a unique resource name from "steer-primary" and store it as "primaryName"
    And I generate a unique API version from "steer-primary" and store it as "primaryVersion"
    And I generate a unique API context from "/steer-primary" and store it as "primaryContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:primaryName}                         |
      | displayName        | ${CTX:primaryName}                         |
      | version            | ${CTX:primaryVersion}                      |
      | template           | openai                                     |
      | spec.context       | ${CTX:primaryContext}                      |
      | spec.upstream.url  | http://testbench:3000/steer-primary-upstream |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    Given I generate a unique resource name from "steer-chosen" and store it as "chosenName"
    And I generate a unique API version from "steer-chosen" and store it as "chosenVersion"
    And I generate a unique API context from "/steer-chosen" and store it as "chosenContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:chosenName}                           |
      | displayName        | ${CTX:chosenName}                           |
      | version            | ${CTX:chosenVersion}                        |
      | template           | openai                                      |
      | spec.context       | ${CTX:chosenContext}                        |
      | spec.upstream.url  | http://testbench:3000/steer-chosen-upstream |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    # Both providers are HEALTHY, deliberately. If the steering decision is honoured the
    # request goes to the chosen one on attempt 1; if it is discarded it goes to the
    # configured primary on attempt 1. Both return 200, so the status code cannot tell them
    # apart and the body is the discriminator - which is exactly how the bug stayed invisible.
    Given I generate a unique resource name from "steer-proxy" and store it as "proxyName"
    And I generate a unique API version from "steer-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/steer-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:primaryName}                                                                                                                                                     |
      | spec.providers | [{"id":"${CTX:primaryName}","isPrimary":true},{"id":"${CTX:chosenName}","alias":"chosen"}]                                                                                                                             |
      | spec.operationPolicies            | [{"name":"llm-header-router","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"headerName":"x-provider","mappings":[{"headerValue":"chosen","provider":"chosen"}]}}]},{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"chosen"}]}]}}]}] |
    Then the response should be successful

    # The header names the non-primary provider, so the steering policy publishes its
    # selection and failover must lead with it. Polled on the BODY rather than the status,
    # because both outcomes are 200 and only the served path says which one happened - which
    # also means the poll cannot succeed until both policies are live.
    When I set header "Content-Type" to "application/json"
    And I set header "x-provider" to "chosen"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until the response body contains "/steer-chosen-upstream" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # ONE attempt: the steered target led and succeeded, so no failover occurred.
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should not contain "/steer-primary-upstream"

    # CONTROL - a header value matching no mapping. With no default provider configured the
    # policy publishes no selection, so this proves it ran and selected nothing, which is
    # stronger than it not running at all.
    #
    # Without a control, "the body says chosen" could not be told apart from "this proxy
    # always routes to chosen", and the scenario would pass with the steering ignored.
    #
    # Not done by clearing headers: that also drops the controller credential the deletes
    # below need, which would fail the scenario at teardown with a 401.
    When I set header "x-provider" to "no-such-provider"
    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/steer-primary-upstream"
    And the response body should not contain "/steer-chosen-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:primaryName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:chosenName}"
    Then the response should be successful
