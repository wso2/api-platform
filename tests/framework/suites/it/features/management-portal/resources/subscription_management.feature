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

@platform-api-subscription-management
Feature: Subscription plans, applications and subscriptions are managed through the control plane
  As an API platform administrator and an API consumer
  I want plans to keep their limits, applications to be found by handle and subscriptions to
  record who changed them
  So that the commercial model behind an API is consistent on every supported database

  Background:
    Given I generate a unique value from "subs" and store it as "word"

  Scenario: A subscription plan keeps its limits through list, update and duplicate attempts
    Given I generate a unique resource name from "plan-a" and store it as "planA"
    And I generate a unique resource name from "plan-b" and store it as "planB"
    And I generate a unique resource name from "plan-c" and store it as "planC"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:planA}", "displayName": "${CTX:word} Plan A", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "limitCount": 10}]}
      """
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/subscription-plans/${CTX:planA}"
    And the JSON response field "createdBy" should be "admin"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:planB}", "displayName": "${CTX:word} Plan B", "status": "ACTIVE", "limits": [{"limitType": "REQUEST_COUNT", "timeUnit": "HOUR", "limitCount": 100}]}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:planC}", "displayName": "${CTX:word} Plan C", "status": "ACTIVE"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id": "${CTX:planA}", "displayName": "${CTX:word} Duplicate", "status": "ACTIVE"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_EXISTS"
    When I send a "GET" request to the control plane at "/subscription-plans?query=${CTX:word}&limit=2"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    And the JSON response field "pagination.total" should be 3
    When I send a "GET" request to the control plane at "/subscription-plans?query=${CTX:word}&limit=100"
    Then the JSON response array field "list" should have 3 items
    And the JSON response array "list" item with "id" equal to "${CTX:planA}" should have "limits[0].limitCount" equal to "10"
    And the JSON response array "list" item with "id" equal to "${CTX:planA}" should have "limits[0].timeUnit" equal to "MINUTE"
    And the JSON response array "list" item with "id" equal to "${CTX:planB}" should have "limits[0].timeUnit" equal to "HOUR"
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planA}" with body:
      """
      {"id": "${CTX:planA}", "displayName": "${CTX:word} Plan A", "status": "ACTIVE", "limits": []}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planA}"
    Then the response status code should be 200
    And the JSON response array field "limits" should have 0 items

  Scenario: An application is found by its handle and an unknown one is not found
    Given I generate a unique resource name from "subs-proj" and store it as "project"
    And I generate a unique resource name from "subs-app" and store it as "application"
    And I create a project "${CTX:project}" on the control plane
    When I send a "POST" request to the control plane at "/applications" as "developer" with body:
      """
      {"id": "${CTX:application}", "displayName": "${CTX:application}", "projectId": "${CTX:project}", "type": "genai"}
      """
    Then the response status code should be 201
    And the JSON response field "createdBy" should be "developer"
    When I send a "GET" request to the control plane at "/applications/${CTX:application}"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:application}"
    And the JSON response field "projectId" should be "${CTX:project}"
    When I send a "GET" request to the control plane at "/applications/${CTX:application}-missing"
    Then the response status code should be 404
    And the JSON response field "code" should be "APPLICATION_NOT_FOUND"

  Scenario: Subscriptions record who changed them and are filtered by status
    Given I generate a unique resource name from "subs-proj" and store it as "project"
    And I generate a unique resource name from "subs-plan" and store it as "plan"
    And I generate a unique resource name from "subs-api" and store it as "apiName"
    And I generate a unique API context from "/subs-api" and store it as "apiContext"
    And I generate a unique resource name from "subs-app-a" and store it as "appA"
    And I generate a unique resource name from "subs-app-b" and store it as "appB"
    And I create a project "${CTX:project}" on the control plane
    And I create a subscription plan "${CTX:plan}" allowing 100 requests per hour via the control plane
    When I send a "POST" request to the control plane at "/rest-apis" with body:
      """
      {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}", "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://backend.example.com"}}, "subscriptionPlans": ["${CTX:plan}"]}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "api"
    When I send a "POST" request to the control plane at "/applications" as "developer" with body:
      """
      {"id": "${CTX:appA}", "displayName": "${CTX:appA}", "projectId": "${CTX:project}", "type": "genai"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/applications" as "developer" with body:
      """
      {"id": "${CTX:appB}", "displayName": "${CTX:appB}", "projectId": "${CTX:project}", "type": "genai"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscriptions" as "developer" with body:
      """
      {"artifactId": "${CTX:api}", "kind": "RestApi", "subscriberId": "${CTX:appA}-subscriber", "applicationId": "${CTX:appA}", "subscriptionPlanId": "${CTX:plan}"}
      """
    Then the response status code should be 201
    And the JSON response field "status" should be "ACTIVE"
    And the JSON response field "createdBy" should be "developer"
    And the JSON response field "updatedBy" should be "developer"
    And I store the JSON response field "id" as "revokedSubscription"
    When I send a "POST" request to the control plane at "/subscriptions" as "developer" with body:
      """
      {"artifactId": "${CTX:api}", "kind": "RestApi", "subscriberId": "${CTX:appB}-subscriber", "applicationId": "${CTX:appB}", "subscriptionPlanId": "${CTX:plan}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "activeSubscription"
    When I send a "PUT" request to the control plane at "/subscriptions/${CTX:revokedSubscription}?subscriberId=${CTX:appA}-subscriber" with body:
      """
      {"status": "REVOKED"}
      """
    Then the response status code should be 200
    And the JSON response field "status" should be "REVOKED"
    And the JSON response field "createdBy" should be "developer"
    And the JSON response field "updatedBy" should be "admin"
    When I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:api}"
    Then the response status code should be 200
    And the JSON response array field "list" should have 2 items
    When I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:api}&status=ACTIVE"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:activeSubscription}"
    When I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:api}&status=REVOKED"
    Then the JSON response array field "list" should have 1 item
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:revokedSubscription}"
    When I send a "GET" request to the control plane at "/subscriptions?artifactId=${CTX:api}&status=UNKNOWN"
    Then the response status code should be 400
