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


@APIPORTAL @apiportal-webhook-delivery
Feature: The API Portal delivers each event once, signed, and records how every delivery went
  As an API Portal administrator
  I want to see whether each event reached each subscriber, and to stop a subscriber receiving any
  So that I can tell a working integration from a failing one, and trust that deliveries are genuine

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-WEBHOOKS-017..020, at the API level. A
  # target ending in ?status=500 makes the testbench sink fail the delivery. Delivery is attempted
  # once; an event is ALL_DELIVERED only when every matching subscriber accepted it.

  @APIP-WEBHOOKS-017
  Scenario: A disabled subscriber is sent nothing while an enabled one is sent the event
    Given I generate a unique resource name from "portal-disabled-subscriber" and store it as "disabledSubscriberId"
    And I generate a unique resource name from "portal-enabled-subscriber" and store it as "enabledSubscriberId"
    And I generate a unique resource name from "portal-disabled-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "disabledSubscriberId":
      """
      {"id":"${CTX:disabledSubscriberId}","displayName":"Disabled Subscriber","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["application.*"],"enabled":false}
      """
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "enabledSubscriberId":
      | id          | ${CTX:enabledSubscriberId} |
      | displayName | ${CTX:enabledSubscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*"] |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    Then the API Portal delivery for subscriber "${CTX:enabledSubscriberId}" and event "application.created" should be "DELIVERED"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:disabledSubscriberId}/deliveries" as "admin"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items

  @APIP-WEBHOOKS-018
  Scenario: A delivery the target refuses is recorded as failed, with the status it returned
    Given I generate a unique resource name from "portal-failing-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-failing-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId}   |
      | displayName | Failing Subscriber     |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook?status=500 |
      | events      | ["application.*"]      |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    And I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    And I store the JSON response field "event_id" as "eventId"
    Then I wait for API Portal webhook event "${CTX:eventId}" to reach status "FAILED"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}/deliveries" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" item with "eventType" equal to "application.created" should have "status" equal to "FAILED"
    And the JSON response array "list" item with "eventType" equal to "application.created" should have "lastHttpStatus" equal to "500"

  @APIP-WEBHOOKS-019
  Scenario: An event every subscriber accepts is recorded as delivered, and only some roles may read it
    Given I generate a unique resource name from "portal-delivered-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-delivered-app" and store it as "appId"
    And I generate a unique resource name from "portal-missing-event" and store it as "missingEventId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*"] |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    And I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    And I store the JSON response field "event_id" as "eventId"
    Then I wait for API Portal webhook event "${CTX:eventId}" to reach status "ALL_DELIVERED"
    When I send an authenticated API Portal "GET" request to "/webhook-events/${CTX:eventId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "eventType" should be "application.created"
    # Deliveries name the subscriber by its internal id, so the scenario's own target identifies it.
    And the JSON response array "deliveries" item with "targetUrl" equal to "http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook" should have "status" equal to "DELIVERED"
    When I send an authenticated API Portal "GET" request to "/webhook-events/${CTX:eventId}" as "publisher"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/webhook-events/${CTX:eventId}" as "developer"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/webhook-events/${CTX:missingEventId}" as "admin"
    Then the response status code should be 404

  @APIP-WEBHOOKS-020 @type:security
  Scenario: A subscriber's secret signs the exact payload it is sent
    Given I generate a unique resource name from "portal-signed-subscriber" and store it as "subscriberId"
    And I generate a unique value from "portal-signing-secret" and store it as "signingSecret"
    And I generate a unique resource name from "portal-signed-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*"] |
      | secret      | ${CTX:signingSecret} |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Webhook Application |
      | description | Receives events        |
    When I wait for an API Portal webhook event "application.created" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.created"
    And the response body should not contain "${CTX:signingSecret}"
    And the API Portal webhook delivery "application.created" containing "${CTX:appId}" should have a valid signature using secret "${CTX:signingSecret}"

  # APIP-WEBHOOKS-026: every delivery repeats its event in X-Api-Portal-Event and -Event-Id, has a
  # delivery id of its own, and carries the envelope a receiver orders and decodes events by. Two
  # regenerations in a row are two events, told apart by event_id and occurred_at.
  @APIP-WEBHOOKS-026
  Scenario: Every delivery repeats its event in its headers and carries the event envelope
    Given a REST API is created in the API Portal and stored as "apiId"
    And I generate a unique resource name from "portal-envelope-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-envelope-app" and store it as "appId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["application.*", "apikey.*"] |
    When a unique API Portal resource is created at "/applications" as "developer" with values and stored as "appId":
      | id          | ${CTX:appId}        |
      | displayName | Envelope Application |
      | description | Receives events     |
    And I send an authenticated API Portal "PUT" request to "/applications/${CTX:appId}" as "developer" with JSON values:
      | displayName | Updated Envelope Application |
      | description | Still receives events        |
    Then the response status code should be 200
    When I send an authenticated API Portal "DELETE" request to "/applications/${CTX:appId}" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "application.deleted" containing "${CTX:appId}"
    Then the JSON response field "event_type" should be "application.deleted"
    And the JSON response should have field "event_id"
    And the JSON response should have field "occurred_at"
    And the JSON response should have field "org.ref_id"
    And the JSON response array field "encrypted_fields" should have 0 items
    And the JSON response field "data.handle" should be "${CTX:appId}"
    And the API Portal webhook deliveries containing "${CTX:appId}" should be 3, each carrying its event headers and envelope

    When a Portal API key for API "${CTX:apiId}" is generated, the value is stored as "keyValue" and the handle is stored as "keyId"
    And I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/apis/${CTX:apiId}/api-keys/regenerate" as "publisher" with JSON values:
      | keyId | ${CTX:keyId} |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "apikey.regenerated" containing "${CTX:keyId}"
    Then the JSON response field "data.handle" should be "${CTX:keyId}"
    And the JSON response should have field "occurred_at"
    And the API Portal webhook deliveries containing "${CTX:keyId}" should be 3, each carrying its event headers and envelope
