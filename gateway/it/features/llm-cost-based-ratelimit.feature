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

@llm-cost-based-ratelimit
Feature: LLM Cost-Based Rate Limiting
  As an API developer
  I want to rate limit LLM APIs based on monetary budgets
  So that I can control costs by setting spending limits while accounting for different token costs

  Background:
    Given the gateway services are running

  Scenario: Enforce cost-based rate limit on LLM API
    Given I authenticate using basic auth as "admin"

    # Create LLM provider template with token extraction paths
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cost-test-openai-template
      spec:
        displayName: Cost Test OpenAI Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "success"

    # Create LLM provider with llm-cost-based-ratelimit policy attached
    # System parameters define cost per token (set by admin)
    # User parameters define budget limits (set by API consumer)
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cost-test-openai-provider
      spec:
        displayName: Cost Test OpenAI Provider
        version: v1.0
        context: /cost-ratelimit
        template: cost-test-openai-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  # User-defined budget limits (API consumer sets these)
                  budgetLimits:
                    - amount: 1.0      # $1.00 budget
                      duration: "1h"   # per hour
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/cost-ratelimit/chat/completions" to be ready

    # Send requests and verify rate limiting based on cost consumption
    # Cost calculation: costs are per 1,000,000 tokens (default costPerNTokens)
    # promptCost = tokens × ($0.01/1000000) = tokens × $0.00000001
    # completionCost = tokens × ($0.02/1000000) = tokens × $0.00000002
    #
    # IMPORTANT: For post-response cost extraction (LLM APIs), the gateway cannot predict
    # the cost before the response since token usage is only known after the LLM responds.
    # Requests are blocked only when the budget is fully exhausted (remaining <= 0).
    Given I set header "Content-Type" to "application/json"

    # First request: 20M prompt + 10M completion = (20M×$0.00000001) + (10M×$0.00000002) = $0.20 + $0.20 = $0.40
    # Budget: $1.00, Consumed: $0.40, Remaining: $0.60
    When I send a POST request to "http://localhost:8080/cost-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {
          "prompt_tokens": 20000000,
          "completion_tokens": 10000000,
          "total_tokens": 30000000
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should exist
    And the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    # Second request: 30M prompt + 20M completion = (30M×$0.00000001) + (20M×$0.00000002) = $0.30 + $0.40 = $0.70
    # Budget: $1.00, Previously consumed: $0.40, This request: $0.70, Total: $1.10 (overage)
    # NOTE: With post-response cost extraction, this request is allowed because remaining > 0.
    # The cost is only consumed after the response is received. This causes an overage.
    When I send a POST request to "http://localhost:8080/cost-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "World"}],
        "usage": {
          "prompt_tokens": 30000000,
          "completion_tokens": 20000000,
          "total_tokens": 50000000
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should exist

    # Third request: Budget is now exhausted (consumed $1.10 > limit $1.00)
    # This request should be blocked because remaining <= 0
    When I send a POST request to "http://localhost:8080/cost-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Test"}],
        "usage": {
          "prompt_tokens": 1000000,
          "completion_tokens": 500000,
          "total_tokens": 1500000
        }
      }
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cost-test-openai-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cost-test-openai-template"
    Then the response status code should be 200

  Scenario: Cost-based rate limit with multiple budget time windows
    Given I authenticate using basic auth as "admin"

    # Create LLM provider template
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: multi-window-cost-template
      spec:
        displayName: Multi Window Cost Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create LLM provider with multiple budget limits
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: multi-window-cost-provider
      spec:
        displayName: Multi Window Cost Provider
        version: v1.0
        context: /multi-window-cost
        template: multi-window-cost-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  # Multiple budget limits: $5/hour AND $10/day
                  budgetLimits:
                    - amount: 5.0       # $5 per hour
                      duration: "1h"
                    - amount: 10.0      # $10 per day
                      duration: "24h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/multi-window-cost/chat/completions" to be ready

    # Must use application/json content-type for the echo backend to parse the body
    # NOTE: For post-response cost extraction, requests are blocked only when budget is exhausted.
    Given I set header "Content-Type" to "application/json"

    # Request 1: 10M prompt + 5M completion = (10M×$0.0000001) + (5M×$0.0000002) = $1.00 + $1.00 = $2.00
    # Hourly: Consumed $2.00, Remaining $3.00 | Daily: Consumed $2.00, Remaining $8.00
    When I send a POST request to "http://localhost:8080/multi-window-cost/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 10000000,
          "completion_tokens": 5000000,
          "total_tokens": 15000000
        }
      }
      """
    Then the response status code should be 200

    # Request 2: 20M prompt + 10M completion = (20M×$0.0000001) + (10M×$0.0000002) = $2.00 + $2.00 = $4.00
    # Hourly: Consumed $6.00 (overage), Remaining $0.00 | Daily: Consumed $6.00, Remaining $4.00
    # NOTE: With post-response cost extraction, this request is allowed because remaining > 0.
    When I send a POST request to "http://localhost:8080/multi-window-cost/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 20000000,
          "completion_tokens": 10000000,
          "total_tokens": 30000000
        }
      }
      """
    Then the response status code should be 200

    # Request 3: Any request should be blocked because hourly budget is now exhausted
    When I send a POST request to "http://localhost:8080/multi-window-cost/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1000000,
          "completion_tokens": 500000,
          "total_tokens": 1500000
        }
      }
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "multi-window-cost-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "multi-window-cost-template"
    Then the response status code should be 200

  Scenario: Cost calculation with different token types
    Given I authenticate using basic auth as "admin"

    # Create LLM provider template
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: token-type-cost-template
      spec:
        displayName: Token Type Cost Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.json.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create LLM provider with different costs for prompt vs completion tokens
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: token-type-cost-provider
      spec:
        displayName: Token Type Cost Provider
        version: v1.0
        context: /token-type-cost
        template: token-type-cost-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  budgetLimits:
                    - amount: 2.0       # $2 budget
                      duration: "1h"
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/token-type-cost/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Request 1: 50M prompt + 10M completion = (50M×$0.00000001) + (10M×$0.00000005) = $0.50 + $0.50 = $1.00
    When I send a POST request to "http://localhost:8080/token-type-cost/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 50000000,
          "completion_tokens": 10000000,
          "total_tokens": 60000000
        }
      }
      """
    Then the response status code should be 200

    # Request 2: 10M prompt + 20M completion = (10M×$0.00000001) + (20M×$0.00000005) = $0.10 + $1.00 = $1.10
    # Total consumed: $1.00 + $1.10 = $2.10 (exceeds $2.00 budget)
    When I send a POST request to "http://localhost:8080/token-type-cost/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 10000000,
          "completion_tokens": 20000000,
          "total_tokens": 30000000
        }
      }
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "token-type-cost-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "token-type-cost-template"
    Then the response status code should be 200

  Scenario: Cost-based rate limit returns proper headers
    Given I authenticate using basic auth as "admin"

    # Create LLM provider template
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cost-headers-template
      spec:
        displayName: Cost Headers Test Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create LLM provider using total token cost (simpler calculation)
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cost-headers-provider
      spec:
        displayName: Cost Headers Test Provider
        version: v1.0
        context: /cost-headers
        template: cost-headers-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  budgetLimits:
                    - amount: 10.0      # $10 budget
                      duration: "1h"
                  totalTokenCost: 0.10    # $0.10 per total token
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/cost-headers/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Send request and check rate limit headers
    # 50M tokens × $0.0000001 = $5.00 cost (costPerNTokens=1000000)
    When I send a POST request to "http://localhost:8080/cost-headers/chat/completions" with body:
      """
      {
        "usage": {
          "total_tokens": 50000000
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should exist
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cost-headers-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cost-headers-template"
    Then the response status code should be 200

  Scenario: Per-provider cost rate limiting isolation
    Given I authenticate using basic auth as "admin"

    # Create two LLM provider templates
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cost-provider-a-template
      spec:
        displayName: Cost Provider A Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cost-provider-b-template
      spec:
        displayName: Cost Provider B Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    # Create two LLM providers with cost rate limit policies
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cost-provider-a
      spec:
        displayName: Cost Provider A
        version: v1.0
        context: /cost-provider-a
        template: cost-provider-a-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: key-a
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  budgetLimits:
                    - amount: 5.0       # $5 budget
                      duration: "1h"
                  totalTokenCost: 1.0     # $1.00 per token
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cost-provider-b
      spec:
        displayName: Cost Provider B
        version: v1.0
        context: /cost-provider-b
        template: cost-provider-b-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: key-b
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  budgetLimits:
                    - amount: 5.0       # $5 budget
                      duration: "1h"
                  totalTokenCost: 1.0     # $1.00 per token
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/cost-provider-a/chat/completions" to be ready
    And I wait for the endpoint "http://localhost:8080/cost-provider-b/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Exhaust Provider A's budget (limit is $5)
    # 3M tokens × $0.000001 = $3.00 (costPerNTokens=1000000)
    When I send a POST request to "http://localhost:8080/cost-provider-a/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 3000000}
      }
      """
    Then the response status code should be 200

    # 2M more tokens × $0.000001 = $2.00 (total: $5.00)
    When I send a POST request to "http://localhost:8080/cost-provider-a/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 2000000}
      }
      """
    Then the response status code should be 200

    # Provider A should be rate limited (budget exhausted)
    When I send a POST request to "http://localhost:8080/cost-provider-a/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1000000}
      }
      """
    Then the response status code should be 429

    # Provider B should still work (independent budget)
    When I send a POST request to "http://localhost:8080/cost-provider-b/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 3000000}
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cost-provider-a"
    Then the response status code should be 200
    When I delete the LLM provider "cost-provider-b"
    Then the response status code should be 200
    When I delete the LLM provider template "cost-provider-a-template"
    Then the response status code should be 200
    When I delete the LLM provider template "cost-provider-b-template"
    Then the response status code should be 200

  Scenario: Zero cost usage should not consume budget
    Given I authenticate using basic auth as "admin"

    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: zero-cost-template
      spec:
        displayName: Zero Cost Test Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: zero-cost-provider
      spec:
        displayName: Zero Cost Test Provider
        version: v1.0
        context: /zero-cost-test
        template: zero-cost-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  budgetLimits:
                    - amount: 5.0       # $5 budget
                      duration: "1h"
                  totalTokenCost: 1.0     # $1.00 per token
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/zero-cost-test/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # First request: 2M tokens × $0.000001 = $2.00 (costPerNTokens=1000000)
    When I send a POST request to "http://localhost:8080/zero-cost-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 2000000}
      }
      """
    Then the response status code should be 200

    # Second request: 0 tokens = $0 cost (should not consume budget)
    When I send a POST request to "http://localhost:8080/zero-cost-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 0}
      }
      """
    Then the response status code should be 200

    # Third request: 2M tokens × $0.000001 = $2.00 (total: $4.00, still within $5 budget)
    When I send a POST request to "http://localhost:8080/zero-cost-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 2000000}
      }
      """
    Then the response status code should be 200

    # Fourth request: 2M tokens × $0.000001 = $2.00 (would exceed $5 budget)
    When I send a POST request to "http://localhost:8080/zero-cost-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 2000000}
      }
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "zero-cost-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "zero-cost-template"
    Then the response status code should be 200

  Scenario: Rate limit window resets after time window expires
    Given I authenticate using basic auth as "admin"

    # Create provider with short 10-second window for testing
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cost-window-test-template
      spec:
        displayName: Cost Window Test Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
        requestModel:
          location: payload
          identifier: $.json.model
        responseModel:
          location: payload
          identifier: $.json.model
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cost-window-test-provider
      spec:
        displayName: Cost Window Test Provider
        version: v1.0
        context: /cost-window-test
        template: cost-window-test-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-api-key
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: llm-cost-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  budgetLimits:
                    - amount: 5.0       # $5 budget
                      duration: "10s"   # Short window for testing
                  totalTokenCost: 1.0     # $1.00 per token
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/cost-window-test/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Exhaust the budget (5M tokens in 10 seconds = $5.00, costPerNTokens=1000000)
    When I send a POST request to "http://localhost:8080/cost-window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5000000}
      }
      """
    Then the response status code should be 200

    # Immediate next request should be rate limited
    When I send a POST request to "http://localhost:8080/cost-window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1000000}
      }
      """
    Then the response status code should be 429

    # Wait for window to reset (10 seconds + small buffer)
    When I wait for 11 seconds

    # After window reset, request should succeed
    When I send a POST request to "http://localhost:8080/cost-window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1000000}
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cost-window-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cost-window-test-template"
    Then the response status code should be 200

