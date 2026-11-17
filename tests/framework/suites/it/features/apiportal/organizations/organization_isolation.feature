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


@APIPORTAL @apiportal-org-isolation
Feature: The API Portal serves only its configured organization
  As an API Portal operator
  I want every page, catalog and registry route of another organization to be unreachable
  So that one portal can never expose or be steered into a tenant it does not serve

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). Each refused
  # route is paired with the same route for the configured organization, which is served, so a
  # broken route cannot pass as an isolated one.

  @APIP-029 @type:security
  Scenario: Another organization's routes are not exposed and an organization header never moves a caller
    # The portal home of another organization is not exposed, while the configured organization's is
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/foreign-org/views/default"
    Then the response status code should be 404
    And the response body should not contain "foreign-org"

    # The API catalog of another organization is not exposed, while the configured organization's is
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/foreign-org/views/default/apis"
    Then the response status code should be 404
    And the response body should not contain "foreign-org"

    # The MCP registry of another organization is not exposed, while the configured organization's is
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/default/v0.1/servers"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/registry/foreign-org/v0.1/servers"
    Then the response status code should be 404
    And the response body should not contain "foreign-org"

    # An organization header of default never moves an authenticated caller to another organization
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin" with header "organization" set to "default"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"

    # An organization header of foreign-org never moves an authenticated caller to another organization
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin" with header "organization" set to "foreign-org"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"
