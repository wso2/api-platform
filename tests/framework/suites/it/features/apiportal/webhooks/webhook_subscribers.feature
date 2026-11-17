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


@APIPORTAL @apiportal-webhook-subscriber
Feature: Webhook subscribers are managed through the API Portal REST API
  As an API Portal administrator
  I want to register, inspect, change and remove the endpoints the portal sends its events to
  So that only the systems I trust receive credential and subscription changes, signed with their secret

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level.
  # Webhook subscribers are administrator-only and organization-wide, which is why this block runs
  # one runner at a time. A subscriber's secret is write-only: responses report hasSecret, never the
  # secret. Every refusal is followed by a read showing nothing was stored or changed.

  @APIP-050
  Scenario: An administrator manages webhook subscribers, and only enabled ones are sent events
    # An administrator manages a webhook subscriber from creation to deletion
    Given I generate a unique resource name from "portal-subscriber" and store it as "subscriberId"
    And I generate a unique value from "portal-subscriber-secret" and store it as "secretValue"
    When a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId}           |
      | displayName | Credential Subscriber          |
      | targetUrl   | https://example.invalid/webhook |
      | secret      | ${CTX:secretValue}            |
      | events      | ["apikey.*"]                   |
    Then the response status code should be 201
    And the JSON response field "hasSecret" should be true
    And the JSON response field "enabled" should be true
    And the JSON response field "timeoutMs" should be 5000
    And the JSON response field "events[0]" should be "apikey.*"
    And the JSON response field "secret" should not exist
    And the response body should not contain "${CTX:secretValue}"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "targetUrl" should be "https://example.invalid/webhook"
    And the response body should not contain "${CTX:secretValue}"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:subscriberId}"

    # An update that sends no secret keeps the stored one.
    When I send an authenticated API Portal "PUT" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin" with JSON values:
      | displayName | Credential Subscriber                   |
      | targetUrl   | https://updated.example.invalid/webhook |
      | events      | ["subscription.*"]                      |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the JSON response field "targetUrl" should be "https://updated.example.invalid/webhook"
    And the JSON response field "events[0]" should be "subscription.*"
    And the JSON response field "hasSecret" should be true

    # The kept secret is the original one: a later delivery is signed with it.
    When I send an authenticated API Portal "PUT" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin" with JSON values:
      | displayName | Credential Subscriber                                         |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["apikey.*"]                                                  |
    Then the response status code should be 200
    Given a REST API is created in the API Portal and stored as "apiId"
    And a "developer" API key for API "${CTX:apiId}" is generated and stored as "keyId"
    When I wait for an API Portal webhook event "apikey.generated" containing "${CTX:keyId}"
    Then the API Portal webhook delivery "apikey.generated" containing "${CTX:keyId}" should have a valid signature using secret "${CTX:secretValue}"

    When I send an authenticated API Portal "DELETE" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "DELETE" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404

    # Subscribers created without an id each get their own, and no secret
    Given I generate a unique value from "Generated subscriber" and store it as "displayName"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "firstSubscriberId":
      | displayName | ${CTX:displayName}            |
      | targetUrl   | https://example.invalid/webhook |
      | events      | ["meter.*"]                    |
    Then the response status code should be 201
    And the JSON response field "hasSecret" should be false
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "secondSubscriberId":
      | displayName | ${CTX:displayName}            |
      | targetUrl   | https://example.invalid/webhook |
      | events      | ["meter.*"]                    |
    Then the response status code should be 201
    And the JSON response field "id" should not equal "${CTX:firstSubscriberId}"

    # A disabled subscriber is sent nothing while an enabled one is sent the event
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

  @APIP-050 @type:security
  Scenario: Invalid subscribers are refused, and only an administrator can see or register one
    # A subscriber without a target URL is refused and stores nothing
    Given I generate a unique resource name from "portal-refused-subscriber" and store it as "subscriberId"
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "admin" with JSON values:
      | id       | ${CTX:subscriberId} |
      | displayName  | Refused Subscriber       |
      | events | ["meter.*"]      |
      | secret  | not-a-real-secret       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404

    # A subscriber without a display name is refused and stores nothing
    Given I generate a unique resource name from "portal-refused-subscriber" and store it as "subscriberId"
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "admin" with JSON values:
      | id       | ${CTX:subscriberId} |
      | targetUrl  | https://example.invalid/webhook       |
      | events | ["meter.*"]      |
      | secret  | not-a-real-secret       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404

    # A subscriber whose target is not a URL is refused and stores nothing
    Given I generate a unique resource name from "portal-refused-subscriber" and store it as "subscriberId"
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "admin" with JSON values:
      | id       | ${CTX:subscriberId} |
      | displayName  | Refused Subscriber       |
      | targetUrl | not a url      |
      | events  | ["meter.*"]       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404

    # A subscriber with a field the portal lacks is refused and stores nothing
    Given I generate a unique resource name from "portal-refused-subscriber" and store it as "subscriberId"
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "admin" with JSON values:
      | id       | ${CTX:subscriberId} |
      | displayName  | Refused Subscriber       |
      | targetUrl | https://example.invalid/webhook      |
      | colour  | red       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404

    # A second subscriber with an existing id is refused
    Given I generate a unique resource name from "portal-duplicate-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId}           |
      | displayName | Original Subscriber            |
      | targetUrl   | https://example.invalid/webhook |
      | events      | ["meter.*"]                    |
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "admin" with JSON values:
      | id          | ${CTX:subscriberId}           |
      | displayName | Duplicate Subscriber           |
      | targetUrl   | https://example.invalid/webhook |
    Then the response status code should be 409
    And the response body should not contain "constraint"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the JSON response field "displayName" should be "Original Subscriber"

    # Only an administrator can see or register webhook subscribers
    Given I generate a unique resource name from "portal-denied-subscriber" and store it as "subscriberId"
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers" as "publisher"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers" as "developer"
    Then the response status code should be 403
    When I send an unauthenticated API Portal "GET" request to "/webhook-subscribers"
    Then the response status code should be 401
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "publisher" with JSON values:
      | id          | ${CTX:subscriberId}           |
      | displayName | Publisher Subscriber           |
      | targetUrl   | https://example.invalid/webhook |
    Then the response status code should be 403
    When I send an authenticated API Portal "POST" request to "/webhook-subscribers" as "developer" with JSON values:
      | id          | ${CTX:subscriberId}           |
      | displayName | Developer Subscriber           |
      | targetUrl   | https://example.invalid/webhook |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/webhook-subscribers/${CTX:subscriberId}" as "admin"
    Then the response status code should be 404
