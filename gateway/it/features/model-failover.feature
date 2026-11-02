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
