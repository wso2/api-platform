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

Feature: API Portal API definition types

  Scenario: A publisher creates and retrieves a GraphQL API
    Given I generate a unique resource name from "portal-graphql" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "graphql" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"GraphQL ${CTX:apiId}","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://graphql.invalid","sandboxURL":"https://graphql-sandbox.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "GraphQL"

  Scenario: A publisher updates a GraphQL API and its schema
    Given I generate a unique resource name from "portal-graphql" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "graphql" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"GraphQL ${CTX:apiId}","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://graphql.invalid","sandboxURL":"https://graphql-sandbox.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata and definition "graphql":
      """
      {"name":"Updated GraphQL","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://updated.invalid","sandboxURL":"https://updated-sandbox.invalid"}}
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Updated GraphQL"

  Scenario: A GraphQL API requires a definition
    When I generate a unique resource name from "portal-graphql-missing" and store it as "apiId"
    And I send an authenticated API Portal "POST" multipart request to "/apis" as "publisher" with metadata only:
      """
      {"id":"${CTX:apiId}","name":"Missing GraphQL","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://graphql.invalid","sandboxURL":"https://graphql.invalid"}}
      """
    Then the response status code should be 400

  Scenario: A publisher retrieves a GraphQL schema
    Given I generate a unique resource name from "portal-graphql-schema" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "graphql" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"GraphQL Schema ${CTX:apiId}","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://graphql.invalid","sandboxURL":"https://graphql.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=definition.graphql" as "publisher"
    Then the response status code should be 200
    And the response body should contain "type Query"

  Scenario: A publisher deletes a GraphQL API
    Given I generate a unique resource name from "portal-graphql-delete" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "graphql" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"GraphQL Delete ${CTX:apiId}","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://graphql.invalid","sandboxURL":"https://graphql.invalid"}}
      """
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

  Scenario: A missing GraphQL schema returns not found
    Given I generate a unique resource name from "portal-graphql-no-schema" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "graphql" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"GraphQL No Schema ${CTX:apiId}","version":"v1.0","type":"GRAPHQL","status":"PUBLISHED","endPoints":{"productionURL":"https://graphql.invalid","sandboxURL":"https://graphql.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=missing.graphql" as "publisher"
    Then the response status code should be 404

  Scenario: A publisher creates and retrieves a SOAP API
    Given I generate a unique resource name from "portal-soap" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "soap" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"SOAP ${CTX:apiId}","version":"v1.0","type":"SOAP","status":"PUBLISHED","endPoints":{"productionURL":"https://soap.invalid","sandboxURL":"https://soap-sandbox.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "SOAP"

  Scenario: A SOAP API requires a definition
    When I generate a unique resource name from "portal-soap-missing" and store it as "apiId"
    And I send an authenticated API Portal "POST" multipart request to "/apis" as "publisher" with metadata only:
      """
      {"id":"${CTX:apiId}","name":"Missing SOAP","version":"v1.0","type":"SOAP","status":"PUBLISHED","endPoints":{"productionURL":"https://soap.invalid","sandboxURL":"https://soap.invalid"}}
      """
    Then the response status code should be 400

  Scenario: A publisher updates a SOAP API
    Given I generate a unique resource name from "portal-soap-update" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "soap" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"SOAP Original ${CTX:apiId}","version":"v1.0","type":"SOAP","status":"PUBLISHED","endPoints":{"productionURL":"https://soap.invalid","sandboxURL":"https://soap.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata and definition "soap":
      """
      {"name":"SOAP Updated ${CTX:apiId}","version":"v1.0","type":"SOAP","status":"PUBLISHED","endPoints":{"productionURL":"https://soap-updated.invalid","sandboxURL":"https://soap-updated.invalid"}}
      """
    Then the response status code should be 200
    And the JSON response field "name" should contain "SOAP Updated"

  Scenario: A publisher deletes a SOAP API
    Given I generate a unique resource name from "portal-soap-delete" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "soap" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"SOAP Delete ${CTX:apiId}","version":"v1.0","type":"SOAP","status":"PUBLISHED","endPoints":{"productionURL":"https://soap.invalid","sandboxURL":"https://soap.invalid"}}
      """
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

  Scenario: A publisher creates and retrieves a WebSocket API
    Given I generate a unique resource name from "portal-websocket" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websocket" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSocket ${CTX:apiId}","version":"v1.0","type":"WS","status":"PUBLISHED","endPoints":{"productionURL":"https://websocket.invalid","sandboxURL":"https://websocket-sandbox.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "WebSocket"

  Scenario: A publisher updates a WebSocket API
    Given I generate a unique resource name from "portal-websocket-update" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websocket" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSocket Original ${CTX:apiId}","version":"v1.0","type":"WS","status":"PUBLISHED","endPoints":{"productionURL":"https://websocket.invalid","sandboxURL":"https://websocket.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata and definition "websocket":
      """
      {"name":"WebSocket Updated ${CTX:apiId}","version":"v1.0","type":"WS","status":"PUBLISHED","endPoints":{"productionURL":"https://websocket-updated.invalid","sandboxURL":"https://websocket-updated.invalid"}}
      """
    Then the response status code should be 200
    And the JSON response field "name" should contain "WebSocket Updated"

  Scenario: A publisher deletes a WebSocket API
    Given I generate a unique resource name from "portal-websocket-delete" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websocket" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSocket Delete ${CTX:apiId}","version":"v1.0","type":"WS","status":"PUBLISHED","endPoints":{"productionURL":"https://websocket.invalid","sandboxURL":"https://websocket.invalid"}}
      """
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404

  Scenario: A publisher creates and retrieves a WebSub API
    Given I generate a unique resource name from "portal-websub" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websub" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSub ${CTX:apiId}","version":"v1.0","type":"WEBSUB","status":"PUBLISHED","endPoints":{"productionURL":"https://websub.invalid","sandboxURL":"https://websub-sandbox.invalid"}}
      """
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "WebSub"

  Scenario: A publisher updates a WebSub API
    Given I generate a unique resource name from "portal-websub-update" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websub" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSub Original ${CTX:apiId}","version":"v1.0","type":"WEBSUB","status":"PUBLISHED","endPoints":{"productionURL":"https://websub.invalid","sandboxURL":"https://websub.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata and definition "websub":
      """
      {"name":"WebSub Updated ${CTX:apiId}","version":"v1.0","type":"WEBSUB","status":"PUBLISHED","endPoints":{"productionURL":"https://websub-updated.invalid","sandboxURL":"https://websub-updated.invalid"}}
      """
    Then the response status code should be 200
    And the JSON response field "name" should contain "WebSub Updated"

  Scenario: A publisher deletes a WebSub API
    Given I generate a unique resource name from "portal-websub-delete" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "websub" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"WebSub Delete ${CTX:apiId}","version":"v1.0","type":"WEBSUB","status":"PUBLISHED","endPoints":{"productionURL":"https://websub.invalid","sandboxURL":"https://websub.invalid"}}
      """
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 404
