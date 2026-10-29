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
Feature: Creating LLM providers from different templates in the AI Workspace
  Sheet AIW-LLM, sub-feature "Templates" (AIW-PRV-027), through the browser. The API-level
  scenario is in suites/it/features/ai-workspace/llm-providers/templates.feature.

  Two vendors' templates set up a provider differently: OpenAI has a built-in endpoint and
  sends the credential in Authorization; Azure OpenAI asks for the endpoint and sends the
  credential in api-key.

  @AIW-PRV-027
  Scenario: Journey: an administrator creates providers from two vendors' templates
    Given the user is signed in

    * sheet check "AIW-PRV-027"
    When the user starts adding a provider from the "OpenAI" template
    Then the add-provider form does not ask for an endpoint
    When the user creates the provider "${UNIQUE:PRV027-OpenAI}" using the template's built-in endpoint
    Then the user is on the provider's overview page
    And the provider overview shows:
      | template | OpenAI |
    When the user opens the provider's Connection tab
    Then the provider's Connection tab shows the endpoint "https://api.openai.com/v1" and the authentication header "Authorization"

    When the user starts adding a provider from the "Azure OpenAI" template
    Then the add-provider form asks for an endpoint
    When the user creates the provider "${UNIQUE:PRV027-Azure}" pointed at the mock LLM
    Then the user is on the provider's overview page
    And the provider overview shows:
      | template | Azure OpenAI |
    When the user opens the provider's Connection tab
    Then the provider's Connection tab shows the endpoint "the mock LLM" and the authentication header "api-key"
