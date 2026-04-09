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

@consumer-token-based-ratelimit
Feature: Consumer Token-Based Rate Limiting
  As an API developer
  I want token limits to be enforced independently per GenAI application
  So that one application exhausting its budget does not block other applications

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Each consumer gets an independent token counter
    # Each app gets 20 total tokens/hour independently.
    # mock-openai returns usage.total_tokens = 10 per request.
    # App A uses 2 requests (20 tokens) and gets blocked on the 3rd.
    # App B is unaffected — its counter is still at 0.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: ctbrl-template
      spec:
        displayName: CTBRL Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: ctbrl-provider
      spec:
        displayName: CTBRL Provider
        version: v1.0
        context: /ctbrl
        template: ctbrl-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: api-key-auth
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  key: x-api-key
                  in: header
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  totalTokenLimits:
                    - count: 20
                      duration: "1h"
                  consumerBased: true
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    # Create API key for App A (pre-set known value, min 36 chars)
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ctbrl-provider/api-keys" with body:
      """
      {
        "name": "ctbrl-app-a",
        "apiKey": "ctbrl-app-a-key-000000000000000000000000"
      }
      """
    Then the response status code should be 201

    # Create API key for App B
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ctbrl-provider/api-keys" with body:
      """
      {
        "name": "ctbrl-app-b",
        "apiKey": "ctbrl-app-b-key-000000000000000000000000"
      }
      """
    Then the response status code should be 201
    And I wait for 2 seconds

    Given I set header "Content-Type" to "application/json"

    # App A: request 1 — consumes 10 tokens (counter: 10/20)
    When I send a POST request to "http://localhost:8080/ctbrl/chat/completions" with header "x-api-key" value "ctbrl-app-a-key-000000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 200

    # App A: request 2 — consumes 10 more tokens (counter: 20/20, limit reached)
    When I send a POST request to "http://localhost:8080/ctbrl/chat/completions" with header "x-api-key" value "ctbrl-app-a-key-000000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 200

    # App A: request 3 — blocked, token budget exhausted
    When I send a POST request to "http://localhost:8080/ctbrl/chat/completions" with header "x-api-key" value "ctbrl-app-a-key-000000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 429

    # App B: request 1 — should succeed, App B has its own independent counter
    When I send a POST request to "http://localhost:8080/ctbrl/chat/completions" with header "x-api-key" value "ctbrl-app-b-key-000000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "ctbrl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "ctbrl-template"
    Then the response status code should be 200

  Scenario: Backend limit blocks all consumers when shared budget is exhausted
    # Backend limit: 30 total tokens/hour shared across all apps.
    # Consumer limit: 30 total tokens/hour per app independently.
    # Each request uses 10 tokens (via echo backend).
    # App A sends 3 requests — exhausts the backend shared counter (30/30).
    # App B's next request is blocked by the backend limit even though
    # App B's own consumer counter is only at 0.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: ctbrl-both-template
      spec:
        displayName: CTBRL Both Template
        totalTokens:
          location: payload
          identifier: $.json.usage.total_tokens
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: ctbrl-both-provider
      spec:
        displayName: CTBRL Both Provider
        version: v1.0
        context: /ctbrl-both
        template: ctbrl-both-template
        upstream:
          url: http://echo-backend-multi-arch:8080/anything
          auth:
            type: api-key
            header: Authorization
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: api-key-auth
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  key: x-api-key
                  in: header
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  totalTokenLimits:
                    - count: 30
                      duration: "1h"
                  algorithm: fixed-window
                  backend: memory
          - name: token-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  totalTokenLimits:
                    - count: 30
                      duration: "1h"
                  consumerBased: true
                  algorithm: fixed-window
                  backend: memory
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    # Create API key for App A
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ctbrl-both-provider/api-keys" with body:
      """
      {
        "name": "ctbrl-both-app-a",
        "apiKey": "ctbrl-both-app-a-key-00000000000000000000000"
      }
      """
    Then the response status code should be 201

    # Create API key for App B
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ctbrl-both-provider/api-keys" with body:
      """
      {
        "name": "ctbrl-both-app-b",
        "apiKey": "ctbrl-both-app-b-key-00000000000000000000000"
      }
      """
    Then the response status code should be 201
    And I wait for 2 seconds

    Given I set header "Content-Type" to "application/json"

    # App A: requests 1-3 — exhausts the shared backend counter (30 tokens)
    When I send a POST request to "http://localhost:8080/ctbrl-both/chat/completions" with header "x-api-key" value "ctbrl-both-app-a-key-00000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/ctbrl-both/chat/completions" with header "x-api-key" value "ctbrl-both-app-a-key-00000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/ctbrl-both/chat/completions" with header "x-api-key" value "ctbrl-both-app-a-key-00000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 200

    # App B: blocked by the shared backend counter even though its own consumer counter is at 0
    When I send a POST request to "http://localhost:8080/ctbrl-both/chat/completions" with header "x-api-key" value "ctbrl-both-app-b-key-00000000000000000000000" with body:
      """
      {
        "model": "gpt-4",
        "messages": [{"role": "user", "content": "Hello"}],
        "usage": {"prompt_tokens": 4, "completion_tokens": 6, "total_tokens": 10}
      }
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "ctbrl-both-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "ctbrl-both-template"
    Then the response status code should be 200
