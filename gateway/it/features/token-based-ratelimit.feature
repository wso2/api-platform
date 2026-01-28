# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

@token-based-ratelimit
Feature: Token-Based Rate Limiting for LLMs
  As an LLM API developer
  I want to limit usage based on token counts extracted from response bodies
  So that I can effectively manage costs and quota for LLM providers

  Background:
    Given the gateway services are running

  Scenario: Enforce rate limit based on tokens extracted from response
    Given I authenticate using basic auth as "admin"
    And I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: mock-llm-template
      spec:
        displayName: Mock LLM Template
        totalTokens:
          location: payload
          identifier: $.usage.total_tokens
      """
    And I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: mock-provider
      spec:
        template: mock-llm-template
        upstream:
          url: http://it-echo-backend:80
        policies:
          - name: token-based-ratelimit
            version: v0.1.0
            params:
              totalTokenLimits:
                - count: 1000
                  duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/mock-provider/v1" to be ready

    # First request: uses 150 tokens. Remaining: 1000 - 150 = 850
    When I send a POST request to "http://localhost:8080/mock-provider/v1" with body:
      """
      {"usage": {"total_tokens": 150}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "850"

    # Second request: uses 800 tokens. Remaining: 850 - 800 = 50
    When I send a POST request to "http://localhost:8080/mock-provider/v1" with body:
      """
      {"usage": {"total_tokens": 800}}
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Remaining" should be "50"

    # Third request: tries to use 100 tokens. Quota exhausted!
    When I send a POST request to "http://localhost:8080/mock-provider/v1" with body:
      """
      {"usage": {"total_tokens": 100}}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

  Scenario: Separate limits for prompt and completion tokens
    Given I authenticate using basic auth as "admin"
    And I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: multi-token-template
      spec:
        displayName: Multi Token Template
        promptTokens:
          location: payload
          identifier: $.usage.prompt
        completionTokens:
          location: payload
          identifier: $.usage.completion
      """
    And I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: multi-token-provider
      spec:
        template: multi-token-template
        upstream:
          url: http://it-echo-backend:80
        policies:
          - name: token-based-ratelimit
            version: v0.1.0
            params:
              promptTokenLimits:
                - count: 100
                  duration: "1h"
              completionTokenLimits:
                - count: 50
                  duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/multi-token-provider/v1" to be ready

    # Use 20 prompt, 40 completion. 
    # Remaining: Prompt 80, Completion 10
    When I send a POST request to "http://localhost:8080/multi-token-provider/v1" with body:
      """
      {"usage": {"prompt": 20, "completion": 40}}
      """
    Then the response status code should be 200
    And the response header "RateLimit" should contain "completion_tokens"
    And the response header "RateLimit" should contain "r=10"

    # Try 20 completion tokens. Exceeds limit (only 10 left).
    When I send a POST request to "http://localhost:8080/multi-token-provider/v1" with body:
      """
      {"usage": {"prompt": 10, "completion": 20}}
      """
    Then the response status code should be 429
    And the response header "X-RateLimit-Quota" should be "completion_tokens"
