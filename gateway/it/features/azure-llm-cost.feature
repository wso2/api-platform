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

# azure-llm-cost writes x-llm-cost, which llm-cost-based-ratelimit deducts from
# a budget, so a request's price is observed through the budget. Each budget
# is two requests' cost less $0.000001: two requests pass and the third is
# refused only if each was priced at least at the expected rate, and a lower
# rate (for example Global instead of Data Zone) leaves room for the third.
# llm-cost-based-ratelimit is listed first because response-phase policies run
# in reverse order, so azure-llm-cost prices the response before it is charged.

@azure-llm-cost
Feature: Azure AI Foundry cost calculation
  As an API developer fronting Azure AI Foundry
  I want each call priced at Azure's rates for the model and deployment type
  So that spending budgets track what Azure bills

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A Foundry-native model is priced from the azure_ai catalog
    # DeepSeek-V3.2 at azure_ai/deepseek-v3.2: 1000*5.8e-7 + 200*1.68e-6 = $0.000916
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: alc-native-provider
      spec:
        displayName: ALC Foundry Native Provider
        version: v1.0
        context: /alc-native
        template: azureai-foundry
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: api-key
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.001831
                      duration: "1h"
          - name: azure-llm-cost
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  modelMappings:
                    - deployment: deepseek-prod
                      model: deepseek-v3.2
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/alc-native/foundry/openai/v1/chat/completions" with body:
      """ json
      {"model": "deepseek-prod", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-native/foundry/openai/v1/chat/completions" with body:
      """ json
      {"model": "deepseek-prod", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-native/foundry/openai/v1/chat/completions" with body:
      """ json
      {"model": "deepseek-prod", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "alc-native-provider"
    Then the response status code should be 200

  Scenario: A deployment name reported by the Responses API is priced through its mapping
    # The response names deployment sol-prod; its mapping prices it as gpt-5.6-sol:
    # 1000*4e-6 + 200*2e-5 = $0.008. Without the mapping the call would go unpriced.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: alc-responses-provider
      spec:
        displayName: ALC Responses Provider
        version: v1.0
        context: /alc-responses
        template: azureai-foundry
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: api-key
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.015999
                      duration: "1h"
          - name: azure-llm-cost
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  modelMappings:
                    - deployment: sol-prod
                      model: gpt-5.6-sol
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/alc-responses/foundry/openai/v1/responses" with body:
      """ json
      {"model": "sol-prod", "input": "Hello"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-responses/foundry/openai/v1/responses" with body:
      """ json
      {"model": "sol-prod", "input": "Hello"}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-responses/foundry/openai/v1/responses" with body:
      """ json
      {"model": "sol-prod", "input": "Hello"}
      """
    Then the response status code should be 429

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "alc-responses-provider"
    Then the response status code should be 200

  Scenario: A Data Zone deployment is priced at its Data Zone rate
    # gpt-4.1-2025-04-14 at azure/us/: 1000*2.2e-6 + 200*8.8e-6 = $0.00396 per call.
    # At the Global rate ($0.0036) two calls would leave $0.000719 and the third would pass.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: alc-datazone-provider
      spec:
        displayName: ALC Data Zone Provider
        version: v1.0
        context: /alc-datazone
        template: azureai-foundry
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: api-key
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.007919
                      duration: "1h"
          - name: azure-llm-cost
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  modelMappings:
                    - deployment: gpt41-us
                      model: gpt-4.1
                      region: us
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/alc-datazone/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-us", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-datazone/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-us", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-datazone/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-us", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "alc-datazone-provider"
    Then the response status code should be 200

  Scenario: The same model on a Global deployment is priced at the Global rate
    # Same responses and budget as the Data Zone scenario, but the deployment is
    # Global: 1000*2e-6 + 200*8e-6 = $0.0036 per call, so a third call still fits.
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProvider
      metadata:
        name: alc-global-provider
      spec:
        displayName: ALC Global Provider
        version: v1.0
        context: /alc-global
        template: azureai-foundry
        upstream:
          url: http://mock-openapi:4010
          auth:
            type: api-key
            header: api-key
            value: test-key
        accessControl:
          mode: allow_all
        policies:
          - name: llm-cost-based-ratelimit
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  budgetLimits:
                    - amount: 0.007919
                      duration: "1h"
          - name: azure-llm-cost
            version: v1
            paths:
              - path: /*
                methods: ['*']
                params:
                  modelMappings:
                    - deployment: gpt41-global
                      model: gpt-4.1
      """
    Then the response status code should be 201
    And I wait for 2 seconds
    And I wait for policy snapshot sync

    Given I set header "Content-Type" to "application/json"
    When I send a POST request to "http://localhost:8080/alc-global/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-global", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-global/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-global", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-global/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-global", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 200
    When I send a POST request to "http://localhost:8080/alc-global/foundry/openai/v1/chat-data-zone" with body:
      """ json
      {"model": "gpt41-global", "messages": [{"role": "user", "content": "Hello"}]}
      """
    Then the response status code should be 429

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider "alc-global-provider"
    Then the response status code should be 200
