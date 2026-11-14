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
      | spec.upstream.url  | http://testbench:3008/anthropic           |
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

    # Vendor Y is the testbench's real Anthropic responder, not an echo, so this asserts the
    # ROUND TRIP rather than the outbound leg alone. It only answers at
    # /anthropic/v1/messages, which the transformer's request half has to produce — the
    # client sent /chat/completions — so a 200 here already means the path was rewritten.
    #
    # This scenario used to point at an echo and assert the request it got back. That worked
    # only because the transformer's RESPONSE half never ran: with failover attached the
    # transformer sits in the per-attempt chain, which is request-only, so a cross-vendor
    # response reached the client in the fallback vendor's shape. The serving chain is now
    # unwound on the response too (FR-037h), so the echo is converted on the way back and is
    # no longer readable — which is the point, and why these assertions moved.

    # OpenAI shape, which the client asked in and must be answered in.
    And the response body should contain "choices"
    And the response body should contain "chat.completion"

    # Real content carried through the conversion, so the reverse half ran on a genuine
    # Anthropic body rather than producing an empty envelope.
    And the response body should contain "How can I help you today"

    # The model the TRANSFORMER substituted, reflected by the responder and surviving the
    # conversion back. The client sent gpt-4o; this can only be here if the request half
    # rewrote it for vendor Y and the response half preserved it.
    And the response body should contain "claude-sonnet-4-5-20250929"

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
      | spec.providers | [{"id":"${CTX:vendorCName}","isPrimary":true},{"id":"${CTX:deadName}","alias":"dead-vendor","auth":{"type":"api-key","header":"x-dead-key","value":"${CTX:deadCredential}"},"transformer":{"type":"openai-to-anthropic-transformer","version":"v0"}},{"id":"${CTX:vendorBName}","alias":"vendor-b","auth":{"type":"api-key","header":"x-api-key","value":"${CTX:vendorBCredential}"}}] |
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
    # absence assertions below could pass on an empty or unreported body.
    #
    # The TRANSFORMER sits on the dead vendor here, not on B. Two reasons. B's upstream is an
    # echo, and a transformer on B would convert that echo on the way back — the serving
    # chain is unwound on the response now (FR-037h) — leaving nothing to read a credential
    # out of, which is the one thing this scenario exists to check. And putting it on the
    # attempt that FAILS makes the leak test sharper: attempt 1 is both credentialed and
    # transformed, so if attempt 2 were built from what attempt 1 left behind it would reach
    # B in the dead vendor's shape as well as with its key.
    And the response body should contain "/vendor-b-upstream/chat/completions"
    And the response body should contain "${CTX:vendorBCredential}"
    # The claim. Attempt 1 had the dead vendor's key injected before its dial failed;
    # attempt 2 must be built from the downstream request, not from what attempt 1 left
    # behind. A failure here is one vendor's API key delivered to another vendor.
    And the response body should not contain "${CTX:deadCredential}"
    # The same claim for attempt 1's BODY: max_tokens is the dead vendor's transformer's
    # work and has no equivalent in what the client sent, so its presence at B would mean
    # attempt 2 inherited attempt 1's transformed request.
    And the response body should not contain "max_tokens"

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
    # The claim. Vendor B and the dead vendor are both configured on this same proxy — B with
    # a credential, the dead vendor with a credential AND a transformer — and none of it may
    # touch an attempt routed to C. The path assertion above already fails if any transformer
    # ran; these name the artefacts directly so the reason is unambiguous.
    And the response body should not contain "${CTX:vendorBCredential}"
    And the response body should not contain "/v1/messages"
    And the response body should not contain "max_tokens"
    And the response body should not contain "${CTX:deadCredential}"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:vendorBName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:vendorCName}"
    Then the response should be successful

  # ====================================================================
  #
  # The scenarios above prove the REQUEST half: each attempt was shaped and credentialed for
  # the vendor it reached. They cannot prove the response half, because every upstream here is
  # an echo backend - what comes back is what the vendor received, so a response that was
  # never adapted on the way out looks exactly like one that was.
  #
  # That is the gap this scenario closes, and it closes it without needing a vendor-shaped
  # backend. A provider's per-attempt chain runs on the response too, by being unwound in
  # reverse at the head of the downstream response phase (FR-037h). Any response-direction
  # policy in that chain is therefore observable from the client: set-headers in response mode
  # stamps a marker, and the client either sees it or the unwind did not happen.
  #
  # Attached to the FALLBACK provider only, so the marker also proves attribution: it must
  # come from the chain of the provider that actually served, not the one that failed first.

  @model-failover-response-unwind
  Scenario: The serving provider's per-attempt chain runs on the response too
    Given I generate a unique resource name from "unwind-dead" and store it as "deadName"
    And I generate a unique API version from "unwind-dead" and store it as "deadVersion"
    And I generate a unique API context from "/unwind-dead" and store it as "deadContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                     |
      | name               | ${CTX:deadName}                               |
      | displayName        | ${CTX:deadName}                               |
      | version            | ${CTX:deadVersion}                            |
      | template           | openai                                        |
      | spec.context       | ${CTX:deadContext}                            |
      | spec.upstream.url  | http://unwind-dead-nonexistent:9080/dead      |
      | accessControl.mode | allow_all                                     |
    Then the response should be successful

    Given I generate a unique resource name from "unwind-live" and store it as "liveName"
    And I generate a unique API version from "unwind-live" and store it as "liveVersion"
    And I generate a unique API context from "/unwind-live" and store it as "liveContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                  |
      | name               | ${CTX:liveName}                            |
      | displayName        | ${CTX:liveName}                            |
      | version            | ${CTX:liveVersion}                         |
      | template           | openai                                     |
      | spec.context       | ${CTX:liveContext}                         |
      | spec.upstream.url  | http://testbench:3000/unwind-live-upstream |
      | accessControl.mode | allow_all                                  |
    Then the response should be successful

    # set-headers in RESPONSE mode, on the fallback provider's own policies list. It runs in
    # that provider's per-attempt chain, which the request direction enters once per attempt -
    # and which the response direction reaches only through the unwind.
    Given I generate a unique resource name from "unwind" and store it as "proxyName"
    And I generate a unique API version from "unwind" and store it as "proxyVersion"
    And I generate a unique API context from "/unwind" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                                                                                                                                                                                                      |
      | name                   | ${CTX:proxyName}                                                                                                                                                                                                                               |
      | displayName            | ${CTX:proxyName}                                                                                                                                                                                                                               |
      | version                | ${CTX:proxyVersion}                                                                                                                                                                                                                            |
      | context                | ${CTX:proxyContext}                                                                                                                                                                                                                            |
      | provider.id            | ${CTX:deadName}                                                                                                                                                                                                                                |
      | spec.providers         | [{"id":"${CTX:deadName}","isPrimary":true},{"id":"${CTX:liveName}","alias":"live-vendor","policies":[{"name":"set-headers","version":"v1","params":{"response":{"headers":[{"name":"x-unwound-by","value":"live-vendor"}]}}}]}]                     |
      | spec.operationPolicies | [{"name":"model-failover","version":"v0","paths":[{"path":"/chat/completions","methods":["POST"],"params":{"suspendDuration":0,"targets":[{"model":"gpt-4o","fallbacks":[{"model":"gpt-4o","provider":"live-vendor"}]}]}}]}]                    |
    Then the response should be successful

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # Two attempts: the primary refused, the fallback served.
    And the response header "x-envoy-attempt-count" should be "2"

    # THE assertion. This header exists only if the serving provider's per-attempt chain was
    # run in the response direction. Without the unwind the request still succeeds and the
    # body is identical - the header is the only difference, which is precisely why a
    # request-path assertion could never have caught this.
    And the response header "x-unwound-by" should be "live-vendor"

    When I delete the LLM proxy "${CTX:proxyName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:deadName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:liveName}"
    Then the response should be successful
