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

@AIW-LLM @llm-provider-configure
Feature: An LLM provider's credential and models are updated through the control plane
  As an AI Workspace administrator
  I want to rotate a provider's credential and curate its model list
  So that the provider keeps working with current keys and advertises the right models

  # Sheet AIW-LLM, sub-features "Connection" (AIW-PRV-010, AIW-PRV-011) and "Models"
  # (AIW-PRV-012, AIW-PRV-013), at the API level. An update replaces the provider, so every
  # update below restates the provider's fields. AIW-PRV-011 is a known product defect (an
  # empty credential is accepted and the stored one kept), tagged @known-issue. The browser journey is
  # suites/ui/features/aiw_llm/provider_configuration.feature.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "PRV Configure Provider" and store it as "displayName"

  @AIW-PRV-010
  Scenario: Rotating a provider's credential keeps it out of every response
    Given I generate a unique resource name from "prv010-initial" and store it as "initialSecret"
    And I generate a unique resource name from "prv010-rotated" and store it as "rotatedSecret"
    And I create a secret "${CTX:initialSecret}" with value "sk-aiw-prv-010-initial" via the control plane
    And I create a secret "${CTX:rotatedSecret}" with value "sk-aiw-prv-010-rotated" via the control plane
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                        |
      | template           | azure-openai                                                                              |
      | upstreamUrl        | http://testbench:3008/openai/v1                                                           |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:initialSecret}\" }}"} |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"

    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                        |
      | template           | azure-openai                                                                              |
      | upstreamUrl        | http://testbench:3008/openai/v1                                                           |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:rotatedSecret}\" }}"} |
    Then the response status code should be 200
    And the response body should not contain "sk-aiw-prv-010-rotated"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response field "upstream.main.auth.header" should be "api-key"
    And the response body should not contain "sk-aiw-prv-010-rotated"
    And the response body should not contain "sk-aiw-prv-010-initial"

    When I send a "GET" request to the control plane at "/secrets/${CTX:rotatedSecret}"
    Then the response status code should be 200
    And the response body should not contain "sk-aiw-prv-010-rotated"

  @AIW-PRV-011 @type:negative @known-issue
  Scenario: Saving a provider's connection with an empty credential is rejected
    Given I generate a unique resource name from "prv011-key" and store it as "secretHandle"
    And I create a secret "${CTX:secretHandle}" with value "sk-aiw-prv-011-initial" via the control plane
    And I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                                                       |
      | template           | azure-openai                                                                             |
      | upstreamUrl        | http://testbench:3008/openai/v1                                                          |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":"{{ secret \"${CTX:secretHandle}\" }}"} |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"

    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName        | ${CTX:displayName}                                 |
      | template           | azure-openai                                       |
      | upstreamUrl        | http://testbench:3008/openai/v1                    |
      | upstream.main.auth | {"type":"api-key","header":"api-key","value":""} |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"

  @AIW-PRV-012
  Scenario: A provider's models are added, edited and removed
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName    | ${CTX:displayName}                                                                                                                                                   |
      | template       | azure-openai                                                                                                                                                         |
      | upstreamUrl    | http://testbench:3008/openai/v1                                                                                                                                      |
      | modelProviders | [{"id":"openai","displayName":"OpenAI","models":[{"id":"gpt-4o-mini","displayName":"gpt-4o-mini"},{"id":"aiw-prv-012-a","displayName":"Model A"}]}] |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response array field "modelProviders[0].models" should have 2 items
    And the JSON response array "modelProviders[0].models" item with "id" equal to "aiw-prv-012-a" should have "displayName" equal to "Model A"

    # One update edits Model A, removes gpt-4o-mini and adds Model B.
    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName    | ${CTX:displayName}                                                                                                                                                       |
      | template       | azure-openai                                                                                                                                                             |
      | upstreamUrl    | http://testbench:3008/openai/v1                                                                                                                                          |
      | modelProviders | [{"id":"openai","displayName":"OpenAI","models":[{"id":"aiw-prv-012-a","displayName":"Model A renamed"},{"id":"aiw-prv-012-b","displayName":"Model B"}]}] |
    Then the response status code should be 200

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response array field "modelProviders[0].models" should have 2 items
    And the JSON response array "modelProviders[0].models" item with "id" equal to "aiw-prv-012-a" should have "displayName" equal to "Model A renamed"
    And the JSON response array "modelProviders[0].models" item with "id" equal to "aiw-prv-012-b" should have "displayName" equal to "Model B"
    And the JSON response array "modelProviders[0].models" should not contain an item with "id" equal to "gpt-4o-mini"

  @AIW-PRV-013 @type:negative
  Scenario: A model list with a duplicate model ID is rejected and the stored list is kept
    Given I create an LLM provider via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName    | ${CTX:displayName}                                                                                       |
      | template       | azure-openai                                                                                             |
      | upstreamUrl    | http://testbench:3008/openai/v1                                                                          |
      | modelProviders | [{"id":"openai","displayName":"OpenAI","models":[{"id":"aiw-prv-013-m1","displayName":"M1"}]}] |
    And the response status code should be 201
    And I store the JSON response field "id" as "providerId"

    When I update the LLM provider "${CTX:providerId}" via the control plane from "resources/templates/llm-provider-cp.yaml" with values:
      | displayName    | ${CTX:displayName}                                                                                                                                          |
      | template       | azure-openai                                                                                                                                                |
      | upstreamUrl    | http://testbench:3008/openai/v1                                                                                                                             |
      | modelProviders | [{"id":"openai","displayName":"OpenAI","models":[{"id":"aiw-prv-013-m1","displayName":"M1"},{"id":"aiw-prv-013-m1","displayName":"M1 again"}]}] |
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should contain "Duplicate model id"

    When I send a "GET" request to the control plane at "/llm-providers/${CTX:providerId}"
    Then the response status code should be 200
    And the JSON response array field "modelProviders[0].models" should have 1 item
