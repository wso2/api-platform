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
      | spec.policies           | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"alive-provider"}]}]}}]}] |
    Then the response should be successful

    # The client sees a 200 it never had to retry for, served by the SECOND target.
    # suspendDuration is 0 in every policy here, deliberately. With suspension live, a target
    # that fails once is withdrawn from the chain, so a polling send can observe a LATER request
    # that went straight to the fallback in one attempt - the failover it is meant to prove
    # having happened on an earlier poll that nothing asserted on. Disabling suspension makes
    # every request take the same path, which is what makes the attempt count below mean
    # something. Suspension has its own scenarios.
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

  Scenario: A successful first attempt makes no further attempts
    # Failover must cost nothing when nothing fails. Both providers are healthy, so a second
    # attempt would mean the policy re-attempted a request that had already succeeded - a
    # duplicated inference call, billed, for no reason.
    Given I generate a unique resource name from "failover-first" and store it as "firstName"
    And I generate a unique value from "failover-first" and store it as "firstDisplayName"
    And I generate a unique API version from "failover-first" and store it as "firstVersion"
    And I generate a unique API context from "/failover-first" and store it as "firstContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}            |
      | name               | ${CTX:firstName}                     |
      | displayName        | ${CTX:firstDisplayName}              |
      | version            | ${CTX:firstVersion}                  |
      | template           | openai                               |
      | spec.context       | ${CTX:firstContext}                  |
      | spec.upstream.url  | http://testbench:3000/first-upstream |
      | accessControl.mode | allow_all                            |
    Then the response should be successful

    Given I generate a unique resource name from "failover-second" and store it as "secondName"
    And I generate a unique value from "failover-second" and store it as "secondDisplayName"
    And I generate a unique API version from "failover-second" and store it as "secondVersion"
    And I generate a unique API context from "/failover-second" and store it as "secondContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}             |
      | name               | ${CTX:secondName}                     |
      | displayName        | ${CTX:secondDisplayName}              |
      | version            | ${CTX:secondVersion}                  |
      | template           | openai                                |
      | spec.context       | ${CTX:secondContext}                  |
      | spec.upstream.url  | http://testbench:3000/second-upstream |
      | accessControl.mode | allow_all                             |
    Then the response should be successful

    Given I generate a unique resource name from "failover-healthy" and store it as "healthyProxy"
    And I generate a unique value from "failover-healthy" and store it as "healthyDisplayName"
    And I generate a unique API version from "failover-healthy" and store it as "healthyVersion"
    And I generate a unique API context from "/failover-healthy" and store it as "healthyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:healthyProxy}                                                                                                                                                    |
      | displayName              | ${CTX:healthyDisplayName}                                                                                                                                              |
      | version                  | ${CTX:healthyVersion}                                                                                                                                                  |
      | context                  | ${CTX:healthyContext}                                                                                                                                                  |
      | provider.id              | ${CTX:firstName}                                                                                                                                                       |
      | spec.additionalProviders | [{"id":"${CTX:secondName}","as":"second-provider"}]                                                                                                                    |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"second-provider"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:healthyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/first-upstream"
    And the response body should not contain "/second-upstream"
    # Asserted as a COUNT, not as the header's absence. include_attempt_count_in_response is set
    # per VIRTUAL HOST, so the header appears on every response in the vhost as soon as ANY route
    # in it fails over - and this block runs twelve runners against one gateway. Demanding the
    # header be missing would assert which sibling runners happen to be mid-scenario.
    And the response should report one upstream attempt

    When I delete the LLM proxy "${CTX:healthyProxy}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:firstName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:secondName}"
    Then the response should be successful

  Scenario: Every target failing returns the gateway's own error, not an invented upstream one
    # Both targets are unreachable, so the chain is spent. The client gets ONE sterile
    # failover-exhausted error (FR-014) rather than the last upstream's body, because
    # forwarding a body this gateway never wrote is how an upstream's internal detail
    # reaches a caller who was never meant to see it.
    Given I generate a unique resource name from "failover-deadone" and store it as "deadOneName"
    And I generate a unique value from "failover-deadone" and store it as "deadOneDisplayName"
    And I generate a unique API version from "failover-deadone" and store it as "deadOneVersion"
    And I generate a unique API context from "/failover-deadone" and store it as "deadOneContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                               |
      | name               | ${CTX:deadOneName}                                      |
      | displayName        | ${CTX:deadOneDisplayName}                               |
      | version            | ${CTX:deadOneVersion}                                   |
      | template           | openai                                                  |
      | spec.context       | ${CTX:deadOneContext}                                   |
      | spec.upstream.url  | http://failover-nonexistent-one:9080/dead-one-upstream  |
      | accessControl.mode | allow_all                                               |
    Then the response should be successful

    Given I generate a unique resource name from "failover-deadtwo" and store it as "deadTwoName"
    And I generate a unique value from "failover-deadtwo" and store it as "deadTwoDisplayName"
    And I generate a unique API version from "failover-deadtwo" and store it as "deadTwoVersion"
    And I generate a unique API context from "/failover-deadtwo" and store it as "deadTwoContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                              |
      | name               | ${CTX:deadTwoName}                                     |
      | displayName        | ${CTX:deadTwoDisplayName}                              |
      | version            | ${CTX:deadTwoVersion}                                  |
      | template           | openai                                                 |
      | spec.context       | ${CTX:deadTwoContext}                                  |
      | spec.upstream.url  | http://failover-nonexistent-two:9080/dead-two-upstream |
      | accessControl.mode | allow_all                                              |
    Then the response should be successful

    Given I generate a unique resource name from "failover-exhausted" and store it as "exhaustedProxy"
    And I generate a unique value from "failover-exhausted" and store it as "exhaustedDisplayName"
    And I generate a unique API version from "failover-exhausted" and store it as "exhaustedVersion"
    And I generate a unique API context from "/failover-exhausted" and store it as "exhaustedContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:exhaustedProxy}                                                                                                                                                  |
      | displayName              | ${CTX:exhaustedDisplayName}                                                                                                                                            |
      | version                  | ${CTX:exhaustedVersion}                                                                                                                                                |
      | context                  | ${CTX:exhaustedContext}                                                                                                                                                |
      | provider.id              | ${CTX:deadOneName}                                                                                                                                                     |
      | spec.additionalProviders | [{"id":"${CTX:deadTwoName}","as":"dead-two-provider"}]                                                                                                                 |
      | spec.policies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"dead-two-provider"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:exhaustedContext}/chat/completions" until status 503 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 503
    And the response body should contain "failover_exhausted"
    # x-envoy-attempt-count is deliberately NOT asserted, and its absence is the point rather
    # than an oversight: the exhaustion response REPLACES the upstream's, headers included, so
    # Envoy's own header does not survive it. How many attempts were made is still recorded - in
    # the telemetry record and the gateway log - it simply stops being something the client is told.

    When I delete the LLM proxy "${CTX:exhaustedProxy}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadOneName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadTwoName}"
    Then the response should be successful
