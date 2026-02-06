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

Feature: LLM Provider Template Management
  As an API administrator
  I want to manage LLM provider templates in the gateway
  So that I can configure token tracking and model extraction metadata for different LLM providers

  Background:
    Given the gateway services are running

  # ========================================
  # Scenario Group 1: Template Lifecycle (Happy Path)
  # ========================================

  Scenario: Complete template lifecycle - create, retrieve, update, and delete
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
    And the JSON response field "status" should be "success"
    And the JSON response field "id" should be "openai-test"
    And the JSON response field "message" should be "LLM provider template created successfully"

    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider template "openai-test"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "template.id" should be "openai-test"
    And the JSON response field "template.configuration.spec.displayName" should be "OpenAI"
    And the JSON response field "template.configuration.spec.promptTokens.location" should be "payload"
    And the JSON response field "template.configuration.spec.promptTokens.identifier" should be "$.usage.prompt_tokens"

    Given I authenticate using basic auth as "admin"
    When I update the LLM provider template "openai-test" with:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
    And the JSON response field "status" should be "success"
    And the JSON response field "id" should be "openai-test"
    And the JSON response field "message" should be "LLM provider template updated successfully"

    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider template "openai-test"
    Then the response status code should be 200
    And the JSON response field "template.configuration.spec.displayName" should be "OpenAI Updated"
    And the JSON response field "template.configuration.spec.promptTokens.location" should be "payload"
    And the JSON response field "template.configuration.spec.promptTokens.identifier" should be "$.usage.promptTokens"

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "openai-test"
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should be "LLM provider template deleted successfully"

    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider template "openai-test"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create template with minimal required fields
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
        kind: LlmProviderTemplate
        metadata:
          name: minimal-template
        spec:
          displayName: Minimal Template
        """
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "id" should be "minimal-template"

    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider template "minimal-template"
    Then the response status code should be 200
    And the JSON response field "template.configuration.spec.displayName" should be "Minimal Template"

    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "minimal-template"
    Then the response status code should be 200

  Scenario: List LLM provider templates returns valid JSON with OOB Templates
    Given I authenticate using basic auth as "admin"
    When I list all LLM provider templates
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response should contain oob-templates

  # ========================================
  # Scenario Group: Error Cases
  # ========================================

  Scenario: Get non-existent LLM provider template returns 404
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider template "non-existent-template-id"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Update non-existent LLM provider template returns 400
    Given I authenticate using basic auth as "admin"
    When I update the LLM provider template "non-existent-update-template" with:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProviderTemplate
      metadata:
        name: non-existent-update-template
      spec:
        displayName: Should Not Work
      """
    Then the response status code should be 400

  Scenario: Delete non-existent LLM provider template returns 404
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "non-existent-delete-template"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: List LLM provider templates with pagination parameters
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/llm-provider-templates?limit=5&offset=0"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: Create LLM provider template with invalid JSON body returns error
    Given I authenticate using basic auth as "admin"
    When I send a POST request to the "gateway-controller" service at "/llm-provider-templates" with body:
      """
      { this is invalid json
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Update LLM provider template with invalid JSON body returns error
    Given I authenticate using basic auth as "admin"
    When I send a PUT request to the "gateway-controller" service at "/llm-provider-templates/some-template" with body:
      """
      { invalid json content
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Get LLM provider template with invalid ID format returns 404
    Given I authenticate using basic auth as "admin"
    When I send a GET request to the "gateway-controller" service at "/llm-provider-templates/invalid@template#id"
    Then the response status should be 404
    And the response should be valid JSON

  # ========================================
  # Scenario Group: Template with All Token Fields
  # ========================================

  Scenario: Create template with header-based token tracking
    Given I authenticate using basic auth as "admin"
    When I create this LLM provider template:
        """
        apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
    And the JSON response field "status" should be "success"
    # Verify creation
    Given I authenticate using basic auth as "admin"
    When I retrieve the LLM provider template "header-tokens-template"
    Then the response status code should be 200
    And the JSON response field "template.configuration.spec.promptTokens.location" should be "header"
    # Cleanup
    Given I authenticate using basic auth as "admin"
    When I delete the LLM provider template "header-tokens-template"
    Then the response status code should be 200
