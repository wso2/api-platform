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

@model-round-robin-multi-provider
Feature: Model Round-Robin Multi-Provider Routing
  Test the model-round-robin policy when round-robin targets select both a model
  and an additional LLM provider. Targets without a "provider" use the LlmProxy
  primary provider; targets naming a provider are routed to that provider's named
  upstream and carry "selected_provider" metadata so conditional provider auth and
  protocol transformers apply.

  Each provider forwards to the sample backend under a distinct upstream path, and
  the sample backend echoes the request (path + body). So the echoed upstream path
  proves which provider was selected, and the echoed model proves the model rewrite.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # ROUND-ROBIN SEQUENCE ACROSS PRIMARY + TWO ADDITIONAL PROVIDERS
  # ====================================================================

  Scenario: Round-robin rotates across primary and two additional providers
    # --- Primary provider ---
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-rr-primary-provider
      spec:
        displayName: MP RR Primary Provider
        version: v1.0
        template: openai
        context: /mp-rr-primary-ctx
        upstream:
          url: http://testbench:3000/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # --- Additional provider 1 ---
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-rr-anthropic-provider
      spec:
        displayName: MP RR Anthropic Provider
        version: v1.0
        template: openai
        context: /mp-rr-anthropic-ctx
        upstream:
          url: http://testbench:3000/anthropic-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # --- Additional provider 2 ---
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-rr-bedrock-provider
      spec:
        displayName: MP RR Bedrock Provider
        version: v1.0
        template: openai
        context: /mp-rr-bedrock-ctx
        upstream:
          url: http://testbench:3000/bedrock-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # --- Proxy wiring primary + two additional providers ---
    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mp-rr-proxy
      spec:
        displayName: MP RR Proxy
        version: v1.0
        context: /mp-rr-proxy
        provider:
          id: mp-rr-primary-provider
        additionalProviders:
          - id: mp-rr-anthropic-provider
            as: anthropic-provider
            auth:
              type: api-key
              header: X-Round-Robin-Provider
              value: anthropic-round-robin-secret
          - id: mp-rr-bedrock-provider
            as: bedrock-provider
        policies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: gpt-4o
                    - model: claude-sonnet-4-5-20250929
                      provider: anthropic-provider
                    - model: bedrock-claude-3-5-sonnet
                      provider: bedrock-provider
                  suspendDuration: 60
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    # The proxy loops back to each provider's own route; wait for those to be
    # programmed before counting. Probing the provider routes directly does not
    # run the proxy's model-round-robin policy, so no rotation slot is consumed.
    And I send a "POST" request to "/mp-rr-primary-ctx/chat/completions" until status 200
    And I send a "POST" request to "/mp-rr-anthropic-ctx/chat/completions" until status 200
    And I send a "POST" request to "/mp-rr-bedrock-ctx/chat/completions" until status 200

    # Request 1 -> primary provider (no provider on target), model gpt-4o
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-rr-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"
    And the JSON response field "body" should be:
      """
      {"messages":[{"content":"Hello","role":"user"}],"model":"gpt-4o"}
      """
    And the response body should not contain "anthropic-round-robin-secret"

    # Request 2 -> anthropic-provider upstream
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-rr-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/anthropic-upstream/chat/completions"
    And the JSON response field "body" should be:
      """
      {"messages":[{"content":"Hello","role":"user"}],"model":"claude-sonnet-4-5-20250929"}
      """
    And the JSON response field "headers.X-Round-Robin-Provider[0]" should be "anthropic-round-robin-secret"

    # Request 3 -> bedrock-provider upstream
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-rr-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/bedrock-upstream/chat/completions"
    And the JSON response field "body" should be:
      """
      {"messages":[{"content":"Hello","role":"user"}],"model":"bedrock-claude-3-5-sonnet"}
      """

    # Request 4 -> cycles back to the primary provider
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-rr-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"
    And the JSON response field "body" should be:
      """
      {"messages":[{"content":"Hello","role":"user"}],"model":"gpt-4o"}
      """

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/mp-rr-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "mp-rr-primary-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mp-rr-anthropic-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mp-rr-bedrock-provider"
    Then the response status code should be 200

  # ====================================================================
  # SUSPENSION IS SCOPED TO THE PROVIDER/MODEL PAIR
  # ====================================================================

  Scenario: Same model name on two providers is suspended independently
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-susp-primary-provider
      spec:
        displayName: MP Susp Primary Provider
        version: v1.0
        template: openai
        context: /mp-susp-primary-ctx
        upstream:
          url: http://testbench:3000/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-susp-alt-provider
      spec:
        displayName: MP Susp Alt Provider
        version: v1.0
        template: openai
        context: /mp-susp-alt-ctx
        upstream:
          url: http://testbench:3000/alt-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Both targets use the SAME model name, on different providers
    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mp-susp-proxy
      spec:
        displayName: MP Susp Proxy
        version: v1.0
        context: /mp-susp-proxy
        provider:
          id: mp-susp-primary-provider
        additionalProviders:
          - id: mp-susp-alt-provider
            as: alt-provider
        policies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: shared-model
                    - model: shared-model
                      provider: alt-provider
                  suspendDuration: 30
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    # Wait for the loopback provider routes before counting (no rotation slot consumed).
    And I send a "POST" request to "/mp-susp-primary-ctx/chat/completions" until status 200
    And I send a "POST" request to "/mp-susp-alt-ctx/chat/completions" until status 200

    # Request 1 -> primary/shared-model, healthy
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-susp-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"

    # Request 2 -> alt-provider/shared-model, forced 429 suspends ONLY that pair
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-susp-proxy/chat/completions?statusCode=429" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Request 3 -> primary/shared-model is NOT suspended and still serves
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-susp-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"

    # Request 4 -> alt pair still suspended, so primary serves again
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-susp-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"
    And the response body should not contain "/alt-upstream"

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/mp-susp-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "mp-susp-primary-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mp-susp-alt-provider"
    Then the response status code should be 200

  # ====================================================================
  # DEFAULT PROVIDER FALLBACK (NO PROVIDER ON ANY TARGET)
  # ====================================================================

  Scenario: Targets without a provider all use the primary provider
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-default-primary-provider
      spec:
        displayName: MP Default Primary Provider
        version: v1.0
        template: openai
        context: /mp-default-primary-ctx
        upstream:
          url: http://testbench:3000/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-default-extra-provider
      spec:
        displayName: MP Default Extra Provider
        version: v1.0
        template: openai
        context: /mp-default-extra-ctx
        upstream:
          url: http://testbench:3000/extra-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # An additional provider is declared but NO target references it
    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mp-default-proxy
      spec:
        displayName: MP Default Proxy
        version: v1.0
        context: /mp-default-proxy
        provider:
          id: mp-default-primary-provider
        additionalProviders:
          - id: mp-default-extra-provider
            as: extra-provider
        policies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: model-one
                    - model: model-two
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync
    # Wait for the loopback provider routes before counting (no rotation slot consumed).
    And I send a "POST" request to "/mp-default-primary-ctx/chat/completions" until status 200
    And I send a "POST" request to "/mp-default-extra-ctx/chat/completions" until status 200

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-default-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"
    And the JSON response field "body" should be:
      """
      {"messages":[{"content":"Hello","role":"user"}],"model":"model-one"}
      """
    And the response body should not contain "/extra-upstream"

    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "/mp-default-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the JSON response field "path" should be "/primary-upstream/chat/completions"
    And the JSON response field "body" should be:
      """
      {"messages":[{"content":"Hello","role":"user"}],"model":"model-two"}
      """
    And the response body should not contain "/extra-upstream"

    # Cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-proxies/mp-default-proxy"
    Then the response status code should be 200
    When I delete the LLM provider "mp-default-primary-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mp-default-extra-provider"
    Then the response status code should be 200

  # ====================================================================
  # INVALID PROVIDER CONFIGURATION
  # ====================================================================

  Scenario: Proxy referencing a non-existent additional provider is rejected
    When I create LLM provider with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mp-invalid-primary-provider
      spec:
        displayName: MP Invalid Primary Provider
        version: v1.0
        template: openai
        context: /mp-invalid-primary-ctx
        upstream:
          url: http://testbench:3000/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create LLM proxy with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mp-invalid-proxy
      spec:
        displayName: MP Invalid Proxy
        version: v1.0
        context: /mp-invalid-proxy
        provider:
          id: mp-invalid-primary-provider
        additionalProviders:
          - id: does-not-exist-provider
            as: ghost-provider
        policies:
          - name: model-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: gpt-4o
                    - model: ghost-model
                      provider: ghost-provider
      """
    # Asserting 404, the CURRENT behaviour, not the correct one: a dangling provider reference is
    # a validation failure and POST declares 400, not 404. Restore 400 when wso2/api-platform#3268
    # is fixed — https://github.com/wso2/api-platform/issues/3268
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the JSON response should have field "message"
    # Rejected means not persisted — a separate claim from the status.
    When I send a "GET" request to the "gateway-controller" service at "/llm-proxies/mp-invalid-proxy"
    Then the response status code should be 404
    # Cleanup
    When I delete the LLM provider "mp-invalid-primary-provider"
    Then the response status code should be 200
