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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@AIW-LLM
Feature: Creating an LLM provider in the AI Workspace
  Sheet AIW-LLM, sub-feature "Create" (AIW-PRV-001..005), through the browser against the
  real control plane. The API-level scenarios for the same IDs are in
  suites/it/features/ai-workspace/llm-providers/create.feature.

  Every provider uses the Azure OpenAI template: it has no built-in endpoint, so the user
  types one (the block's mock LLM) along with an API key, as the sheet describes.

  The journey covers the IDs that build on each other: a provider is created and listed
  (AIW-PRV-001), the same name is refused as a conflict (AIW-PRV-004), and a name full of
  spaces and punctuation still yields a valid ID (AIW-PRV-005). Each `sheet check` step
  marks where an ID's checks begin, so a failure names the ID it broke and the IDs after it
  are reported as not reached.

  Form validation (AIW-PRV-002, AIW-PRV-003) needs no created provider and runs in
  scenarios of its own. Both are known product defects, tagged @known-issue: the form gives
  no inline error for empty required fields (it only disables Add Provider) and accepts any
  endpoint text. Excluding them must not exclude the journey.

  @AIW-PRV-001 @AIW-PRV-004 @AIW-PRV-005
  Scenario: Journey: an administrator creates LLM providers from a template
    Given the user is signed in

    * sheet check "AIW-PRV-001"
    When the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV001-Provider}" pointed at the mock LLM with the API key "sk-aiw-prv-001-ui"
    Then the user is on the provider's overview page
    And the provider "${UNIQUE:PRV001-Provider}" is listed in the provider list

    * sheet check "AIW-PRV-004"
    When the user starts adding a provider from the "Azure OpenAI" template
    And the user tries to create the provider "${UNIQUE:PRV001-Provider}" pointed at the mock LLM with the API key "sk-aiw-prv-004-ui"
    Then the provider creation is rejected with HTTP 409
    And the user sees "An LLM provider with this ID already exists." on the page
    And the user is still on the add-provider form
    And exactly one provider named "${UNIQUE:PRV001-Provider}" exists
    And the API key secret of the rejected attempt was removed

    * sheet check "AIW-PRV-005"
    When the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "My Provider (Beta) & Co! ${UNIQUE:PRV005}" pointed at the mock LLM
    Then the provider was created with an ID starting with "my-provider-beta-co-prv005-"

  @AIW-PRV-002 @known-issue
  Scenario: Submitting the add-provider form with its required fields empty shows inline errors and creates nothing
    Given the user is signed in
    When the user starts adding a provider from the "Azure OpenAI" template
    And the user tries to submit the provider form with its required fields empty
    Then the provider form shows an error on its Name field
    And the provider form shows an error on its Endpoint field
    And no provider was created

  @AIW-PRV-003 @known-issue
  Scenario: An invalid endpoint URL on the add-provider form shows a URL validation error
    Given the user is signed in
    When the user starts adding a provider from the "Azure OpenAI" template
    And the user enters the provider name "${UNIQUE:PRV003-Provider}"
    And the user enters "not-a-url" as the provider endpoint
    Then the provider form shows an error on its Endpoint field
