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

Feature: API Portal organization isolation

  Scenario: The configured organization portal home is available
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default"
    Then the response status code should be 200

  Scenario: A foreign organization portal home is not exposed
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/foreign-org/views/default"
    Then the response status code should be 404

  Scenario: A foreign organization API listing is not exposed
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/foreign-org/views/default/apis"
    Then the response status code should be 404

  Scenario: A foreign organization MCP registry is not exposed
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/foreign-org/v0.1/servers"
    Then the response status code should be 404

  Scenario: A foreign organization handle is not reflected in the error page
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/foreign-org/views/default"
    Then the response status code should be 404
    And the response body should not contain "foreign-org"

  Scenario: The portal root redirects to the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/api-portal"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default"

  Scenario: An authenticated caller accesses the organization without a header
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200

  Scenario: An authenticated caller accesses the organization with its own header
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin" with header "organization" set to "default"
    Then the response status code should be 200

  Scenario: An organization header cannot redirect an authenticated caller
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin" with header "organization" set to "foreign-org"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"

  Scenario: The discovery index names the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/llms.txt"
    Then the response status code should be 200
    And the response body should contain "/default/views/"
    And the response body should not contain "{orgName}"
