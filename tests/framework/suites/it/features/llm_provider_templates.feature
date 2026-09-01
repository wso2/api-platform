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

@llm-provider-templates
Feature: LLM Provider Template Management
  As an API administrator
  I want to manage LLM provider templates in the gateway
  So that I can configure token tracking and model extraction metadata for different LLM providers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ========================================
  # Scenario Group 1: Template Lifecycle (Happy Path)
  # ========================================

  Scenario: Complete template lifecycle - create, retrieve, update, and delete
    When I create LLM provider template with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProviderTemplate
        metadata:
          name: openai-test
        spec:
          displayName: OpenAI
          promptTokens:
            location: payload
            identifier: $.usage.prompt_tokens
          completionTokens:
            location: payload
            identifier: $.usage.completion_tokens
          totalTokens:
            location: payload
            identifier: $.usage.total_tokens
          remainingTokens:
            location: header
            identifier: x-ratelimit-remaining-tokens
          requestModel:
            location: payload
            identifier: $.model
          responseModel:
            location: payload
            identifier: $.model
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "openai-test"
    And the JSON response field "metadata.name" should be "openai-test"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/openai-test"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "openai-test"
    And the JSON response field "spec.displayName" should be "OpenAI"
    And the JSON response field "spec.promptTokens.location" should be "payload"
    And the JSON response field "spec.promptTokens.identifier" should be "$.usage.prompt_tokens"

    When I update the LLM provider template "openai-test" with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProviderTemplate
        metadata:
          name: openai-test
        spec:
          displayName: OpenAI Updated
          promptTokens:
            location: payload
            identifier: $.usage.promptTokens
          completionTokens:
            location: payload
            identifier: $.usage.completion_tokens
          totalTokens:
            location: payload
            identifier: $.usage.total_tokens
          remainingTokens:
            location: header
            identifier: x-ratelimit-remaining-tokens
          requestModel:
            location: payload
            identifier: $.model
          responseModel:
            location: payload
            identifier: $.model
        """
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "openai-test"
    And the JSON response field "metadata.name" should be "openai-test"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/openai-test"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "OpenAI Updated"
    And the JSON response field "spec.promptTokens.location" should be "payload"
    And the JSON response field "spec.promptTokens.identifier" should be "$.usage.promptTokens"

    When I delete the LLM provider template "openai-test"
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should be "LLM provider template deleted successfully"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/openai-test"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create template with minimal required fields
    When I create LLM provider template with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProviderTemplate
        metadata:
          name: minimal-template
        spec:
          displayName: Minimal Template
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "minimal-template"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/minimal-template"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "Minimal Template"

    When I delete the LLM provider template "minimal-template"
    Then the response status code should be 200

  Scenario: List LLM provider templates returns valid JSON with OOB Templates
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response should be an oob-template list

  # ========================================
  # Scenario Group: Error Cases
  # ========================================

  Scenario: Get non-existent LLM provider template returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/non-existent-template-id"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Update non-existent LLM provider template returns 404
    When I update the LLM provider template "non-existent-update-template" with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: non-existent-update-template
      spec:
        displayName: Should Not Work
      """
    Then the response status code should be 404

  Scenario: Delete non-existent LLM provider template returns 404
    When I delete the LLM provider template "non-existent-delete-template"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List LLM provider templates with pagination parameters
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates?limit=5&offset=0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Create LLM provider template with invalid JSON body returns error
    When I send a "POST" request to the "gateway-controller" service at "/llm-provider-templates" with body:
      """
      { this is invalid json
      """
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: Update an existing LLM provider template with invalid JSON body returns error
    When I create LLM provider template with configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: LlmProviderTemplate
      metadata:
        name: invalid-update-template
      spec:
        displayName: Invalid Update Template
        promptTokens:
          location: payload
          identifier: $.usage.prompt_tokens
        completionTokens:
          location: payload
          identifier: $.usage.completion_tokens
        totalTokens:
          location: payload
          identifier: $.usage.total_tokens
        remainingTokens:
          location: header
          identifier: x-ratelimit-remaining-tokens
        requestModel:
          location: payload
          identifier: $.model
        responseModel:
          location: payload
          identifier: $.model
      """
    Then the response status code should be 201
    # The template exists, so the malformed body reaches the parser instead of dying at the
    # existence check — that path is covered by "Update non-existent LLM provider template".
    When I send a "PUT" request to the "gateway-controller" service at "/llm-provider-templates/invalid-update-template" with body:
      """
      { invalid json content
      """
    Then the response status code should be 400
    And the response should be valid JSON
    When I delete the LLM provider template "invalid-update-template"
    Then the response status code should be 200

  Scenario: Get LLM provider template with invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/invalid@template#id"
    Then the response status code should be 404
    And the response should be valid JSON

  # ========================================
  # Scenario Group: Template with All Token Fields
  # ========================================

  Scenario: Create template with header-based token tracking
    When I create LLM provider template with configuration:
        """
        apiVersion: gateway.api-platform.wso2.com/v1
        kind: LlmProviderTemplate
        metadata:
          name: header-tokens-template
        spec:
          displayName: Header Tokens Template
          promptTokens:
            location: header
            identifier: x-prompt-tokens
          completionTokens:
            location: header
            identifier: x-completion-tokens
          totalTokens:
            location: header
            identifier: x-total-tokens
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "header-tokens-template"
    # Verify creation
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/header-tokens-template"
    Then the response status code should be 200
    And the JSON response field "spec.promptTokens.location" should be "header"
    # Cleanup
    When I delete the LLM provider template "header-tokens-template"
    Then the response status code should be 200
