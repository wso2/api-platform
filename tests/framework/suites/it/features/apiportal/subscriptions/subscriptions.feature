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


@APIPORTAL @apiportal-subscription
Feature: Developers subscribe to APIs through the API Portal REST API
  As an API developer
  I want to subscribe to an API under one of its plans, switch, suspend, rotate and end it
  So that I hold exactly one working subscription token per API

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API
  # level. The fixture API offers the Gold and Silver plans. Every refused change is followed by a
  # read that shows the subscription is unchanged. Subscriptions belong to the user who created them,
  # so the administrator, who holds every subscription scope, is the second user of the ownership
  # scenario.

  @APIP-024 @type:negative @type:smoke
  Scenario: A developer subscribes, changes plan and unsubscribes, and invalid changes leave the subscription as it was
    # A developer manages a subscription from subscribing to unsubscribing
    Given a REST API is created in the API Portal and stored as "apiId"
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    And the JSON response field "artifactId" should be "${CTX:apiId}"
    And the JSON response field "subscriptionPlanName" should be "Gold"
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response should have field "subscriptionToken"
    And I store the JSON response field "subscriptionToken" as "originalToken"

    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | planId | Silver |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the JSON response field "subscriptionPlanName" should be "Silver"
    And the JSON response field "subscriptionToken" should be "${CTX:originalToken}"

    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | INACTIVE |
    Then the response status code should be 200
    And the JSON response field "status" should be "INACTIVE"
    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | ACTIVE |
    Then the response status code should be 200
    And the JSON response field "status" should be "ACTIVE"

    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionToken" should not equal "${CTX:originalToken}"
    And I store the JSON response field "subscriptionToken" as "rotatedToken"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the JSON response field "subscriptionToken" should be "${CTX:rotatedToken}"

    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 404

    # An invalid change to a subscription is refused and the subscription is unchanged
    Given a REST API is created in the API Portal and stored as "apiId"
    Given I generate a unique resource name from "portal-missing-subscription" and store it as "missingSubscriptionId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | planId | DoesNotExist |
    Then the response status code should be 400
    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "developer" with JSON values:
      | status | PAUSED |
    Then the response status code should be 400
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlanName" should be "Gold"
    And the JSON response field "status" should be "ACTIVE"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:missingSubscriptionId}" as "developer"
    Then the response status code should be 404

  @APIP-025 @type:negative @type:security
  Scenario: Subscriptions follow the plans an API offers and belong to their owner
    # A subscription to a plan or API that is not offered is refused
    Given a REST API is created in the API Portal and stored as "apiId"
    Given I generate a unique resource name from "portal-missing-api" and store it as "missingApiId"
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON values:
      | artifactId         | ${CTX:apiId} |
      | subscriptionPlanId | Bronze       |
    Then the response status code should be 400
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON values:
      | artifactId         | ${CTX:missingApiId} |
      | subscriptionPlanId | Gold                |
    Then the response status code should be 404
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON values:
      | artifactId         | ${CTX:apiId} |
      | subscriptionPlanId | Silver       |
    Then the response status code should be 409

    # A subscription without a plan or to an API without plans is refused, and a plan change without a plan changes nothing
    Given a REST API is created in the API Portal and stored as "apiId"
    Given a REST API is created in the API Portal and stored as "planlessApiId"
    When I update the API Portal API "${CTX:planlessApiId}" from "resources/templates/api-portal-api.yaml" as "publisher" with definition "none" with values:
      | id                | ${CTX:planlessApiId} |
      | name              | ${CTX:planlessApiId} |
      | subscriptionPlans | []                   |
    Then the response status code should be 200
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON values:
      | artifactId | ${CTX:apiId} |
    Then the response status code should be 400
    And the response body should contain "subscriptionPlanId"
    And the response body should not contain "stack"
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON values:
      | artifactId         | ${CTX:planlessApiId} |
      | subscriptionPlanId | Gold                 |
    Then the response status code should be 400
    And the response body should contain "This API does not support subscriptions"
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:planlessApiId}" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:apiId}" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 0 items

    Given a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON body:
      """
      {}
      """
    Then the response status code should be 400
    And the response body should contain "planId"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON values:
      | artifactId | ${CTX:planlessApiId} |
      | planId     | Silver               |
    Then the response status code should be 400
    And the response body should contain "artifactId does not match this subscription"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlanName" should be "Gold"
    And the JSON response field "status" should be "ACTIVE"

    # Subscriptions are listed per API, and an unknown API is not found
    Given a REST API is created in the API Portal and stored as "apiId"
    Given a REST API is created in the API Portal and stored as "otherApiId"
    And I generate a unique resource name from "portal-unknown-api" and store it as "unknownApiId"
    And a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    And a developer subscription for API "${CTX:otherApiId}" using plan "Silver" is created and stored as "otherSubscriptionId"
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:apiId}" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "subscriptionId" equal to "${CTX:subscriptionId}"
    And the JSON response array "list" should not contain an item with "subscriptionId" equal to "${CTX:otherSubscriptionId}"
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:otherApiId}" as "developer"
    Then the response status code should be 200
    And the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "subscriptionId" equal to "${CTX:otherSubscriptionId}"
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:unknownApiId}" as "developer"
    Then the response status code should be 404
    And the JSON response field "code" should be "RESOURCE_NOT_FOUND"

    # Another user can neither see nor change a developer's subscription
    Given a REST API is created in the API Portal and stored as "apiId"
    Given a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    And I store the JSON response field "subscriptionToken" as "originalToken"
    When I send an authenticated API Portal "GET" request to "/subscriptions?artifactId=${CTX:apiId}" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "subscriptionId" equal to "${CTX:subscriptionId}"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "PUT" request to "/subscriptions/${CTX:subscriptionId}" as "admin" with JSON values:
      | status | INACTIVE |
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "admin" with JSON values:
      | planId | Silver |
    Then the response status code should be 404
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "admin"
    Then the response status code should be 404
    And the response body should not contain "${CTX:originalToken}"
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "admin"
    Then the response status code should be 404
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlanName" should be "Gold"
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response field "subscriptionToken" should be "${CTX:originalToken}"

    # A publisher without the subscription manage scope cannot rotate a subscription token
    # No actor of this block holds dp:subscription:update without
    # dp:subscription:manage, so the refusal is shown for the publisher, which holds only
    # dp:subscription:read.
    Given a REST API is created in the API Portal and stored as "apiId"
    Given a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    And I store the JSON response field "subscriptionToken" as "originalToken"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "publisher"
    Then the response status code should be 403
    And the response body should not contain "${CTX:originalToken}"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the JSON response field "subscriptionToken" should be "${CTX:originalToken}"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionToken" should not equal "${CTX:originalToken}"
