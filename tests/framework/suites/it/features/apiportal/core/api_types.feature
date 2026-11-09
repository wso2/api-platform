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

@APIPORTAL @apiportal-api-types
Feature: GraphQL, SOAP, WebSocket and WebSub APIs are managed through the API Portal REST API
  As an API publisher
  I want every API type the portal supports to be created, updated and deleted with its own definition
  So that developers find each API with the right type and contract

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-060..063 and 125, at the API level:
  # the /api-portal/api/v0.9/apis endpoints. REST APIs are covered in rest_apis.feature. Every API is
  # rendered from resources/templates/api-portal-api.yaml with its type set in the values table; the
  # portal reports the stored type (GRAPHQL, SOAP, WS, WebSubApi), which the scenarios assert exactly.

  @APIP-CORE-060
  Scenario Outline: A publisher manages a <kind> API from creation to deletion
    Given I generate a unique resource name from "portal-<definition>-api" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "<definition>" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | <type>       |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "<stored type>"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "<definition>" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-renamed |
      | type | <type>               |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:apiId}-renamed"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

    Examples:
      | kind      | type    | definition | stored type |
      | GraphQL   | GRAPHQL | graphql    | GRAPHQL     |
      | SOAP      | SOAP    | soap       | SOAP        |
      | WebSocket | WS      | websocket  | WS          |
      | WebSub    | WEBSUB  | websub     | WebSubApi   |

  @APIP-CORE-061
  Scenario: A GraphQL API's schema is stored as its definition
    Given I generate a unique resource name from "portal-graphql-schema" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | GRAPHQL      |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=definition.graphql" as "publisher"
    Then the response status code should be 200
    And the response body should contain "type Query"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=missing.graphql" as "publisher"
    Then the response status code should be 404

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | GRAPHQL      |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=definition.graphql" as "publisher"
    Then the response status code should be 200
    And the response body should contain "type Query"

  @APIP-CORE-062 @type:negative
  Scenario Outline: A <kind> API without a definition is refused and stores nothing
    Given I generate a unique resource name from "portal-<definition>-no-definition" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | <type>       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "<definition>" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | <type>       |
    Then the response status code should be 201

    Examples:
      | kind      | type    | definition |
      | GraphQL   | GRAPHQL | graphql    |
      | SOAP      | SOAP    | soap       |
      | WebSocket | WS      | websocket  |
      | WebSub    | WEBSUB  | websub     |

  @APIP-CORE-063 @type:negative
  Scenario: A GraphQL API cannot be changed into a REST API
    Given I generate a unique resource name from "portal-graphql-retype" and store it as "apiId"
    And I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | GRAPHQL      |
    And the response status code should be 201
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-changed |
      | type | REST                 |
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "type" should be "GRAPHQL"
    And the JSON response field "name" should be "${CTX:apiId}"
    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-changed |
      | type | GRAPHQL              |
    Then the response status code should be 200

  @APIP-CORE-125
  Scenario: Every API type is listed under APIs with its own badge, and MCP servers only under MCP Servers
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
