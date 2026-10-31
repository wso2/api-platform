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

@AIW-LLM @llm-provider-templates
Feature: LLM providers take their API definition from their template
  As an AI Workspace administrator
  I want a provider created from a vendor's template to carry that vendor's API definition
  So that its resources and deployment match the vendor it fronts

  # Sheet AIW-LLM, sub-feature "Templates" (AIW-PRV-027), at the API level. The platform
  # derives a provider's OpenAPI definition from its template when the provider is created;
  # endpoint and credential header defaults are applied by the AI Workspace form and are
  # checked in suites/ui/features/aiw_llm/provider_templates.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @AIW-PRV-027
  Scenario: Providers created from two vendors' templates each carry their vendor's API definition
    Given I generate a unique value from "PRV-027 OpenAI" and store it as "openaiName"
    And I generate a unique value from "PRV-027 Azure" and store it as "azureName"
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:openaiName}         |
      | template    | openai                    |
      | upstreamUrl | https://api.openai.com/v1 |
    Then the response status code should be 201
    And I store the JSON response field "id" as "openaiId"
    When I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName | ${CTX:azureName}                |
      | template    | azure-openai                    |
      | upstreamUrl | http://testbench:3008/openai/v1 |
    Then the response status code should be 201
    And I store the JSON response field "id" as "azureId"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:openaiId}"
    Then the response status code should be 200
    And the JSON response field "template" should be "openai"
    And the JSON response field "openapi" should contain "title: OpenAI API"
    When I send a "GET" request to the control plane at "/llm-providers/${CTX:azureId}"
    Then the response status code should be 200
    And the JSON response field "template" should be "azure-openai"
    And the JSON response field "openapi" should contain "title: Azure OpenAI Service API"
