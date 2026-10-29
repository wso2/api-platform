# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
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

@analytics-ai-context
Feature: Analytics LLM ai_context block
  As a platform administrator publishing analytics to Moesif
  I want every LLM call to carry the ai_context block
  So that Moesif can price the call from its provider, model and token usage

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I reset the analytics collector

  # ai_context is published beside the aiMetadata and aiTokenUsage metadata keys, which keep their
  # existing shape. The testbench returns a fixed usage, so each published value is asserted exactly.
  # Cost is left to Moesif: a cost_metrics object on the event would switch its calculation off.
  Scenario: An LLM provider chat call publishes its provider, model and usage in ai_context
    Given I generate a unique resource name from "ai-context-chat" and store it as "providerName"
    And I generate a unique value from "ai-context-chat" and store it as "providerDisplayName"
    And I generate a unique API version from "ai-context-chat" and store it as "providerVersion"
    And I generate a unique API context from "/ai-context-chat" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerDisplayName}      |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have metadata field "apiType" with value "LlmProvider"
    And I wait for the analytics collector to settle

    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have AI context field "provider" with value "openai"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have AI context field "model" with value "gpt-4.1-2025-04-14"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have AI context field "model_type" with value "chat"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have AI context field "usage.prompt_tokens" with value "19"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have AI context field "usage.completion_tokens" with value "10"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have AI context field "usage.total_tokens" with value "29"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should not have AI context field "cost_metrics"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have metadata field "aiMetadata.model" with value "gpt-4.1-2025-04-14"
    And the latest analytics event for path "${CTX:providerContext}/chat/completions" should have metadata field "aiTokenUsage.promptTokens" with value "19"

  # An embeddings response reports no completion count. It is left out of ai_context rather than
  # sent as 0, which Moesif would price as a call that produced no output tokens.
  Scenario: An embeddings call leaves the unreported completion count out of ai_context
    Given I generate a unique resource name from "ai-context-embed" and store it as "providerName"
    And I generate a unique value from "ai-context-embed" and store it as "providerDisplayName"
    And I generate a unique API version from "ai-context-embed" and store it as "providerVersion"
    And I generate a unique API context from "/ai-context-embed" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}  |
      | name               | ${CTX:providerName}        |
      | displayName        | ${CTX:providerDisplayName} |
      | version            | ${CTX:providerVersion}     |
      | template           | openai                     |
      | spec.context       | ${CTX:providerContext}     |
      | spec.upstream.url  | http://testbench:3004/v1   |
      | accessControl.mode | allow_all                  |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/embeddings" until status 200 with body:
      """
      {"model":"text-embedding-3-small","input":"Hello embeddings"}
      """
    And the latest analytics event for path "${CTX:providerContext}/embeddings" should have metadata field "apiType" with value "LlmProvider"
    And I wait for the analytics collector to settle

    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:providerContext}/embeddings" with body:
      """
      {"model":"text-embedding-3-small","input":"Hello embeddings"}
      """
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    And the latest analytics event for path "${CTX:providerContext}/embeddings" should have AI context field "model" with value "text-embedding-3-small"
    And the latest analytics event for path "${CTX:providerContext}/embeddings" should have AI context field "model_type" with value "embeddings"
    And the latest analytics event for path "${CTX:providerContext}/embeddings" should have AI context field "usage.prompt_tokens" with value "4"
    And the latest analytics event for path "${CTX:providerContext}/embeddings" should have AI context field "usage.total_tokens" with value "4"
    And the latest analytics event for path "${CTX:providerContext}/embeddings" should not have AI context field "usage.completion_tokens"

  # Only the proxy's own event is published for a call through an LLM proxy, so ai_context must be
  # on that event rather than on the provider's loopback hop.
  Scenario: An LLM proxy call publishes ai_context on the proxy's event
    Given I generate a unique resource name from "ai-context-proxied" and store it as "providerName"
    And I generate a unique value from "ai-context-proxied" and store it as "providerDisplayName"
    And I generate a unique API version from "ai-context-proxied" and store it as "providerVersion"
    And I generate a unique API context from "/ai-context-proxied" and store it as "providerContext"
    When I create LLM provider from "resources/templates/llm-provider.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}       |
      | name               | ${CTX:providerName}             |
      | displayName        | ${CTX:providerDisplayName}      |
      | version            | ${CTX:providerVersion}          |
      | template           | openai                          |
      | spec.context       | ${CTX:providerContext}          |
      | spec.upstream.url  | http://testbench:3008/openai/v1 |
      | accessControl.mode | allow_all                       |
    Then the response should be successful

    Given I generate a unique resource name from "ai-context-proxy" and store it as "proxyName"
    And I generate a unique value from "ai-context-proxy" and store it as "proxyDisplayName"
    And I generate a unique API version from "ai-context-proxy" and store it as "proxyVersion"
    And I generate a unique API context from "/ai-context-proxy" and store it as "proxyContext"
    When I create LLM proxy from "resources/templates/llm-proxy.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:proxyName}          |
      | displayName | ${CTX:proxyDisplayName}   |
      | version     | ${CTX:proxyVersion}       |
      | context     | ${CTX:proxyContext}       |
      | provider.id | ${CTX:providerName}       |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" until status 200 with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have metadata field "apiType" with value "LlmProxy"
    And I wait for the analytics collector to settle

    Given I reset the analytics collector
    When I set header "Content-Type" to "application/json"
    And I send a "POST" request to "${CTX:proxyContext}/chat/completions" with body:
      """
      {"model":"gpt-4.1-2025-04-14","messages":[{"role":"user","content":"Hello"}]}
      """
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have metadata field "apiType" with value "LlmProxy"
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "provider" with value "openai"
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "model" with value "gpt-4.1-2025-04-14"
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "model_type" with value "chat"
    And the latest analytics event for path "${CTX:proxyContext}/chat/completions" should have AI context field "usage.total_tokens" with value "29"
