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

Feature: API Portal REST API management

  Scenario: A publisher creates and retrieves a REST API
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response field "type" should be "RestApi"

  Scenario: A publisher updates a REST API
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {
      "name": "Updated REST API",
      "version": "v1.0",
      "type": "REST",
      "status": "PUBLISHED",
      "endPoints": {
        "productionURL": "https://updated.example.invalid",
        "sandboxURL": "https://updated-sandbox.example.invalid"
      }
      }
      """
    Then the response status code should be 200
    And the JSON response field "name" should be "Updated REST API"

  Scenario: A publisher deletes a REST API
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "DELETE" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until status 404
    Then the response status code should be 404

  Scenario: A publisher filters REST APIs by exact name, version, and tag
    Given I generate a unique resource name from "portal-filter-name" and store it as "filterName"
    And I generate a unique resource name from "portal-filter-tag" and store it as "filterTag"
    And a REST API with metadata values is created in the API Portal and stored as "matchingApiId":
      | name    | ${CTX:filterName}           |
      | version | v2.0                        |
      | tags    | ["${CTX:filterTag}"]       |
    And a REST API with metadata values is created in the API Portal and stored as "wrongVersionApiId":
      | name    | ${CTX:filterName}           |
      | version | v1.0                        |
      | tags    | ["${CTX:filterTag}"]       |
    And a REST API with metadata values is created in the API Portal and stored as "wrongTagApiId":
      | name    | Wrong Tag API               |
      | version | v2.0                        |
      | tags    | ["other-filter-tag"]       |
    When I send an authenticated API Portal "GET" request to "/apis?name=${CTX:filterName}&version=v2.0&tags=${CTX:filterTag}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:matchingApiId}"
    And the response body should not contain "${CTX:wrongVersionApiId}"
    And the response body should not contain "${CTX:wrongTagApiId}"

  Scenario: API creation rejects an invalid OpenAPI definition
    When I generate a unique resource name from "portal-invalid-api" and store it as "apiId"
    And I send an authenticated API Portal "POST" multipart request to "/apis" as "publisher" with metadata and definition "not a valid openapi document":
      """
      {
        "id": "${CTX:apiId}",
        "name": "Invalid REST API",
        "version": "v1.0",
        "type": "REST",
        "status": "PUBLISHED",
        "endPoints": {
          "productionURL": "https://backend.example.invalid",
          "sandboxURL": "https://sandbox.example.invalid"
        }
      }
      """
    Then the response status code should be 400

  Scenario: API creation rejects a missing type
    When I generate a unique resource name from "portal-no-type-api" and store it as "apiId"
    And I send an authenticated API Portal "POST" multipart request to "/apis" as "publisher" with metadata:
      """
      {
        "id": "${CTX:apiId}",
        "name": "No Type REST API",
        "version": "v1.0",
        "status": "PUBLISHED",
        "endPoints": {
          "productionURL": "https://backend.example.invalid",
          "sandboxURL": "https://sandbox.example.invalid"
        }
      }
      """
    Then the response status code should be 400

  Scenario: API update rejects a missing type
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {
        "name": "Updated Without Type",
        "version": "v1.0",
        "status": "PUBLISHED",
        "endPoints": {
          "productionURL": "https://updated.example.invalid",
          "sandboxURL": "https://updated-sandbox.example.invalid"
        }
      }
      """
    Then the response status code should be 400

  Scenario: API update rejects changing REST type
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {
        "name": "Still REST API",
        "version": "v1.0",
        "type": "SOAP",
        "status": "PUBLISHED",
        "endPoints": {
          "productionURL": "https://updated.example.invalid",
          "sandboxURL": "https://updated-sandbox.example.invalid"
        }
      }
      """
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the JSON response field "type" should be "RestApi"

  Scenario: An unauthenticated caller cannot list REST APIs
    When I send an unauthenticated API Portal "GET" request to "/apis"
    Then the response status code should be 401

  Scenario: API creation rejects a request whose type is MCP
    When I generate a unique resource name from "portal-mcp-as-api" and store it as "apiId"
    And I send an authenticated API Portal "POST" multipart request to "/apis" as "publisher" with metadata:
      """
      {
        "id": "${CTX:apiId}",
        "name": "MCP Submitted As API",
        "version": "v1.0",
        "type": "MCP",
        "status": "PUBLISHED",
        "endPoints": {
          "productionURL": "https://backend.example.invalid",
          "sandboxURL": "https://sandbox.example.invalid"
        }
      }
      """
    Then the response status code should be 400

  Scenario: A developer cannot create a REST API
    When I generate a unique resource name from "portal-forbidden-api" and store it as "apiId"
    And I send an authenticated API Portal "POST" multipart request to "/apis" as "developer" with metadata:
      """
      {
        "id": "${CTX:apiId}",
        "name": "Forbidden REST API",
        "version": "v1.0",
        "type": "REST",
        "status": "PUBLISHED",
        "endPoints": {
          "productionURL": "https://backend.example.invalid",
          "sandboxURL": "https://sandbox.example.invalid"
        }
      }
      """
    Then the response status code should be 403

  Scenario: A publisher retrieves an uploaded OpenAPI definition
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=definition.json" as "publisher"
    Then the response status code should be 200
    And the response should be valid JSON
    And the JSON response field "openapi" should be "3.0.3"

  Scenario: A missing API definition returns not found
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/assets?type=API_DEFINITION&fileName=missing.json" as "publisher"
    Then the response status code should be 404

  Scenario: A REST API returns its labels
    Given I generate a unique resource name from "portal-api-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"REST API Label"}
      """
    And I generate a unique value from "portal_rest_api_labelled" and store it as "apiName"
    And a REST API with metadata values is created in the API Portal and stored as "apiId":
      | name    | ${CTX:apiName}     |
      | version | v1.0               |
      | labels  | ["${CTX:labelId}"] |
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 1 item
    And the JSON response field "labels[0]" should be "${CTX:labelId}"

  Scenario: A publisher searches REST APIs by free-text query
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "GET" request to "/apis?query=portal_rest_api" as "publisher"
    Then the response status code should be 200
    And the response should be valid JSON

  Scenario: API update replaces its labels
    Given I generate a unique resource name from "portal-rest-label-a" and store it as "labelA"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelA":
      """
      {"id":"${CTX:labelA}","displayName":"Original Label"}
      """
    And I generate a unique resource name from "portal-rest-label-b" and store it as "labelB"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelB":
      """
      {"id":"${CTX:labelB}","displayName":"Replacement Label"}
      """
    And I generate a unique resource name from "portal-rest-labeled-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Labeled API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelA}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {"name":"Labeled API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelB}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:labelB}"
    And the response body should not contain "${CTX:labelA}"

  Scenario: API update preserves the same labels
    Given I generate a unique resource name from "portal-rest-same-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Persistent Label"}
      """
    And I generate a unique resource name from "portal-rest-same-label-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Persistent API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {"name":"Persistent API","version":"v1.1","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:labelId}"

  Scenario: API update adds labels to an unlabeled API
    Given I generate a unique resource name from "portal-rest-added-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Added Label"}
      """
    And I generate a unique resource name from "portal-rest-unlabeled-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Unlabeled API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":[],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {"name":"Unlabeled API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher" until the response body contains "${CTX:labelId}"
    Then the response body should contain "${CTX:labelId}"

  Scenario: API update removes all labels when an empty list is sent
    Given I generate a unique resource name from "portal-rest-removed-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id":"${CTX:labelId}","displayName":"Removed Label"}
      """
    And I generate a unique resource name from "portal-rest-remove-label-api" and store it as "apiId"
    And a unique API Portal multipart resource is created at "/apis" as "publisher" with metadata and definition "rest" stored as "apiId":
      """
      {"id":"${CTX:apiId}","name":"Relabeled API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":["${CTX:labelId}"],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata:
      """
      {"name":"Relabeled API","version":"v1.0","type":"REST","status":"PUBLISHED","labels":[],"endPoints":{"productionURL":"https://api.invalid","sandboxURL":"https://api.invalid"}}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "publisher"
    Then the response status code should be 200
    And the response body should not contain "${CTX:labelId}"
