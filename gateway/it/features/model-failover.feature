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
