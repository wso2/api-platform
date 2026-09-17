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

Feature: API Portal subscriptions

  Background:
    Given a REST API is created in the API Portal and stored as "apiId"

  Scenario: A developer creates and retrieves an API subscription
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    And the JSON response field "artifactId" should be "${CTX:apiId}"
    And the JSON response field "subscriptionPlanName" should be "Gold"
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response should have field "subscriptionToken"
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "artifactId" should be "${CTX:apiId}"

  Scenario: A developer changes an API subscription plan
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON body:
      """
      {"planId":"Silver"}
      """
    Then the response status code should be 200
    And the JSON response field "subscriptionPlanName" should be "Silver"

  Scenario: A developer regenerates an API subscription token
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    And I store the JSON response field "subscriptionToken" as "oldToken"
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/regenerate-token" as "developer" with JSON body:
      """
      {}
      """
    Then the response status code should be 200
    And the JSON response should have field "subscriptionToken"

  Scenario: A developer deletes an API subscription
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 404

  Scenario: A developer cannot subscribe an API to an unlinked plan
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiId}","subscriptionPlanId":"Bronze"}
      """
    Then the response status code should be 400

  Scenario: A developer cannot subscribe to a missing API
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON body:
      """
      {"artifactId":"does-not-exist","subscriptionPlanId":"Gold"}
      """
    Then the response status code should be 404

  Scenario: A developer cannot change to a missing subscription plan
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201
    When I send an authenticated API Portal "POST" request to "/subscriptions/${CTX:subscriptionId}/change-plan" as "developer" with JSON body:
      """
      {"planId":"DoesNotExist"}
      """
    Then the response status code should be 400
