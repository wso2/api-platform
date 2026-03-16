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

@llm-cost-based-ratelimit
Feature: LLM Cost-Based Rate Limiting
  As an API developer
  I want to rate limit LLM APIs based on monetary budgets
  So that I can control costs by setting spending limits

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Enforce cost-based rate limit on LLM API
    # mock-openai returns gpt-4.1-2025-04-14: 19 prompt × $2/1M + 10 completion × $8/1M = $0.0001180000
    # Budget: $0.000236 = exactly 2 requests worth
    # Request 1: allowed (remaining $0.000118), request 2: allowed (remaining $0), request 3: blocked
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-enforce-template
      spec:
        displayName: CBL Enforce Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-enforce-provider
      spec:
        displayName: CBL Enforce Provider
        version: v1.0
        context: /cbl-enforce
        template: cbl-enforce-template
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

    # Request 1: allowed — budget remaining drops to $0.000118
    When I send a POST request to "http://localhost:8080/cbl-enforce/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    # Request 2: allowed — budget reaches exactly $0
    When I send a POST request to "http://localhost:8080/cbl-enforce/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 3: blocked — budget exhausted
    When I send a POST request to "http://localhost:8080/cbl-enforce/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-enforce-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-enforce-template"
    Then the response status code should be 200

  Scenario: Cost-based rate limit with multiple budget time windows
    # Budget: $0.000236/1m (minute) AND $0.001180/1h (hourly — 10× per-request cost)
    # 2 requests exhaust the minute window ($0.000236) even though hourly budget still has room
    # The tighter per-minute window triggers the 429
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-multiwin-template
      spec:
        displayName: CBL Multi-Window Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-multiwin-provider
      spec:
        displayName: CBL Multi-Window Provider
        version: v1.0
        context: /cbl-multiwin
        template: cbl-multiwin-template
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
                      duration: "1m"
                    - amount: 0.001180
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

    # Request 1: allowed — both windows have budget remaining
    When I send a POST request to "http://localhost:8080/cbl-multiwin/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 2: allowed — minute budget reaches $0, hourly still has $0.000944 remaining
    When I send a POST request to "http://localhost:8080/cbl-multiwin/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Request 3: blocked — minute budget exhausted (hourly still has room, but minute wins)
    When I send a POST request to "http://localhost:8080/cbl-multiwin/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-multiwin-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-multiwin-template"
    Then the response status code should be 200

  Scenario: Cost accumulates correctly across real LLM responses
    # Uses Anthropic mock: claude-3-5-haiku-20241022
    # 50 input × $0.80/1M + 25 output × $4.00/1M = $0.0000400000 + $0.0001000000 = $0.0001400000 per request
    # Budget: $0.000280 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-anthropic-template
      spec:
        displayName: CBL Anthropic Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-anthropic-provider
      spec:
        displayName: CBL Anthropic Provider
        version: v1.0
        context: /cbl-anthropic
        template: cbl-anthropic-template
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
                    - amount: 0.000280
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

    # Request 1: allowed — cost $0.000140 deducted, remaining $0.000140
    When I send a POST request to "http://localhost:8080/cbl-anthropic/anthropic/v1/messages" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001400000"

    # Request 2: allowed — cost $0.000140 deducted, remaining $0
    When I send a POST request to "http://localhost:8080/cbl-anthropic/anthropic/v1/messages" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    # Request 3: blocked — budget exhausted
    When I send a POST request to "http://localhost:8080/cbl-anthropic/anthropic/v1/messages" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-anthropic-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-anthropic-template"
    Then the response status code should be 200

  Scenario: Cost-based rate limit returns proper rate limit headers
    # Budget $0.001 — large enough that a single request does not exhaust it
    # Verifies that x-ratelimit-cost-limit-dollars and x-ratelimit-cost-remaining-dollars
    # are present in the response after the llm-cost policy sets x-llm-cost
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-headers-template
      spec:
        displayName: CBL Headers Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-headers-provider
      spec:
        displayName: CBL Headers Provider
        version: v1.0
        context: /cbl-headers
        template: cbl-headers-template
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
                    - amount: 0.001
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

    When I send a POST request to "http://localhost:8080/cbl-headers/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"
    And the response header "x-ratelimit-cost-limit-dollars" should exist
    And the response header "x-ratelimit-cost-remaining-dollars" should exist

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-headers-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-headers-template"
    Then the response status code should be 200

  Scenario: Per-provider cost rate limiting is isolated
    # Two separate providers each with a $0.000236 budget (2 requests each)
    # Exhausting Provider A's budget does not affect Provider B's budget
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-prov-a-template
      spec:
        displayName: CBL Provider A Template
      """
    Then the response status code should be 201
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-prov-b-template
      spec:
        displayName: CBL Provider B Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-prov-a
      spec:
        displayName: CBL Provider A
        version: v1.0
        context: /cbl-prov-a
        template: cbl-prov-a-template
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
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-prov-b
      spec:
        displayName: CBL Provider B
        version: v1.0
        context: /cbl-prov-b
        template: cbl-prov-b-template
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

    # Exhaust Provider A's budget (2 requests × $0.000118 = $0.000236)
    When I send a POST request to "http://localhost:8080/cbl-prov-a/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/cbl-prov-a/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Provider A budget exhausted
    When I send a POST request to "http://localhost:8080/cbl-prov-a/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Provider B is unaffected — its budget is independent
    When I send a POST request to "http://localhost:8080/cbl-prov-b/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-prov-a"
    Then the response status code should be 200
    When I delete the LLM provider "cbl-prov-b"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-prov-a-template"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-prov-b-template"
    Then the response status code should be 200

  Scenario: Zero cost requests do not consume budget
    # Requests for an unknown model return x-llm-cost=0 (not_calculated)
    # These should not consume budget, leaving it intact for subsequent known-model requests
    # Budget: $0.000236 (2 OpenAI requests worth)
    # - 2 unknown-model requests → x-llm-cost=0, no budget consumed
    # - 2 OpenAI requests → x-llm-cost=0.0001180000 each, budget consumed
    # - 3rd OpenAI request → 429
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-zero-template
      spec:
        displayName: CBL Zero Cost Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-zero-provider
      spec:
        displayName: CBL Zero Cost Provider
        version: v1.0
        context: /cbl-zero
        template: cbl-zero-template
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

    # Unknown model — x-llm-cost=0, budget not consumed
    When I send a POST request to "http://localhost:8080/cbl-zero/unknown-llm/v1/chat" with body:
      """ json
      {"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000000000"
    And the response header "x-llm-cost-status" should be "not_calculated"

    When I send a POST request to "http://localhost:8080/cbl-zero/unknown-llm/v1/chat" with body:
      """ json
      {"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Known model — budget now starts being consumed (full $0.000236 still available)
    When I send a POST request to "http://localhost:8080/cbl-zero/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"

    When I send a POST request to "http://localhost:8080/cbl-zero/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Budget now exhausted — the zero-cost requests did not eat into it
    When I send a POST request to "http://localhost:8080/cbl-zero/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-zero-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-zero-template"
    Then the response status code should be 200

  Scenario: Rate limit window resets after time window expires
    # Budget: $0.000236 with a 10-second window
    # After exhaustion and window expiry, new requests are allowed
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-reset-template
      spec:
        displayName: CBL Reset Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-reset-provider
      spec:
        displayName: CBL Reset Provider
        version: v1.0
        context: /cbl-reset
        template: cbl-reset-template
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
    When I send a POST request to "http://localhost:8080/cbl-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/cbl-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Budget exhausted
    When I send a POST request to "http://localhost:8080/cbl-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Wait for the 10-second window to reset
    When I wait for 11 seconds

    # After window reset, budget is restored and requests are allowed
    When I send a POST request to "http://localhost:8080/cbl-reset/openai/v1/chat/completions" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001180000"

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-reset-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-reset-template"
    Then the response status code should be 200
