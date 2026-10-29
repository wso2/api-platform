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
Feature: Guardrails and rate limits on an LLM provider in the AI Workspace
  Sheet AIW-LLM, sub-features "Guardrails" (AIW-PRV-014) and "Rate limiting" (AIW-PRV-017,
  AIW-PRV-019), through the browser. The API-level scenarios are in
  suites/it/features/ai-workspace/llm-providers/policies.feature.

  The guardrail catalogue comes from the external Policy Hub; the browser is served a fixed
  copy of it (Word Count Guardrail, which the gateway also ships), so these scenarios do not
  depend on that service. Changes on the provider overview's tabs are staged and persisted
  by its Save button. Whether a guardrail or limit is enforced on traffic is covered with
  deployment (AIW-PRV-015, AIW-PRV-016, AIW-PRV-018).

  @AIW-PRV-014 @AIW-PRV-017
  Scenario: Journey: an administrator attaches a guardrail and sets rate limits on a provider
    Given the user is signed in
    And the Policy Hub serves a fixed guardrail catalogue
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV014-Provider}" pointed at the mock LLM
    And the user is on the provider's overview page

    * sheet check "AIW-PRV-014"
    When the user opens the provider's Guardrails & Policies tab
    And the user attaches the "Word Count Guardrail" guardrail with:
      | request.min | 1 |
      | request.max | 5 |
    And the user saves the provider changes
    And the user reloads the page
    And the user opens the provider's Guardrails & Policies tab
    Then the provider's guardrails are:
      | Word Count Guardrail (v1) |

    * sheet check "AIW-PRV-017"
    When the user opens the provider's Rate Limiting tab
    And the user sets the Backend Request Count limit to "100" per "1" hour
    And the user sets the Backend Token Count limit to "5000" per "2" day
    And the user saves the provider changes
    And the user reloads the page
    And the user opens the provider's Rate Limiting tab
    Then the Backend Request Count limit is "100" per "1" hour
    And the Backend Token Count limit is "5000" per "2" day

  @AIW-PRV-019
  Scenario Outline: A provider request limit of <case> is refused with a clear message
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV019-Refused}" pointed at the mock LLM
    And the user is on the provider's overview page
    When the user opens the provider's Rate Limiting tab
    And the user sets the Backend Request Count limit to "<quota>" per "1" hour
    And the user tries to save the provider changes
    Then the user sees "<message>" on the page
    When the user reloads the page
    And the user opens the provider's Rate Limiting tab
    Then the Backend Request Count limit is not set

    Examples:
      | case        | quota | message                          |
      | zero        | 0     | Request Count quota must be >= 1 |
      | negative    | -5    | Request Count quota must be >= 1 |
      | non-numeric | abc   | Request Count quota must be >= 1 |

  @AIW-PRV-019
  Scenario Outline: A provider request limit of <case> is saved exactly
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV019-Saved}" pointed at the mock LLM
    And the user is on the provider's overview page
    When the user opens the provider's Rate Limiting tab
    And the user sets the Backend Request Count limit to "<quota>" per "1" hour
    And the user saves the provider changes
    And the user reloads the page
    And the user opens the provider's Rate Limiting tab
    Then the Backend Request Count limit is "<quota>" per "1" hour

    Examples:
      | case                             | quota            |
      | the minimum                      | 1                |
      | the largest exact browser number | 9007199254740991 |

  # Known product defect: the browser rounds numbers above 9007199254740991, so the
  # platform's maximum is sent as 9223372036854775808 and refused with only a generic
  # "Failed to save provider changes."
  @AIW-PRV-019 @known-issue
  Scenario: The platform's maximum provider request limit is saved exactly
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV019-Maximum}" pointed at the mock LLM
    And the user is on the provider's overview page
    When the user opens the provider's Rate Limiting tab
    And the user sets the Backend Request Count limit to "9223372036854775807" per "1" hour
    And the user saves the provider changes
    And the user reloads the page
    And the user opens the provider's Rate Limiting tab
    Then the Backend Request Count limit is "9223372036854775807" per "1" hour

  # Known product defect: a decimal passes the browser's check, the API stores whole numbers
  # and refuses it, and the user sees only a generic "Failed to save provider changes."
  @AIW-PRV-019 @known-issue
  Scenario: A decimal provider request limit is refused with a clear message
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV019-Decimal}" pointed at the mock LLM
    And the user is on the provider's overview page
    When the user opens the provider's Rate Limiting tab
    And the user sets the Backend Request Count limit to "1.5" per "1" hour
    And the user tries to save the provider changes
    Then the user sees an error notification about "Request Count quota"
    When the user reloads the page
    And the user opens the provider's Rate Limiting tab
    Then the Backend Request Count limit is not set
