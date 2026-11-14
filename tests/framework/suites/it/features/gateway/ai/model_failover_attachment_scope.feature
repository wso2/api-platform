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

@model-failover-attachment-scope
Feature: Model failover attachment scope
  Where a publisher attaches model-failover decides WHICH operations it covers. It must not
  decide whether failover works at all.

  A chain is keyed by the model the client asked for and never by path, so "fall back from
  gpt-4o wherever this API serves it" is the ordinary intent - which makes "globalPolicies"
  the attachment point a publisher reaches for first. It previously produced a silent no-op:
  the policy ran, cost a round trip per request and forced request-body buffering, while the
  route carried no retry policy and no priority-group plugin for it to act through.

  The attempt count is what separates "attached and working" from "attached and inert" here.
  A 200 alone proves nothing: an inert attachment also returns 200, because the request just
  goes to its normal upstream. Two attempts is the whole claim.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Failover attached to the whole API rescues a request
    # The primary resolves to nothing, so its attempt always fails at the transport layer.
    Given I generate a unique resource name from "fo-scope-dead" and store it as "deadName"
    And I generate a unique value from "fo-scope-dead" and store it as "deadDisplayName"
    And I generate a unique API version from "fo-scope-dead" and store it as "deadVersion"
    And I generate a unique API context from "/fo-scope-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                            |
      | name               | ${CTX:deadName}                                      |
      | displayName        | ${CTX:deadDisplayName}                               |
      | version            | ${CTX:deadVersion}                                   |
      | template           | openai                                               |
      | spec.context       | ${CTX:deadContext}                                   |
      | spec.upstream.url  | http://fo-scope-nonexistent-host:9080/dead-upstream  |
      | accessControl.mode | allow_all                                            |
    Then the response should be successful

    Given I generate a unique resource name from "fo-scope-alive" and store it as "aliveName"
    And I generate a unique value from "fo-scope-alive" and store it as "aliveDisplayName"
    And I generate a unique API version from "fo-scope-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/fo-scope-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:aliveName}                          |
      | displayName        | ${CTX:aliveDisplayName}                   |
      | version            | ${CTX:aliveVersion}                       |
      | template           | openai                                    |
      | spec.context       | ${CTX:aliveContext}                       |
      | spec.upstream.url  | http://testbench:3000/scope-alive-upstream |
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    # The ONLY difference from the core rescue scenario: spec.globalPolicies rather than
    # spec.operationPolicies, and so no "paths" - a global entry carries its params directly.
    Given I generate a unique resource name from "fo-scope-proxy" and store it as "proxyName"
    And I generate a unique value from "fo-scope-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "fo-scope-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/fo-scope-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                                                                                 |
      | name                | ${CTX:proxyName}                                                                                                                                          |
      | displayName         | ${CTX:proxyDisplayName}                                                                                                                                   |
      | version             | ${CTX:proxyVersion}                                                                                                                                       |
      | context             | ${CTX:proxyContext}                                                                                                                                       |
      | provider.id         | ${CTX:deadName}                                                                                                                                           |
      | spec.providers      | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:aliveName}","alias":"scope-alive"}]                                                               |
      | spec.globalPolicies | [{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"scope-alive"}]}]}}] |
    Then the response should be successful

    # suspendDuration is 0 so every request takes the same path - see model_failover_core.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # TWO attempts. One would mean the API-level attachment was accepted and inert - which is
    # exactly what it used to be, and what a 200 on its own cannot distinguish.
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/scope-alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful

  Scenario: An operation's own failover overrides the one attached to the whole API
    # A route carries ONE failover configuration, so the two attachments cannot be merged.
    # Both chains below are valid and point at DIFFERENT fallbacks, so whichever upstream
    # answers names the winner - an assertion neither chain could satisfy by accident.
    Given I generate a unique resource name from "fo-ovr-dead" and store it as "deadName"
    And I generate a unique value from "fo-ovr-dead" and store it as "deadDisplayName"
    And I generate a unique API version from "fo-ovr-dead" and store it as "deadVersion"
    And I generate a unique API context from "/fo-ovr-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                         |
      | name               | ${CTX:deadName}                                   |
      | displayName        | ${CTX:deadDisplayName}                            |
      | version            | ${CTX:deadVersion}                                |
      | template           | openai                                            |
      | spec.upstream.url  | http://fo-ovr-nonexistent-host:9080/dead-upstream |
      | spec.context       | ${CTX:deadContext}                                |
      | accessControl.mode | allow_all                                         |
    Then the response should be successful

    # The fallback the API-level chain names. It must NOT serve.
    Given I generate a unique resource name from "fo-ovr-global" and store it as "globalName"
    And I generate a unique value from "fo-ovr-global" and store it as "globalDisplayName"
    And I generate a unique API version from "fo-ovr-global" and store it as "globalVersion"
    And I generate a unique API context from "/fo-ovr-global" and store it as "globalContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                   |
      | name               | ${CTX:globalName}                           |
      | displayName        | ${CTX:globalDisplayName}                    |
      | version            | ${CTX:globalVersion}                        |
      | template           | openai                                      |
      | spec.context       | ${CTX:globalContext}                        |
      | spec.upstream.url  | http://testbench:3000/global-chain-upstream |
      | accessControl.mode | allow_all                                   |
    Then the response should be successful

    # The fallback the operation's own chain names. This is the one that must serve.
    Given I generate a unique resource name from "fo-ovr-op" and store it as "opName"
    And I generate a unique value from "fo-ovr-op" and store it as "opDisplayName"
    And I generate a unique API version from "fo-ovr-op" and store it as "opVersion"
    And I generate a unique API context from "/fo-ovr-op" and store it as "opContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                      |
      | name               | ${CTX:opName}                                  |
      | displayName        | ${CTX:opDisplayName}                           |
      | version            | ${CTX:opVersion}                               |
      | template           | openai                                         |
      | spec.context       | ${CTX:opContext}                               |
      | spec.upstream.url  | http://testbench:3000/operation-chain-upstream |
      | accessControl.mode | allow_all                                      |
    Then the response should be successful

    Given I generate a unique resource name from "fo-ovr-proxy" and store it as "proxyName"
    And I generate a unique value from "fo-ovr-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "fo-ovr-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/fo-ovr-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion            | ${CTX:gatewaySpecVersion}                                                                                                                                   |
      | name                  | ${CTX:proxyName}                                                                                                                                            |
      | displayName           | ${CTX:proxyDisplayName}                                                                                                                                     |
      | version               | ${CTX:proxyVersion}                                                                                                                                         |
      | context               | ${CTX:proxyContext}                                                                                                                                         |
      | provider.id           | ${CTX:deadName}                                                                                                                                             |
      | spec.providers        | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:globalName}","alias":"global-chain"},{"id":"${CTX:opName}","alias":"operation-chain"}]              |
      | spec.globalPolicies   | [{"name":"model-failover","version":"v0","params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"global-chain"}]}]}}] |
      | spec.operationPolicies | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"operation-chain"}]}]}}]}] |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    # The operation's own chain won.
    And the response body should contain "/operation-chain-upstream"
    # And the API-level chain did not also run. Keeping both instances would not merely pick a
    # winner: each resolves its own chain and each grades the response, and grading is not
    # idempotent, so the duplicate charges one outcome to a target's health twice.
    And the response body should not contain "/global-chain-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:globalName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:opName}"
    Then the response should be successful

  Scenario: Overriding discards the API-level instance rather than running both
    # The override above is visible in routing, so it would still pass with BOTH instances
    # left in the chain - the operation's is the one that routes either way. What a duplicate
    # actually costs is health accounting: the two share a store (keyed by route and by
    # suspension settings, which are identical here), and each grades the same response, so
    # one failed request would be charged to the target twice.
    #
    # suspendAfterFailures is 2, which turns that into an attempt count. Charged once, the
    # target survives the first request and is still in rotation for the second, which takes
    # two attempts. Charged twice, one request suspends it and the second skips it in one
    # attempt. Both chains are identical, so routing cannot account for the difference.
    Given I generate a unique resource name from "fo-dup-dead" and store it as "deadName"
    And I generate a unique value from "fo-dup-dead" and store it as "deadDisplayName"
    And I generate a unique API version from "fo-dup-dead" and store it as "deadVersion"
    And I generate a unique API context from "/fo-dup-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                         |
      | name               | ${CTX:deadName}                                   |
      | displayName        | ${CTX:deadDisplayName}                            |
      | version            | ${CTX:deadVersion}                                |
      | template           | openai                                            |
      | spec.context       | ${CTX:deadContext}                                |
      | spec.upstream.url  | http://fo-dup-nonexistent-host:9080/dead-upstream |
      | accessControl.mode | allow_all                                         |
    Then the response should be successful

    Given I generate a unique resource name from "fo-dup-alive" and store it as "aliveName"
    And I generate a unique value from "fo-dup-alive" and store it as "aliveDisplayName"
    And I generate a unique API version from "fo-dup-alive" and store it as "aliveVersion"
    And I generate a unique API context from "/fo-dup-alive" and store it as "aliveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                |
      | name               | ${CTX:aliveName}                         |
      | displayName        | ${CTX:aliveDisplayName}                  |
      | version            | ${CTX:aliveVersion}                      |
      | template           | openai                                   |
      | spec.context       | ${CTX:aliveContext}                      |
      | spec.upstream.url  | http://testbench:3000/dup-alive-upstream |
      | accessControl.mode | allow_all                                |
    Then the response should be successful

    Given I generate a unique resource name from "fo-dup-proxy" and store it as "proxyName"
    And I generate a unique value from "fo-dup-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "fo-dup-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/fo-dup-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                                   |
      | name                   | ${CTX:proxyName}                                                                                                                                            |
      | displayName            | ${CTX:proxyDisplayName}                                                                                                                                     |
      | version                | ${CTX:proxyVersion}                                                                                                                                         |
      | context                | ${CTX:proxyContext}                                                                                                                                         |
      | provider.id            | ${CTX:deadName}                                                                                                                                             |
      | spec.providers         | [{"id":"${CTX:deadName}","alias":"dup-dead","isPrimary":true},{"id":"${CTX:aliveName}","alias":"dup-alive"}]                                                |
      | spec.globalPolicies    | [{"name":"model-failover","version":"v0","params":{"suspendDuration":300,"suspendAfterFailures":2,"targets":[{"model":"gpt-4o","provider":"dup-dead","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"dup-alive"}]}]}}] |
      | spec.operationPolicies | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":300,"suspendAfterFailures":2,"targets":[{"model":"gpt-4o","provider":"dup-dead","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"dup-alive"}]}]}}]}] |
    Then the response should be successful

    # Request one. Polling here is safe and does not itself move the failure count: until the
    # proxy is live a poll is a 404 or a cold-start 503, neither of which is an upstream
    # failure, and the poll stops at the first response that actually failed over.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until header "x-envoy-attempt-count" is "2" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request two, sent once rather than polled: a poll would retry past the very difference
    # this asserts. Two attempts means the target was still in rotation, so request one was
    # charged exactly once. One attempt would mean a single request had already spent the
    # two-failure budget, which only a duplicate instance can do.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/dup-alive-upstream"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:aliveName}"
    Then the response should be successful
