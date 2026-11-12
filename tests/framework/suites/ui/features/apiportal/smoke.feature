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
Feature: API Portal shell, navigation and error pages
  Journey APIPUI-001: the portal frame everyone sees first. The portal's roots and health checks
  answer, the home page leads to the catalogues, the sidebar keeps its state, each role gets its
  own menu, unknown pages show the Page not found page, and the layout works on a phone. Who may
  open Settings is checked in access.feature.

  @APIP-008
  Scenario: Journey: a visitor finds their way around the portal shell, and unknown pages show the Page not found page
    Given the user is signed in to the API Portal
    And the API Portal has portal-access fixtures
    And the user signs out of the API Portal
    When the user requests API Portal path "/" without following redirects
    Then the API Portal response status is 302
    And the API Portal redirect contains "/api-portal"
    When the user requests API Portal path "/api-portal" without following redirects
    Then the API Portal response status is 302
    And the API Portal redirect contains "/api-portal/default/views/default"
    When the user requests API Portal path "/"
    Then the API Portal response status is 200
    When the user requests API Portal path "/health"
    Then the API Portal response status is 200
    And the API Portal response JSON status is "ok"
    When the user requests API Portal path "/api-portal/health"
    Then the API Portal response status is 200
    And the API Portal response JSON status is "ok"
    When the user requests API Portal path "/nope"
    Then the API Portal response status is 404
    And the API Portal response content type contains "text/plain"
    And the API Portal response body does not contain "<html"
    When the user requests API Portal path "/some-other-portal/apis"
    Then the API Portal response status is 404
    And the API Portal response content type contains "text/plain"
    And the API Portal response body does not contain "<html"
    When the user requests API Portal path "/api/v0.9/apis"
    Then the API Portal response status is 404
    And the API Portal response content type contains "text/plain"
    And the API Portal response body does not contain "<html"
    When the user requests API Portal path "/api-portal/styles/main.css"
    Then the API Portal response status is one of 200, 304, or 404
    When the user requests API Portal path "/api-portal/some-other-org/views/default"
    Then the API Portal response status is 404
    When the user opens the API Portal
    Then the API Portal home shows the hero and the APIs, MCP Servers and API Workflows entry points
    And the API Portal page does not contain "Cannot GET"
    And the API Portal shell shows the anonymous menu
    And the API Portal sidebar is collapsed by default and lists navigation items
    When the user pins the API Portal sidebar open and it persists across APIs navigation and reload
    And the user collapses the API Portal sidebar
    Then the API Portal sidebar is force-collapsed
    When the user opens the API Portal page "/no-such-page-here"
    Then the API Portal shows the Page not found page with a link back to the portal
    When the user opens the page of an API that does not exist
    Then the API Portal shows the Page not found page with a link back to the portal
    When the developer signs in to the API Portal
    Then the API Portal shell shows the developer menu
    When the administrator signs in to the API Portal
    Then the API Portal shell shows the administrator menu
    And the API Portal home, an API page and the settings fit a mobile-sized window with a working sidebar toggle
