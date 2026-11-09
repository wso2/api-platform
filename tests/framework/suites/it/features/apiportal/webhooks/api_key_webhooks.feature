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


@APIPORTAL @apiportal-api-key-webhook
Feature: API key changes are delivered to webhook subscribers, with the key only ever encrypted
  As the control plane that lets gateways accept the API Portal's API keys
  I want an event for every key generated, rotated, revoked or linked to an application
  So that gateways accept exactly the keys the portal issued, and a key never travels in plain text

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-WEBHOOKS-011..013, at the API level. A
  # subscriber without a secret is sent no key at all; one with a secret is sent the key encrypted
  # with that secret, and never in plain text. A refused change sends no event.

  Background:
    Given a REST API is created in the API Portal and stored as "apiId"

  @APIP-WEBHOOKS-011
  Scenario: An API key's life is delivered, without the key, to a subscriber that has no secret
    Given I generate a unique resource name from "portal-key-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["apikey.*"] |
    When a Portal API key for API "${CTX:apiId}" is generated, the value is stored as "keyValue" and the handle is stored as "keyId"
    And I wait for an API Portal webhook event "apikey.generated" containing "${CTX:keyId}"
    Then the JSON response field "data.handle" should be "${CTX:keyId}"
    And the JSON response array field "encrypted_fields" should have 0 items
    And the JSON response field "data.key" should not exist
    And the response body should not contain "${CTX:keyValue}"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "apikey.generated" should be "DELIVERED"

    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "apikey.regenerated" containing "${CTX:keyId}"
    Then the JSON response field "data.handle" should be "${CTX:keyId}"
    And the JSON response field "data.key" should not exist

    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/revoke" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I wait for an API Portal webhook event "apikey.revoked" containing "${CTX:keyId}"
    Then the JSON response field "data.handle" should be "${CTX:keyId}"

    # A revoked key cannot be rotated, and the refused rotation reports nothing.
    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 409
    And I verify no API Portal webhook event "apikey.regenerated" containing "${CTX:keyId}" is delivered

  @APIP-WEBHOOKS-012
  Scenario: Linking an API key to an application, and unlinking it, are delivered
    Given I generate a unique resource name from "portal-key-app-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-key-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["apikey.*"] |
    And a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Key Application    |
      | description | Receives events        |
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/associate" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
      | appId | ${CTX:appId} |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:appId}"
    Then the JSON response field "data.handle" should be "${CTX:keyId}"
    And the JSON response field "data.application.handle" should be "${CTX:appId}"

    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/dissociate" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 204
    When I wait for an API Portal webhook event "apikey.application_updated" containing "${CTX:keyId}"
    Then the JSON response field "data.application" should be "null"

    # A key that is no longer linked has nothing to unlink, so a second unlink reports nothing.
    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/dissociate" as "developer" with JSON values:
      | keyId | ${CTX:keyId} |
    And I verify no API Portal webhook event "apikey.application_updated" containing "${CTX:keyId}" is delivered

  @APIP-WEBHOOKS-013 @type:security
  Scenario: A subscriber with a secret is sent the key encrypted with it, never in plain text
    Given I generate a unique resource name from "portal-encrypted-key-subscriber" and store it as "subscriberId"
    And I generate a unique value from "portal-key-secret" and store it as "secretValue"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["apikey.*"] |
      | secret      | ${CTX:secretValue} |
    When a Portal API key for API "${CTX:apiId}" is generated, the value is stored as "keyValue" and the handle is stored as "keyId"
    And I wait for an API Portal webhook event "apikey.generated" containing "${CTX:keyId}"
    Then the JSON response array field "encrypted_fields" should have 1 item
    And the JSON response field "encrypted_fields[0]" should be "key"
    And the response body should not contain "${CTX:keyValue}"
    And the API Portal webhook encrypted field "data.key" should decrypt to "${CTX:keyValue}" using secret "${CTX:secretValue}"
    And the API Portal webhook delivery "apikey.generated" containing "${CTX:keyId}" should have a valid signature using secret "${CTX:secretValue}"
