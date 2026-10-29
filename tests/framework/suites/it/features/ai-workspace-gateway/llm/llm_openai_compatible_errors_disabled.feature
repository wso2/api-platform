# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Runs in the llm-openai-compatible-errors-disabled block, whose overlay sets
# [policy_engine.llm_errors] disable_openai_format = true.

@llm @llm-openai-compatible-errors-disabled
Feature: Opting out of OpenAI-compatible error responses for LLM APIs
  As an operator whose LLM clients parse the gateway's previous error bodies
  I want to turn the OpenAI error envelope off
  So that those clients keep working while they migrate

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A policy rejection keeps the policy's own body
    Given I generate a unique resource name from "oai-off-guard" and store it as "providerName"
    And I generate a unique value from "oai-off-guard-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-off-guard" and store it as "providerVersion"
    And I generate a unique API context from "/oai-off-guard" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                             |
      | name                | ${CTX:providerName}                                                                                   |
      | displayName         | ${CTX:providerDisplayName}                                                                            |
      | version             | ${CTX:providerVersion}                                                                                |
      | template            | openai                                                                                                |
      | spec.context        | ${CTX:providerContext}                                                                                |
      | spec.upstream.url   | http://testbench:3000                                                                                 |
      | accessControl.mode  | allow_all                                                                                             |
      | spec.globalPolicies | [{"name":"word-count-guardrail","version":"v1","params":{"request":{"min":1,"max":5,"jsonPath":""}}}] |
    Then the response status code should be 201
    And I send a "POST" request to "${CTX:providerContext}/post" until status 200 with body:
      """
      hello
      """

    When I send a "POST" request to "${CTX:providerContext}/post" with body:
      """
      this request body has far more than five words in it
      """
    Then the response status code should be 422
    And the JSON response field "type" should be "WORD_COUNT_GUARDRAIL"
    And the JSON response field "message.interveningGuardrail" should be "word-count-guardrail"
    And the JSON response field "error" should not exist
    And the response body should not contain "invalid_request_error"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  # The upstream refuses connections, so the router writes the 503 itself and sends it as written.
  Scenario: A router failure keeps the router's own reply
    Given I generate a unique resource name from "oai-off-down" and store it as "providerName"
    And I generate a unique value from "oai-off-down-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-off-down" and store it as "providerVersion"
    And I generate a unique API context from "/oai-off-down" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion          | ${CTX:gatewaySpecVersion}                                                                                          |
      | name                | ${CTX:providerName}                                                                                                |
      | displayName         | ${CTX:providerDisplayName}                                                                                         |
      | version             | ${CTX:providerVersion}                                                                                             |
      | template            | openai                                                                                                             |
      | spec.context        | ${CTX:providerContext}                                                                                             |
      | spec.upstream.url   | http://testbench:1                                                                                                 |
      | accessControl.mode  | allow_all                                                                                                          |
      | spec.globalPolicies | [{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"x-oai-off-test","value":"down"}]}}}] |
    Then the response status code should be 201

    When I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 503 with body:
      """
      {"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}
      """
    Then the response status code should be 503
    And the response body should contain "upstream connect error"
    And the response body should not contain "server_error"

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful

  # The opt-out changes only what the gateway produces: a provider's own error is unaffected.
  Scenario: A backend error is preserved with or without a body
    Given I generate a unique resource name from "oai-off-backend" and store it as "providerName"
    And I generate a unique value from "oai-off-backend-display" and store it as "providerDisplayName"
    And I generate a unique API version from "oai-off-backend" and store it as "providerVersion"
    And I generate a unique API context from "/oai-off-backend" and store it as "providerContext"
    And I generate a unique resource name from "oai-off-empty" and store it as "emptyProviderName"
    And I generate a unique value from "oai-off-empty-display" and store it as "emptyProviderDisplayName"
    And I generate a unique API version from "oai-off-empty" and store it as "emptyProviderVersion"
    And I generate a unique API context from "/oai-off-empty" and store it as "emptyProviderContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerDisplayName} |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3000      |
      | accessControl.mode | allow_all                  |
    Then the response status code should be 201
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:emptyProviderName}        |
      | displayName        | ${CTX:emptyProviderDisplayName} |
      | version            | ${CTX:emptyProviderVersion}     |
      | template           | openai                          |
      | spec.context       | ${CTX:emptyProviderContext}     |
      | spec.upstream.url  | http://testbench:3002           |
      | accessControl.mode | allow_all                       |
    Then the response status code should be 201
    And I send a "GET" request to "${CTX:providerContext}/get" until status 200
    And I send a "GET" request to "${CTX:emptyProviderContext}/get" until status 200

    When I send a "GET" request to "${CTX:providerContext}/get?statusCode=418"
    Then the response status code should be 418
    And the JSON response field "method" should be "GET"
    And the JSON response field "error" should not exist

    When I send a "GET" request to "${CTX:emptyProviderContext}/status/429"
    Then the response status code should be 429
    And the response body should be empty

    When I delete the LLM provider "${CTX:providerName}"
    Then the response should be successful
    When I delete the LLM provider "${CTX:emptyProviderName}"
    Then the response should be successful
