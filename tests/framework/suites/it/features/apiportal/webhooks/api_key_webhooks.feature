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

Feature: API Portal API-key webhook events

  Scenario: API-key generation is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-apikey-generated-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"API Key Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    When a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    Then the response status code should be 201
    When I wait for an API Portal webhook event "apikey.generated" containing "${CTX:keyId}"
    Then the JSON response field "event_type" should be "apikey.generated"
    And the JSON response field "data.handle" should be "${CTX:keyId}"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "apikey.generated" should be "DELIVERED"

  Scenario: API-key regeneration is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-apikey-regenerated-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"API Key Regeneration Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 200
    When I wait for an API Portal webhook event "apikey.regenerated" containing "${CTX:keyId}"
    Then the JSON response field "event_type" should be "apikey.regenerated"
    And the JSON response field "data.handle" should be "${CTX:keyId}"

  Scenario: API-key revocation is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-apikey-revoked-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"API Key Revocation Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 204
    When I wait for an API Portal webhook event "apikey.revoked" containing "${CTX:keyId}"
    Then the JSON response field "event_type" should be "apikey.revoked"
    And the JSON response field "data.handle" should be "${CTX:keyId}"

  Scenario: Associating an API key with an application is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-apikey-application-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"API Key Application Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-apikey-application" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"API Key Application","description":"webhook"}
      """
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON body:
      """
      {"keyId":"${CTX:keyId}","appId":"${CTX:appId}"}
      """
    Then the response status code should be 200
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "apikey.application_updated"
    And the JSON response field "data.application.handle" should be "${CTX:appId}"

  Scenario: Dissociating an API key clears its application and delivers an update
    Given I generate a unique resource name from "portal-apikey-dissociate-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"API Key Dissociation Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-apikey-dissociate-app" and store it as "appId"
    And a unique API Portal resource is created at "/applications" as "developer" with body and stored as "appId":
      """
      {"id":"${CTX:appId}","displayName":"Dissociated Application","description":"webhook"}
      """
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON body:
      """
      {"keyId":"${CTX:keyId}","appId":"${CTX:appId}"}
      """
    Then the response status code should be 200
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "apikey.application_updated"
    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/dissociate" as "developer" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}/api-keys" as "developer"
    Then the response status code should be 200
    And the response body should contain "${CTX:keyId}"
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:keyId}"
    Then the JSON response field "event_type" should be "apikey.application_updated"
    And the JSON response field "data.application" should be:
      """
      null
      """

  Scenario: An API-key value is encrypted for a subscriber with a secret
    Given I generate a unique resource name from "portal-apikey-encrypted-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-apikey-webhook-secret" and store it as "secretValue"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Encrypted API Key Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","secret":"${CTX:secretValue}","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    When a Portal API key for API "${CTX:apiId}" is generated, the value is stored as "keyValue" and the handle is stored as "keyId"
    Then the response status code should be 201
    When I wait for an API Portal webhook event "apikey.generated" containing "${CTX:keyId}"
    Then the JSON response field "event_type" should be "apikey.generated"
    And the JSON response array field "encrypted_fields" should have 1 item
    And the response body should contain "ciphertext"
    And the JSON response should have field "data.key"
    And the API Portal webhook encrypted field "data.key" should decrypt to "${CTX:keyValue}" using secret "${CTX:secretValue}"

  Scenario: A rejected API-key regeneration does not publish an event
    Given I generate a unique resource name from "portal-apikey-rejected-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Rejected API Key Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["apikey.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a publisher API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 204
    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON body:
      """
      {"keyId":"${CTX:keyId}"}
      """
    Then the response status code should be 409
    And I verify no API Portal webhook event "apikey.regenerated" containing "${CTX:keyId}" is delivered
