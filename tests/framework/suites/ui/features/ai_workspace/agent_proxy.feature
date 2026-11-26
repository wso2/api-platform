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

Feature: Agent proxy lifecycle from an A2A agent URL
  Creating an A2A agent proxy from the agent's URL and retiring both the proxy and its
  owning project through the UI. The upstream agent is deliberately unreachable: fetching
  the Agent Card seeds the transports when it succeeds, but failing it warns rather than
  blocking creation, so the journey needs no agent behind the URL.

  Scenario: An administrator creates an agent proxy, then removes it and its project
    Given the user is signed in
    When the user creates a project named "${UNIQUE:E2E-Agent-Project}"
    Then the user sees "${UNIQUE:E2E-Agent-Project}" among the projects

    When the user opens the project "${UNIQUE:E2E-Agent-Project}"
    And the user opens Agent Proxies
    And the user creates an agent proxy "${UNIQUE:E2E-Agent-Proxy}" for an unreachable agent
    Then the user is on the agent proxy's overview page
    And the user sees "${UNIQUE:E2E-Agent-Proxy}" on the page

    When the user opens Agent Proxies
    And the user deletes the agent proxy "${UNIQUE:E2E-Agent-Proxy}"
    Then the user no longer sees "${UNIQUE:E2E-Agent-Proxy}"

    When the user returns to the organization level
    And the user opens the projects list
    And the user deletes the project "${UNIQUE:E2E-Agent-Project}"
    Then the user no longer sees "${UNIQUE:E2E-Agent-Project}"
