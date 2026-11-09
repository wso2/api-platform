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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level:
  # the /api-portal/api/v0.9/apis endpoints. REST APIs are covered in rest_apis.feature. Every API is
  # rendered from resources/templates/api-portal-api.yaml with its type set in the values table; the
  # portal reports the stored type (GRAPHQL, SOAP, WS, WebSubApi), which the scenarios assert exactly.

  @APIP-004 @type:negative
  Scenario: GraphQL, WebSocket, WebSub and SOAP APIs are managed with their own definitions and refused without one
    # A publisher manages a GraphQL API from creation to deletion
    Given I generate a unique resource name from "portal-graphql-api" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | GRAPHQL       |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "GRAPHQL"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-renamed |
      | type | GRAPHQL               |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:apiId}-renamed"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

    # A publisher manages a SOAP API from creation to deletion
    Given I generate a unique resource name from "portal-soap-api" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "soap" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | SOAP       |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "SOAP"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "soap" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-renamed |
      | type | SOAP               |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:apiId}-renamed"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

    # A publisher manages a WebSocket API from creation to deletion
    Given I generate a unique resource name from "portal-websocket-api" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websocket" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | WS       |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "WS"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websocket" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-renamed |
      | type | WS               |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:apiId}-renamed"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

    # A publisher manages a WebSub API from creation to deletion
    Given I generate a unique resource name from "portal-websub-api" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websub" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | WEBSUB       |
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "WebSubApi"

    When I update the API Portal API "${CTX:apiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websub" with values:
      | id   | ${CTX:apiId}         |
      | name | ${CTX:apiId}-renamed |
      | type | WEBSUB               |
    Then the response status code should be 200
    And the JSON response field "name" should be "${CTX:apiId}-renamed"

    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

    # A GraphQL API's schema is stored as its definition
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

    # A GraphQL API without a definition is refused and stores nothing
    Given I generate a unique resource name from "portal-graphql-no-definition" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | GRAPHQL       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "graphql" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | GRAPHQL       |
    Then the response status code should be 201

    # A SOAP API without a definition is refused and stores nothing
    Given I generate a unique resource name from "portal-soap-no-definition" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | SOAP       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "soap" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | SOAP       |
    Then the response status code should be 201

    # A WebSocket API without a definition is refused and stores nothing
    Given I generate a unique resource name from "portal-websocket-no-definition" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | WS       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websocket" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | WS       |
    Then the response status code should be 201

    # A WebSub API without a definition is refused and stores nothing
    Given I generate a unique resource name from "portal-websub-no-definition" and store it as "apiId"
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | WEBSUB       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    And the response body should not contain "node_modules"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
    When I create an API Portal API from "resources/templates/api-portal-api.yaml" as "publisher" with definition "websub" with values:
      | id   | ${CTX:apiId} |
      | name | ${CTX:apiId} |
      | type | WEBSUB       |
    Then the response status code should be 201

    # A GraphQL API cannot be changed into a REST API
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
