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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
          - name: llm-cost
            version: v1
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
            version: v1
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
            version: v1
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000280
                      duration: "1h"
          - name: llm-cost
            version: v1
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
    # are present in the response after the llm-cost policy stores cost in shared context
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.001
                      duration: "1h"
          - name: llm-cost
            version: v1
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
          - name: llm-cost
            version: v1
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
          - name: llm-cost
            version: v1
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
    # Requests for an unknown model return cost=0 (not_calculated) in shared context
    # These should not consume budget, leaving it intact for subsequent known-model requests
    # Budget: $0.000236 (2 OpenAI requests worth)
    # - 2 unknown-model requests → cost=0 (not_calculated), no budget consumed
    # - 2 OpenAI requests → cost=0.0001180000 each, budget consumed
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # Unknown model — cost=0 (not_calculated), budget not consumed
    When I send a POST request to "http://localhost:8080/cbl-zero/unknown-llm/v1/chat" with body:
      """ json
      {"model": "my-unknown-model-xyz", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "10s"
          - name: llm-cost
            version: v1
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

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-reset-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-reset-template"
    Then the response status code should be 200

  Scenario: Gemini cost calculation triggers rate limit at correct token count
    # Model: gemini-1.5-flash-002 — input=7.5e-8/token, output=3e-7/token
    # Usage: 100 prompt + 100 completion = (100*7.5e-8) + (100*3e-7) = 7.5e-6 + 3e-5 = 0.0000375000
    # Budget: $0.000075 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-gemini-template
      spec:
        displayName: CBL Gemini Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-gemini-provider
      spec:
        displayName: CBL Gemini Provider
        version: v1.0
        context: /cbl-gemini
        template: cbl-gemini-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000075
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # Request 1: allowed — cost $0.0000375 deducted, remaining $0.0000375
    When I send a POST request to "http://localhost:8080/cbl-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200

    # Request 2: allowed — budget reaches exactly $0
    When I send a POST request to "http://localhost:8080/cbl-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200

    # Request 3: blocked — budget exhausted
    When I send a POST request to "http://localhost:8080/cbl-gemini/gemini/v1/models/gemini-1.5-flash-002:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-gemini-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-gemini-template"
    Then the response status code should be 200

  Scenario: Anthropic geo and speed multipliers inflate cost correctly
    # Model: claude-opus-4-6 — input=5e-6/token, output=2.5e-5/token
    # PSE: {us: 1.1, fast: 6.0} — speed=fast, inference_geo=us echoed in response
    # Usage: 20 input + 10 output
    # baseCost = 20*5e-6 + 10*2.5e-5 = 1e-4 + 2.5e-4 = 3.5e-4
    # multiplier = 1.1 (us) × 6.0 (fast) = 6.6
    # finalCost = 3.5e-4 × 6.6 = 0.0023100000
    # Budget: $0.004620 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-anthropic-geo-speed-template
      spec:
        displayName: CBL Anthropic Geo Speed Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-anthropic-geo-speed-provider
      spec:
        displayName: CBL Anthropic Geo Speed Provider
        version: v1.0
        context: /cbl-anthropic-geo-speed
        template: cbl-anthropic-geo-speed-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.004620
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-anthropic-geo-speed/anthropic/v1/messages-geo-speed" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100, "speed": "fast"}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-geo-speed/anthropic/v1/messages-geo-speed" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100, "speed": "fast"}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-geo-speed/anthropic/v1/messages-geo-speed" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100, "speed": "fast"}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-anthropic-geo-speed-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-anthropic-geo-speed-template"
    Then the response status code should be 200

  Scenario: Anthropic 1-hour TTL cache writes billed at higher rate
    # Model: claude-opus-4-6 — 5m_write=6.25e-6, 1hr_write=1e-5, input=5e-6, output=2.5e-5
    # Usage: 10 regular input + 5 output + 100 5m-write + 500 1hr-write
    # cost = 10*5e-6 + 5*2.5e-5 + 100*6.25e-6 + 500*1e-5
    #      = 0.00005 + 0.000125 + 0.000625 + 0.005 = 0.0058000000
    # Budget: $0.011600 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-anthropic-cache1hr-template
      spec:
        displayName: CBL Anthropic Cache 1hr Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-anthropic-cache1hr-provider
      spec:
        displayName: CBL Anthropic Cache 1hr Provider
        version: v1.0
        context: /cbl-anthropic-cache1hr
        template: cbl-anthropic-cache1hr-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.011600
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-anthropic-cache1hr/anthropic/v1/messages-cache-1hr" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-cache1hr/anthropic/v1/messages-cache-1hr" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-cache1hr/anthropic/v1/messages-cache-1hr" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-anthropic-cache1hr-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-anthropic-cache1hr-template"
    Then the response status code should be 200

  Scenario: Anthropic web search tool per-query cost added to token cost
    # Model: claude-3-5-haiku-20241022 — input=8e-7/token, output=4e-6/token, web_search=0.01/query (medium)
    # Usage: 50 input + 25 output + 2 web search queries
    # cost = 50*8e-7 + 25*4e-6 + 2*0.01 = 0.00004 + 0.00010 + 0.02 = 0.0201400000
    # Budget: $0.040280 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-anthropic-websearch-template
      spec:
        displayName: CBL Anthropic Web Search Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-anthropic-websearch-provider
      spec:
        displayName: CBL Anthropic Web Search Provider
        version: v1.0
        context: /cbl-anthropic-websearch
        template: cbl-anthropic-websearch-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.040280
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-anthropic-websearch/anthropic/v1/messages-web-search" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Search the web"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-websearch/anthropic/v1/messages-web-search" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Search the web"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-websearch/anthropic/v1/messages-web-search" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Search the web"}], "max_tokens": 100}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-anthropic-websearch-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-anthropic-websearch-template"
    Then the response status code should be 200

  Scenario: Gemini context caching — cached tokens billed at reduced rate
    # Model: gemini-2.0-flash — input=1e-7/token, output=4e-7/token, cache_read=2.5e-8/token
    # Usage: 500 prompt (200 cached) + 100 completion
    # cost = (500-200)*1e-7 + 200*2.5e-8 + 100*4e-7
    #      = 3e-5 + 5e-6 + 4e-5 = 0.0000750000
    # Budget: $0.000150 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-gemini-cached-template
      spec:
        displayName: CBL Gemini Cached Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-gemini-cached-provider
      spec:
        displayName: CBL Gemini Cached Provider
        version: v1.0
        context: /cbl-gemini-cached
        template: cbl-gemini-cached-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000150
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-gemini-cached/gemini/v1/cached/gemini-2.0-flash:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-gemini-cached/gemini/v1/cached/gemini-2.0-flash:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-gemini-cached/gemini/v1/cached/gemini-2.0-flash:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-gemini-cached-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-gemini-cached-template"
    Then the response status code should be 200

  Scenario: Gemini thinking model — thinking tokens billed at reasoning rate
    # Model: gemini-2.5-flash-preview-04-17 — input=1.5e-7, output=6e-7, reasoning=3.5e-6
    # Usage: 100 prompt + 50 candidates + 30 thoughts (exclusive mode)
    # cost = 100*1.5e-7 + (80-30)*6e-7 + 30*3.5e-6
    #      = 1.5e-5 + 3e-5 + 1.05e-4 = 0.0001500000
    # Budget: $0.000300 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-gemini-thinking-template
      spec:
        displayName: CBL Gemini Thinking Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-gemini-thinking-provider
      spec:
        displayName: CBL Gemini Thinking Provider
        version: v1.0
        context: /cbl-gemini-thinking
        template: cbl-gemini-thinking-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000300
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-gemini-thinking/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-gemini-thinking/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-gemini-thinking/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-gemini-thinking-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-gemini-thinking-template"
    Then the response status code should be 200

  Scenario: Anthropic cache reads billed at reduced cache read rate
    # Model: claude-3-5-haiku-20241022 — input=8e-7/token, output=4e-6/token, cache_read=8e-8/token
    # Usage: 50 regular input + 200 cache_read + 25 output
    # cost = 50*8e-7 + 200*8e-8 + 25*4e-6
    #      = 4e-5 + 1.6e-5 + 1e-4 = 0.0001560000
    # Budget: $0.000312 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-anthropic-cache-read-template
      spec:
        displayName: CBL Anthropic Cache Read Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-anthropic-cache-read-provider
      spec:
        displayName: CBL Anthropic Cache Read Provider
        version: v1.0
        context: /cbl-anthropic-cache-read
        template: cbl-anthropic-cache-read-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000312
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-anthropic-cache-read/anthropic/v1/messages-cache-read" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-cache-read/anthropic/v1/messages-cache-read" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-anthropic-cache-read/anthropic/v1/messages-cache-read" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-anthropic-cache-read-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-anthropic-cache-read-template"
    Then the response status code should be 200

  Scenario: OpenAI prompt caching — cached tokens billed at reduced rate
    # Model: gpt-4.1-2025-04-14 — input=2e-6/token, output=8e-6/token, cache_read=5e-7/token
    # Usage: 200 prompt (100 cached) + 50 completion
    # cost = (200-100)*2e-6 + 100*5e-7 + 50*8e-6
    #      = 2e-4 + 5e-5 + 4e-4 = 0.0006500000
    # Budget: $0.001300 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-openai-cached-template
      spec:
        displayName: CBL OpenAI Cached Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-openai-cached-provider
      spec:
        displayName: CBL OpenAI Cached Provider
        version: v1.0
        context: /cbl-openai-cached
        template: cbl-openai-cached-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.001300
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-openai-cached/openai/v1/chat-cached" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-cached/openai/v1/chat-cached" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-cached/openai/v1/chat-cached" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-openai-cached-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-openai-cached-template"
    Then the response status code should be 200

  Scenario: OpenAI flex service tier — lower rates applied for flex tier
    # Model: gpt-5.4 — input_flex=1.25e-6/token, output_flex=7.5e-6/token
    # Usage: 100 prompt + 50 completion, service_tier=flex
    # cost = 100*1.25e-6 + 50*7.5e-6 = 1.25e-4 + 3.75e-4 = 0.0005000000
    # Budget: $0.001000 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-openai-flex-template
      spec:
        displayName: CBL OpenAI Flex Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-openai-flex-provider
      spec:
        displayName: CBL OpenAI Flex Provider
        version: v1.0
        context: /cbl-openai-flex
        template: cbl-openai-flex-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.001000
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-openai-flex/openai/v1/chat-flex" with body:
      """ json
      {"model": "gpt-5.4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-flex/openai/v1/chat-flex" with body:
      """ json
      {"model": "gpt-5.4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-flex/openai/v1/chat-flex" with body:
      """ json
      {"model": "gpt-5.4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-openai-flex-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-openai-flex-template"
    Then the response status code should be 200

  Scenario: OpenAI priority service tier — higher rates applied for priority tier
    # Model: gpt-4.1 — input_priority=3.5e-6/token, output_priority=1.4e-5/token
    # Usage: 100 prompt + 50 completion, service_tier=priority
    # cost = 100*3.5e-6 + 50*1.4e-5 = 3.5e-4 + 7.0e-4 = 0.0010500000
    # Budget: $0.002100 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-openai-priority-template
      spec:
        displayName: CBL OpenAI Priority Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-openai-priority-provider
      spec:
        displayName: CBL OpenAI Priority Provider
        version: v1.0
        context: /cbl-openai-priority
        template: cbl-openai-priority-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.002100
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-openai-priority/openai/v1/chat-priority" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-priority/openai/v1/chat-priority" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-priority/openai/v1/chat-priority" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-openai-priority-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-openai-priority-template"
    Then the response status code should be 200

  Scenario: OpenAI batch service tier — batch rates applied for batch tier
    # Model: gpt-4.1 — input_batches=1e-6/token, output_batches=4e-6/token
    # Usage: 100 prompt + 50 completion, service_tier=batch
    # cost = 100*1e-6 + 50*4e-6 = 1e-4 + 2e-4 = 0.0003000000
    # Budget: $0.000600 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-openai-batch-template
      spec:
        displayName: CBL OpenAI Batch Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-openai-batch-provider
      spec:
        displayName: CBL OpenAI Batch Provider
        version: v1.0
        context: /cbl-openai-batch
        template: cbl-openai-batch-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000600
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-openai-batch/openai/v1/chat-batch" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-batch/openai/v1/chat-batch" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-batch/openai/v1/chat-batch" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-openai-batch-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-openai-batch-template"
    Then the response status code should be 200

  Scenario: OpenAI reasoning tokens billed at standard output rate
    # Model: o4-mini-2025-04-16 — input=1.1e-6/token, output=4.4e-6/token
    # Usage: 100 prompt + 80 completion (includes 30 reasoning tokens billed at output rate)
    # cost = 100*1.1e-6 + 80*4.4e-6 = 1.1e-4 + 3.52e-4 = 0.0004620000
    # Budget: $0.000924 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-openai-reasoning-template
      spec:
        displayName: CBL OpenAI Reasoning Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-openai-reasoning-provider
      spec:
        displayName: CBL OpenAI Reasoning Provider
        version: v1.0
        context: /cbl-openai-reasoning
        template: cbl-openai-reasoning-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000924
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-openai-reasoning/openai/v1/chat-reasoning" with body:
      """ json
      {"model": "o4-mini-2025-04-16", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-reasoning/openai/v1/chat-reasoning" with body:
      """ json
      {"model": "o4-mini-2025-04-16", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-reasoning/openai/v1/chat-reasoning" with body:
      """ json
      {"model": "o4-mini-2025-04-16", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-openai-reasoning-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-openai-reasoning-template"
    Then the response status code should be 200

  Scenario: OpenAI web search tool — url_citation annotation adds flat per-call fee
    # Model: gpt-4.1-2025-04-14 — input=2e-6/token, output=8e-6/token, web_search=0.01/call
    # Usage: 50 prompt + 25 completion + 1 web search call (url_citation annotation present)
    # cost = 50*2e-6 + 25*8e-6 + 0.01 = 1e-4 + 2e-4 + 0.01 = 0.0103000000
    # Budget: $0.020600 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-openai-web-search-template
      spec:
        displayName: CBL OpenAI Web Search Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-openai-web-search-provider
      spec:
        displayName: CBL OpenAI Web Search Provider
        version: v1.0
        context: /cbl-openai-web-search
        template: cbl-openai-web-search-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.020600
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-openai-web-search/openai/v1/chat-web-search" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-web-search/openai/v1/chat-web-search" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-openai-web-search/openai/v1/chat-web-search" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-openai-web-search-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-openai-web-search-template"
    Then the response status code should be 200

  Scenario: Mistral — bare model name resolved and cost calculated correctly
    # Model: mistral-small-latest — input=1e-7/token, output=3e-7/token
    # Usage: 100 prompt + 50 completion = (100*1e-7) + (50*3e-7) = 1e-5 + 1.5e-5 = 0.0000250000
    # Budget: $0.000050 = exactly 2 requests worth
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-mistral-template
      spec:
        displayName: CBL Mistral Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-mistral-provider
      spec:
        displayName: CBL Mistral Provider
        version: v1.0
        context: /cbl-mistral
        template: cbl-mistral-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000050
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    When I send a POST request to "http://localhost:8080/cbl-mistral/mistral/v1/chat/completions" with body:
      """ json
      {"model": "mistral-small-latest", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-mistral/mistral/v1/chat/completions" with body:
      """ json
      {"model": "mistral-small-latest", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-mistral/mistral/v1/chat/completions" with body:
      """ json
      {"model": "mistral-small-latest", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-mistral-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-mistral-template"
    Then the response status code should be 200

  Scenario: No model field in response — zero cost never triggers rate limit
    # The upstream response contains no model field — cost calculation returns 0
    # With a very tight budget ($0.000001), 3 requests still all pass because cost=0
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: cbl-no-model-template
      spec:
        displayName: CBL No Model Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: cbl-no-model-provider
      spec:
        displayName: CBL No Model Provider
        version: v1.0
        context: /cbl-no-model
        template: cbl-no-model-template
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
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
                params:
                  budgetLimits:
                    - amount: 0.000001
                      duration: "1h"
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods:
                  - '*'
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"

    # All 3 requests pass — zero cost never consumes the budget
    When I send a POST request to "http://localhost:8080/cbl-no-model/unknown-llm/v1/no-model-field" with body:
      """ json
      {"messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-no-model/unknown-llm/v1/no-model-field" with body:
      """ json
      {"messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    When I send a POST request to "http://localhost:8080/cbl-no-model/unknown-llm/v1/no-model-field" with body:
      """ json
      {"messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "cbl-no-model-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "cbl-no-model-template"
    Then the response status code should be 200
