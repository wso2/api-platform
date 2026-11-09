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

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level. A
  # subscriber without a secret is sent no key at all; one with a secret is sent the key encrypted
  # with that secret, and never in plain text. A refused change sends no event.

  @APIP-052
  Scenario: Application and API key events reach only the subscribers that asked for them
    # An application's whole life is delivered to a subscriber
    Given I generate a unique resource name from "portal-app-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-webhook-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*"] |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.created"
    And the JSON response field "data.handle" should be "${CTX:appId}"
    And the JSON response field "data.display_name" should be "Webhook Application"
    And the JSON response field "data.type" should be "web"
    And the JSON response array field "encrypted_fields" should have 0 items
    And the JSON response should have field "org.ref_id"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "application.created" should be "DELIVERED"

    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON values:
      | displayName | Updated Webhook Application |
      | description | after                       |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.updated" containing "${CTX:appId}"
    Then the JSON response field "data.display_name" should be "Updated Webhook Application"
    And the JSON response field "data.description" should be "after"

    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.deleted" containing "${CTX:appId}"
    Then the JSON response field "data.handle" should be "${CTX:appId}"

    # Deleting it again finds nothing, and so reports nothing.
    And I reset API Portal webhook events
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 404
    And I verify no API Portal webhook event "application.deleted" containing "${CTX:appId}" is delivered

    # A subscriber is not sent application events it did not ask for
    Given I generate a unique resource name from "portal-key-only-subscriber" and store it as "keySubscriberId"
    And I generate a unique resource name from "portal-app-control-subscriber" and store it as "appSubscriberId"
    And I generate a unique resource name from "portal-unmatched-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "keySubscriberId":
      | id          | ${CTX:keySubscriberId} |
      | displayName | ${CTX:keySubscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["apikey.*"] |
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "appSubscriberId":
      | id          | ${CTX:appSubscriberId} |
      | displayName | ${CTX:appSubscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*"] |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    Then the API Portal delivery for subscriber "${CTX:appSubscriberId}" and event "application.created" should be "DELIVERED"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:keySubscriberId}/deliveries" as "admin"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items

    # A subscriber with an exact event name receives only that event
    Given I generate a unique resource name from "portal-exact-subscriber" and store it as "exactSubscriberId"
    And I generate a unique resource name from "portal-all-apps-subscriber" and store it as "controlSubscriberId"
    And I generate a unique resource name from "portal-exact-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "exactSubscriberId":
      | id          | ${CTX:exactSubscriberId} |
      | displayName | ${CTX:exactSubscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.created"] |
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "controlSubscriberId":
      | id          | ${CTX:controlSubscriberId} |
      | displayName | ${CTX:controlSubscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*"] |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    Then the API Portal delivery for subscriber "${CTX:exactSubscriberId}" and event "application.created" should be "DELIVERED"
    When I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON values:
      | displayName | Renamed Webhook Application |
      | description | Renamed                     |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.updated" containing "${CTX:appId}"
    Then the API Portal delivery for subscriber "${CTX:controlSubscriberId}" and event "application.updated" should be "DELIVERED"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:exactSubscriberId}/deliveries" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "eventType" equal to "application.updated"

    # An API key's life is delivered, without the key, to a subscriber that has no secret
    Given a REST API is created in the API Portal and stored as "apiId"
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

    # Linking an API key to an application, and unlinking it, are delivered
    Given a REST API is created in the API Portal and stored as "apiId"
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
