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
Feature: API Portal catalogue browsing and search
  Journey APIPUI-002: a developer browses and searches the APIs page. Cards show each API's type,
  tags, plan count and AI Ready and Deprecated badges; search finds only the matching API and never
  an MCP server; a view without APIs shows an empty state.

  @APIP-006 @APIP-007
  Scenario: Journey: a developer browses and searches the API catalogue
    Given the user is signed in to the API Portal
    And the API Portal has catalogue fixtures
    When the developer signs in to the API Portal
    And the developer subscribes to the catalogue REST API
    And the user opens the API listing
    Then each catalogue API card shows its type, tags, plan count and badges, and no MCP server is listed
    When the user searches the API listing for the catalogue GraphQL API
    Then the API listing shows only the catalogue GraphQL API, counted once
    When the user clears the API listing search
    Then the API listing shows every catalogue API again
    When the user searches the API listing for the catalogue MCP server
    Then the API listing shows no catalogue API
    When the user opens the API listing of a view with no APIs
    Then the API listing shows that no APIs are available yet
