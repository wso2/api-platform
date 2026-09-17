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

@llm-provider-template-management
Feature: LLM provider template management
  As an API administrator
  I want to manage LLM provider templates through the management API
  So that I can configure token tracking and model extraction metadata for different
  LLM providers

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Complete template lifecycle - create, retrieve, update, and delete
    Given I generate a unique resource name from "lptm-lifecycle" and store it as "templateName"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:templateName}               |
      | displayName           | OpenAI                              |
      | spec.promptTokens     | {"location":"payload","identifier":"$.usage.prompt_tokens"}     |
      | spec.completionTokens | {"location":"payload","identifier":"$.usage.completion_tokens"} |
      | spec.totalTokens      | {"location":"payload","identifier":"$.usage.total_tokens"}      |
      | spec.remainingTokens  | {"location":"header","identifier":"x-ratelimit-remaining-tokens"} |
      | spec.requestModel     | {"location":"payload","identifier":"$.model"}                    |
      | spec.responseModel    | {"location":"payload","identifier":"$.model"}                    |
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:templateName}"
    And the JSON response field "metadata.name" should be "${CTX:templateName}"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/${CTX:templateName}"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:templateName}"
    And the JSON response field "spec.displayName" should be "OpenAI"
    And the JSON response field "spec.promptTokens.location" should be "payload"
    And the JSON response field "spec.promptTokens.identifier" should be "$.usage.prompt_tokens"

    When I update LLM provider template "${CTX:templateName}" from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:templateName}               |
      | displayName           | OpenAI Updated                     |
      | spec.promptTokens     | {"location":"payload","identifier":"$.usage.promptTokens"}      |
      | spec.completionTokens | {"location":"payload","identifier":"$.usage.completion_tokens"} |
      | spec.totalTokens      | {"location":"payload","identifier":"$.usage.total_tokens"}      |
      | spec.remainingTokens  | {"location":"header","identifier":"x-ratelimit-remaining-tokens"} |
      | spec.requestModel     | {"location":"payload","identifier":"$.model"}                    |
      | spec.responseModel    | {"location":"payload","identifier":"$.model"}                    |
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:templateName}"
    And the JSON response field "metadata.name" should be "${CTX:templateName}"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/${CTX:templateName}"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "OpenAI Updated"
    And the JSON response field "spec.promptTokens.location" should be "payload"
    And the JSON response field "spec.promptTokens.identifier" should be "$.usage.promptTokens"

    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200
    And the JSON response field "status" should be "success"
    And the JSON response field "message" should be "LLM provider template deleted successfully"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/${CTX:templateName}"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Create template with minimal required fields
    Given I generate a unique resource name from "lptm-minimal" and store it as "templateName"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | Minimal Template                   |
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:templateName}"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/${CTX:templateName}"
    Then the response status code should be 200
    And the JSON response field "spec.displayName" should be "Minimal Template"

    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200

  Scenario: List LLM provider templates returns valid JSON with built-in templates
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be greater than 1
    And the response body should contain "openai"

  Scenario: Get non-existent LLM provider template returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/non-existent-template-id"
    Then the response status code should be 404
    And the response should be valid JSON
    And the JSON response field "status" should be "error"

  Scenario: Update non-existent LLM provider template returns 404
    Given I generate a unique resource name from "lptm-nonexistent-update" and store it as "templateName"
    When I update LLM provider template "${CTX:templateName}" from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion  | gateway.api-platform.wso2.com/v1 |
      | name        | ${CTX:templateName}              |
      | displayName | Should Not Work                    |
    Then the response status code should be 404

  Scenario: Delete non-existent LLM provider template returns 404
    When I send a "DELETE" request to the "gateway-controller" service at "/llm-provider-templates/non-existent-delete-template"
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
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Update LLM provider template with invalid JSON body returns error
    When I send a "PUT" request to the "gateway-controller" service at "/llm-provider-templates/some-template" with body:
      """
      { invalid json content
      """
    Then the response should be a client error
    And the response should be valid JSON

  Scenario: Get LLM provider template with invalid ID format returns 404
    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/invalid@template#id"
    Then the response status should be 404
    And the response should be valid JSON

  Scenario: Create template with header-based token tracking
    Given I generate a unique resource name from "lptm-header-tokens" and store it as "templateName"
    When I create LLM provider template from "resources/templates/llm-provider-template.yaml" with values:
      | apiVersion            | gateway.api-platform.wso2.com/v1 |
      | name                  | ${CTX:templateName}              |
      | displayName           | Header Tokens Template              |
      | spec.promptTokens     | {"location":"header","identifier":"x-prompt-tokens"}     |
      | spec.completionTokens | {"location":"header","identifier":"x-completion-tokens"} |
      | spec.totalTokens      | {"location":"header","identifier":"x-total-tokens"}      |
    Then the response status code should be 201
    And the response should be valid JSON
    And the JSON response field "status.id" should be "${CTX:templateName}"

    When I send a "GET" request to the "gateway-controller" service at "/llm-provider-templates/${CTX:templateName}"
    Then the response status code should be 200
    And the JSON response field "spec.promptTokens.location" should be "header"

    When I delete the LLM provider template "${CTX:templateName}"
    Then the response status code should be 200
