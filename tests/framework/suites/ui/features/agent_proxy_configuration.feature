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

Feature: Configuring an agent proxy through its overview tabs
  An agent proxy's overview carries its Agent Card, guardrail policies and backend
  connection on separate tabs, and an edit page of its own. The upstream agent is
  deliberately unreachable: every tab here renders from the stored proxy, so the
  journey needs no agent behind the URL.

  Background:
    Given the user is signed in
    When the user creates a project named "${UNIQUE:E2E-Agent-Config-Project}"
    And the user opens the project "${UNIQUE:E2E-Agent-Config-Project}"
    And the user opens Agent Proxies
    And the user creates an agent proxy "${UNIQUE:E2E-Agent-Config-Proxy}" for an unreachable agent
    Then the user is on the agent proxy's overview page

  Scenario: An administrator reviews the Agent Card and guardrail policies
    When the user opens the agent proxy's "Agent Card" tab
    Then the user sees "${UNIQUE:E2E-Agent-Config-Proxy}" on the page

    When the user opens the agent proxy's "Guardrails & Policies" tab
    Then the user sees "Global Operation Policies" on the page
    And the user sees "Public Agent Card Policies" on the page

    When the user adds a policy under "Global Operation Policies"
    Then the policy drawer is open

    When the user searches the policies for "guard"
    And the user closes the policy drawer
    Then the user sees "Global Operation Policies" on the page

  Scenario: An administrator changes the agent proxy's backend credentials
    When the user opens the agent proxy's "Backend Connection" tab
    And the user sets the agent proxy's authentication to "api-key"
    And the user sets the agent proxy's credential header "X-Api-Key" to "ui-suite-credential"
    And the user saves the agent proxy
    Then the user sees "${UNIQUE:E2E-Agent-Config-Proxy}" on the page

  Scenario: An administrator opens the agent proxy's edit page
    When the user opens the agent proxy's edit page
    Then the user sees "Edit Agent Proxy" on the page

  Scenario: Saving a credentialed connection with nothing filled in is refused
    When the user opens the agent proxy's "Backend Connection" tab
    And the user sets the agent proxy's authentication to "api-key"
    And the user saves the agent proxy
    Then the user sees an error notification

  Scenario: An administrator deploys the agent proxy to a gateway
    When the user deploys it to the gateway
    Then the user sees the deployment is active

  Scenario: An administrator serves an authored card instead of the upstream's
    When the user opens the agent proxy's "Agent Card" tab
    Then the "Public Card" is served "Passthrough"
    And the "Protected Card" is served "Passthrough"

    When the user sets the "Public Card" to "Managed"
    Then the "Public Card" is served "Managed"
    And the "Protected Card" is served "Passthrough"

    When the user saves the agent proxy
    And the user opens the agent proxy's "Agent Card" tab
    Then the "Public Card" is served "Managed"

  Scenario: An administrator stops the gateway rewriting the card's urls
    When the user opens the agent proxy's "Agent Card" tab
    And the user turns off URL rewriting for the "Public Card"
    And the user saves the agent proxy
    Then the user sees "${UNIQUE:E2E-Agent-Config-Proxy}" on the page
