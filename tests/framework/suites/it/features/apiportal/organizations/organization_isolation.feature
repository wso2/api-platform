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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-104..106. Each refused
  # route is paired with the same route for the configured organization, which is served, so a
  # broken route cannot pass as an isolated one.

  @APIP-CORE-104 @type:security
  Scenario Outline: The <route> of another organization is not exposed, while the configured organization's is
    When I send an unauthenticated API Portal public "GET" request to "<configured path>"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "<foreign path>"
    Then the response status code should be 404
    And the response body should not contain "foreign-org"

    Examples:
      | route        | configured path                            | foreign path                                   |
      | portal home  | /api-portal/default/views/default          | /api-portal/foreign-org/views/default          |
      | API catalog  | /api-portal/default/views/default/apis     | /api-portal/foreign-org/views/default/apis     |
      | MCP registry | /api-portal/registry/default/v0.1/servers  | /api-portal/registry/foreign-org/v0.1/servers  |

  @APIP-CORE-105
  Scenario: The portal root redirects to the configured organization
    When I send an unauthenticated API Portal public "GET" request to "/api-portal"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/default"

  @APIP-CORE-106 @type:security
  Scenario Outline: An organization header of <header> never moves an authenticated caller to another organization
    When I send an authenticated API Portal "GET" request to "/organizations/default" as "admin" with header "organization" set to "<header>"
    Then the response status code should be 200
    And the JSON response field "id" should be "default"

    Examples:
      | header      |
      | default     |
      | foreign-org |
