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

@subscription-plan-management
Feature: Subscription plan management
  As an operator managing subscription plans through the gateway management API
  I want to clear a plan's optional settings and get clear errors for conflicting changes
  So that plans can be changed in place without deleting and recreating them

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Clearing a plan's throttle limit makes its subscriptions unlimited, and setting one throttles again
    Given I generate a unique value from "plan-mgmt-limit" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","billingPlan":"COMMERCIAL","throttleLimitCount":3,"throttleLimitUnit":"Min","expiryTime":"2099-12-31T23:59:59Z"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup

    Given I generate a unique value from "plan-mgmt-limit" and store it as "apiName"
    And I generate a unique API context from "/plan-mgmt-limit" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.subscriptionPlans | ["${CTX:planName}"]        |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    Given I generate a unique value from "plan-mgmt-limit" and store it as "subRef"
    And I generate a unique value from "plan-mgmt-limit-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:planId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup

    # 3 requests per minute: the 4th is throttled.
    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 3 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 429

    # Clearing only one half of the limit is rejected and changes nothing.
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"throttleLimitCount":null}
      """
    Then the response status should be 400
    And the JSON response field "message" should be "throttleLimitCount and throttleLimitUnit must be set or cleared together"
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 3
    And the JSON response field "throttleLimitUnit" should be "Min"

    # Clear the limit, billing plan and expiry with explicit nulls; other fields stay.
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"throttleLimitCount":null,"throttleLimitUnit":null,"billingPlan":null,"expiryTime":null}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should not exist
    And the JSON response field "throttleLimitUnit" should not exist
    And the JSON response field "billingPlan" should not exist
    And the JSON response field "expiryTime" should not exist
    And the JSON response field "planName" should be "${CTX:planName}"
    And the JSON response field "status" should be "ACTIVE"
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should not exist
    And the JSON response field "billingPlan" should not exist

    # The throttled subscription is no longer rate limited.
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 5 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 200

    # Setting a limit again throttles the same subscription.
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"throttleLimitCount":2,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 2
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 429

  Scenario: Renaming to a taken name and deleting a plan in use are rejected with 409
    Given I generate a unique value from "plan-mgmt-gold" and store it as "goldPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:goldPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "goldPlanId"
    And I register the "subscription-plan" "${CTX:goldPlanId}" for cleanup
    Given I generate a unique value from "plan-mgmt-silver" and store it as "silverPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:silverPlanName}","throttleLimitCount":50,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "silverPlanId"
    And I register the "subscription-plan" "${CTX:silverPlanId}" for cleanup

    # Renaming Silver to Gold collides with the existing plan name and changes nothing.
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:silverPlanId}" with body:
      """
      {"planName":"${CTX:goldPlanName}","throttleLimitCount":1,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "A subscription plan with this name already exists"
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:silverPlanId}"
    Then the response status should be 200
    And the JSON response field "planName" should be "${CTX:silverPlanName}"
    And the JSON response field "throttleLimitCount" should be 50

    # Renaming to an unused name works.
    Given I generate a unique value from "plan-mgmt-bronze" and store it as "bronzePlanName"
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:silverPlanId}" with body:
      """
      {"planName":"${CTX:bronzePlanName}"}
      """
    Then the response status should be 200
    And the JSON response field "planName" should be "${CTX:bronzePlanName}"

    # A plan that a subscription still uses can't be deleted.
    Given I generate a unique value from "plan-mgmt-inuse" and store it as "apiName"
    And I generate a unique API context from "/plan-mgmt-inuse" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.subscriptionPlans | ["${CTX:goldPlanName}"]    |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    Given I generate a unique value from "plan-mgmt-inuse" and store it as "subRef"
    And I generate a unique value from "plan-mgmt-inuse-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:goldPlanId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/subscription-plans/${CTX:goldPlanId}"
    Then the response status should be 409
    And the JSON response field "message" should be "Subscription plan is in use by one or more subscriptions"
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:goldPlanId}"
    Then the response status should be 200

    # Once the subscription is gone, the plan can be deleted.
    When I send a "DELETE" request to the "gateway-controller" service at "/subscriptions/${CTX:subRef}"
    Then the response status should be 204
    When I send a "DELETE" request to the "gateway-controller" service at "/subscription-plans/${CTX:goldPlanId}"
    Then the response status should be 204
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:goldPlanId}"
    Then the response status should be 404
