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

@AIW-LLM @llm-provider-create
Feature: LLM providers are created through the control plane
  As an AI Workspace administrator
  I want a provider created from a template to be validated, uniquely identified and listed
  So that what the AI Workspace and the CLI create is exactly what the platform stores

  # Sheet AIW-LLM, sub-feature "Create" (AIW-PRV-001..005), at the API level: the publisher
  # REST API the AI Workspace, the CLI and customers all call. The browser journey for the
  # same IDs is suites/ui/features/aiw_llm/provider_creation.feature. Every provider here
  # uses the Azure OpenAI template, which has no built-in endpoint, pointed at the
  # testbench's OpenAI-dialect mock.
  #
  # AIW-PRV-003 is a known product defect (the API stores any endpoint text), tagged
  # @known-issue. AIW-PRV-004 is a conflict on the provider ID only: display names are not
  # unique by design, and the AI Workspace derives the ID from the name.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @AIW-PRV-001 @type:smoke
  Scenario: A provider created from a template with an endpoint and an API key is stored and listed
    Given I generate a unique value from "PRV-001 Provider" and store it as "displayName"
    And I generate a unique resource name from "prv001-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" with value "sk-aiw-prv-001-upstream" via the control plane
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                       |
      | template           | azure-openai                                                                             |
      | upstreamUrl        | http://testbench:3008/openai/v1                                                          |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:secretHandle}\" }}"} |
    Then the response status code should be 201
    And I store the JSON response field "id" as "providerId"
    And the response header "Location" should be "/api/v0.9/llm-providers/${CTX:providerId}"
    And the JSON response field "displayName" should be "${CTX:displayName}"
    And the JSON response field "template" should be "azure-openai"
    And the JSON response field "upstream.main.url" should be "http://testbench:3008/openai/v1"
    And the response body should not contain "sk-aiw-prv-001-upstream"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:displayName}"
    And the JSON response field "upstream.main.auth.header" should be "api-key"
    And the response body should not contain "sk-aiw-prv-001-upstream"

    # The list is newest-first, so the provider just created is on the first page.
    When I send a "GET" request to the control plane at "/llm-providers?limit=100"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:providerId}"
    And the JSON response array "list" item with "id" equal to "${CTX:providerId}" should have "displayName" equal to "${CTX:displayName}"

  @AIW-PRV-002 @type:negative
  Scenario: A provider with its required fields empty is rejected and nothing is created
    Given I generate a unique resource name from "prv002" and store it as "providerId"
    When I send a "POST" request to the control plane at "/llm-providers" with body:
      """
      {"id": "${CTX:providerId}", "displayName": "", "version": "", "template": "", "upstream": {"main": {"url": ""}}, "accessControl": {"mode": "allow_all"}}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 404

  @AIW-PRV-003 @type:negative @known-issue
  Scenario: A provider whose endpoint is not a valid URL is rejected and nothing is created
    Given I generate a unique resource name from "prv003" and store it as "providerId"
    And I generate a unique value from "PRV-003 Provider" and store it as "displayName"
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | id          | ${CTX:providerId}  |
      | displayName | ${CTX:displayName} |
      | template    | azure-openai       |
      | upstreamUrl | not-a-url          |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 404

  @AIW-PRV-004 @type:negative
  Scenario: Creating a provider with an ID that already exists is a conflict and leaves the original unchanged
    Given I generate a unique resource name from "prv004" and store it as "providerId"
    And I generate a unique value from "PRV-004 Original" and store it as "originalName"
    And I generate a unique value from "PRV-004 Duplicate" and store it as "duplicateName"
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | id          | ${CTX:providerId}               |
      | displayName | ${CTX:originalName}             |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    And the response status code should be 201

    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | id          | ${CTX:providerId}               |
      | displayName | ${CTX:duplicateName}            |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    Then the response status code should be 409
    And the JSON response field "code" should be "LLM_PROVIDER_EXISTS"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:originalName}"

  @AIW-PRV-005
  Scenario: A display name with spaces and special characters yields a valid generated ID
    Given I generate a unique value from "PRV005" and store it as "suffix"
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | My Provider (Beta) & Co! ${CTX:suffix} |
      | template    | azure-openai                           |
      | upstreamUrl | http://testbench:3008/openai/v1        |
    Then the response status code should be 201
    And the JSON response field "displayName" should be "My Provider (Beta) & Co! ${CTX:suffix}"
    And the JSON response field "id" should match pattern "^[a-z0-9]+(-[a-z0-9]+)*$"
    And the JSON response string field "id" should have length less than 41
    And the JSON response string field "id" should have length greater than 2
