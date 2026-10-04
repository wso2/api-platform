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

Feature: API Portal subscription webhook events

  Scenario: Subscription creation is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-subscription-created-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Subscription Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["subscription.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    When I wait for an API Portal webhook event "subscription.created" containing "${CTX:subscriptionId}"
    Then the JSON response field "event_type" should be "subscription.created"
    And the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"
    And the API Portal delivery for subscriber "${CTX:subscriberId}" and event "subscription.created" should be "DELIVERED"

  Scenario: Subscription plan changes are delivered to a matching subscriber
    Given I generate a unique resource name from "portal-subscription-plan-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Subscription Plan Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["subscription.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON body:
      """
      {"planId":"Silver"}
      """
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.plan_changed" containing "${CTX:subscriptionId}"
    Then the JSON response field "event_type" should be "subscription.plan_changed"
    And the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"

  Scenario: An invalid subscription plan change does not deliver an event
    Given I generate a unique resource name from "portal-subscription-invalid-plan-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Rejected Subscription Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["subscription.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    And I reset API Portal webhook events
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON body:
      """
      {"planId":"DoesNotExist"}
      """
    Then the response status code should be 400
    And I verify no API Portal webhook event "subscription.plan_changed" containing "${CTX:subscriptionId}" is delivered

  Scenario: Subscription token regeneration is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-subscription-token-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Subscription Token Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["subscription.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer" with JSON body:
      """
      {}
      """
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.token_regenerated" containing "${CTX:subscriptionId}"
    Then the JSON response field "event_type" should be "subscription.token_regenerated"
    And the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"

  Scenario: Subscription deletion is delivered to a matching subscriber
    Given I generate a unique resource name from "portal-subscription-deleted-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Subscription Deletion Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","events":["subscription.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription.deleted" containing "${CTX:subscriptionId}"
    Then the JSON response field "event_type" should be "subscription.deleted"
    And the JSON response field "data.subscription_id" should be "${CTX:subscriptionId}"

  Scenario: A subscription token is encrypted for a subscriber with a secret
    Given I generate a unique resource name from "portal-subscription-encrypted-subscriber" and store it as "subscriberId"
    And I generate a unique resource name from "portal-subscription-webhook-secret" and store it as "secretValue"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with body and stored as "subscriberId":
      """
      {"id":"${CTX:subscriberId}","displayName":"Encrypted Subscription Events","targetUrl":"http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook","secret":"${CTX:secretValue}","events":["subscription.*"]}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer" with JSON body:
      """
      {}
      """
    Then the response status code should be 200
    And I store the JSON response field "subscriptionToken" as "regeneratedToken"
    When I wait for an API Portal webhook event "subscription.token_regenerated" containing "${CTX:subscriptionId}"
    Then the JSON response field "event_type" should be "subscription.token_regenerated"
    And the JSON response array field "encrypted_fields" should have 1 item
    And the response body should contain "ciphertext"
    And the JSON response should have field "data.token"
    And the API Portal webhook encrypted field "data.token" should decrypt to "${CTX:regeneratedToken}" using secret "${CTX:secretValue}"
