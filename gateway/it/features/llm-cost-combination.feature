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

@llm-cost-combination
Feature: Combined LLM Cost Calculation and Budget Rate Limiting
  As an API developer
  I want to attach both the llm-cost and llm-cost-based-ratelimit policies to an LLM provider
  So that the gateway calculates real costs from LLM responses and enforces spending budgets

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Budget exhausted after real cost is calculated from LLM response
    # mock-openai returns: prompt_tokens=19, completion_tokens=10, model=gpt-4.1-2025-04-14
    # gpt-4.1-2025-04-14 pricing: $2.00/1M input, $8.00/1M output
    # Cost per request: (19 × 2e-6) + (10 × 8e-6) = 3.8e-5 + 8.0e-5 = 0.0001180000
    #
    # Budget: $0.000236 (exactly 2 × $0.000118)
    # Request 1: remaining $0.000118 → allowed
    # Request 2: remaining $0.000000 → allowed
    # Request 3: remaining <= 0 → 429 (budget exhausted)
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: combo-openai-template
      spec:
        displayName: Combo OpenAI Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: combo-openai-provider
      spec:
        displayName: Combo OpenAI Provider
        version: v1.0
        context: /combo-openai
        template: combo-openai-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: llm-cost-based-ratelimit
            version: v0
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
          - name: llm-cost
            version: v0
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # Request 1: allowed — cost is calculated by llm-cost and budget is deducted
    When I send a POST request to "http://localhost:8080/combo-openai/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    # Request 2: allowed — budget reaches exactly zero after this request
    When I send a POST request to "http://localhost:8080/combo-openai/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"

    # Request 3: blocked — budget is exhausted (remaining <= 0)
    When I send a POST request to "http://localhost:8080/combo-openai/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "combo-openai-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "combo-openai-template"
    Then the response status code should be 200

  Scenario: Budget window resets after expiry — requests succeed again
    # Same cost calculation as above: $0.000118 per request
    # Budget: $0.000236 with a 10-second window
    # After exhaustion and window expiry, new requests are allowed
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: combo-reset-template
      spec:
        displayName: Combo Reset Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: combo-reset-provider
      spec:
        displayName: Combo Reset Provider
        version: v1.0
        context: /combo-reset
        template: combo-reset-template
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: llm-cost-based-ratelimit
            version: v0
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "10s"
          - name: llm-cost
            version: v0
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # Exhaust the budget (2 requests × $0.000118 = $0.000236)
    When I send a POST request to "http://localhost:8080/combo-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/combo-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Budget exhausted — next request blocked
    When I send a POST request to "http://localhost:8080/combo-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Wait for the 10-second window to reset
    When I wait for 11 seconds

    # After window reset, requests are allowed again
    When I send a POST request to "http://localhost:8080/combo-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "combo-reset-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "combo-reset-template"
    Then the response status code should be 200
