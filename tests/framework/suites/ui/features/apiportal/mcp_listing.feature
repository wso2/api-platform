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
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@APIPORTAL
Feature: API Portal MCP servers in the catalogue and in the settings
  Journey APIPUI-006: the MCP Servers catalogue lists only MCP servers and searches them; a
  server's page shows its URL, tools, resources and client configuration and opens the MCP
  Playground; an administrator adds an MCP server in the wizard, deprecates it and deletes it.
  Calling a tool from the Playground needs a gateway, which this stack does not run.

  @APIP-017
  Scenario: Journey: a developer explores MCP servers, and an administrator adds, deprecates and deletes one
    Given the user is signed in to the API Portal
    And the API Portal has MCP listing fixtures
    When the MCP listing shows both servers but not the REST API
    And the MCP listing search returns only the first server
    And the MCP listing search excludes the REST API
    Then the first MCP listing server page shows its URL, tools, resources and client configuration, and no prompts
    And the first MCP listing server documentation opens the MCP Playground
    When the first MCP listing server has a thumbnail and a landing page
    Then the first MCP listing server shows its thumbnail
    And the first MCP listing server page shows its landing text
    When the user creates an MCP server in the wizard
    Then the wizard MCP server row shows its thumbnail
    And the wizard MCP server shows its thumbnail when edited
    And the wizard MCP server page shows its tools, resources and prompts
    When the user imports a content package into the wizard MCP server
    And the user deprecates the wizard MCP server from its row menu
    Then the MCP catalogue marks the wizard MCP server as deprecated
    When the user publishes the wizard MCP server from its row menu
    Then the MCP catalogue no longer marks the wizard MCP server as deprecated
    When the user deletes the wizard MCP server from its row menu
    Then the MCP catalogue no longer lists the wizard MCP server
    When the user signs out of the API Portal
    Then the first MCP listing server shows its thumbnail
