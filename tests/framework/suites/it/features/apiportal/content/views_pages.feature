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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml). Pages
  # are anonymous. Every hidden page is paired with the same page served by the view that holds the
  # API's label, so a broken route cannot pass as a hidden one. The default view is never deleted
  # here: other runners in the block read it.

  @APIP-006
  Scenario: A view lists and serves only the APIs and MCP servers that carry its label
    # A view's API listing holds only the APIs that carry its label
    Given I generate a unique resource name from "portal-included-label" and store it as "includedLabel"
    And I generate a unique resource name from "portal-filtered-view" and store it as "viewId"
    And I generate a unique resource name from "portal-labelled-api" and store it as "apiId"
    And I generate a unique resource name from "portal-unlabelled-api" and store it as "otherApiId"
    And a unique API Portal resource is created at "/labels" as "admin" with values and stored as "includedLabel":
      | id          | ${CTX:includedLabel} |
      | displayName | Included Label |
    And a unique API Portal resource is created at "/views" as "admin" with values and stored as "viewId":
      | id          | ${CTX:viewId}            |
      | displayName | Filtered View            |
      | labels      | ["${CTX:includedLabel}"] |
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:apiId}             |
      | name   | ${CTX:apiId}             |
      | labels | ["${CTX:includedLabel}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id     | ${CTX:otherApiId} |
      | name   | ${CTX:otherApiId} |
      | labels | []                |
    And the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis?view=${CTX:viewId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    And the response body should not contain "${CTX:otherApiId}"
    # Listing without a view covers the APIs every view shows, so an API with no labels is in none.
    When I send an authenticated API Portal "GET" request to "/apis" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:apiId}"
    And the response body should not contain "${CTX:otherApiId}"
    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:otherApiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:otherApiId}"

    # An API's page, Markdown and specification are served only by a view that holds its label
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

    # An MCP server's page is served only by a view that holds its label
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

    # Every API type is listed under APIs with its own badge, and MCP servers only under MCP Servers
    Given I generate a unique resource name from "portal-types-tag" and store it as "tag"
    And I generate a unique resource name from "portal-types-rest" and store it as "restId"
    And I generate a unique resource name from "portal-types-graphql" and store it as "graphqlId"
    And I generate a unique resource name from "portal-types-ws" and store it as "wsId"
    And I generate a unique resource name from "portal-types-websub" and store it as "websubId"
    And I generate a unique resource name from "portal-types-soap" and store it as "soapId"
    And I generate a unique resource name from "portal-types-mcp" and store it as "mcpId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:restId}  |
      | name | ${CTX:restId}  |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:graphqlId} |
      | name | ${CTX:graphqlId} |
      | type | GRAPHQL          |
      | tags | ["${CTX:tag}"]   |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websocket" with values:
      | id   | ${CTX:wsId}    |
      | name | ${CTX:wsId}    |
      | type | WS             |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websub" with values:
      | id   | ${CTX:websubId} |
      | name | ${CTX:websubId} |
      | type | WEBSUB          |
      | tags | ["${CTX:tag}"]  |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "soap" with values:
      | id   | ${CTX:soapId}  |
      | name | ${CTX:soapId}  |
      | type | SOAP           |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" at "/mcp-servers" as "publisher" with definition "mcp" with values:
      | id   | ${CTX:mcpId}   |
      | name | ${CTX:mcpId}   |
      | type | MCP            |
      | tags | ["${CTX:tag}"] |
    And the response status code should be 201

    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/apis?query=${CTX:tag}"
    Then the response status code should be 200
    And the response body should contain "apiCard-${CTX:restId}"
    And the response body should contain "apiCard-${CTX:graphqlId}"
    And the response body should contain "apiCard-${CTX:wsId}"
    And the response body should contain "apiCard-${CTX:websubId}"
    And the response body should contain "apiCard-${CTX:soapId}"
    And the response body should not contain "apiCard-${CTX:mcpId}"
    And the response body should contain "dp-badge--rest"
    And the response body should contain "dp-badge--graphql"
    And the response body should contain "dp-badge--ws"
    And the response body should contain "dp-badge--soap"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/mcps?query=${CTX:tag}"
    Then the response status code should be 200
    And the response body should contain "apiCard-${CTX:mcpId}"
    And the response body should not contain "apiCard-${CTX:restId}"
    And the response body should not contain "apiCard-${CTX:soapId}"

    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:soapId}"
    Then the response status code should be 200
    And the response body should contain "dp-badge--soap"
    And the response body should contain "bi-download"
    And the response body should not contain "/api/${CTX:soapId}/docs/specification"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:soapId}/docs/specification.xml"
    Then the response status code should be 200
    And the response body should contain "schemas.xmlsoap.org/wsdl"
    When I send an unauthenticated API Portal public "GET" request to "/api-portal/default/views/default/api/${CTX:restId}"
    Then the response status code should be 200
    And the response body should contain "/api/${CTX:restId}/docs/specification"
