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

Feature: MCP proxy lifecycle from the sample endpoint
  The journey ported from the product's own Cypress suite (002-mcp-proxy-sample-url),
  creating an MCP proxy from the form's built-in sample endpoint and retiring both the
  proxy and its owning project through the UI.

  Scenario: An administrator creates an MCP proxy from the sample endpoint, then removes it and its project
    Given the user is signed in
    When the user creates a project named "E2E MCP Project"
    Then the user sees "E2E MCP Project" among the projects

    When the user opens the project "E2E MCP Project"
    And the user opens MCP Proxies
    And the user creates an MCP proxy "E2E MCP Proxy" using the sample URL
    Then the user is on the MCP proxy's overview page
    And the user sees "E2E MCP Proxy" on the page

    When the user opens MCP Proxies
    And the user deletes the MCP proxy "E2E MCP Proxy"
    Then the user no longer sees "E2E MCP Proxy"

    When the user returns to the organization level
    And the user opens the projects list
    And the user deletes the project "E2E MCP Project"
    Then the user no longer sees "E2E MCP Project"
