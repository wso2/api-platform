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

@model-failover-suspension
Feature: Taking a failing target out of rotation, and putting it back
  Failing over rescues ONE request. Suspension is what stops every request after it paying
  the same failed attempt first: without it, a provider down for an hour costs a wasted
  attempt on every request for that hour, and the client is charged that latency each time.

  The attempt count is what makes suspension observable. A suspended target that was
  recorded but ignored still produces a 200, so the status code cannot tell the two apart;
  only the count can. It also shows that a skip costs no attempt - were skips to consume the
  budget, a chain of mostly suspended targets would exhaust itself stepping over dead entries.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A suspended target stops being attempted on later requests
    Given I generate a unique resource name from "susp-dead" and store it as "deadName"
    And I generate a unique API version from "susp-dead" and store it as "deadVersion"
    And I generate a unique API context from "/susp-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:deadName}                           |
      | displayName        | ${CTX:deadName}                           |
      | version            | ${CTX:deadVersion}                        |
      | template           | openai                                    |
      | spec.context       | ${CTX:deadContext}                        |
      | spec.upstream.url  | http://susp-nonexistent:9080/dead-upstream|
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    Given I generate a unique resource name from "susp-alive" and store it as "aliveName"
    And I generate a unique API version from "susp-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/susp-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:aliveName}                          |
      | displayName        | ${CTX:aliveName}                          |
      | version            | ${CTX:aliveVersion}                       |
      | template           | openai                                    |
      | spec.context       | ${CTX:aliveContext}                       |
      | spec.upstream.url  | http://testbench:3000/susp-alive-upstream |
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    # One failure suspends, for a window far longer than the scenario, so it cannot expire
    # mid-run and make a pass look like a pass for the wrong reason.
    Given I generate a unique resource name from "susp-proxy" and store it as "proxyName"
    And I generate a unique API version from "susp-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/susp-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # Poll on the attempt count, not on 200, so the request this stops at is necessarily the
    # one that paid the failed attempt and was rescued - the failure that suspends the
    # leading target. A 200 poll could stop on a request that was already being skipped.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/susp-alive-upstream"

    # The next request does NOT pay it. The suspended target is dropped where the chain is
    # resolved, so the request leads with the healthy one and takes ONE attempt.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/susp-alive-upstream"

    # And it stays out: the one after is served the same way, not re-probed.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: A suspension expires and the target is admitted as a probe
    # Suspension has to be temporary. A target taken out permanently on one bad minute never
    # comes back, and a publisher who configured three providers ends up running on one with
    # no indication why.
    #
    # The probe here FAILS - the target is still dead - and that is deliberate. What is
    # pinned is the re-admission, observable on its own as the attempt count going back up.
    # A scenario that also needed the probe to succeed would have to mutate a provider
    # mid-run, and would be testing two things at once.
    Given I generate a unique resource name from "expire-dead" and store it as "deadName"
    And I generate a unique API version from "expire-dead" and store it as "deadVersion"
    And I generate a unique API context from "/expire-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:deadName}                             |
      | displayName        | ${CTX:deadName}                             |
      | version            | ${CTX:deadVersion}                          |
      | template           | openai                                      |
      | spec.context       | ${CTX:deadContext}                          |
      | spec.upstream.url  | http://expire-nonexistent:9080/dead-upstream|
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    Given I generate a unique resource name from "expire-alive" and store it as "aliveName"
    And I generate a unique API version from "expire-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/expire-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:aliveName}                            |
      | displayName        | ${CTX:aliveName}                            |
      | version            | ${CTX:aliveVersion}                         |
      | template           | openai                                      |
      | spec.context       | ${CTX:aliveContext}                         |
      | spec.upstream.url  | http://testbench:3000/expire-alive-upstream |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    # A short window, so it can expire inside one scenario. maxSuspendMultiplier is 1 so the
    # re-suspension the failed probe causes lasts the same 5s rather than doubling - without
    # it the scenario's timing would depend on the escalation ladder.
    Given I generate a unique resource name from "expire-proxy" and store it as "proxyName"
    And I generate a unique API version from "expire-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/expire-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:deadName}                                                                                                                                                        |
      | spec.additionalProviders | [{"id":"${CTX:aliveName}","as":"alive"}]                                                                                                                               |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":5,"suspendAfterFailures":1,"maxSuspendMultiplier":1,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive"}]}]}}]}] |
    Then the response should be successful
    And I wait for policy snapshot sync

    # The failure that suspends it.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Immediately after: suspended, skipped, one attempt is enough. This is sent as a single
    # request rather than a poll because the window is only 5s - a poll would sit inside it
    # and could observe either side of the expiry.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # Past the window. Polling for the count to return to 2 is how the expiry is awaited:
    # the condition IS the re-admission, so the wait ends exactly when the thing under test
    # happens rather than after a guessed interval. Every poll before that is a skipped
    # target answering in one attempt, and a skip is not a failure, so polling cannot itself
    # extend the suspension it is waiting out.
    #
    # Two attempts means the target re-entered the chain and was tried. Had the suspension
    # been permanent this would stay at 1 forever, with the client none the wiser.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/expire-alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
