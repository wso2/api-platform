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

@model-failover-per-provider
Feature: Each attempt carries its own provider's transformer and credential
  A fallback on another vendor needs a DIFFERENT request: different path, different body
  shape, different auth header. Adapting once downstream cannot work, because the downstream
  chain runs before any target is chosen - it would have to guess, and either guess breaks
  one of the two attempts. So each provider gets its own per-attempt chain, with its
  credential appended last so no author-written policy can observe it.

  Every upstream here is the testbench echo, so what a response reports IS what that vendor
  received. suspendDuration is 0 throughout: a withdrawn target would change which attempt
  served the request, and these scenarios are about what each attempt carried.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A cross-vendor fallback is sent that vendor's format and credential
    Given I generate a unique resource name from "vendor-x" and store it as "vendorXName"
    And I generate a unique API version from "vendor-x" and store it as "vendorXVersion"
    And I generate a unique API context from "/vendor-x" and store it as "vendorXContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:vendorXName}                         |
      | displayName        | ${CTX:vendorXName}                         |
      | version            | ${CTX:vendorXVersion}                      |
      | template           | openai                                     |
      | spec.context       | ${CTX:vendorXContext}                      |
      | spec.upstream.url  | http://vendor-x-nonexistent:9080/vendor-x  |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    Given I generate a unique resource name from "vendor-y" and store it as "vendorYName"
    And I generate a unique API version from "vendor-y" and store it as "vendorYVersion"
    And I generate a unique API context from "/vendor-y" and store it as "vendorYContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:vendorYName}                        |
      | displayName        | ${CTX:vendorYName}                        |
      | version            | ${CTX:vendorYVersion}                     |
      | template           | openai                                    |
      | spec.context       | ${CTX:vendorYContext}                     |
      | spec.upstream.url  | http://testbench:3000/vendor-y-upstream   |
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    # Vendor Y carries its own transformer and its own credential, both appended by the
    # controller to that provider's per-attempt chain.
    Given I generate a unique resource name from "xvendor" and store it as "proxyName"
    And I generate a unique API version from "xvendor" and store it as "proxyVersion"
    And I generate a unique API context from "/xvendor" and store it as "proxyContext"
    And I generate a unique value from "xvendor-credential" and store it as "vendorYCredential"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:vendorXName}                                                                                                                                                     |
      | spec.providers | [{"id":"${CTX:vendorXName}","isPrimary":true},{"id":"${CTX:vendorYName}","alias":"vendor-y","auth":{"type":"api-key","header":"x-api-key","value":"${CTX:vendorYCredential}"},"transformer":{"type":"openai-to-anthropic-transformer","version":"v0"}}] |
      | spec.operationPolicies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"vendor-y"}]}]}}]}] |
    Then the response should be successful

    # The client sends OpenAI's shape and gets a success.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # The transformer rewrote the PATH for the second vendor. /chat/completions arriving at
    # vendor Y would mean no adaptation happened.
    And the response body should contain "/vendor-y-upstream/v1/messages"

    # And the BODY: max_tokens is required by the second vendor and has no equivalent in
    # what the client sent, so its presence cannot come from the original request being
    # forwarded unchanged.
    And the response body should contain "max_tokens"
    And the response body should contain "claude-sonnet-4-5-20250929"

    # And the credential for THAT provider, applied on the attempt that reached it.
    And the response body should contain "${CTX:vendorYCredential}"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:vendorXName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:vendorYName}"
    Then the response should be successful

  Scenario: A provider's credential and transformer reach only the attempt routed to it
    # The scenario above proves the positive half: the attempt routed to vendor Y carried
    # Y's transformer and Y's credential. That alone does not establish isolation - a chain
    # that ran on EVERY attempt would satisfy it just as well. This asserts the negative
    # half in both directions, which is where the security property actually lives:
    #
    #   1. a credential belonging to the attempt that FAILED must not travel to the attempt
    #      that succeeds, and
    #   2. a provider configured on the proxy but not selected must not have its chain run
    #      on somebody else's attempt.
    #
    # Both failures are silent. The client gets its 200 either way, the transformer reports
    # success either way, and the only trace is a credential sitting in a request body that
    # a different vendor received - which is to say, a key handed to a third party.
    #
    # The dead provider is credentialed on purpose: its chain RUNS, injecting the credential
    # before the dial is attempted, and only then does the dial fail. That is exactly the
    # state from which a leak into the second attempt could happen.
    Given I generate a unique resource name from "iso-dead" and store it as "deadName"
    And I generate a unique API version from "iso-dead" and store it as "deadVersion"
    And I generate a unique API context from "/iso-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                 |
      | name               | ${CTX:deadName}                           |
      | displayName        | ${CTX:deadName}                           |
      | version            | ${CTX:deadVersion}                        |
      | template           | openai                                    |
      | spec.context       | ${CTX:deadContext}                        |
      | spec.upstream.url  | http://iso-nonexistent:9080/iso-dead      |
      | accessControl.mode | allow_all                                 |
    Then the response should be successful

    Given I generate a unique resource name from "iso-vendor-b" and store it as "vendorBName"
    And I generate a unique API version from "iso-vendor-b" and store it as "vendorBVersion"
    And I generate a unique API context from "/iso-vendor-b" and store it as "vendorBContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}             |
      | name               | ${CTX:vendorBName}                    |
      | displayName        | ${CTX:vendorBName}                    |
      | version            | ${CTX:vendorBVersion}                 |
      | template           | openai                                |
      | spec.context       | ${CTX:vendorBContext}                 |
      | spec.upstream.url  | http://testbench:3000/vendor-b-upstream |
      | accessControl.mode | allow_all                             |
    Then the response should be successful

    # Vendor C is the proxy's main provider and carries NO transformer and no credential of
    # its own, so anything vendor-specific appearing in what C received came from a chain
    # that had no business running.
    Given I generate a unique resource name from "iso-vendor-c" and store it as "vendorCName"
    And I generate a unique API version from "iso-vendor-c" and store it as "vendorCVersion"
    And I generate a unique API context from "/iso-vendor-c" and store it as "vendorCContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}               |
      | name               | ${CTX:vendorCName}                      |
      | displayName        | ${CTX:vendorCName}                      |
      | version            | ${CTX:vendorCVersion}                   |
      | template           | openai                                  |
      | spec.context       | ${CTX:vendorCContext}                   |
      | spec.upstream.url  | http://testbench:3000/vendor-c-upstream |
      | accessControl.mode | allow_all                               |
    Then the response should be successful

    # Two chains on one proxy so both directions are reachable from one deployment: the
    # first ends on vendor B after a dead first attempt, the second lands on vendor C and
    # never touches B at all.
    Given I generate a unique resource name from "iso-proxy" and store it as "proxyName"
    And I generate a unique API version from "iso-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/iso-proxy" and store it as "proxyContext"
    And I generate a unique value from "iso-dead-credential" and store it as "deadCredential"
    And I generate a unique value from "iso-b-credential" and store it as "vendorBCredential"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion}                                                                                                                                              |
      | name                     | ${CTX:proxyName}                                                                                                                                                       |
      | displayName              | ${CTX:proxyName}                                                                                                                                                       |
      | version                  | ${CTX:proxyVersion}                                                                                                                                                    |
      | context                  | ${CTX:proxyContext}                                                                                                                                                    |
      | provider.id              | ${CTX:vendorCName}                                                                                                                                                     |
      | spec.providers | [{"id":"${CTX:vendorCName}","isPrimary":true},{"id":"${CTX:deadName}","alias":"dead-vendor","auth":{"type":"api-key","header":"x-dead-key","value":"${CTX:deadCredential}"}},{"id":"${CTX:vendorBName}","alias":"vendor-b","auth":{"type":"api-key","header":"x-api-key","value":"${CTX:vendorBCredential}"},"transformer":{"type":"openai-to-anthropic-transformer","version":"v0"}}] |
      | spec.operationPolicies            | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","provider":"dead-vendor","fallbacks":[{"model":"claude-sonnet-4-5-20250929","provider":"vendor-b"}]},{"model":"gpt-4o-mini"}]}}]}] |
    Then the response should be successful

    # Direction 1: the failed attempt's credential must not travel forward.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    # Anchors: vendor B's own chain did run on the attempt routed to B. Without these the
    # absence assertion below could pass on an empty or unreported body.
    And the response body should contain "/vendor-b-upstream/v1/messages"
    And the response body should contain "${CTX:vendorBCredential}"
    # The claim. Attempt 1 had the dead vendor's key injected before its dial failed;
    # attempt 2 must be built from the downstream request, not from what attempt 1 left
    # behind. A failure here is one vendor's API key delivered to another vendor.
    And the response body should not contain "${CTX:deadCredential}"

    # Direction 2: an unselected provider's chain must not run at all.
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # One attempt: this chain has no fallbacks, so the request ends on vendor C.
    And the response should report one upstream attempt
    # Anchor: C received the request, and in the client's original shape.
    And the response body should contain "/vendor-c-upstream/chat/completions"
    # The claim. Vendor B is configured on this same proxy with a credential and a
    # transformer, and neither may touch an attempt routed to C. The path assertion above
    # already fails if B's transformer ran; these name the two artefacts directly so the
    # reason is unambiguous.
    And the response body should not contain "${CTX:vendorBCredential}"
    And the response body should not contain "/v1/messages"
    And the response body should not contain "${CTX:deadCredential}"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:vendorBName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:vendorCName}"
    Then the response should be successful
