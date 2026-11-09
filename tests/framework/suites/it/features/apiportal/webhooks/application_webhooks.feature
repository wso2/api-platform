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


@APIPORTAL @apiportal-application-webhook
Feature: Application changes are delivered to the webhook subscribers that ask for them
  As a system that mirrors the API Portal's applications
  I want a signed event for every application created, changed or deleted, and only those I subscribed to
  So that my copy of each developer's applications never drifts from the portal's

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-WEBHOOKS-009..010, at the API level. Each
  # subscriber points at this scenario's partition of the testbench webhook sink. A subscriber that is
  # not sent an event is checked through its own delivery list, beside a subscriber that is sent it.

  @APIP-WEBHOOKS-009
  Scenario: An application's whole life is delivered to a subscriber
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

  @APIP-WEBHOOKS-010
  Scenario: A subscriber is not sent application events it did not ask for
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
