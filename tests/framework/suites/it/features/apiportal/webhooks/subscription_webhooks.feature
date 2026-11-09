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


@APIPORTAL @apiportal-subscription-webhook
Feature: Subscription changes are delivered to webhook subscribers, with the token only ever encrypted
  As the control plane that enforces the API Portal's subscriptions on gateways
  I want an event for every subscription created, switched, suspended, rotated or ended
  So that gateways honour exactly the subscriptions and tokens the portal holds

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-WEBHOOKS-014..016, at the API level. The
  # fixture API offers the Gold and Silver plans. A subscriber without a secret is sent no token; one
  # with a secret is sent it encrypted. A refused change sends no event and changes nothing.

  Background:
    Given a REST API is created in the API Portal and stored as "apiId"

  @APIP-WEBHOOKS-014
  Scenario: A subscription's whole life is delivered, without its token, to a subscriber that has no secret
    Given I generate a unique resource name from "portal-subscription-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["subscription.*"] |
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I wait for an API Portal webhook event "subscription.created" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"
    And the JSON response array field "encrypted_fields" should have 0 items
    And the JSON response field "data.token" should not exist
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "subscription.created" should be "DELIVERED"

    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | planId | Silver |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.plan_changed" containing "${CTX:subscriptionId}"
    Then the JSON response should have field "data.previous_plan"
    And the JSON response should have field "data.subscription_plan"

    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | INACTIVE |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.updated" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.status" should be "INACTIVE"

    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.token_regenerated" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.token" should not exist

    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.deleted" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"

  @APIP-WEBHOOKS-015 @type:negative
  Scenario: A refused plan change sends no event and leaves the subscription unchanged
    Given I generate a unique resource name from "portal-refused-change-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["subscription.*"] |
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    And I wait for an API Portal webhook event "subscription.created" containing "${CTX:subscriptionId}"
    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | planId | DoesNotExist |
    Then the response status code should be 400
    And I verify no API Portal webhook event "subscription.plan_changed" containing "${CTX:subscriptionId}" is delivered
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the JSON response field "subscriptionPlanName" should be "Gold"

  @APIP-WEBHOOKS-016 @type:security
  Scenario: A subscriber with a secret is sent the rotated token encrypted with it, never in plain text
    Given I generate a unique resource name from "portal-encrypted-token-subscriber" and store it as "subscriberId"
    And I generate a unique value from "portal-token-secret" and store it as "secretValue"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["subscription.*"] |
      | secret      | ${CTX:secretValue} |
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer"
    Then the response status code should be 200
    And I store the JSON response field "subscriptionToken" as "rotatedToken"
    When I wait for an API Portal webhook event "subscription.token_regenerated" containing "${CTX:subscriptionId}"
    Then the JSON response array field "encrypted_fields" should have 1 item
    And the JSON response field "encrypted_fields[0]" should be "token"
    And the response body should not contain "${CTX:rotatedToken}"
    And the API Portal webhook encrypted field "data.token" should decrypt to "${CTX:rotatedToken}" using secret "${CTX:secretValue}"

  # APIP-WEBHOOKS-027: a plan change names the plan it left, and resuming a suspended subscription is
  # delivered as an update back to ACTIVE.
  @APIP-WEBHOOKS-027
  Scenario: A resumed subscription is delivered as updated to ACTIVE, and a plan change names the previous plan
    Given I generate a unique resource name from "portal-resume-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId} |
      | displayName | ${CTX:subscriberId} |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["subscription.*"] |
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    And I wait for an API Portal webhook event "subscription.created" containing "${CTX:subscriptionId}"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | planId | Silver |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.plan_changed" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"
    And the JSON response field "data.previous_plan.name" should be "Gold"
    And the JSON response should have field "data.subscription_plan"

    When I reset API Portal webhook events
    And I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | INACTIVE |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.updated" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"
    And the JSON response field "data.status" should be "INACTIVE"
    When I reset API Portal webhook events
    And I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | ACTIVE |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.updated" containing "${CTX:subscriptionId}"
    Then the JSON response field "data.status" should be "ACTIVE"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "subscription.updated" should be "DELIVERED"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the JSON response field "status" should be "ACTIVE"
