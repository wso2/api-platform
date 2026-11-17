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
Feature: API Portal sign-in and page access
  Journey APIPUI-018: signing in with the local form, signing out, and which pages need sign-in or
  admin rights. Anonymous users can read the catalogues; Applications and API keys send them to
  login and back; Settings is for administrators only.

  @APIP-031 @APIP-033
  Scenario: Journey: a developer signs in and out, and pages that need sign-in or admin rights are protected
    Given the user is signed in to the API Portal
    And the API Portal has portal-access fixtures
    And the user signs out of the API Portal
    When the user submits the API Portal login with an incorrect password
    Then the API Portal local login error is visible
    And the user shows and hides the password on the API Portal login form
    When the developer signs in to the API Portal through the local form
    And the user signs out of the API Portal through the profile menu
    Then the API Portal login form is visible
    When the user browses the API listing and opens the seeded API
    Then the API Portal API detail page is visible
    When the user browses the MCP servers listing
    Then the API Portal MCP listing is visible with a server
    When the user opens the API Workflows page
    Then the API Portal page does not contain "Cannot GET"
    And the API Portal page does not contain "500"
    When the user opens the Applications page while signed out
    Then the API Portal login form is visible
    When the developer signs in and returns to Applications
    Then the API Portal Applications page is visible
    When the user opens the seeded API's API Keys page while signed out
    Then the API Portal login form is visible
    When the developer signs in and returns to the seeded API's API Keys page
    Then the API Portal API Keys page is visible
    When the user opens the API Portal settings
    Then the API Portal shows the Access denied page
    When the administrator signs in to the API Portal
    And the administrator opens the API Portal settings from the sidebar
    Then the API Portal settings page is shown
