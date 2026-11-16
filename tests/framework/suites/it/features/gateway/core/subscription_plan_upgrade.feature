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


@GWPLAN @subscription-plan-upgrade
Feature: Subscription plans stored before plan conditions keep their rate limit in an upgraded gateway
  As an operator upgrading a gateway that holds subscription plans
  I want each plan's throttle limit to stay its rate limit and to read as its request-count condition
  So that subscribers are throttled as before, and a control plane can then send the plan's conditions

  # Scenarios GWPLAN (scenarios/on-prem/GWPLAN.yaml). A plan sent with only throttle limit fields, as
  # control planes before plan conditions send it, is stored with no conditions (NULL), the same
  # row a gateway before plan conditions stored. Restarting gateway-controller loads it again from
  # the database, as an upgraded gateway does. The block runs alone because the scenarios restart
  # gateway-controller.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @GWPLAN-003 @type:smoke
  Scenario: A plan stored without conditions keeps its rate limit across a restart and takes conditions from the control plane
    Given I generate a unique value from "gw-upg-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":3,"throttleLimitUnit":"Min","stopOnQuotaReach":true}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup
    And the gateway database should store subscription plan "${CTX:planName}" without conditions, as gateways before plan conditions stored every plan

    Given I generate a unique value from "gw-upg-api" and store it as "apiName"
    And I generate a unique API context from "/gw-upg-api" and store it as "apiContext"
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
    Given I generate a unique value from "gw-upg-sub" and store it as "subRef"
    And I generate a unique value from "gw-upg-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:planId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup

    # After a restart the plan and its subscription are loaded from the database: still 3 a minute
    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 3
    And the JSON response array field "conditions" should have 1 item
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response field "conditions[0].type" should be "request-count"
    And the JSON response field "conditions[0].params.limits[0].count" should be 3
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "MINUTE"
    And the JSON response field "conditions[0].params.onExceed" should be "reject"
    # Reading the plan as a condition stores nothing new
    And the gateway database should store subscription plan "${CTX:planName}" without conditions, as gateways before plan conditions stored every plan
    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 3 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 429

    # A control plane before plan conditions updates only the throttle limit; the plan stays without conditions
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"throttleLimitCount":5,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 5
    And the gateway database should store subscription plan "${CTX:planName}" without conditions, as gateways before plan conditions stored every plan

    # A control plane with plan conditions sends them; they are stored, and their one-minute window is the limit
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"conditions":[{"type":"request-count","params":{"limits":[{"count":2,"timeUnit":"MINUTE"},{"count":1000,"timeUnit":"DAY"}]}}]}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 2
    And the gateway database should store 1 condition for subscription plan "${CTX:planName}"
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 429

    # The stored conditions are what a restarted gateway loads
    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status should be 200
    And the JSON response array field "conditions[0].params.limits" should have 2 items
    And the JSON response field "throttleLimitCount" should be 2

  @GWPLAN-004
  Scenario: A plan stored without conditions that lets requests past its limit reads as onExceed allow
    Given I generate a unique value from "gw-upg-soft" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":100,"throttleLimitUnit":"Hour","stopOnQuotaReach":false}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup
    And the gateway database should store subscription plan "${CTX:planName}" without conditions, as gateways before plan conditions stored every plan
    When I restart the "gateway-controller" service
    And I wait for the gateway controller health endpoint
    And I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status should be 200
    And the JSON response field "conditions[0].params.limits[0].count" should be 100
    And the JSON response field "conditions[0].params.limits[0].timeUnit" should be "HOUR"
    And the JSON response field "conditions[0].params.onExceed" should be "allow"
    # stopOnQuotaReach alone changes onExceed and keeps the window
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"stopOnQuotaReach":true}
      """
    Then the response status should be 200
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the JSON response field "conditions[0].params.onExceed" should be "reject"
    And the JSON response field "conditions[0].params.limits[0].count" should be 100
