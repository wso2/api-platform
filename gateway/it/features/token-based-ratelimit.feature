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
Feature: Token-Based Rate Limiting
  As an API developer
  I want to rate limit LLM APIs based on token usage
  So that I can control costs and prevent abuse based on actual resource consumption

  Background:
    Given the gateway services are running

  Scenario: Enforce token-based rate limit on LLM API
    Given I authenticate using basic auth as "admin"
    
    # Create LLM provider template with token extraction paths
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: test-openai-template
      spec:
        displayName: Test OpenAI Template
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

    # Create LLM provider with token-based-ratelimit policy attached
    # Note: Upstream, accessControl, and policies are required fields
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: test-openai-provider
      spec:
        displayName: Test OpenAI Provider
        version: v1.0
        context: /token-ratelimit
        template: test-openai-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 10
                      duration: "1m"
                  totalTokenLimits:
                    - count: 20
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "success"
    And I wait for the endpoint "http://localhost:8080/token-ratelimit/chat/completions" to be ready

    # Send requests and verify rate limiting based on token consumption
    # The echo backend wraps the request body in a 'json' field in the response
    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"
    
    # First request: consume 5 prompt tokens
    When I send a POST request to "http://localhost:8080/token-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 3,
          "total_tokens": 8
        }
      }
      """
    Then the response status code should be 200

    # Second request: consume 5 more prompt tokens (total 10/10)
    When I send a POST request to "http://localhost:8080/token-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "World"}],
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 3,
          "total_tokens": 8
        }
      }
      """
    Then the response status code should be 200

    # Third request: should be rate limited (prompt token quota exhausted)
    When I send a POST request to "http://localhost:8080/token-ratelimit/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Test"}],
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "test-openai-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "test-openai-template"
    Then the response status code should be 200

  Scenario: Token-based rate limit with multiple quotas
    Given I authenticate using basic auth as "admin"
    
    # Create LLM provider template
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: multi-quota-template
      spec:
        displayName: Multi Quota Template
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

    # Create LLM provider with multiple token quota policies
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: multi-quota-provider
      spec:
        displayName: Multi Quota Provider
        version: v1.0
        context: /multi-quota
        template: multi-quota-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 5
                      duration: "1m"
                  completionTokenLimits:
                    - count: 10
                      duration: "1m"
                  totalTokenLimits:
                    - count: 15
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/multi-quota/chat/completions" to be ready

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # Request 1: consume 5 prompt tokens (exhausts prompt quota)
    When I send a POST request to "http://localhost:8080/multi-quota/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 5,
          "completion_tokens": 5,
          "total_tokens": 10
        }
      }
      """
    Then the response status code should be 200

    # Request 2: should fail due to prompt token limit (5/5 exhausted)
    # even though completion (5/10) and total (10/15) quotas have room
    When I send a POST request to "http://localhost:8080/multi-quota/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "multi-quota-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "multi-quota-template"
    Then the response status code should be 200

  Scenario: Token-based rate limit returns proper headers
    Given I authenticate using basic auth as "admin"
    
    # Create LLM provider template
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: headers-test-template
      spec:
        displayName: Headers Test Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
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

    # Create LLM provider with rate limit policy
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: headers-test-provider
      spec:
        displayName: Headers Test Provider
        version: v1.0
        context: /headers-ratelimit
        template: headers-test-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 100
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/headers-ratelimit/chat/completions" to be ready

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # Send request and check rate limit headers
    When I send a POST request to "http://localhost:8080/headers-ratelimit/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 10,
          "completion_tokens": 5,
          "total_tokens": 15
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-RateLimit-Limit" should exist
    And the response header "X-RateLimit-Remaining" should exist
    And the response header "X-RateLimit-Reset" should exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "headers-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "headers-test-template"
    Then the response status code should be 200

  Scenario: Per-provider rate limiting isolation
    Given I authenticate using basic auth as "admin"
    
    # Create two LLM provider templates
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: provider-a-template
      spec:
        displayName: Provider A Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
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
        name: provider-b-template
      spec:
        displayName: Provider B Template
        promptTokens:
          location: payload
          identifier: $.json.usage.prompt_tokens
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

    # Create two LLM providers with rate limit policies
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: provider-a
      spec:
        displayName: Provider A
        version: v1.0
        context: /provider-a
        template: provider-a-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: provider-b
      spec:
        displayName: Provider B
        version: v1.0
        context: /provider-b
        template: provider-b-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/provider-a/chat/completions" to be ready
    And I wait for the endpoint "http://localhost:8080/provider-b/chat/completions" to be ready

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # Exhaust Provider A's quota (limit is 5 per hour)
    # Use multiple requests since cost is extracted from response
    When I send a POST request to "http://localhost:8080/provider-a/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 2,
          "completion_tokens": 1,
          "total_tokens": 3
        }
      }
      """
    Then the response status code should be 200

    # Second request: 2 more tokens (total: 5, remaining: 0)
    When I send a POST request to "http://localhost:8080/provider-a/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 200

    # Provider A should be rate limited (quota exhausted)
    When I send a POST request to "http://localhost:8080/provider-a/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 0,
          "total_tokens": 1
        }
      }
      """
    Then the response status code should be 429

    # Provider B should still work (independent quota)
    When I send a POST request to "http://localhost:8080/provider-b/chat/completions" with body:
      """
      {
        "usage": {
          "prompt_tokens": 3,
          "completion_tokens": 3,
          "total_tokens": 6
        }
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "provider-a"
    Then the response status code should be 200
    When I delete the LLM provider "provider-b"
    Then the response status code should be 200
    When I delete the LLM provider template "provider-a-template"
    Then the response status code should be 200
    When I delete the LLM provider template "provider-b-template"
    Then the response status code should be 200

  Scenario: Multiple quotas are enforced independently with correct quota identified
    Given I authenticate using basic auth as "admin"
    
    # Create template with all three token types
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: multi-quota-detailed-template
      spec:
        displayName: Multi Quota Detailed Template
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

    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: multi-quota-detailed-provider
      spec:
        displayName: Multi Quota Detailed Provider
        version: v1.0
        context: /multi-quota-detailed
        template: multi-quota-detailed-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  promptTokenLimits:
                    - count: 10
                      duration: "1m"
                  completionTokenLimits:
                    - count: 20
                      duration: "1m"
                  totalTokenLimits:
                    - count: 25
                      duration: "1m"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/multi-quota-detailed/chat/completions" to be ready

    # Must use application/json content-type for the echo backend to parse the body
    Given I set header "Content-Type" to "application/json"

    # First request: 8 prompt + 15 completion = 23 total
    # All within limits: prompt(8/10), completion(15/20), total(23/25)
    When I send a POST request to "http://localhost:8080/multi-quota-detailed/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 8,
          "completion_tokens": 15,
          "total_tokens": 23
        }
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "2"

    # Second request: 3 prompt + 5 completion = 8 total
    # With response-phase cost extraction, request is allowed (cost not known yet)
    # completion_tokens becomes 15+5=20 (exhausted)
    # prompt_tokens becomes 8+3=11 (exceeded after consumption)
    # total_tokens becomes 23+8=31 (exceeded after consumption)
    When I send a POST request to "http://localhost:8080/multi-quota-detailed/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 3,
          "completion_tokens": 5,
          "total_tokens": 8
        }
      }
      """
    Then the response status code should be 200
    # completion_tokens is now exhausted (20/20), which is the most restrictive
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Third request: should be blocked because completion_tokens quota is exhausted
    When I send a POST request to "http://localhost:8080/multi-quota-detailed/chat/completions" with body:
      """
      {
        "model": "gpt-4",
        "usage": {
          "prompt_tokens": 1,
          "completion_tokens": 1,
          "total_tokens": 2
        }
      }
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"
    # Verify correct quota is identified in header (if supported)
    And the response header "X-Ratelimit-Quota" should exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "multi-quota-detailed-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "multi-quota-detailed-template"
    Then the response status code should be 200

  Scenario: Rate limit window resets after time window expires
    Given I authenticate using basic auth as "admin"
    
    # Create provider with short 10-second window for testing
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: window-test-template
      spec:
        displayName: Window Test Template
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
        name: window-test-provider
      spec:
        displayName: Window Test Provider
        version: v1.0
        context: /window-test
        template: window-test-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "10s"  # Short window for testing
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/window-test/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Exhaust the quota (5 tokens in 10 seconds)
    When I send a POST request to "http://localhost:8080/window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Immediate next request should be rate limited
    When I send a POST request to "http://localhost:8080/window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1}
      }
      """
    Then the response status code should be 429

    # Wait for window to reset (10 seconds + small buffer)
    When I wait for 11 seconds

    # After window reset, request should succeed
    When I send a POST request to "http://localhost:8080/window-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "4"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "window-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "window-test-template"
    Then the response status code should be 200


  Scenario: Zero token usage should not consume quota
    Given I authenticate using basic auth as "admin"
    
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: zero-token-template
      spec:
        displayName: Zero Token Test Template
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
        name: zero-token-provider
      spec:
        displayName: Zero Token Test Provider
        version: v1.0
        context: /zero-token-test
        template: zero-token-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 10
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/zero-token-test/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # First request: 5 tokens
    When I send a POST request to "http://localhost:8080/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "5"

    # Second request: 0 tokens should not consume quota
    When I send a POST request to "http://localhost:8080/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 0}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "5"

    # Third request: 5 tokens (should still be within limit)
    When I send a POST request to "http://localhost:8080/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 5}
      }
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Fourth request: should be rate limited
    When I send a POST request to "http://localhost:8080/zero-token-test/chat/completions" with body:
      """
      {
        "usage": {"total_tokens": 1}
      }
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "zero-token-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "zero-token-template"
    Then the response status code should be 200

  Scenario: Cost extraction from request headers blocks immediately
    Given I authenticate using basic auth as "admin"
    
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: header-cost-template
      spec:
        displayName: Header Cost Test Template
        totalTokens:
          location: header
          identifier: X-Token-Cost
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
        name: header-cost-provider
      spec:
        displayName: Header Cost Test Provider
        version: v1.0
        context: /header-cost-test
        template: header-cost-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 10
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/header-cost-test/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"
    And I set header "X-Token-Cost" to "6"

    # First request: 6 tokens via header (request-phase extraction)
    When I send a POST request to "http://localhost:8080/header-cost-test/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "4"

    # Second request: 6 tokens would exceed limit (10), should block immediately
    When I send a POST request to "http://localhost:8080/header-cost-test/chat/completions" with body:
      """
      {"model": "gpt-4"}
      """
    Then the response status code should be 429
    And the response body should contain "Rate limit exceeded"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "header-cost-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "header-cost-template"
    Then the response status code should be 200

  Scenario: Template change triggers cache invalidation
    Given I authenticate using basic auth as "admin"
    
    # Create initial template
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: change-test-template
      spec:
        displayName: Change Test Template
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

    # Create provider with limit of 5
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: change-test-provider
      spec:
        displayName: Change Test Provider
        version: v1.0
        context: /change-test
        template: change-test-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/change-test/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Use up the quota (5/5)
    When I send a POST request to "http://localhost:8080/change-test/chat/completions" with body:
      """
      {"usage": {"total_tokens": 5}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # Next request should be blocked
    When I send a POST request to "http://localhost:8080/change-test/chat/completions" with body:
      """
      {"usage": {"total_tokens": 1}}
      """
    Then the response status code should be 429

    # Delete and recreate provider with higher limit
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "change-test-provider"
    Then the response status code should be 200

    # Small delay to allow cleanup
    When I wait for 2 seconds

    # Recreate provider with limit of 10 (should get fresh quota)
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: change-test-provider
      spec:
        displayName: Change Test Provider
        version: v1.0
        context: /change-test
        template: change-test-template
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
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 10
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/change-test/chat/completions" to be ready

    # Request with 5 tokens should now work (new limit is 10)
    When I send a POST request to "http://localhost:8080/change-test/chat/completions" with body:
      """
      {"usage": {"total_tokens": 5}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "5"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "change-test-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "change-test-template"
    Then the response status code should be 200

  Scenario: Different providers with same template have isolated quotas
    Given I authenticate using basic auth as "admin"
    
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: shared-template
      spec:
        displayName: Shared Template
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

    # Create first provider
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: provider-alpha
      spec:
        displayName: Provider Alpha
        version: v1.0
        context: /provider-alpha
        template: shared-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: key-alpha
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201

    # Create second provider with same template
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: provider-beta
      spec:
        displayName: Provider Beta
        version: v1.0
        context: /provider-beta
        template: shared-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: key-beta
        accessControl:
          mode: deny_all
          exceptions:
            - path: /chat/completions
              methods: [POST, GET]
        policies:
          - name: token-based-ratelimit
            version: v0.1.0
            paths:
              - path: /chat/completions
                methods: [POST]
                params:
                  totalTokenLimits:
                    - count: 5
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for the endpoint "http://localhost:8080/provider-alpha/chat/completions" to be ready
    And I wait for the endpoint "http://localhost:8080/provider-beta/chat/completions" to be ready

    Given I set header "Content-Type" to "application/json"

    # Exhaust provider-alpha's quota
    When I send a POST request to "http://localhost:8080/provider-alpha/chat/completions" with body:
      """
      {"usage": {"total_tokens": 5}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "0"

    # provider-alpha should now be blocked
    When I send a POST request to "http://localhost:8080/provider-alpha/chat/completions" with body:
      """
      {"usage": {"total_tokens": 1}}
      """
    Then the response status code should be 429

    # provider-beta should still have full quota
    When I send a POST request to "http://localhost:8080/provider-beta/chat/completions" with body:
      """
      {"usage": {"total_tokens": 3}}
      """
    Then the response status code should be 200
    And the response header "X-Ratelimit-Remaining" should be "2"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "provider-alpha"
    Then the response status code should be 200
    When I delete the LLM provider "provider-beta"
    Then the response status code should be 200
    When I delete the LLM provider template "shared-template"
    Then the response status code should be 200
