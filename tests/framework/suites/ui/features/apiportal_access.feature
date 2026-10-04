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

Feature: API Portal access

  Background:
    Given the user is signed in to the API Portal
    And the API Portal has portal-access fixtures

  Scenario: Portal home loads and shows the hero section
    When the user opens the API Portal
    Then the API Portal hero section is visible

  Scenario: Browsing the APIs listing opens an API detail page
    When the user browses the API listing and opens the seeded API
    Then the API Portal API detail page is visible

  Scenario: Browsing the MCP servers listing shows a server
    When the user browses the MCP servers listing
    Then the API Portal MCP listing is visible with a server

  Scenario: Opening API Workflows shows a valid page
    When the user opens the API Workflows page
    Then the API Portal page is visible
    And the API Portal page does not contain "Cannot GET"
    And the API Portal page does not contain "500"

  Scenario: Applications redirects to login and returns after signing in
    When the user opens the Applications page while signed out
    Then the API Portal login form is visible
    When the user signs in and returns to Applications
    Then the API Portal Applications page is visible

  Scenario: An API's API Keys page redirects to login and returns after signing in
    When the user opens the seeded API's API Keys page while signed out
    Then the API Portal login form is visible
    When the user signs in and returns to the seeded API's API Keys page
    Then the API Portal API Keys page is visible

  Scenario: The collapsed sidebar lists the navigation items
    When the user opens the API Portal
    Then the API Portal sidebar is collapsed by default and lists navigation items

  Scenario: The sidebar stays expanded across navigation and reload
    When the user opens the API Portal
    And the user pins the API Portal sidebar open and it persists across APIs navigation and reload

  Scenario: The sidebar can be force-collapsed
    When the user opens the API Portal
    And the user pins the API Portal sidebar open and it persists across APIs navigation and reload
    When the user collapses the API Portal sidebar
    Then the API Portal sidebar is force-collapsed
