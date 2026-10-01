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

Feature: API Portal API keys

  Scenario: A publisher generates and lists an API key
    Given a REST API is created in the API Portal and stored as "apiId"
    When a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    Then the response status code should be 201
    And the JSON response should have field "key"
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:keyId}"

  Scenario: A publisher regenerates an API key
    Given a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 200
    And the JSON response should have field "key"

  Scenario: A publisher revokes an API key
    Given a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 204

  Scenario: A publisher cannot regenerate a revoked API key
    Given a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 204
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 409

  Scenario: API key generation rejects an invalid identifier
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/generate" as "publisher" with JSON body:
      """
      {"id":"Invalid ID!"}
      """
    Then the response status code should be 400

  Scenario: An unauthenticated caller cannot list API keys
    Given a REST API is created in the API Portal and stored as "apiId"
    When I send an unauthenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys"
    Then the response status code should be 401

  Scenario: A developer associates an API key with an application
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-associated-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Associated Application","description":"association"}
      """
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON body:
      """
      {"keyId":"${CTX:keyId}","appId":"${CTX:appId}"}
      """
    Then the response status code should be 200
    And the JSON response field "application.id" should be "${CTX:appId}"

  Scenario: A developer dissociates an API key from an application
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-dissociated-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Dissociated Application","description":"association"}
      """
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON body:
      """
      {"keyId":"${CTX:keyId}","appId":"${CTX:appId}"}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/dissociate" as "developer" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 204

  Scenario: A publisher lists API keys across APIs with owning API metadata
    Given a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "GET" request to "/api-keys" as "publisher"
    Then the response status code should be 200
    And the response body should contain "${CTX:keyId}"
    And the response body should contain "apiName"
    And the response body should contain "apiVersion"

  Scenario: API-key listing rejects an invalid status filter
    When I send an authenticated API Portal "GET" request to "/api-keys?status=BOGUS" as "publisher"
    Then the response status code should be 400
