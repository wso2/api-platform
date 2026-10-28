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
Feature: Finding and inspecting LLM providers in the AI Workspace
  Sheet AIW-LLM, sub-features "List", "Overview" and "Isolation" (AIW-PRV-007, AIW-PRV-008,
  AIW-PRV-031), through the browser. The API-level scenarios are in
  suites/it/features/ai-workspace/llm-providers/inspect.feature.

  The journey opens a provider it just created and checks its overview and Connection tab
  against what was entered (AIW-PRV-008), finds it among others by searching the list
  (AIW-PRV-007), and confirms that an organization-level provider is offered to every
  project (AIW-PRV-031: providers belong to the organization, not to a project).

  @AIW-PRV-008 @AIW-PRV-007 @AIW-PRV-031
  Scenario: Journey: an administrator finds and inspects an LLM provider
    Given the user is signed in

    * sheet check "AIW-PRV-008"
    When the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV008-Provider}" pointed at the mock LLM with the API key "sk-aiw-prv-008-ui"
    Then the user is on the provider's overview page
    And the provider overview shows:
      | name        | ${UNIQUE:PRV008-Provider}                        |
      | template    | Azure OpenAI                                     |
      | version     | v1.0                                             |
      | description | UI suite provider from the mock-backed template. |
      | creator     | admin                                            |
    And the provider overview shows the context derived from its ID
    And the page never shows the credential "sk-aiw-prv-008-ui"
    When the user opens the provider's Connection tab
    Then the provider's Connection tab shows the mock LLM endpoint and a masked credential
    And the page never shows the credential "sk-aiw-prv-008-ui"

    * sheet check "AIW-PRV-007"
    Given a provider named "${UNIQUE:PRV007-Alpha}" exists
    And a provider named "${UNIQUE:PRV007-Beta}" exists
    When the user searches the provider list for "${UNIQUE:PRV007-Alpha}"
    Then the provider list shows only "${UNIQUE:PRV007-Alpha}"
    When the user searches the provider list for "${UNIQUE:PRV007-NoSuchProvider}"
    Then the provider list shows no providers
    When the user clears the provider search
    Then the provider list shows:
      | ${UNIQUE:PRV007-Alpha}    |
      | ${UNIQUE:PRV007-Beta}     |
      | ${UNIQUE:PRV008-Provider} |

    * sheet check "AIW-PRV-031"
    Given the user creates a project named "${UNIQUE:PRV031-Project-A}"
    And the user creates a project named "${UNIQUE:PRV031-Project-B}"
    Then the provider "${UNIQUE:PRV008-Provider}" is offered when creating an app LLM proxy in project "${UNIQUE:PRV031-Project-A}"
    And the provider "${UNIQUE:PRV008-Provider}" is offered when creating an app LLM proxy in project "${UNIQUE:PRV031-Project-B}"

  # Known product defect: the list loads only the newest 20 providers and searches just
  # those, so an older provider cannot be found. Once fixed, the 20 extra providers this
  # scenario creates no longer hide other runners' providers either.
  @AIW-PRV-007 @known-issue
  Scenario: A provider created before twenty newer ones can still be found by searching the list
    Given the user is signed in
    And a provider named "${UNIQUE:PRV007-Oldest}" exists
    And 20 more providers exist
    When the user searches the provider list for "${UNIQUE:PRV007-Oldest}"
    Then the provider list shows only "${UNIQUE:PRV007-Oldest}"
