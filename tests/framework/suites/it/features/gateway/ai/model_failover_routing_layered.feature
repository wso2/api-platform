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

@model-failover-layered
Feature: Failover layered over a routing policy
  A routing policy decides WHICH target leads; failover decides what happens when that target
  fails. The routing policy publishes an ordered ranking once per client request, and failover
  traverses it.

  Every scenario here sends each request individually rather than polling, and that is
  load-bearing rather than stylistic: a rotation advances its cursor once per client request,
  so a polling send would consume turns and the alternation these scenarios measure would stop
  meaning anything. Propagation is awaited with the policy-snapshot sync, which reaches the
  policy engine directly and sends nothing through the gateway.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Failover continues through the routing policy's published order
    # Two providers, one dead and one live, so the rotation visits a failing target on a known
    # turn and a healthy one on the others.
    Given I generate a unique resource name from "layered-dead" and store it as "deadName"
    And I generate a unique API version from "layered-dead" and store it as "deadVersion"
    And I generate a unique API context from "/layered-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                     |
      | name               | ${CTX:deadName}                               |
      | displayName        | ${CTX:deadName}                               |
      | version            | ${CTX:deadVersion}                            |
      | template           | openai                                        |
      | spec.context       | ${CTX:deadContext}                            |
      | spec.upstream.url  | http://layered-nonexistent:9080/dead-upstream |
      | accessControl.mode | allow_all                                     |
    Then the response should be successful

    Given I generate a unique resource name from "layered-alive" and store it as "aliveName"
    And I generate a unique API version from "layered-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/layered-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:aliveName}                     |
      | displayName        | ${CTX:aliveName}                     |
      | version            | ${CTX:aliveVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:aliveContext}                  |
      | spec.upstream.url  | http://testbench:3000/alive-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    # The rotation alternates between a target on the DEAD primary and one on the live
    # provider. model-failover carries no fallbacks of its own, so the only depth available is
    # the ranking the routing policy published - which is exactly what this measures.
    #
    # suspendDuration is 0 so the dead target is never withdrawn. With suspension live the
    # alternation would decay into "always the live one" after the first failure, and the
    # turn-by-turn counts below would stop describing the rotation.
    Given I generate a unique resource name from "layered-order" and store it as "proxyName"
    And I generate a unique API version from "layered-order" and store it as "proxyVersion"
    And I generate a unique API context from "/layered-order" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive-provider"}]                                                                                                                      |
      | spec.policies            | [{"name":"model-round-robin","version":"v1","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"models":[{"model":"gpt-4o"},{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}}]},{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o"}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # The three turns below assert the ALTERNATION, not absolute attempt counts, and that is a
    # correction rather than a weakening. The claim this scenario exists to make is that the
    # routing policy advances its cursor once per CLIENT request and not once per attempt -
    # which alternation proves on its own, whatever position the cursor happened to be in.
    #
    # Absolute counts cannot be asserted here. The route and the policy chain do not become
    # reachable at the same instant, so an early request can reach the rotation while failover
    # is not yet attached, get a 503, and still advance the cursor. Pinning "turn one takes two
    # attempts" therefore asserts a deploy-timing accident. Measured: it took one.
    #
    # With a two-entry rotation the counts can only be 1 or 2, so "the next differs" and "the
    # one after matches" together mean both values occurred - the failover did happen.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And I store the response header "x-envoy-attempt-count" as "turnOneAttempts"

    # The cursor advanced, so this turn leads with the other entry and its attempt count must
    # differ. A rotation that did not advance - or advanced per attempt rather than per request
    # - fails here.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should not be "${CTX:turnOneAttempts}"

    # And back again. Had the cursor advanced twice on the turn that failed over - once per
    # attempt rather than once per request - it would be out of phase by now and this turn
    # would not match turn one.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/alive-upstream"
    And the response header "x-envoy-attempt-count" should be "${CTX:turnOneAttempts}"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
