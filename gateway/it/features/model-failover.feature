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

@model-failover
Feature: Automatic fallback to a healthy model
  The acceptance scenarios of User Story 1. `failover-core.feature` already covers
  the single rescue and the untouched-success cases; this file carries the ones it
  does not — a chain deeper than two, and the guarantee that a rescue is invisible
  to the client.

  A dead target is produced by pointing a provider's upstream at a host that does
  not resolve. On an LlmProxy that works because each provider sits behind a
  loopback cluster which is always healthy: the DNS failure happens on the
  provider's own inner route and reaches the proxy as an ordinary upstream 503,
  which is retriable. On a DIRECT upstream there is no second hop and the same
  failure is terminal (poc-results.md).

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # US1 SCENARIO 3 — a chain deeper than two
  # ====================================================================

  Scenario: Two failing targets in a row are both passed over
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-dead-first-provider
      spec:
        displayName: MF Dead First
        version: v1.0
        template: openai
        context: /mf-dead-first-ctx
        upstream:
          url: http://mf-nonexistent-first:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-dead-second-provider
      spec:
        displayName: MF Dead Second
        version: v1.0
        template: openai
        context: /mf-dead-second-ctx
        upstream:
          url: http://mf-nonexistent-second:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-third-provider
      spec:
        displayName: MF Third
        version: v1.0
        template: openai
        context: /mf-third-ctx
        upstream:
          url: http://sample-backend:9080/third-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # Three targets in declared order. The first two cannot answer, so only a chain
    # that keeps walking past a SECOND failure reaches the third.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-deep-proxy
      spec:
        displayName: MF Deep Proxy
        version: v1.0
        context: /mf-deep-proxy
        provider:
          id: mf-dead-first-provider
        additionalProviders:
          - id: mf-dead-second-provider
            as: second
          - id: mf-third-provider
            as: third
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: second
                        - model: command-r
                          provider: third
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # THREE attempts, and the third target's response. A count of 2 would mean the
    # chain gave up after one failure; a 503 would mean it never reached the third.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-deep-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/third-upstream"
    And the response header "x-envoy-attempt-count" should be "3"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-deep-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-dead-first-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-dead-second-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-third-provider"
    Then the response status code should be 200

  # ====================================================================
  # FR-012g — a rescue is invisible to the client
  # ====================================================================

  Scenario: A rescued request leaks nothing about the attempt that failed
    # The client asked one question and got one answer. Everything the gateway did
    # to produce it -- which provider it tried first, how it steered the retry --
    # is internal, and a response that carries traces of it turns an implementation
    # detail into something a client can come to depend on.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-quiet-dead-provider
      spec:
        displayName: MF Quiet Dead
        version: v1.0
        template: openai
        context: /mf-quiet-dead-ctx
        upstream:
          url: http://mf-quiet-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-quiet-alive-provider
      spec:
        displayName: MF Quiet Alive
        version: v1.0
        template: openai
        context: /mf-quiet-alive-ctx
        upstream:
          url: http://sample-backend:9080/quiet-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-quiet-proxy
      spec:
        displayName: MF Quiet Proxy
        version: v1.0
        context: /mf-quiet-proxy
        provider:
          id: mf-quiet-dead-provider
        additionalProviders:
          - id: mf-quiet-alive-provider
            as: alive
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response status should be 201
    And I wait for 5 seconds

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-quiet-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # A rescue happened: two attempts, answered by the fallback.
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/quiet-upstream"

    # The steering headers the gateway uses internally must not reach the client.
    And the response header "x-target-upstream" should not exist
    And the response header "x-wso2-internal-loopback" should not exist

    # No trace of the target that failed, in a body the client is meant to read as
    # an ordinary model response.
    And the response body should not contain "mf-quiet-dead"
    And the response body should not contain "dead-upstream"
    # And no gateway-invented error text alongside the success it did return.
    And the response body should not contain "no healthy upstream"
    And the response body should not contain "upstream connect error"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-quiet-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-quiet-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-quiet-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # US2 — the same model under two providers is two targets, not one
  # ====================================================================

  Scenario: The same model name under a second provider is reached with that provider's own endpoint
    # A publisher running one model in two regions, or through two accounts, names
    # the SAME model twice. If the gateway keyed a target on the model alone the
    # second entry would be a duplicate of the first and the fallback would be a
    # no-op -- the request would simply re-attempt the broken configuration.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-region-a-provider
      spec:
        displayName: MF Region A
        version: v1.0
        template: openai
        context: /mf-region-a-ctx
        upstream:
          url: http://mf-region-a-nonexistent:9080/region-a
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-region-b-provider
      spec:
        displayName: MF Region B
        version: v1.0
        template: openai
        context: /mf-region-b-ctx
        upstream:
          url: http://sample-backend:9080/region-b-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # Both targets name gpt-4o. Only the provider differs.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-same-model-proxy
      spec:
        displayName: MF Same Model Proxy
        version: v1.0
        context: /mf-same-model-proxy
        provider:
          id: mf-region-a-provider
        additionalProviders:
          - id: mf-region-b-provider
            as: region-b
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: gpt-4o
                          provider: region-b
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # Two attempts, and the response comes from region B's OWN endpoint. One attempt
    # would mean the second entry was treated as a duplicate of the first and the
    # chain had nowhere to go.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-same-model-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/region-b-upstream"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-same-model-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-region-a-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-region-b-provider"
    Then the response status code should be 200

  # ====================================================================
  # US4 — a target that keeps failing is taken out of rotation
  # ====================================================================

  Scenario: A suspended target stops being attempted on later requests
    # Failing over rescues ONE request. Suspension is what stops every request
    # after it paying the same failed attempt first: without it, a provider that
    # is down for an hour costs a wasted attempt on every single request for that
    # hour, and the latency of that attempt is charged to the client each time.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-susp-dead-provider
      spec:
        displayName: MF Suspend Dead
        version: v1.0
        template: openai
        context: /mf-susp-dead-ctx
        upstream:
          url: http://mf-susp-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-susp-alive-provider
      spec:
        displayName: MF Suspend Alive
        version: v1.0
        template: openai
        context: /mf-susp-alive-ctx
        upstream:
          url: http://sample-backend:9080/susp-alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # One failure suspends, for long enough that the window cannot expire during
    # the scenario and make a pass look like a pass for the wrong reason.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-suspend-proxy
      spec:
        displayName: MF Suspend Proxy
        version: v1.0
        context: /mf-suspend-proxy
        provider:
          id: mf-susp-dead-provider
        additionalProviders:
          - id: mf-susp-alive-provider
            as: alive
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 300
                  suspendAfterFailures: 1
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # Request 1 pays the failed attempt and is rescued. That failure suspends the
    # leading target.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-suspend-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/susp-alive-upstream"

    # Request 2 does NOT pay it. The suspended target is skipped where the chain is
    # resolved, so the request leads with the healthy one and takes ONE attempt.
    #
    # This single number is the whole scenario. A count of 2 would mean the
    # suspension was recorded but ignored -- the client still gets a 200 either
    # way, so the status code cannot tell the two apart. It also proves a skip
    # costs no attempt: were skips to consume the budget, a chain of mostly
    # suspended targets would exhaust itself stepping over dead entries.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-suspend-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"
    And the response body should contain "/susp-alive-upstream"

    # And it stays out: a third request is served the same way, not re-probed.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-suspend-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-suspend-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-susp-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-susp-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # US4 — a suspension expires, and the target is tried again
  # ====================================================================

  Scenario: A suspension expires and the target is admitted as a probe
    # Suspension has to be temporary. A target taken out permanently on one bad
    # minute never comes back, and a publisher who configured three providers ends
    # up running on one with no indication why.
    #
    # The probe here FAILS -- the target is still dead -- and that is deliberate.
    # What this scenario pins is the re-admission, which is observable on its own:
    # the attempt count going back up proves the target re-entered the chain. A
    # scenario that also needed the probe to succeed would have to mutate a
    # provider mid-run and would be testing two things at once.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-expire-dead-provider
      spec:
        displayName: MF Expire Dead
        version: v1.0
        template: openai
        context: /mf-expire-dead-ctx
        upstream:
          url: http://mf-expire-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-expire-alive-provider
      spec:
        displayName: MF Expire Alive
        version: v1.0
        template: openai
        context: /mf-expire-alive-ctx
        upstream:
          url: http://sample-backend:9080/expire-alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # A short window, so it can expire inside one scenario.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-expire-proxy
      spec:
        displayName: MF Expire Proxy
        version: v1.0
        context: /mf-expire-proxy
        provider:
          id: mf-expire-dead-provider
        additionalProviders:
          - id: mf-expire-alive-provider
            as: alive
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 5
                  suspendAfterFailures: 1
                  maxSuspendMultiplier: 1
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # The failure that suspends it.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-expire-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # Immediately after: suspended, so it is skipped and one attempt suffices.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-expire-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "1"

    # Past the window. maxSuspendMultiplier is 1, so the re-suspension that the
    # failed probe causes lasts the same 5s rather than doubling -- without that
    # the timing of this scenario would depend on the escalation ladder.
    When I wait for 8 seconds
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-expire-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    # Two attempts again: the target was re-admitted and tried. Had the suspension
    # been permanent this would still read 1, with the client none the wiser.
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/expire-alive-upstream"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-expire-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-expire-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-expire-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # US3 — a fallback on another vendor is adapted to that vendor
  # ====================================================================

  Scenario: A cross-vendor fallback is sent that vendor's format and credential
    # This is the scenario the per-attempt chain exists for. A fallback on another
    # vendor needs a DIFFERENT request: different path, different body shape,
    # different auth header. Adapting once downstream cannot work, because the
    # downstream chain runs before any target is chosen -- it would have to guess,
    # and either guess breaks one of the two attempts.
    #
    # The upstream here is an echo backend, so what it reports IS what the vendor
    # would have received.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-vendor-x-provider
      spec:
        displayName: MF Vendor X
        version: v1.0
        template: openai
        context: /mf-vendor-x-ctx
        upstream:
          url: http://mf-vendor-x-nonexistent:9080/vendor-x
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-vendor-y-provider
      spec:
        displayName: MF Vendor Y
        version: v1.0
        template: openai
        context: /mf-vendor-y-ctx
        upstream:
          url: http://sample-backend:9080/vendor-y-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # Vendor Y carries its own transformer and its own credential. Both are
    # appended by the controller to that provider's per-attempt chain, the
    # credential always last so no author-written policy can observe it.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-xvendor-proxy
      spec:
        displayName: MF Cross Vendor Proxy
        version: v1.0
        context: /mf-xvendor-proxy
        provider:
          id: mf-vendor-x-provider
        additionalProviders:
          - id: mf-vendor-y-provider
            as: vendor-y
            auth:
              type: api-key
              header: x-api-key
              value: mf-xvendor-credential
            transformer:
              type: openai-to-anthropic-transformer
              version: v0
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: vendor-y
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # The client sends OpenAI's shape and gets a success.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-xvendor-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    # The transformer rewrote the PATH for the second vendor. /chat/completions
    # reaching vendor Y would mean no adaptation happened.
    And the response body should contain "/vendor-y-upstream/v1/messages"

    # And the BODY: max_tokens is required by the second vendor and has no
    # equivalent in what the client sent, so its presence cannot come from the
    # original request being forwarded unchanged.
    And the response body should contain "max_tokens"
    And the response body should contain "claude-sonnet-4-5-20250929"

    # And the credential for THAT provider, applied on the attempt that reached it.
    And the response body should contain "mf-xvendor-credential"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-xvendor-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-vendor-x-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-vendor-y-provider"
    Then the response status code should be 200

  # ====================================================================
  # FR-030d — attaching failover to one API changes nothing for another
  # ====================================================================

  Scenario: An API without failover is untouched by one that has it, even sharing a provider
    # Per-attempt chains are keyed by CLUSTER, and two APIs attaching the same
    # provider reach it through API-scoped clusters. If that keying were wrong --
    # if a chain were stored under a cluster name both APIs share -- one API's
    # chain would run on the other's requests, and since a provider's credential
    # is the last policy in that chain, API A's key would be applied to API B's
    # request. This scenario is the check on that claim.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-iso-dead-provider
      spec:
        displayName: MF Isolation Dead
        version: v1.0
        template: openai
        context: /mf-iso-dead-ctx
        upstream:
          url: http://mf-iso-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-iso-shared-provider
      spec:
        displayName: MF Isolation Shared
        version: v1.0
        template: openai
        context: /mf-iso-shared-ctx
        upstream:
          url: http://sample-backend:9080/iso-shared-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # API A: failover attached, and it reaches the shared provider with ITS key.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-iso-a-proxy
      spec:
        displayName: MF Isolation A
        version: v1.0
        context: /mf-iso-a-proxy
        provider:
          id: mf-iso-dead-provider
        additionalProviders:
          - id: mf-iso-shared-provider
            as: shared
            auth:
              type: api-key
              header: x-api-key
              value: mf-iso-a-credential
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: shared
      """
    Then the response status should be 201

    # API B: the SAME provider, no failover at all, and a marker of its own set by
    # an ordinary operation policy. A marker rather than a credential deliberately:
    # primary-provider auth and additionalProviders auth do not reach the upstream
    # by the same route, and this scenario is about chain isolation, not about
    # characterising that difference.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-iso-b-proxy
      spec:
        displayName: MF Isolation B
        version: v1.0
        context: /mf-iso-b-proxy
        provider:
          id: mf-iso-shared-provider
        operationPolicies:
          - name: set-headers
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  request:
                    headers:
                      - name: x-iso-b-marker
                        value: mf-iso-b-marker
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # Drive API A first, so its per-attempt chain is live before B is observed.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-iso-a-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "mf-iso-a-credential"

    # Now API B, through the same provider.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-iso-b-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/iso-shared-upstream"

    # B's own configuration ran -- the marker its policy sets is on the request
    # the backend received.
    And the response body should contain "mf-iso-b-marker"

    # And nothing of A's did. The echo backend reports what it actually received,
    # so A's credential appearing on B's request would be a real leak between two
    # APIs, applied by a chain that should never have run here.
    And the response body should not contain "mf-iso-a-credential"

    # B DOES carry x-envoy-attempt-count, and that is a known deviation rather
    # than the intent. Envoy exposes include_attempt_count_in_response on a
    # VirtualHost only -- there is no per-route equivalent -- so enabling it for
    # A's routes enables it for every API sharing that vhost. B's responses gain
    # an informational header it would not otherwise have.
    #
    # Asserted as it behaves, not as FR-030d words it, so the day someone scopes
    # this properly this line fails and is updated deliberately. See
    # poc-results.md, "The attempt-count header is virtual-host scoped".
    And the response header "x-envoy-attempt-count" should be "1"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-iso-a-proxy"
    Then the response should be successful
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-iso-b-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-iso-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-iso-shared-provider"
    Then the response status code should be 200

  # ====================================================================
  # FR-012c — a body mutation survives into the attempt that serves
  # ====================================================================

  Scenario: PII redacted downstream stays redacted on the fallback attempt
    # The downstream chain runs ONCE and the upstream chain runs per attempt. If
    # the upstream position read a snapshot of the body taken before the
    # downstream chain mutated it, the first attempt would carry redacted data and
    # every retry would carry the ORIGINAL -- so a fallback provider receives the
    # raw PII that was redacted precisely so it would not.
    #
    # That failure is silent. The client gets its 200 from the fallback either
    # way, the redaction policy reports success either way, and the only place the
    # difference exists is in the bytes the second provider received. So this
    # asserts on those bytes, not on policy logs.
    #
    # Redaction rather than masking deliberately: masking restores the original
    # value on the response, which would put the raw email back into what this
    # scenario reads and hide the very thing it is looking for.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-pii-dead-provider
      spec:
        displayName: MF PII Dead
        version: v1.0
        template: openai
        context: /mf-pii-dead-ctx
        upstream:
          url: http://mf-pii-nonexistent:9080/dead-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-pii-alive-provider
      spec:
        displayName: MF PII Alive
        version: v1.0
        template: openai
        context: /mf-pii-alive-ctx
        upstream:
          url: http://sample-backend:9080/pii-alive-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    # Redaction runs downstream, once. Failover walks the chain.
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-pii-proxy
      spec:
        displayName: MF PII Proxy
        version: v1.0
        context: /mf-pii-proxy
        provider:
          id: mf-pii-dead-provider
        additionalProviders:
          - id: mf-pii-alive-provider
            as: alive
        operationPolicies:
          - name: pii-masking-regex
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  customPIIEntities:
                    - piiEntity: "EMAIL"
                      piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
                  jsonPath: ""
                  redactPII: true
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response status should be 201
    And I wait for 5 seconds

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-pii-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "contact me at leak-probe@example.com please"}]}
      """
    # The first target failed and the SECOND served it.
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/pii-alive-upstream"

    # THE assertion. This is the body the fallback provider received, echoed back.
    # The raw address appearing here would mean the retry was built from a
    # pre-redaction snapshot and the redaction was undone by the failover.
    And the response body should not contain "leak-probe@example.com"

    # And the body reached it INTACT, with only the address taken out. Without
    # these two, the assertion above would pass just as well on an empty body or
    # a dropped field -- proving the address was absent rather than that it was
    # redacted from a body that otherwise arrived whole.
    And the response body should contain "contact me at"
    And the response body should contain "please"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-pii-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-pii-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-pii-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # FR-030a — the same configuration works on a kind that is not an LlmProxy
  # ====================================================================

  Scenario: The same failover configuration behaves the same on a RestApi
    # Applicability is decided by whether a target RESOLVES, not by a list of
    # permitted kinds. On an LlmProxy a target's provider names an
    # additionalProviders alias; on a RestApi it names an upstreamDefinition.
    # Name them the same and the policy block is byte-for-byte identical. The
    # `params` block below is literally the one the PII scenario above attaches
    # to an LlmProxy; only the wrapper around it differs (`paths:` there,
    # `operations[].policies` here), because that is each kind's own attachment
    # schema and not part of the failover configuration.
    #
    # The dead upstream here REFUSES rather than failing DNS, and that difference
    # is the point rather than an incidental choice. A RestApi has no loopback
    # hop: Envoy answers an unresolvable host locally with no_healthy_upstream,
    # which is terminal, and no failover occurs. On an LlmProxy the same name
    # fails on the provider's inner route and returns a retriable 503. A refused
    # connection is retriable on both, so it is the failure mode that lets one
    # configuration be compared across the two kinds at all.
    #
    # The two definitions carry DIFFERENT basePaths deliberately. Selecting the
    # fallback cluster and sending it the fallback's own base path are separate
    # mechanisms: the first is Envoy's priority_group and was always kind-
    # agnostic, the second runs in the per-attempt upstream filter chain. That
    # chain used to be built for an LlmProxy only, so a RestApi fallback was
    # reached but served on the PRIMARY's base path — a 200 from the wrong
    # resource, with no error anywhere. Give both definitions the same basePath
    # and this scenario passes either way, which is exactly why it must not.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-restapi-failover-v1.0
      spec:
        displayName: MF RestApi Failover
        version: v1.0
        context: /mf-restapi/$version
        upstreamDefinitions:
          - name: primary
            basePath: /rest-dead-upstream
            upstreams:
              - url: http://sample-backend:9998
          - name: alive
            basePath: /rest-alive-upstream
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: primary
        operations:
          - method: POST
            path: /chat/completions
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response should be successful
    And I wait for 5 seconds

    # Identical outcome to the LlmProxy scenarios above: the first target refuses,
    # the second answers, and the client sees one success after two attempts.
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-restapi/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/rest-alive-upstream"

    # Cleanup
    When I delete the API "mf-restapi-failover-v1.0"
    Then the response status code should be 200

  # ====================================================================
  # FR-030b / FR-030e — a kind with no model concept
  # ====================================================================

  Scenario: Attached to a kind with no model concept, the policy deploys and does nothing
    # An Mcp has no model concept at all, but it is transformed by the same
    # RestAPITransformer as every other kind, so buildFailoverConfig genuinely
    # runs here rather than being skipped by kind. Every member names a provider,
    # and an Mcp has no upstreamDefinitions for one to resolve against, so every
    # member is dropped and the whole config collapses to nil — no priority group,
    # no retry policy, nothing on the virtual host.
    #
    # Naming a provider on the PRIMARY target is load-bearing. A bare `model:`
    # with no provider resolves to the main upstream, which would leave a
    # one-member chain — accepted and harmless, but not the inert case FR-030b
    # describes. See TestBuildFailoverConfig_InertOnlyWhenNoMemberResolves.
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Mcp
      metadata:
        name: mf-inert-mcp-v1.0
      spec:
        displayName: MF Inert MCP
        version: v1.0
        context: /mf-inert-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001/mcp
        policies:
          - name: model-failover
            version: v0
            params:
              targets:
                - model: gpt-4o
                  provider: openai-primary
                  fallbacks:
                    - model: claude-sonnet-4-5-20250929
                      provider: anthropic-secondary
        tools: []
        resources: []
        prompts: []
      """
    # Accepted, not rejected: an unresolvable target must never block a deployment.
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I reset the analytics collector
    And I wait for 2 seconds

    # Behaves exactly as the same proxy does with NO policy attached. This is the
    # identical call and identical expected answer as the policy-free MCP scenario
    # in mcp_deploy.feature — that equivalence IS the requirement.
    When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mf-inert-mcp/mcp"
    Then the response should be successful
    When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mf-inert-mcp/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # No failover configuration was generated. The header is added to a virtual
    # host only when one of its routes actually fails over, so its absence is the
    # direct observable for "no priority group exists" — and it is paired with the
    # positive assertion above so the absence cannot pass vacuously on a request
    # that never reached the gateway.
    And the response header "x-envoy-attempt-count" should not exist

    # The request is recorded in analytics exactly as an unpolicied one is: an
    # ordinary event, nothing added and nothing missing.
    #
    # Deliberately NOT asserted here: "the event carries no failover: keys". A
    # probe established that failover telemetry does not reach the collector even
    # when failover demonstrably happens (attempt-count 2, and the policy's own
    # log line present), so that assertion cannot currently fail and would be
    # pure false confidence. It belongs here once the analytics gap is closed —
    # tracked as T091, which owns both the gap and this assertion.
    When I wait 5 seconds for analytics to be published
    Then the analytics collector should have received at least 1 event
    When I send a GET request to the analytics collector events endpoint
    Then the response should be successful
    And the response body should contain "mf-inert-mcp"

    # Cleanup
    When I delete the MCP proxy "mf-inert-mcp-v1.0"
    Then the response should be successful

  # ====================================================================
  # FR-013 — failover stops once the response has started
  # ====================================================================

  Scenario: A failure after the response has started is not failed over
    # Both operations below carry byte-identical failover configuration, and
    # that pairing is the whole point. /status/503 fails BEFORE any body, so it
    # must fail over — it is the control proving failover is live on this API.
    # /drip sends its headers in about 20ms and then streams for 10s, so the 2s
    # route timeout cuts it mid-body: a failure AFTER the response started.
    # Without the control, "no second attempt" on /drip would be indistinguishable
    # from failover never having engaged here at all.
    #
    # Envoy cannot rescue the second case and should not try: a 200 that has
    # already reached the client cannot be un-sent (envoyproxy/envoy#5876 — mid-
    # stream retry is closed by design, not a gap). The client-visible result is
    # a real 200 whose body stops early, which is what this asserts.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-stream-safety-v1.0
      spec:
        displayName: MF Stream Safety
        version: v1.0
        context: /mf-stream-safety/$version
        upstreamDefinitions:
          - name: primary
            upstreams:
              - url: http://echo-backend:80
          - name: alive
            basePath: /mf-stream-fallback
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: primary
        resilience:
          timeout: 2s
        operations:
          - method: GET
            path: /status/503
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
          - method: GET
            path: /drip
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: alive
      """
    Then the response should be successful
    And I wait for 6 seconds

    # CONTROL — the 503 is in the headers, so nothing has been sent to the client
    # yet and the attempt is retriable. Failover works on this API.
    When I send a GET request to "http://localhost:8080/mf-stream-safety/v1.0/status/503"
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/mf-stream-fallback"
    And the response body should not have been cut off

    # SUBJECT — same configuration, same API, failure moved to after the headers.
    When I send a GET request to "http://localhost:8080/mf-stream-safety/v1.0/drip?duration=10&numbytes=60" allowing a cut-off body
    # The headers were real and already delivered — this is not an error response.
    Then the response status code should be 200
    # One attempt only: the fallback was never reached.
    And the response header "x-envoy-attempt-count" should be "1"
    # And the client saw the consequence rather than a silent rescue.
    And the response body should have been cut off
    And the response body should not contain "/mf-stream-fallback"

    # Cleanup
    When I delete the API "mf-stream-safety-v1.0"
    Then the response status code should be 200

  # ====================================================================
  # Traversal stops when the caller goes away
  # ====================================================================

  Scenario: A caller that gives up mid-chain stops the traversal
    # With the caller gone there is no response to assert on, so the fallback is
    # pointed at a SECOND API on this gateway. If it is attempted, that API
    # records its own analytics event under its own URI; if traversal stopped,
    # no such event exists. Both halves are read from ONE payload, so the
    # positive control sits beside the negative claim rather than in a previous
    # run that a late flush could blur.
    #
    # The primary must FAIL, and slowly. /delay/7 and /delay/8 answer 200 after
    # sleeping, so perTryTimeout of 3s is what turns them into failures worth
    # failing over — without it a connected caller would simply get its 200 and
    # the fallback would be untouched for an entirely different reason.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-disconnect-sink-v1.0
      spec:
        displayName: MF Disconnect Sink
        version: v1.0
        context: /mf-disconnect-sink/$version
        upstream:
          main:
            url: http://sample-backend:9080/disconnect-sink-upstream
        operations:
          - method: GET
            path: /delay/7
          - method: GET
            path: /delay/8
      """
    Then the response should be successful

    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-disconnect-main-v1.0
      spec:
        displayName: MF Disconnect Main
        version: v1.0
        context: /mf-disconnect-main/$version
        upstreamDefinitions:
          - name: primary
            upstreams:
              - url: http://echo-backend:80
          - name: sink
            basePath: /mf-disconnect-sink/v1.0
            upstreams:
              - url: http://gateway-runtime:8080
        upstream:
          main:
            ref: primary
        operations:
          - method: GET
            path: /delay/7
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  perTryTimeout: 3
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: sink
          - method: GET
            path: /delay/8
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  perTryTimeout: 3
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: sink
      """
    Then the response should be successful
    And I wait for 6 seconds

    # CONTROL — identical configuration and timing, caller stays. The primary
    # sleeps past its 3s per-attempt budget, so the chain advances to the sink.
    When I send a GET request to "http://localhost:8080/mf-disconnect-main/v1.0/delay/7"
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"
    And the response body should contain "/disconnect-sink-upstream"

    # SUBJECT — same shape, caller gives up at 1.5s, well inside the first
    # attempt's 3s budget, so it leaves while the chain is still being walked.
    When I send a GET request to "http://localhost:8080/mf-disconnect-main/v1.0/delay/8" and give up after 1500ms
    Then the request should have been abandoned

    # Long enough that a fallback attempt, had one been made, would have run its
    # course and been published — otherwise the absence below only means "early".
    When I wait 12 seconds for analytics to be published
    And I send a GET request to the analytics collector events endpoint
    Then the response should be successful
    # The control reached the sink, so the sink IS recorded this way...
    And the response body should contain "/mf-disconnect-sink/v1.0/delay/7"
    # ...and the abandoned request never did.
    And the response body should not contain "/mf-disconnect-sink/v1.0/delay/8"

    # Cleanup
    When I delete the API "mf-disconnect-main-v1.0"
    Then the response status code should be 200
    When I delete the API "mf-disconnect-sink-v1.0"
    Then the response status code should be 200

  # ====================================================================
  # SC-011 — a failed-over request is still ONE request
  # ====================================================================

  Scenario: Three attempts consume one rate-limit unit and produce one analytics record
    # The chain is three deep on purpose: SC-011 asks for this across at least
    # three attempts, and a per-attempt miscount is only visibly wrong when the
    # chain is longer than two.
    #
    # The rate limit is the sharp instrument here. With a limit of 2, a request
    # counted once per ATTEMPT would exhaust the whole budget on its own and the
    # second request would already be 429. Asserting that the second request
    # still succeeds and only the third is refused pins the count at exactly one
    # per client request — which no single-request assertion could do.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-accounting-v1.0
      spec:
        displayName: MF Accounting
        version: v1.0
        context: /mf-accounting/$version
        upstreamDefinitions:
          - name: primary
            basePath: /accounting-dead-one
            upstreams:
              - url: http://sample-backend:9998
          - name: mid
            basePath: /accounting-dead-two
            upstreams:
              - url: http://sample-backend:9997
          - name: alive
            basePath: /accounting-alive
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: primary
        operations:
          - method: POST
            path: /chat/completions
            policies:
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: mid
                        - model: claude-opus-4-1-20250805
                          provider: alive
              - name: basic-ratelimit
                version: v1
                params:
                  limits:
                    - requests: 2
                      duration: "1h"
      """
    Then the response should be successful
    And I wait for 6 seconds

    # Reset LAST, immediately before the request. Analytics publish
    # asynchronously, so resetting any earlier lets a previous scenario's event
    # land afterwards and be counted as this one's.
    When I reset the analytics collector
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-accounting/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # Two dead targets passed over, the third served — three attempts, one response.
    And the response header "x-envoy-attempt-count" should be "3"
    And the response body should contain "/accounting-alive"

    # ONE analytics record for the whole chain, not one per attempt.
    When I wait 8 seconds for analytics to be published
    Then the analytics collector should have received 1 event

    # ONE rate-limit unit. Counted per attempt, the budget of 2 would already be
    # gone and this would be 429.
    When I send a POST request to "http://localhost:8080/mf-accounting/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "3"

    # ...and the limit is real: the third request is refused.
    When I send a POST request to "http://localhost:8080/mf-accounting/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the API "mf-accounting-v1.0"
    Then the response status code should be 200

  # ====================================================================
  # SC-011 — the tokens billed are the serving attempt's
  # ====================================================================

  Scenario: A rescued request is billed for the attempt that actually served it
    # The alive provider is the OpenAI mock, which answers deterministically:
    # model gpt-4.1-2025-04-14, usage 19 prompt / 10 completion / 29 total. Fixed
    # numbers are what make this assertable at all — an echo backend returns no
    # usage block, so there would be nothing to attribute.
    #
    # Note what the client asked for and what answered differ: the request names
    # gpt-4o, the dead primary never responds, and the attempt that serves reports
    # a different model entirely. Billing has to follow the response, not the ask.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-tokens-dead-provider
      spec:
        displayName: MF Tokens Dead
        version: v1.0
        template: openai
        context: /mf-tokens-dead-ctx
        upstream:
          url: http://mf-tokens-nonexistent:9080/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mf-tokens-alive-provider
      spec:
        displayName: MF Tokens Alive
        version: v1.0
        template: openai
        context: /mf-tokens-alive-ctx
        upstream:
          url: http://mock-openapi:4010/openai/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201

    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mf-tokens-proxy
      spec:
        displayName: MF Tokens Proxy
        version: v1.0
        context: /mf-tokens-proxy
        provider:
          id: mf-tokens-dead-provider
        additionalProviders:
          - id: mf-tokens-alive-provider
            as: alive
        operationPolicies:
          - name: model-failover
            version: v0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: gpt-4o
                          provider: alive
      """
    Then the response status should be 201
    And I wait for 5 seconds

    # Reset LAST, immediately before the request, so a previous scenario's event
    # cannot flush into this window and be read as this request's.
    When I reset the analytics collector
    And I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mf-tokens-proxy/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "2"

    When I wait 8 seconds for analytics to be published
    Then the analytics collector should have received 1 event
    When I send a GET request to the analytics collector events endpoint
    Then the response should be successful
    # The serving attempt's own numbers, exactly — not zero, and not doubled.
    And the latest analytics event should report 19 prompt tokens and 29 total tokens
    # Attribution follows the attempt that answered: the model the dead primary
    # was asked for is not the model recorded.
    And the response body should contain "gpt-4.1-2025-04-14"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mf-tokens-proxy"
    Then the response should be successful
    When I delete the LLM provider "mf-tokens-dead-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mf-tokens-alive-provider"
    Then the response status code should be 200

  # ====================================================================
  # SC-011 — one unit of quota, however many attempts it took
  # ====================================================================

  Scenario: Three attempts consume one unit of subscription quota
    # Same reasoning as the rate-limit scenario, through the quota path instead.
    # A plan of 2 requests/minute against a three-deep chain is the sharp part:
    # if a rescued request spent one unit PER ATTEMPT, the very first request
    # would overrun the plan on its own and the second would already be refused.
    When I create a subscription plan "MFQuotaPlan" with 2 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "planId"

    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mf-quota-v1.0
      spec:
        displayName: MF Quota
        version: v1.0
        context: /mf-quota/$version
        subscriptionPlans:
          - MFQuotaPlan
        upstreamDefinitions:
          - name: primary
            basePath: /quota-dead-one
            upstreams:
              - url: http://sample-backend:9998
          - name: mid
            basePath: /quota-dead-two
            upstreams:
              - url: http://sample-backend:9997
          - name: alive
            basePath: /quota-alive
            upstreams:
              - url: http://sample-backend:9080
        upstream:
          main:
            ref: primary
        operations:
          - method: POST
            path: /chat/completions
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
              - name: model-failover
                version: v0
                params:
                  suspendDuration: 0
                  targets:
                    - model: gpt-4o
                      fallbacks:
                        - model: claude-sonnet-4-5-20250929
                          provider: mid
                        - model: claude-opus-4-1-20250805
                          provider: alive
      """
    Then the response should be successful
    And I wait for 6 seconds

    When I create a subscription for API "mf-quota-v1.0" with plan and token "mf-quota-subscription-token"
    Then the response status should be 201

    When I set header "Content-Type" to "application/json"
    And I set header "Subscription-Key" to "mf-quota-subscription-token"
    And I send a POST request to "http://localhost:8080/mf-quota/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    # Two targets passed over, the third served.
    And the response header "x-envoy-attempt-count" should be "3"
    And the response body should contain "/quota-alive"

    # The second unit. Counted per attempt, the plan would already be spent.
    When I send a POST request to "http://localhost:8080/mf-quota/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-envoy-attempt-count" should be "3"

    # ...and the plan is real: the third request is refused.
    When I send a POST request to "http://localhost:8080/mf-quota/v1.0/chat/completions" with body:
      """
      {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    When I delete the API "mf-quota-v1.0"
    Then the response should be successful
    When I delete the subscription plan with stored id "planId"
    Then the response should be successful
