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

@consumer-cost-based-ratelimit
Feature: Consumer Cost-Based Rate Limiting
  As an API developer
  I want cost limits to be enforced independently per GenAI application
  So that one application exhausting its budget does not block other applications

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Each consumer gets an independent cost budget
    # mock-openai returns gpt-4.1-2025-04-14: 19 prompt × $2/1M + 10 completion × $8/1M = $0.0001180000
    # Budget per consumer: $0.000236 = exactly 2 requests worth
    # App A sends 2 requests (budget exhausted) and is blocked on the 3rd.
    # App B is unaffected — its budget counter is still at $0.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: ccbrl-template
      spec:
        displayName: CCBRL Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: ccbrl-provider
      spec:
        displayName: CCBRL Provider
        version: v1.0
        context: /ccbrl
        template: ccbrl-template
        upstream:
          url: http://mock-openapi:4010
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
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
                  consumerBased: true
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods: ['*']
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    # Create API key for App A
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ccbrl-provider/api-keys" with body:
      """
      {
        "name": "ccbrl-app-a",
        "apiKey": "ccbrl-app-a-key-000000000000000000000000"
      }
      """
    Then the response status code should be 201

    # Create API key for App B
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ccbrl-provider/api-keys" with body:
      """
      {
        "name": "ccbrl-app-b",
        "apiKey": "ccbrl-app-b-key-000000000000000000000000"
      }
      """
    Then the response status code should be 201
    And I wait for 2 seconds

    Given I set header "Content-Type" to "application/json"

    # App A: request 1 — allowed, budget drops to $0.000118
    When I send a POST request to "http://localhost:8080/ccbrl/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-app-a-key-000000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App A: request 2 — allowed, budget reaches exactly $0
    When I send a POST request to "http://localhost:8080/ccbrl/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-app-a-key-000000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App A: request 3 — blocked, budget exhausted
    When I send a POST request to "http://localhost:8080/ccbrl/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-app-a-key-000000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # App B: request 1 — should succeed, App B has its own independent cost counter
    When I send a POST request to "http://localhost:8080/ccbrl/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-app-b-key-000000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "ccbrl-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "ccbrl-template"
    Then the response status code should be 200

  Scenario: Backend cost limit blocks all consumers when shared budget is exhausted
    # Backend limit: $0.000236/hour shared across all apps (exactly 2 requests worth).
    # Consumer limit: $0.000236/hour per app independently.
    # App A sends 2 requests — exhausts the shared backend budget.
    # App B's next request is blocked by the backend limit even though
    # App B's own consumer budget is still at $0.
    When I create this LLM provider template:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: ccbrl-both-template
      spec:
        displayName: CCBRL Both Template
      """
    Then the response status code should be 201

    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: ccbrl-both-provider
      spec:
        displayName: CCBRL Both Provider
        version: v1.0
        context: /ccbrl-both
        template: ccbrl-both-template
        upstream:
          url: http://mock-openapi:4010
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
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.000236
                      duration: "1h"
                  consumerBased: true
          - name: llm-cost
            version: v1
            paths:
              - path: /*
                methods: ['*']
      """
    Then the response status code should be 201
    And I wait for policy snapshot sync

    # Create API key for App A
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ccbrl-both-provider/api-keys" with body:
      """
      {
        "name": "ccbrl-both-app-a",
        "apiKey": "ccbrl-both-app-a-key-00000000000000000000000"
      }
      """
    Then the response status code should be 201

    # Create API key for App B
    When I send a POST request to the "gateway-controller" service at "/llm-providers/ccbrl-both-provider/api-keys" with body:
      """
      {
        "name": "ccbrl-both-app-b",
        "apiKey": "ccbrl-both-app-b-key-00000000000000000000000"
      }
      """
    Then the response status code should be 201
    And I wait for 2 seconds

    Given I set header "Content-Type" to "application/json"

    # App A: request 1 — allowed, shared backend budget drops to $0.000118
    When I send a POST request to "http://localhost:8080/ccbrl-both/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-both-app-a-key-00000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App A: request 2 — allowed, shared backend budget reaches exactly $0
    When I send a POST request to "http://localhost:8080/ccbrl-both/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-both-app-a-key-00000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200

    # App B: blocked by the shared backend budget even though its own consumer budget is at $0
    When I send a POST request to "http://localhost:8080/ccbrl-both/openai/v1/chat/completions" with header "x-api-key" value "ccbrl-both-app-b-key-00000000000000000000000" with body:
      """ json
      {"model": "gpt-4.1-2025-04-14", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "ccbrl-both-provider"
    Then the response status code should be 200
    When I delete the LLM provider template "ccbrl-both-template"
    Then the response status code should be 200
