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


@APIPORTAL @apiportal-api-key
Feature: API keys are generated, rotated and revoked through the API Portal REST API
  As an API publisher or developer
  I want an API key for an API that I can rotate, link to an application and revoke
  So that a leaked key can be replaced or stopped at once, and keys are never shown again after creation

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-CORE-089..092 and 190..196, at the API
  # level. Key listings must never carry the key value. Every refusal is paired with the request
  # that succeeds for the owner. A key belongs to the user who generated it, so the administrator is
  # the second user of the ownership scenario.

  Background:
    Given a REST API is created in the API Portal and stored as "apiId"

  @APIP-CORE-089 @type:smoke
  Scenario: A publisher manages an API key from generation to revocation
    When a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    Then the response status code should be 201
    And the JSON response should have field "key"
    And I store the JSON response field "key" as "originalKey"

    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "publisher"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:keyId}"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should not have field "key"
    When I send an authenticated API Portal "GET" request to "/api-keys" as "publisher"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "apiName" equal to "${CTX:apiId}"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "apiVersion" equal to "v1"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should not have field "key"

    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 200
    And the JSON response field "key" should not equal "${CTX:originalKey}"

    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "publisher"
    Then the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "status" equal to "REVOKED"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 409
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 409

  @APIP-CORE-090
  Scenario: A developer links an API key to an application and unlinks it
    Given I generate a unique resource name from "portal-key-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Key Application     |
      | description | Holds linked keys   |
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
      | appId | ${CTX:appId} |
    Then the response status code should be 200
    And the JSON response field "application.id" should be "${CTX:appId}"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys?appId=${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:keyId}"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/dissociate" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys?appId=${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:keyId}"

  @APIP-CORE-091 @type:negative
  Scenario: A revoked API key cannot be linked to an application
    Given I generate a unique resource name from "portal-revoked-key-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}          |
      | displayName | Revoked Key App       |
      | description | Refuses revoked keys  |
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
      | appId | ${CTX:appId} |
    Then the response status code should be 409
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys?appId=${CTX:appId}" as "developer"
    Then the JSON response array "list" should not contain an item with "id" equal to "${CTX:keyId}"

  @APIP-CORE-092 @type:security
  Scenario: API key requests that are not allowed or that name nothing are refused
    Given I generate a unique resource name from "portal-missing-key" and store it as "missingKeyId"
    When I send an unauthenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys"
    Then the response status code should be 401
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/generate" as "publisher" with JSON values:
      | id | Invalid ID! |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:missingKeyId} |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/api-keys?limit=0" as "publisher"
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/api-keys?limit=100" as "publisher"
    Then the response status code should be 200

  # Workbook row APIP-027 step 1.
  @APIP-CORE-190 @type:security
  Scenario: Another user can neither see, rotate, revoke nor link a developer's API key
    Given I generate a unique resource name from "portal-other-user-app" and store it as "adminAppId"
    And a unique API Portal resource is created at "/applications" as "admin" with values and stored as "adminAppId":
      | id          | ${CTX:adminAppId}     |
      | displayName | Other User App         |
      | description | Owned by another user  |
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:keyId}"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 404
    And the JSON response field "key" should not exist
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "admin" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "admin" with JSON values:
      | keyId | ${CTX:keyId}      |
      | appId | ${CTX:adminAppId} |
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "status" equal to "ACTIVE"
    And the JSON response array "list" item with "id" equal to "${CTX:keyId}" should have "appId" equal to "null"

  # Workbook row APIP-027 step 2.
  @APIP-CORE-191 @type:security
  Scenario: API key requests without a session are refused
    When I send an unauthenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/generate"
    Then the response status code should be 401
    And the JSON response field "key" should not exist
    When I send an unauthenticated API Portal "GET" request to "/api-keys"
    Then the response status code should be 401
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the JSON response array field "list" should have 0 items
    When a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    Then the response status code should be 201
    When I send an authenticated API Portal "GET" request to "/api-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:keyId}"

  # Workbook row APIP-026 step 3.
  @APIP-CORE-192
  Scenario Outline: A key with <case> is issued with that expiry
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/generate" as "developer" with JSON body:
      """
      {"id": "<id>", "expiresAt": <expiresAt>}
      """
    And I register API Portal API key "<id>" of API "${CTX:apiId}" owned by "developer" for cleanup
    Then the response status code should be 201
    And the JSON response should have field "key"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the JSON response array "list" item with "id" equal to "<id>" should have "status" equal to "ACTIVE"
    And the JSON response array "list" item with "id" equal to "<id>" should have "expiresAt" equal to "<stored>"

    Examples:
      | case                        | id                | expiresAt                   | stored                   |
      | an ISO time with an offset  | iso-offset-key    | "2035-01-01T05:30:00+05:30" | 2035-01-01T00:00:00.000Z |
      | an expiry in epoch seconds  | epoch-seconds-key | 2051222400                  | 2035-01-01T00:00:00.000Z |
      | an expiry in epoch millis   | epoch-millis-key  | 2051222400000               | 2035-01-01T00:00:00.000Z |
      | the longest id, 128 letters | k0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcde | "2035-01-01T00:00:00Z" | 2035-01-01T00:00:00.000Z |

  # Workbook row APIP-026 step 3.
  @APIP-CORE-193 @type:negative
  Scenario Outline: A key whose <problem> is refused and no key is issued
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/generate" as "developer" with JSON values:
      | id        | <id>        |
      | expiresAt | <expiresAt> |
    And I register API Portal API key "<id>" of API "${CTX:apiId}" owned by "developer" for cleanup
    Then the response status code should be 400
    And the JSON response field "key" should not exist
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items

    Examples:
      | problem                         | id          | expiresAt            |
      | id has a space and capitals     | Bad Id      | 2035-01-01T00:00:00Z |
      | id is 129 characters            | k0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdex | 2035-01-01T00:00:00Z |
      | expiry has no time zone         | no-zone-key | 2035-01-01T00:00:00  |
      | expiry is after the year 2100   | late-key    | 2101-01-01T00:00:00Z |

  # Workbook row APIP-026 steps 2 (portal side) and 6.
  @APIP-CORE-195
  Scenario: A key without an expiry never expires, and both key listings page through their results
    Given a "developer" API key for API "${CTX:apiId}" is generated and stored as "firstKeyId"
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "secondKeyId"
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "thirdKeyId"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 3 items
    And the JSON response array "list" item with "id" equal to "${CTX:firstKeyId}" should have "expiresAt" equal to "null"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/generate" as "developer" with JSON values:
      | id | ${CTX:firstKeyId} |
    Then the response status code should be 409
    And the JSON response field "key" should not exist
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys?limit=2" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response field "pagination.total" should be 3
    And the JSON response field "pagination.limit" should be 2
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys?limit=2&offset=2" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.offset" should be 2
    When I send an authenticated API Portal "GET" request to "/api-keys?limit=1" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response field "pagination.limit" should be 1
    And the JSON response field "pagination.total" should be greater than 2
