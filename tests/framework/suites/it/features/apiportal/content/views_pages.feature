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


@APIPORTAL @apiportal-view-pages
Feature: A view serves exactly the API pages its labels allow
  As an API Portal administrator
  I want each view to address its pages by handle and show only the APIs that carry its labels
  So that an audience never reaches an API, its Markdown or its specification through another view

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-071..075. Pages
  # are anonymous. Every hidden page is paired with the same page served by the view that holds the
  # API's label, so a broken route cannot pass as a hidden one. The default view is never deleted
  # here: other runners in the block read it.

  @APIP-CORE-071
  Scenario Outline: The <root> redirects to a view of the configured organization
    When I send an unauthenticated API Portal public "GET" request to "<path>"
    Then the response status code should be 302
    And the response header "Location" should contain "/api-portal/default/views/"

    Examples:
      | root                                | path                 |
      | portal root                         | /api-portal          |
      | organization root                   | /api-portal/default  |
      | organization root with a slash      | /api-portal/default/ |

  @APIP-CORE-072
  Scenario: A view is addressed by its handle, never by its display name
    Given I generate a unique resource name from "portal-handle-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}          |
      | displayName | Handle Resolution View |
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Handle Resolution View"
    When I send an authenticated API Portal "GET" request to "/views/Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis?view=Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/mcp-servers?view=${CTX:viewId}" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/mcp-servers?view=Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/views/Handle%20Resolution%20View" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200

  @APIP-CORE-073
  Scenario: An administrator deletes a view, and deleting it again is not found
    Given I generate a unique resource name from "portal-deletable-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}  |
      | displayName | Deletable View |
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/views/default" as "admin"
    Then the response status code should be 200

  @APIP-CORE-074
  Scenario: An API's page, Markdown and specification are served only by a view that holds its label
    Given I generate a unique resource name from "portal-page-label" and store it as "labelId"
    And I generate a unique resource name from "portal-page-view" and store it as "viewId"
    And I generate a unique resource name from "portal-outside-view" and store it as "outsideViewId"
    And I generate a unique resource name from "portal-page-api" and store it as "apiId"
    And I generate a unique resource name from "portal-missing-page-api" and store it as "missingApiId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId} |
      | displayName | Page Label     |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | Page View          |
      | labels      | ["${CTX:labelId}"] |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "outsideViewId":
      | id          | ${CTX:outsideViewId} |
      | displayName | Outside View         |
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}       |
      | name   | ${CTX:apiId}       |
      | labels | ["${CTX:labelId}"] |
    And the response status code should be 201

    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}.md"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:outsideViewId}/api/${CTX:apiId}"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:outsideViewId}/api/${CTX:apiId}.md"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:outsideViewId}/api/${CTX:apiId}/docs/specification.json"
    Then the response status code should be 404
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:missingApiId}"
    Then the response status code should be 404

    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "admin"
    Then the response body should contain "${CTX:apiId}"
    When I send an authenticated API Portal "PUT" request to "/views/${CTX:viewId}" as "admin" with JSON values:
      | displayName | Page View |
      | labels      | []        |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "admin"
    Then the response body should not contain "${CTX:apiId}"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/api/${CTX:apiId}"
    Then the response status code should be 404

  @APIP-CORE-075
  Scenario: An MCP server's page is served only by a view that holds its label
    Given I generate a unique resource name from "portal-mcp-page-label" and store it as "labelId"
    And I generate a unique resource name from "portal-mcp-page-view" and store it as "viewId"
    And I generate a unique resource name from "portal-mcp-outside-view" and store it as "outsideViewId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "labelId":
      | id          | ${CTX:labelId}  |
      | displayName | MCP Page Label  |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}      |
      | displayName | MCP Page View      |
      | labels      | ["${CTX:labelId}"] |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "outsideViewId":
      | id          | ${CTX:outsideViewId} |
      | displayName | MCP Outside View     |
    And a unique API Portal MCP server with label "${CTX:labelId}" is created and stored as "mcpId"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:viewId}/mcp/${CTX:mcpId}"
    Then the response status code should be 200
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/${CTX:outsideViewId}/mcp/${CTX:mcpId}"
    Then the response status code should be 404
