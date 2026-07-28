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

@model-weighted-round-robin-multi-provider
Feature: Model Weighted Round-Robin Multi-Provider Routing
  Test the model-weighted-round-robin policy when weighted targets select both a
  model and an additional LLM provider. Weights build a repeating sequence; each
  entry may name a provider. Targets without a "provider" use the LlmProxy primary
  provider, and targets naming a provider route to that provider's named upstream
  with "selected_provider" metadata set.

  Each provider forwards to the sample backend under a distinct upstream path, and
  the sample backend echoes the request, so the echoed path proves provider
  selection and the echoed model proves the model rewrite.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ====================================================================
  # WEIGHTED SEQUENCE ACROSS PRIMARY + TWO ADDITIONAL PROVIDERS
  # ====================================================================

  Scenario: Weighted sequence distributes across providers by weight
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-primary-provider
      spec:
        displayName: MPW Primary Provider
        version: v1.0
        template: openai
        context: /mpw-primary-ctx
        upstream:
          url: http://sample-backend:9080/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-anthropic-provider
      spec:
        displayName: MPW Anthropic Provider
        version: v1.0
        template: openai
        context: /mpw-anthropic-ctx
        upstream:
          url: http://sample-backend:9080/anthropic-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-bedrock-provider
      spec:
        displayName: MPW Bedrock Provider
        version: v1.0
        template: openai
        context: /mpw-bedrock-ctx
        upstream:
          url: http://sample-backend:9080/bedrock-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Weights 2 / 1 / 1 -> sequence: gpt-4o, gpt-4o, claude, bedrock
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mpw-proxy
      spec:
        displayName: MPW Proxy
        version: v1.0
        context: /mpw-proxy
        provider:
          id: mpw-primary-provider
        additionalProviders:
          - id: mpw-anthropic-provider
            as: anthropic-provider
          - id: mpw-bedrock-provider
            as: bedrock-provider
        policies:
          - name: model-weighted-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: gpt-4o
                      weight: 2
                    - model: claude-sonnet-4-5-20250929
                      provider: anthropic-provider
                      weight: 1
                    - model: bedrock-claude-3-5-sonnet
                      provider: bedrock-provider
                      weight: 1
                  suspendDuration: 60
      """
    Then the response status should be 201
    And I wait for policy snapshot sync

    # Request 1 -> gpt-4o on the primary provider (weight slot 1 of 2)
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"

    # Request 2 -> gpt-4o again on the primary provider (weight slot 2 of 2)
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"

    # Request 3 -> anthropic-provider
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/anthropic-upstream"
    And the response body should contain "claude-sonnet-4-5-20250929"

    # Request 4 -> bedrock-provider
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/bedrock-upstream"
    And the response body should contain "bedrock-claude-3-5-sonnet"

    # Request 5 -> sequence wraps back to gpt-4o on the primary provider
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "gpt-4o"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mpw-proxy"
    Then the response should be successful
    When I delete the LLM provider "mpw-primary-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mpw-anthropic-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mpw-bedrock-provider"
    Then the response status code should be 200

  # ====================================================================
  # SUSPENSION IS SCOPED TO THE PROVIDER/MODEL PAIR
  # ====================================================================

  Scenario: Weighted same model name on two providers is suspended independently
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-susp-primary-provider
      spec:
        displayName: MPW Susp Primary Provider
        version: v1.0
        template: openai
        context: /mpw-susp-primary-ctx
        upstream:
          url: http://sample-backend:9080/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-susp-alt-provider
      spec:
        displayName: MPW Susp Alt Provider
        version: v1.0
        template: openai
        context: /mpw-susp-alt-ctx
        upstream:
          url: http://sample-backend:9080/alt-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Equal weights, SAME model name, different providers
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mpw-susp-proxy
      spec:
        displayName: MPW Susp Proxy
        version: v1.0
        context: /mpw-susp-proxy
        provider:
          id: mpw-susp-primary-provider
        additionalProviders:
          - id: mpw-susp-alt-provider
            as: alt-provider
        policies:
          - name: model-weighted-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: shared-model
                      weight: 1
                    - model: shared-model
                      provider: alt-provider
                      weight: 1
                  suspendDuration: 30
      """
    Then the response status should be 201
    And I wait for policy snapshot sync

    # Request 1 -> primary/shared-model, healthy
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-susp-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"

    # Request 2 -> alt-provider/shared-model, forced 500 suspends ONLY that pair
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-susp-proxy/chat/completions?statusCode=500" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 500

    # Request 3 -> primary/shared-model still serves
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-susp-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"

    # Request 4 -> alt pair still suspended, primary serves again
    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-susp-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should not contain "/alt-upstream"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mpw-susp-proxy"
    Then the response should be successful
    When I delete the LLM provider "mpw-susp-primary-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mpw-susp-alt-provider"
    Then the response status code should be 200

  # ====================================================================
  # DEFAULT PROVIDER FALLBACK
  # ====================================================================

  Scenario: Weighted targets without a provider all use the primary provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-default-primary-provider
      spec:
        displayName: MPW Default Primary Provider
        version: v1.0
        template: openai
        context: /mpw-default-primary-ctx
        upstream:
          url: http://sample-backend:9080/primary-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: mpw-default-extra-provider
      spec:
        displayName: MPW Default Extra Provider
        version: v1.0
        template: openai
        context: /mpw-default-extra-ctx
        upstream:
          url: http://sample-backend:9080/extra-upstream
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I deploy this LLM proxy configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProxy
      metadata:
        name: mpw-default-proxy
      spec:
        displayName: MPW Default Proxy
        version: v1.0
        context: /mpw-default-proxy
        provider:
          id: mpw-default-primary-provider
        additionalProviders:
          - id: mpw-default-extra-provider
            as: extra-provider
        policies:
          - name: model-weighted-round-robin
            version: v1
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  models:
                    - model: model-one
                      weight: 2
                    - model: model-two
                      weight: 1
      """
    Then the response status should be 201
    And I wait for policy snapshot sync

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-default-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "model-one"
    And the response body should not contain "/extra-upstream"

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-default-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "model-one"

    When I set header "Content-Type" to "application/json"
    And I send a POST request to "http://localhost:8080/mpw-default-proxy/chat/completions" with body:
      """
      {"model": "original-model", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response body should contain "/primary-upstream"
    And the response body should contain "model-two"
    And the response body should not contain "/extra-upstream"

    # Cleanup
    When I send a DELETE request to the "gateway-controller" service at "/llm-proxies/mpw-default-proxy"
    Then the response should be successful
    When I delete the LLM provider "mpw-default-primary-provider"
    Then the response status code should be 200
    When I delete the LLM provider "mpw-default-extra-provider"
    Then the response status code should be 200
