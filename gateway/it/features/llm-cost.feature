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
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: llm-cost-openai-test
      spec:
        displayName: LLM Cost OpenAI Test
        version: v1.0
        context: /llm-cost-openai
        upstream:
          main:
            url: http://mock-openapi:4010
        operations:
          - method: POST
            path: /openai/v1/chat/completions
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai/openai/v1/chat/completions" to be ready with method "POST" and body '{"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"
    When I delete the API "llm-cost-openai-test"
    Then the response should be successful

  Scenario: Anthropic response — cost is calculated using Anthropic token fields
    # Model: claude-3-5-haiku-20241022 — input=8e-7/token, output=4e-6/token
    # Usage: 50 input + 25 output = (50*8e-7) + (25*4e-6) = 4e-5 + 1e-4 = 0.0001400000
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: llm-cost-anthropic-test
      spec:
        displayName: LLM Cost Anthropic Test
        version: v1.0
        context: /llm-cost-anthropic
        upstream:
          main:
            url: http://mock-openapi:4010
        operations:
          - method: POST
            path: /anthropic/v1/messages
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/llm-cost-anthropic/anthropic/v1/messages" to be ready with method "POST" and body '{"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-anthropic/anthropic/v1/messages" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001400000"
    When I delete the API "llm-cost-anthropic-test"
    Then the response should be successful

  Scenario: Gemini native response — cost is calculated from usageMetadata fields
    # Model: gemini-1.5-flash-002 — input=7.5e-8/token, output=3e-7/token
    # Usage: 100 prompt + 100 completion = (100*7.5e-8) + (100*3e-7) = 7.5e-6 + 3e-5 = 0.0000375000
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: llm-cost-gemini-test
      spec:
        displayName: LLM Cost Gemini Test
        version: v1.0
        context: /llm-cost-gemini
        upstream:
          main:
            url: http://mock-openapi:4010
        operations:
          - method: POST
            path: /gemini/v1/models/{model}:generateContent
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/llm-cost-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" to be ready with method "POST" and body '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000375000"
    When I delete the API "llm-cost-gemini-test"
    Then the response should be successful

  Scenario: Unknown model — x-llm-cost is set to zero without blocking the request
    # Model: my-unknown-model-xyz — not in pricing DB, cost should be 0
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: llm-cost-unknown-test
      spec:
        displayName: LLM Cost Unknown Model Test
        version: v1.0
        context: /llm-cost-unknown
        upstream:
          main:
            url: http://mock-openapi:4010
        operations:
          - method: POST
            path: /unknown-llm/v1/chat
      """
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/llm-cost-unknown/unknown-llm/v1/chat" to be ready with method "POST" and body '{"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-unknown/unknown-llm/v1/chat" with body:
      """ json
      {"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000000000"
    When I delete the API "llm-cost-unknown-test"
    Then the response should be successful
