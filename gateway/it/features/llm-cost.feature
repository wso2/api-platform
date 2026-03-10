# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
# either express or implied.
# --------------------------------------------------------------------

@llm-cost
Feature: LLM Cost System Policy
  As a gateway operator
  I want the llm-cost system policy to automatically calculate the monetary cost of LLM calls
  So that downstream policies (e.g. budget rate limiting) can enforce spending limits

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: OpenAI response — cost is calculated and injected as x-llm-cost header
    # Model: gpt-4.1-2025-04-14 — input=2e-6/token, output=8e-6/token
    # Usage: 19 prompt + 10 completion = (19*2e-6) + (10*8e-6) = 3.8e-5 + 8e-5 = 0.0001180000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-template
      spec:
        displayName: LLM Cost OpenAI Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-provider
      spec:
        displayName: LLM Cost OpenAI Provider
        version: v1.0
        context: /llm-cost-openai
        template: llm-cost-openai-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai/openai/v1/chat/completions" to be ready with method "POST" and body '{"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-template"
    Then the response status code should be 200

  Scenario: Anthropic response — cost is calculated using Anthropic token fields
    # Model: claude-3-5-haiku-20241022 — input=8e-7/token, output=4e-6/token
    # Usage: 50 input + 25 output = (50*8e-7) + (25*4e-6) = 4e-5 + 1e-4 = 0.0001400000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-anthropic-template
      spec:
        displayName: LLM Cost Anthropic Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-anthropic-provider
      spec:
        displayName: LLM Cost Anthropic Provider
        version: v1.0
        context: /llm-cost-anthropic
        template: llm-cost-anthropic-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/llm-cost-anthropic/anthropic/v1/messages" to be ready with method "POST" and body '{"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-anthropic/anthropic/v1/messages" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001400000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-anthropic-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-anthropic-template"
    Then the response status code should be 200

  Scenario: Gemini native response — cost is calculated from usageMetadata fields
    # Model: gemini-1.5-flash-002 — input=7.5e-8/token, output=3e-7/token
    # Usage: 100 prompt + 100 completion = (100*7.5e-8) + (100*3e-7) = 7.5e-6 + 3e-5 = 0.0000375000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-gemini-template
      spec:
        displayName: LLM Cost Gemini Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-gemini-provider
      spec:
        displayName: LLM Cost Gemini Provider
        version: v1.0
        context: /llm-cost-gemini
        template: llm-cost-gemini-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/llm-cost-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" to be ready with method "POST" and body '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000375000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-gemini-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-gemini-template"
    Then the response status code should be 200

  Scenario: Unknown model — x-llm-cost is set to zero without blocking the request
    # Model: my-unknown-model-xyz — not in pricing DB, cost should be 0
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-unknown-template
      spec:
        displayName: LLM Cost Unknown Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-unknown-provider
      spec:
        displayName: LLM Cost Unknown Provider
        version: v1.0
        context: /llm-cost-unknown
        template: llm-cost-unknown-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/llm-cost-unknown/unknown-llm/v1/chat" to be ready with method "POST" and body '{"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-unknown/unknown-llm/v1/chat" with body:
      """ json
      {"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000000000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-unknown-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-unknown-template"
    Then the response status code should be 200

  Scenario: Custom pricing — new model added via pricing file is calculated correctly
    # Model: my-enterprise-llm-v1 — not in embedded DB, only in custom pricing file
    # Custom price: input=1e-5/token, output=2e-5/token
    # Usage: 80 prompt + 40 completion = (80*1e-5) + (40*2e-5) = 8e-4 + 8e-4 = 0.0016000000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-custom-new-template
      spec:
        displayName: LLM Cost Custom New Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-custom-new-provider
      spec:
        displayName: LLM Cost Custom New Provider
        version: v1.0
        context: /llm-cost-custom-new
        template: llm-cost-custom-new-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/llm-cost-custom-new/custom-llm/v1/chat" to be ready with method "POST" and body '{"model": "my-enterprise-llm-v1", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-custom-new/custom-llm/v1/chat" with body:
      """ json
      {"model": "my-enterprise-llm-v1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0016000000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-custom-new-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-custom-new-template"
    Then the response status code should be 200

  Scenario: Custom pricing — existing model price overridden via pricing file
    # Model: gpt-3.5-turbo — overridden with a negotiated lower rate
    # Embedded price: input=5e-7/token, output=1.5e-6/token
    # Custom override: input=2e-7/token, output=5e-7/token (~60% discount)
    # Usage: 100 prompt + 50 completion
    # WITH override: (100*2e-7) + (50*5e-7) = 2e-5 + 2.5e-5 = 0.0000450000
    # WITHOUT override: (100*5e-7) + (50*1.5e-6) = 5e-5 + 7.5e-5 = 0.0001250000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-custom-override-template
      spec:
        displayName: LLM Cost Custom Override Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-custom-override-provider
      spec:
        displayName: LLM Cost Custom Override Provider
        version: v1.0
        context: /llm-cost-custom-override
        template: llm-cost-custom-override-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/llm-cost-custom-override/custom-llm/v1/gpt35" to be ready with method "POST" and body '{"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-custom-override/custom-llm/v1/gpt35" with body:
      """ json
      {"model": "gpt-3.5-turbo", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000450000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-custom-override-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-custom-override-template"
    Then the response status code should be 200

  Scenario: RestApi kind — x-llm-cost header is not injected (policy is LLM-only)
    # llm-cost is a system policy restricted to LlmProvider/LlmProxy kinds.
    # A plain RestApi pointing at an LLM backend must NOT receive the x-llm-cost header.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: llm-cost-restapi-exclusion-test
      spec:
        displayName: LLM Cost RestApi Exclusion Test
        version: v1.0
        context: /llm-cost-restapi-exclusion
        upstream:
          main:
            url: http://mock-openapi:4010
        operations:
          - method: POST
            path: /openai/v1/chat/completions
      """
    Then the response should be successful
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/llm-cost-restapi-exclusion/openai/v1/chat/completions" to be ready with method "POST" and body '{"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-restapi-exclusion/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should not exist
    Given I authenticate using basic auth as "admin"
    When I delete the API "llm-cost-restapi-exclusion-test"
    Then the response should be successful

