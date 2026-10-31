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
Feature: Deploying and running an LLM provider from the AI Workspace
  Sheet AIW-LLM, sub-features "Deploy", "Undeploy", "Redeploy", "Invoke", "Guardrails",
  "Rate limiting" and "Connection" (AIW-PRV-009, 015, 016, 018, 020, 022, 023, 024, 025,
  026), through the browser against the block's real gateway. The API-level scenarios are in
  suites/it/features/ai-workspace/llm-providers/deploy.feature.

  Every provider fronts the testbench's keyed OpenAI-dialect mock, which accepts only its
  valid upstream key; calls use an API key generated on the provider's overview. A change
  made after deployment reaches traffic once the provider is deployed again from its deploy
  screen. The guardrail catalogue is served from a fixed copy of the Policy Hub's.
  Rate-limit windows in the UI are an hour or longer, so the journey checks that calls over
  the limit are refused; the window resetting is checked at the API level.

  @AIW-PRV-020 @AIW-PRV-024 @AIW-PRV-026 @AIW-PRV-015 @AIW-PRV-016 @AIW-PRV-025
  Scenario: Journey: an administrator deploys a guarded provider and calls it through the gateway
    Given the user is signed in
    And the Policy Hub serves a fixed guardrail catalogue
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV020-Provider}" pointed at the keyed mock LLM with the API key "sk-testbench-valid-key"
    And the user is on the provider's overview page
    And the user opens the provider's Guardrails & Policies tab
    And the user attaches the "Word Count Guardrail" guardrail with:
      | request.min | 1 |
      | request.max | 5 |
    And the user saves the provider changes

    * sheet check "AIW-PRV-020"
    When the user deploys it to the gateway
    Then the user sees the deployment is Active

    * sheet check "AIW-PRV-024"
    When the user returns to the provider overview
    And the user generates an API key named "${UNIQUE:prv020-consumer-key}"
    Then the user's call to the provider with "Hello there" returns HTTP 200
    And the call is answered with:
      | answer | Hello! How can I assist you today? |

    * sheet check "AIW-PRV-026"
    Then the user's streaming call to the provider with "Hello" returns HTTP 200
    And the call streamed "This is a streamed reply from the mock model." in order and ended normally

    * sheet check "AIW-PRV-015"
    Then the user's call to the provider with "one two three four five six seven eight" returns HTTP 422
    And the call's response contains "word"

    * sheet check "AIW-PRV-016"
    When the user opens the provider's Guardrails & Policies tab
    And the user edits the "Word Count Guardrail (v1)" guardrail with:
      | request.max | 10 |
    And the user saves the provider changes
    And the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    Then the user's call to the provider with "one two three four five six seven eight" returns HTTP 200
    And the user's call to the provider with "one two three four five six seven eight nine ten eleven twelve" returns HTTP 422
    When the user opens the provider's Guardrails & Policies tab
    And the user removes the "Word Count Guardrail (v1)" guardrail
    And the user saves the provider changes
    And the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    Then the user's call to the provider with "one two three four five six seven eight nine ten eleven twelve" returns HTTP 200

    * sheet check "AIW-PRV-025"
    When the user opens the provider's Connection tab
    And the user changes the provider's credential to "sk-aiw-prv-025-wrong-key"
    And the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    # The gateway refuses the consumer key with its own 401 until the key reaches it, so wait
    # for the upstream's refusal specifically.
    Then the user's call to the provider with "Hello" is answered by "invalid_api_key"
    And the call returned HTTP 401
    And the call's response does not contain "sk-aiw-prv-025-wrong-key"
    And the page never shows the credential "sk-aiw-prv-025-wrong-key"

  @AIW-PRV-009 @AIW-PRV-022 @AIW-PRV-018
  Scenario: Journey: an administrator changes, stops and limits a deployed provider
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV009-Provider}" pointed at the keyed mock LLM with the API key "sk-testbench-valid-key"
    And the user is on the provider's overview page
    And the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    And the user generates an API key named "${UNIQUE:prv009-consumer-key}"
    And the user's call to the provider with "Hello" returns HTTP 200
    And the call is answered with:
      | model | gpt-4.1-2025-04-14 |

    * sheet check "AIW-PRV-009"
    When the user opens the provider's Connection tab
    And the user changes the provider's endpoint to the Mistral mock LLM
    And the user saves the provider changes
    Then the user's call to the provider with "Hello" returns HTTP 200
    And the call is answered with:
      | model | gpt-4.1-2025-04-14 |
    When the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    Then the user's call to the provider with "Hello" is answered by "mistral-small-latest"
    And the call returned HTTP 200

    * sheet check "AIW-PRV-022"
    When the user opens the provider's deploy screen
    And the user sees the deployment is Active
    And the user stops the deployment
    Then the user sees the deployment is Suspended
    When the user returns to the provider overview
    Then the user's call to the provider with "Hello" returns HTTP 404

    * sheet check "AIW-PRV-018"
    When the user opens the provider's Rate Limiting tab
    And the user sets the Backend Request Count limit to "2" per "1" hour
    And the user saves the provider changes
    And the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    And the user's call to the provider with "Hello" returns HTTP 200
    Then the user's repeated calls to the provider are refused with HTTP 429

  # Redeploy restores the stopped deployment, which still carries the settings it was deployed
  # with, so the changed settings never reach the gateway: a product defect, tagged @known-issue.
  @AIW-PRV-023 @known-issue
  Scenario: Redeploying a stopped provider after its settings changed puts the new settings in effect
    Given the user is signed in
    And the user starts adding a provider from the "Azure OpenAI" template
    And the user creates the provider "${UNIQUE:PRV023-Provider}" pointed at the keyed mock LLM with the API key "sk-testbench-valid-key"
    And the user is on the provider's overview page
    And the user deploys it to the gateway
    And the user sees the deployment is Active
    And the user returns to the provider overview
    And the user generates an API key named "${UNIQUE:prv023-consumer-key}"
    And the user's call to the provider with "Hello" returns HTTP 200
    When the user opens the provider's deploy screen
    And the user stops the deployment
    And the user sees the deployment is Suspended
    And the user returns to the provider overview
    And the user opens the provider's Connection tab
    And the user changes the provider's endpoint to the Mistral mock LLM
    And the user saves the provider changes
    And the user opens the provider's deploy screen
    And the user redeploys the stopped deployment
    Then the user sees the deployment is Active
    When the user returns to the provider overview
    Then the user's call to the provider with "Hello" is answered by "mistral-small-latest"
    And the call returned HTTP 200
