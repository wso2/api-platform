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

  Scenario: Anthropic geo and speed multipliers — x-llm-cost reflects combined 6.6x multiplier
    # Model: claude-opus-4-6 — input=5e-6/token, output=2.5e-5/token
    # PSE: {us: 1.1, fast: 6.0} — request sends speed=fast, response echoes inference_geo=us
    # Usage: 20 input + 10 output, no cache
    # baseCost = 20*5e-6 + 10*2.5e-5 = 1e-4 + 2.5e-4 = 3.5e-4
    # multiplier = 1.1 (us) × 6.0 (fast) = 6.6
    # finalCost = 3.5e-4 × 6.6 = 0.0023100000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-anthropic-geo-speed-template
      spec:
        displayName: LLM Cost Anthropic Geo Speed Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-anthropic-geo-speed-provider
      spec:
        displayName: LLM Cost Anthropic Geo Speed Provider
        version: v1.0
        context: /llm-cost-anthropic-geo-speed
        template: llm-cost-anthropic-geo-speed-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-anthropic-geo-speed/anthropic/v1/messages-geo-speed" to be ready with method "POST" and body '{"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100, "speed": "fast"}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-anthropic-geo-speed/anthropic/v1/messages-geo-speed" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100, "speed": "fast"}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0023100000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-anthropic-geo-speed-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-anthropic-geo-speed-template"
    Then the response status code should be 200

  Scenario: Anthropic 1-hour TTL cache writes — billed at higher rate than 5-minute TTL
    # Model: claude-opus-4-6 — 5m_write=6.25e-6, 1hr_write=1e-5, input=5e-6, output=2.5e-5
    # Usage: 10 input (regular) + 5 output + 100 5m-write + 500 1hr-write (via cache_creation breakdown)
    # PromptTokens = 10+600+0 = 610; regularPrompt = 610-0-100-500 = 10
    # cost = 10*5e-6 + 5*2.5e-5 + 100*6.25e-6 + 500*1e-5
    #      = 0.00005 + 0.000125 + 0.000625 + 0.005 = 0.0058000000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-anthropic-cache1hr-template
      spec:
        displayName: LLM Cost Anthropic Cache 1hr Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-anthropic-cache1hr-provider
      spec:
        displayName: LLM Cost Anthropic Cache 1hr Provider
        version: v1.0
        context: /llm-cost-anthropic-cache1hr
        template: llm-cost-anthropic-cache1hr-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-anthropic-cache1hr/anthropic/v1/messages-cache-1hr" to be ready with method "POST" and body '{"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-anthropic-cache1hr/anthropic/v1/messages-cache-1hr" with body:
      """ json
      {"model": "claude-opus-4-6", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0058000000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-anthropic-cache1hr-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-anthropic-cache1hr-template"
    Then the response status code should be 200

  Scenario: Anthropic web search tool — per-query cost added on top of token cost
    # Model: claude-3-5-haiku-20241022 — input=8e-7/token, output=4e-6/token, web_search=0.01/query (medium)
    # Usage: 50 input + 25 output + 2 web search queries (no search_context_size → defaults to medium)
    # cost = 50*8e-7 + 25*4e-6 + 2*0.01 = 0.00004 + 0.00010 + 0.02 = 0.0201400000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-anthropic-websearch-template
      spec:
        displayName: LLM Cost Anthropic Web Search Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-anthropic-websearch-provider
      spec:
        displayName: LLM Cost Anthropic Web Search Provider
        version: v1.0
        context: /llm-cost-anthropic-websearch
        template: llm-cost-anthropic-websearch-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-anthropic-websearch/anthropic/v1/messages-web-search" to be ready with method "POST" and body '{"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Search the web"}], "max_tokens": 100}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-anthropic-websearch/anthropic/v1/messages-web-search" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Search the web"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0201400000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-anthropic-websearch-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-anthropic-websearch-template"
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

  Scenario: Gemini context caching — cached tokens billed at reduced cache read rate
    # Model: gemini-2.0-flash — input=1e-7/token, output=4e-7/token, cache_read=2.5e-8/token
    # Usage: 500 prompt (includes 200 cached) + 100 completion
    # cost = (500-200)*1e-7 + 200*2.5e-8 + 100*4e-7
    #      = 3e-5 + 5e-6 + 4e-5 = 0.0000750000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-gemini-cached-template
      spec:
        displayName: LLM Cost Gemini Cached Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-gemini-cached-provider
      spec:
        displayName: LLM Cost Gemini Cached Provider
        version: v1.0
        context: /llm-cost-gemini-cached
        template: llm-cost-gemini-cached-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-gemini-cached/gemini/v1/cached/gemini-2.0-flash:generateContent" to be ready with method "POST" and body '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-gemini-cached/gemini/v1/cached/gemini-2.0-flash:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000750000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-gemini-cached-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-gemini-cached-template"
    Then the response status code should be 200

  Scenario: Gemini thinking model — thinking tokens billed at reasoning rate (exclusive mode)
    # Model: gemini-2.5-flash-preview-04-17 — input=1.5e-7, output=6e-7, reasoning=3.5e-6
    # EXCLUSIVE: totalTokenCount = prompt + candidates + thoughts (100+50+30=180)
    # After normalize: CompletionTokens=80 (50+30), ReasoningTokens=30
    # cost = 100*1.5e-7 + (80-30)*6e-7 + 30*3.5e-6
    #      = 1.5e-5 + 3e-5 + 1.05e-4 = 0.0001500000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-gemini-thinking-template
      spec:
        displayName: LLM Cost Gemini Thinking Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-gemini-thinking-provider
      spec:
        displayName: LLM Cost Gemini Thinking Provider
        version: v1.0
        context: /llm-cost-gemini-thinking
        template: llm-cost-gemini-thinking-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-gemini-thinking/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" to be ready with method "POST" and body '{"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-gemini-thinking/gemini/v1/thinking/gemini-2.5-flash-preview-04-17:generateContent" with body:
      """ json
      {"contents": [{"role": "user", "parts": [{"text": "Hello"}]}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001500000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-gemini-thinking-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-gemini-thinking-template"
    Then the response status code should be 200

  Scenario: Anthropic cache reads — previously cached tokens billed at reduced cache read rate
    # Model: claude-3-5-haiku-20241022 — input=8e-7/token, output=4e-6/token, cache_read=8e-8/token
    # Usage: 50 regular input + 200 cache_read + 25 output
    # PromptTokens = 50+200 = 250; regularPrompt = 250-200 = 50
    # cost = 50*8e-7 + 200*8e-8 + 25*4e-6
    #      = 4e-5 + 1.6e-5 + 1e-4 = 0.0001560000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-anthropic-cache-read-template
      spec:
        displayName: LLM Cost Anthropic Cache Read Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-anthropic-cache-read-provider
      spec:
        displayName: LLM Cost Anthropic Cache Read Provider
        version: v1.0
        context: /llm-cost-anthropic-cache-read
        template: llm-cost-anthropic-cache-read-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-anthropic-cache-read/anthropic/v1/messages-cache-read" to be ready with method "POST" and body '{"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-anthropic-cache-read/anthropic/v1/messages-cache-read" with body:
      """ json
      {"model": "claude-3-5-haiku-20241022", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 100}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0001560000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-anthropic-cache-read-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-anthropic-cache-read-template"
    Then the response status code should be 200

  Scenario: OpenAI prompt caching — cached prompt tokens billed at reduced rate
    # Model: gpt-4.1-2025-04-14 — input=2e-6/token, output=8e-6/token, cache_read=5e-7/token
    # Usage: 200 prompt (100 cached) + 50 completion
    # cost = (200-100)*2e-6 + 100*5e-7 + 50*8e-6
    #      = 2e-4 + 5e-5 + 4e-4 = 0.0006500000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-cached-template
      spec:
        displayName: LLM Cost OpenAI Cached Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-cached-provider
      spec:
        displayName: LLM Cost OpenAI Cached Provider
        version: v1.0
        context: /llm-cost-openai-cached
        template: llm-cost-openai-cached-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai-cached/openai/v1/chat-cached" to be ready with method "POST" and body '{"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai-cached/openai/v1/chat-cached" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0006500000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-cached-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-cached-template"
    Then the response status code should be 200

  Scenario: OpenAI flex service tier — lower rates applied when response reports service_tier=flex
    # Model: gpt-5.4 — input_flex=1.25e-6/token, output_flex=7.5e-6/token
    # Usage: 100 prompt + 50 completion, service_tier=flex
    # cost = 100*1.25e-6 + 50*7.5e-6 = 1.25e-4 + 3.75e-4 = 0.0005000000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-flex-template
      spec:
        displayName: LLM Cost OpenAI Flex Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-flex-provider
      spec:
        displayName: LLM Cost OpenAI Flex Provider
        version: v1.0
        context: /llm-cost-openai-flex
        template: llm-cost-openai-flex-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai-flex/openai/v1/chat-flex" to be ready with method "POST" and body '{"model": "gpt-5.4", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai-flex/openai/v1/chat-flex" with body:
      """ json
      {"model": "gpt-5.4", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0005000000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-flex-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-flex-template"
    Then the response status code should be 200

  Scenario: OpenAI priority service tier — higher rates applied when response reports service_tier=priority
    # Model: gpt-4.1 — input_priority=3.5e-6/token, output_priority=1.4e-5/token
    # Usage: 100 prompt + 50 completion, service_tier=priority
    # cost = 100*3.5e-6 + 50*1.4e-5 = 3.5e-4 + 7.0e-4 = 0.0010500000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-priority-template
      spec:
        displayName: LLM Cost OpenAI Priority Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-priority-provider
      spec:
        displayName: LLM Cost OpenAI Priority Provider
        version: v1.0
        context: /llm-cost-openai-priority
        template: llm-cost-openai-priority-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai-priority/openai/v1/chat-priority" to be ready with method "POST" and body '{"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai-priority/openai/v1/chat-priority" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0010500000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-priority-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-priority-template"
    Then the response status code should be 200

  Scenario: OpenAI batch service tier — batch rates applied when response reports service_tier=batch
    # Model: gpt-4.1 — input_batches=1e-6/token, output_batches=4e-6/token
    # Usage: 100 prompt + 50 completion, service_tier=batch
    # cost = 100*1e-6 + 50*4e-6 = 1e-4 + 2e-4 = 0.0003000000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-batch-template
      spec:
        displayName: LLM Cost OpenAI Batch Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-batch-provider
      spec:
        displayName: LLM Cost OpenAI Batch Provider
        version: v1.0
        context: /llm-cost-openai-batch
        template: llm-cost-openai-batch-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai-batch/openai/v1/chat-batch" to be ready with method "POST" and body '{"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai-batch/openai/v1/chat-batch" with body:
      """ json
      {"model": "gpt-4.1", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0003000000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-batch-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-batch-template"
    Then the response status code should be 200

  Scenario: OpenAI reasoning tokens — o-series model reasoning tokens billed at standard output rate
    # Model: o4-mini-2025-04-16 — input=1.1e-6/token, output=4.4e-6/token
    # Usage: 100 prompt + 80 completion (includes 30 reasoning_tokens)
    # Reasoning tokens have no separate rate; they bill at the standard output rate.
    # cost = 100*1.1e-6 + 80*4.4e-6 = 1.1e-4 + 3.52e-4 = 0.0004620000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-reasoning-template
      spec:
        displayName: LLM Cost OpenAI Reasoning Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-reasoning-provider
      spec:
        displayName: LLM Cost OpenAI Reasoning Provider
        version: v1.0
        context: /llm-cost-openai-reasoning
        template: llm-cost-openai-reasoning-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai-reasoning/openai/v1/chat-reasoning" to be ready with method "POST" and body '{"model": "o4-mini-2025-04-16", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai-reasoning/openai/v1/chat-reasoning" with body:
      """ json
      {"model": "o4-mini-2025-04-16", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0004620000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-reasoning-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-reasoning-template"
    Then the response status code should be 200

  Scenario: OpenAI web search tool — url_citation annotation triggers flat per-call fee
    # Model: gpt-4.1-2025-04-14 — input=2e-6/token, output=8e-6/token, web_search=0.01/call
    # Usage: 50 prompt + 25 completion + 1 web search call (url_citation annotation present)
    # cost = 50*2e-6 + 25*8e-6 + 0.01 = 1e-4 + 2e-4 + 0.01 = 0.0103000000
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-openai-web-search-template
      spec:
        displayName: LLM Cost OpenAI Web Search Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-openai-web-search-provider
      spec:
        displayName: LLM Cost OpenAI Web Search Provider
        version: v1.0
        context: /llm-cost-openai-web-search
        template: llm-cost-openai-web-search-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-openai-web-search/openai/v1/chat-web-search" to be ready with method "POST" and body '{"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-openai-web-search/openai/v1/chat-web-search" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0103000000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-openai-web-search-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-openai-web-search-template"
    Then the response status code should be 200

  Scenario: Mistral response — bare model name resolved and cost injected as x-llm-cost header
    # Model: mistral-small-latest — input=$0.10/1M (1e-7/token), output=$0.30/1M (3e-7/token)
    # Usage: 100 prompt + 50 completion = (100*1e-7) + (50*3e-7) = 1e-5 + 1.5e-5 = 0.0000250000
    # Also validates: bare model name "mistral-small-latest" (no prefix) is resolved to
    # "mistral/mistral-small-latest" in the pricing map, and prompt_audio_seconds is tolerated.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: llm-cost-mistral-template
      spec:
        displayName: LLM Cost Mistral Template
      """
    Then the response status code should be 201
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: llm-cost-mistral-provider
      spec:
        displayName: LLM Cost Mistral Provider
        version: v1.0
        context: /llm-cost-mistral
        template: llm-cost-mistral-template
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
    And I wait for the endpoint "http://localhost:8080/llm-cost-mistral/mistral/v1/chat/completions" to be ready with method "POST" and body '{"model": "mistral-small-latest", "messages": [{"role": "user", "content": "Hello"}]}'
    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/llm-cost-mistral/mistral/v1/chat/completions" with body:
      """ json
      {"model": "mistral-small-latest", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    And the response header "x-llm-cost" should be "0.0000250000"
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "llm-cost-mistral-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "llm-cost-mistral-template"
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

