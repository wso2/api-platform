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

Feature: MCP proxy upstream authentication after deployment
  As an MCP proxy owner
  I want the gateway to send my proxy's upstream credential on every forwarded request
  So that a backend that requires an API key accepts traffic through the proxy

  The upstream is the testbench capture service, which records what the gateway actually
  forwarded, so each scenario proves the credential arrived rather than inferring it from a
  successful response.

  Scenario: A proxy stored with the legacy "header" auth type deploys and sends its credential upstream
    Given the user is signed in
    And a secret "${UNIQUE:mcp-legacy-key}" already holds the value "legacy-upstream-credential"
    And the user creates a project named "${UNIQUE:MCP-Upstream-Auth-Legacy}"
    And an MCP proxy "${UNIQUE:MCP-Legacy-Auth-Proxy}" at the capture upstream already stores the legacy auth type with the header "X-API-Key" referencing "${UNIQUE:mcp-legacy-key}"

    When the user opens the project "${UNIQUE:MCP-Upstream-Auth-Legacy}"
    And the user opens MCP Proxies
    And the user opens the MCP proxy "${UNIQUE:MCP-Legacy-Auth-Proxy}"
    Then the user is on the MCP proxy's overview page

    When the user deploys it to the gateway
    Then the user sees the deployment is active

    When the MCP proxy is invoked through the gateway
    Then the MCP proxy's upstream received the header "X-API-Key" with the value "legacy-upstream-credential"

  Scenario: A proxy created in the UI with an auth header deploys and sends its credential upstream
    Given the user is signed in
    And the user creates a project named "${UNIQUE:MCP-Upstream-Auth-UI}"
    And the user opens the project "${UNIQUE:MCP-Upstream-Auth-UI}"
    And the user opens MCP Proxies
    When the user creates the MCP proxy "${UNIQUE:MCP-UI-Auth-Proxy}" at the capture upstream with the auth header "X-API-Key" set to "ui-upstream-credential"
    Then the user is on the MCP proxy's overview page

    When the user deploys it to the gateway
    Then the user sees the deployment is active

    When the MCP proxy is invoked through the gateway
    Then the MCP proxy's upstream received the header "X-API-Key" with the value "ui-upstream-credential"
