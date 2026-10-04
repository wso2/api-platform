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

Feature: API Portal smoke behavior

  Scenario: Root redirects to the default organization view
    When the user requests API Portal path "/" without following redirects
    Then the API Portal response status is 302
    And the API Portal redirect contains "/api-portal"
    When the user requests API Portal path "/api-portal" without following redirects
    Then the API Portal response status is 302
    And the API Portal redirect contains "/api-portal/default/views/default"
    When the user requests API Portal path "/"
    Then the API Portal response status is 200

  Scenario: The default organization view loads
    When the user opens the API Portal
    Then the API Portal page is visible
    And the API Portal page does not contain "500"
    And the API Portal page does not contain "Cannot GET"

  Scenario: Health is available at the root and under the prefix
    When the user requests API Portal path "/health"
    Then the API Portal response status is 200
    And the API Portal response JSON status is "ok"
    When the user requests API Portal path "/api-portal/health"
    Then the API Portal response status is 200
    And the API Portal response JSON status is "ok"

  Scenario: Paths outside the mount prefix return plain 404 responses
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

  Scenario: An unknown path inside the prefix renders the portal error page
    When the user requests API Portal path "/api-portal/no-such-page-here"
    Then the API Portal response status is 404
    And the API Portal response content type contains "text/html"

  Scenario: The main CSS asset is served or correctly absent
    When the user requests API Portal path "/api-portal/styles/main.css"
    Then the API Portal response status is one of 200, 304, or 404

  Scenario: An unknown organization view returns 404
    When the user requests API Portal path "/api-portal/some-other-org/views/default"
    Then the API Portal response status is 404
