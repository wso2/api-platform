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

@subscription-plan-management-api
Feature: Subscription plan management REST API
  As an operator managing subscription plans through the gateway management API
  I want to clear a plan's optional settings and get clear errors for conflicting changes
  So that plans can be changed in place without deleting and recreating them

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Clearing a plan's throttle limit makes its subscriptions unlimited, and setting one throttles again
    When I create a subscription plan via the management API with body:
      """
      {"planName": "PlanMgmtLimit", "billingPlan": "COMMERCIAL", "throttleLimitCount": 3, "throttleLimitUnit": "Min", "expiryTime": "2099-12-31T23:59:59Z"}
      """
    Then the response status should be 201
    And I store the last response field "id" as "limitPlanId"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: plan-mgmt-limit-api
      spec:
        displayName: Plan Management Limit API
        version: v1.0
        context: /plan-mgmt-limit/$version
        subscriptionPlans:
          - PlanMgmtLimit
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/plan-mgmt-limit/v1.0/health" to return 403

    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-plan-limit-sub", "apiId": "plan-mgmt-limit-api", "subscriptionPlanId": "${limitPlanId}", "subscriptionToken": "it-plan-limit-token-0123456789abcdef01"}
      """
    Then the response status should be 201

    # 3 requests per minute: the 4th is throttled.
    When I send a GET request to "http://localhost:8080/plan-mgmt-limit/v1.0/health" with header "Subscription-Key" value "it-plan-limit-token-0123456789abcdef01"
    Then the response status code should be 200
    When I send 3 GET requests to "http://localhost:8080/plan-mgmt-limit/v1.0/health" with header "Subscription-Key" value "it-plan-limit-token-0123456789abcdef01"
    Then the response status code should be 429

    # Clearing only one half of the limit is rejected and changes nothing.
    When I update the subscription plan with stored id "limitPlanId" with body:
      """
      {"throttleLimitCount": null}
      """
    Then the response status should be 400
    And the JSON response field "message" should be "throttleLimitCount and throttleLimitUnit must be set or cleared together"
    When I send a GET request to the management API at "/subscription-plans/${limitPlanId}"
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 3
    And the JSON response field "throttleLimitUnit" should be "Min"

    # Clear the limit, billing plan and expiry with explicit nulls; other fields stay.
    When I update the subscription plan with stored id "limitPlanId" with body:
      """
      {"throttleLimitCount": null, "throttleLimitUnit": null, "billingPlan": null, "expiryTime": null}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should not exist
    And the JSON response field "throttleLimitUnit" should not exist
    And the JSON response field "billingPlan" should not exist
    And the JSON response field "expiryTime" should not exist
    And the JSON response field "planName" should be "PlanMgmtLimit"
    And the JSON response field "status" should be "ACTIVE"
    When I send a GET request to the management API at "/subscription-plans/${limitPlanId}"
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should not exist
    And the JSON response field "billingPlan" should not exist

    # The same subscription is no longer rate limited.
    When I send 5 GET requests to "http://localhost:8080/plan-mgmt-limit/v1.0/health" with header "Subscription-Key" value "it-plan-limit-token-0123456789abcdef01"
    Then the response status code should be 200

    # Setting a limit again throttles the same subscription.
    When I update the subscription plan with stored id "limitPlanId" with body:
      """
      {"throttleLimitCount": 2, "throttleLimitUnit": "Min"}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 2
    When I send 3 GET requests to "http://localhost:8080/plan-mgmt-limit/v1.0/health" with header "Subscription-Key" value "it-plan-limit-token-0123456789abcdef01"
    Then the response status code should be 429

    # Cleanup
    When I send a DELETE request to the management API at "/subscriptions/it-plan-limit-sub"
    Then the response status should be 204
    When I delete the API "plan-mgmt-limit-api"
    Then the response should be successful
    When I delete the subscription plan with stored id "limitPlanId"
    Then the response status should be 204


  Scenario: Renaming to a taken name and deleting a plan in use are rejected with 409
    When I create a subscription plan "PlanMgmtGold" with 100 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "goldPlanId"
    When I create a subscription plan "PlanMgmtSilver" with 50 requests per minute
    Then the response status should be 201
    And I store the last response field "id" as "silverPlanId"

    # Renaming Silver to Gold collides with the existing plan name.
    When I update the subscription plan with stored id "silverPlanId" with body:
      """
      {"planName": "PlanMgmtGold", "throttleLimitCount": 1, "throttleLimitUnit": "Min"}
      """
    Then the response status should be 409
    And the JSON response field "message" should be "A subscription plan with this name already exists"
    When I send a GET request to the management API at "/subscription-plans/${silverPlanId}"
    Then the response status should be 200
    And the JSON response field "planName" should be "PlanMgmtSilver"
    And the JSON response field "throttleLimitCount" should be 50

    # Renaming to an unused name works.
    When I update the subscription plan with stored id "silverPlanId" with body:
      """
      {"planName": "PlanMgmtBronze"}
      """
    Then the response status should be 200
    And the JSON response field "planName" should be "PlanMgmtBronze"

    # A plan that a subscription still uses can't be deleted.
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: plan-mgmt-inuse-api
      spec:
        displayName: Plan Management In Use API
        version: v1.0
        context: /plan-mgmt-inuse/$version
        subscriptionPlans:
          - PlanMgmtGold
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /health
            policies:
              - name: subscription-validation
                version: v1
                params:
                  subscriptionKeyHeader: "Subscription-Key"
      """
    Then the response should be successful
    When I create a subscription via the management API with body:
      """
      {"externalRefId": "it-plan-inuse-sub", "apiId": "plan-mgmt-inuse-api", "subscriptionPlanId": "${goldPlanId}", "subscriptionToken": "it-plan-inuse-token-0123456789abcdef01"}
      """
    Then the response status should be 201
    When I send a DELETE request to the management API at "/subscription-plans/${goldPlanId}"
    Then the response status should be 409
    And the JSON response field "message" should be "Subscription plan is in use by one or more subscriptions"
    When I send a GET request to the management API at "/subscription-plans/${goldPlanId}"
    Then the response status should be 200

    # Once the subscription is gone, the plan can be deleted.
    When I send a DELETE request to the management API at "/subscriptions/it-plan-inuse-sub"
    Then the response status should be 204
    When I send a DELETE request to the management API at "/subscription-plans/${goldPlanId}"
    Then the response status should be 204
    When I send a GET request to the management API at "/subscription-plans/${goldPlanId}"
    Then the response status should be 404

    # Cleanup
    When I delete the API "plan-mgmt-inuse-api"
    Then the response should be successful
    When I delete the subscription plan with stored id "silverPlanId"
    Then the response status should be 204
