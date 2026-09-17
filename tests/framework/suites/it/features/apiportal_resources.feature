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

Feature: API Portal resource management

  Scenario: A developer creates and retrieves an application
    Given I generate a unique resource name from "portal-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id": "${CTX:appId}", "displayName": "Test Application", "description": "API Portal integration application"}
      """
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Test Application"

  Scenario: A developer updates an application
    Given I generate a unique resource name from "portal-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id": "${CTX:appId}", "displayName": "Original Application", "description": "original"}
      """
    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON body:
      """
      {"displayName": "Renamed Application", "description": "updated"}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed Application"

  Scenario: A developer deletes an application
    Given I generate a unique resource name from "portal-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id": "${CTX:appId}", "displayName": "To Delete", "description": "temporary"}
      """
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 404

  Scenario: Application creation rejects missing required fields
    When I generate a unique resource name from "portal-app" and store it as "appId"
    And I send an authenticated API Portal "POST" request to "/applications" as "developer" with JSON body:
      """
      {"id": "${CTX:appId}"}
      """
    Then the response status code should be 400

  Scenario: A developer lists applications
    Given I generate a unique resource name from "portal-listed-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Listed Application","description":"listed"}
      """
    When I send an authenticated API Portal "GET" request to "/applications" as "developer"
    Then the response status code should be 200
    And the response body should contain "${CTX:appId}"

  Scenario: An administrator creates and retrieves a label
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id": "${CTX:labelId}", "displayName": "Premium APIs"}
      """
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Premium APIs"

  Scenario: An administrator updates a label
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id": "${CTX:labelId}", "displayName": "Original Label"}
      """
    When I send an authenticated API Portal "PUT" request to "/labels/${CTX:labelId}" as "admin" with JSON body:
      """
      {"id": "${CTX:labelId}", "displayName": "Renamed Label"}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed Label"

  Scenario: An administrator deletes a label
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id": "${CTX:labelId}", "displayName": "To Delete"}
      """
    When I send an authenticated API Portal "DELETE" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/labels/${CTX:labelId}" as "admin"
    Then the response status code should be 404

  Scenario: An administrator creates a view with a label
    Given I generate a unique resource name from "portal-label" and store it as "labelId"
    And a unique API Portal resource is created at "/labels" as "admin" with body and stored as "labelId":
      """
      {"id": "${CTX:labelId}", "displayName": "View Label"}
      """
    And I generate a unique resource name from "portal-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id": "${CTX:viewId}", "displayName": "Partner APIs", "labels": ["${CTX:labelId}"]}
      """
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Partner APIs"
    And the JSON response array field "labels" should have 1 item

  Scenario: An administrator creates a view without labels
    Given I generate a unique resource name from "portal-view" and store it as "viewId"
    And a unique API Portal resource is created at "/views" as "admin" with body and stored as "viewId":
      """
      {"id": "${CTX:viewId}", "displayName": "Unlabelled View"}
      """
    When I send an authenticated API Portal "GET" request to "/views/${CTX:viewId}" as "admin"
    Then the response status code should be 200
    And the JSON response array field "labels" should have 0 items

  Scenario: An administrator creates and retrieves a key manager
    Given I generate a unique resource name from "portal-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id": "${CTX:kmId}", "displayName": "Test Key Manager", "tokenEndpoint": "https://token.example.invalid/oauth2/token"}
      """
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Test Key Manager"

  Scenario: An administrator updates a key manager
    Given I generate a unique resource name from "portal-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id": "${CTX:kmId}", "displayName": "Original KM", "tokenEndpoint": "https://token.example.invalid/oauth2/token"}
      """
    When I send an authenticated API Portal "PUT" request to "/key-managers/${CTX:kmId}" as "admin" with JSON body:
      """
      {"displayName": "Renamed KM"}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Renamed KM"

  Scenario: An administrator deletes a key manager
    Given I generate a unique resource name from "portal-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id": "${CTX:kmId}", "displayName": "To Delete", "tokenEndpoint": "https://token.example.invalid/oauth2/token"}
      """
    When I send an authenticated API Portal "DELETE" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/key-managers/${CTX:kmId}" as "admin"
    Then the response status code should be 404

  Scenario: An administrator lists key managers for the organization
    Given I generate a unique resource name from "portal-listed-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id":"${CTX:kmId}","displayName":"Listed Key Manager","tokenEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token"}
      """
    When I send an authenticated API Portal "GET" request to "/key-managers" as "admin"
    Then the response status code should be 200
    And the response body should contain "${CTX:kmId}"

  Scenario: A developer generates a token through an application key-manager mapping
    Given I generate a unique resource name from "portal-token-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id":"${CTX:kmId}","displayName":"Mock Token Key Manager","tokenEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token"}
      """
    And I generate a unique resource name from "portal-token-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Token Test Application","description":"token generation"}
      """
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON body:
      """
      {"keyManager":"${CTX:kmId}","type":"PRODUCTION","consumerKey":"test-client"}
      """
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON body:
      """
      {"consumerSecret":"test-secret"}
      """
    Then the response status code should be 200
    And the response body should contain "mock-token-1-issued"

  Scenario: Token generation rejects an incorrect consumer secret
    Given I generate a unique resource name from "portal-invalid-token-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id":"${CTX:kmId}","displayName":"Invalid Secret Key Manager","tokenEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token"}
      """
    And I generate a unique resource name from "portal-invalid-token-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Invalid Secret Application","description":"token generation"}
      """
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON body:
      """
      {"keyManager":"${CTX:kmId}","type":"PRODUCTION","consumerKey":"test-client"}
      """
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON body:
      """
      {"consumerSecret":"wrong-secret"}
      """
    Then the response should be a client error

  Scenario: Token generation applies default scope and validity
    Given I generate a unique resource name from "portal-default-token-km" and store it as "kmId"
    And a unique API Portal resource is created at "/key-managers" as "admin" with body and stored as "kmId":
      """
      {"id":"${CTX:kmId}","displayName":"Default Token Key Manager","tokenEndpoint":"http://testbench:3011/${CTX:apiPortalRunnerPartition}/oauth2/token?ttl=3600"}
      """
    And I generate a unique resource name from "portal-default-token-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Default Token Application","description":"token generation"}
      """
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/generate-keys" as "developer" with JSON body:
      """
      {"keyManager":"${CTX:kmId}","type":"PRODUCTION","consumerKey":"test-client"}
      """
    Then the response status code should be 200
    And I store the JSON response field "keyMappingId" as "keyMappingId"
    And I register API Portal application key mapping "${CTX:keyMappingId}" for application "${CTX:appId}" for cleanup
    When I send an authenticated API Portal "POST" request to "/applications/${CTX:appId}/oauth-keys/${CTX:keyMappingId}/generate-token" as "developer" with JSON body:
      """
      {"consumerSecret":"test-secret"}
      """
    Then the response status code should be 200
    And the response body should contain "mock-token"
    And the JSON response field "validityTime" should be 3600
