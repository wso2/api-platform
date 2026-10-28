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
Feature: Configuring an LLM provider's credential and models in the AI Workspace
  Sheet AIW-LLM, sub-features "Connection" (AIW-PRV-010, AIW-PRV-011) and "Models"
  (AIW-PRV-012, AIW-PRV-013), through the browser. The API-level scenarios are in
  suites/it/features/ai-workspace/llm-providers/configure.feature.

  The journey curates the provider's model list and checks it survives a reload
  (AIW-PRV-012), tries to add a model that is already listed (AIW-PRV-013), then rotates
  the credential and checks it stays masked after a reload (AIW-PRV-010). A provider made
  from a template starts with that template's model catalog. Changes on the overview's
  tabs are staged and persisted by its Save button. The credential rotation runs last
  because it marks the page sensitive, which withholds failure screenshots from then on.

  @AIW-PRV-012 @AIW-PRV-013 @AIW-PRV-010
  Scenario: Journey: an administrator rotates a provider's credential and curates its models
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV010-Provider}" pointed at the mock LLM with the API key "sk-aiw-prv-010-initial"
    And the user is on the provider's overview page

    * sheet check "AIW-PRV-012"
    When the user opens the provider's Models tab
    Then the provider's models are:
      | gpt-4o-mini  |
      | gpt-4.1-mini |
      | o4-mini      |
    When the user adds the model "aiw-prv-012-model"
    And the user removes the model "o4-mini"
    And the user saves the provider changes
    And the user reloads the page
    And the user opens the provider's Models tab
    Then the provider's models are:
      | gpt-4o-mini       |
      | gpt-4.1-mini      |
      | aiw-prv-012-model |

    * sheet check "AIW-PRV-013"
    When the user adds the model "aiw-prv-012-model"
    Then the user sees "This model already exists for the selected provider." on the page
    And the provider's models are:
      | gpt-4o-mini       |
      | gpt-4.1-mini      |
      | aiw-prv-012-model |

    * sheet check "AIW-PRV-010"
    When the user opens the provider's Connection tab
    And the user changes the provider's credential to "sk-aiw-prv-010-rotated"
    Then the provider was updated
    And a secret was created for that credential
    And the provider was updated with a placeholder referencing that secret, not the credential "sk-aiw-prv-010-rotated"
    When the user reloads the page
    And the user opens the provider's Connection tab
    Then the provider's Connection tab shows the mock LLM endpoint and a masked credential
    And the page never shows the credential "sk-aiw-prv-010-rotated"
    And the page never shows the credential "sk-aiw-prv-010-initial"

  # Known product defect: an emptied credential is silently ignored (Save stays disabled,
  # no error shown) instead of being called out.
  @AIW-PRV-011 @known-issue
  Scenario: Saving a provider's connection with an empty credential shows a validation error
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV011-Provider}" pointed at the mock LLM with the API key "sk-aiw-prv-011-initial"
    And the user is on the provider's overview page
    When the user opens the provider's Connection tab
    And the user clears the provider's credential and saves
    Then the user sees a validation error on the provider's credential
    And the provider was not updated
