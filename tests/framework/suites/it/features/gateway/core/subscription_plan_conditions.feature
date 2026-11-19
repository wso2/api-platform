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


@GWPLAN @subscription-plan-conditions
Feature: Subscription plan conditions in the gateway
  As a control plane syncing subscription plans to the gateway management API
  I want a plan's conditions to be stored as sent and its request-count condition to set the plan's rate limit
  So that subscription-validation enforces the plan the control plane defined

  # Scenarios GWPLAN (scenarios/on-prem/GWPLAN.yaml). subscription-validation v1 enforces one
  # throttle limit, taken from the request-count condition: its first one-unit window.
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @GWPLAN-001 @type:smoke
  Scenario: A plan's request-count condition rate limits its subscriptions, and changing the conditions changes the limit
    Given I generate a unique value from "gw-cond-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","conditions":[
        {"type":"request-count","params":{"limits":[{"count":1000,"timeAmount":2,"timeUnit":"DAY"},{"count":3,"timeUnit":"MINUTE"}]}},
        {"type":"token-count","params":{"limits":[{"count":10000,"timeUnit":"HOUR"}],"onExceed":"allow"}},
        {"id":"region-limit","type":"custom","params":{"regions":["eu"],"policy":{"name":"region-guard","version":"v1"}}}]}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup
    # The one-minute window is the v1 limit, not the first window
    And the JSON response field "throttleLimitCount" should be 3
    And the JSON response field "throttleLimitUnit" should be "Min"
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status should be 200
    And the JSON response array field "conditions" should have 3 items
    And the JSON response field "conditions[0].id" should be "requests"
    And the JSON response field "conditions[0].params.onExceed" should be "reject"
    And the JSON response field "conditions[1].params.tokenType" should be "total"
    And the JSON response field "conditions[2].params.policy.name" should be "region-guard"

    Given I generate a unique value from "gw-cond-api" and store it as "apiName"
    And I generate a unique API context from "/gw-cond-api" and store it as "apiContext"
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

    Given I generate a unique value from "gw-cond-sub" and store it as "subRef"
    And I generate a unique value from "gw-cond-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"externalRefId":"${CTX:subRef}","apiId":"${CTX:apiName}","subscriptionPlanId":"${CTX:planId}","subscriptionToken":"${CTX:subToken}"}
      """
    Then the response status should be 201
    And I register the "subscription" "${CTX:subRef}" for cleanup

    # 3 requests per minute: the 4th is throttled
    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 3 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 429

    # Conditions without request-count leave the plan without a rate limit
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"conditions":[{"id":"region-limit","type":"custom","params":{"regions":["eu"]}}]}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should not exist
    And the JSON response array field "conditions" should have 1 item
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I send 5 "GET" requests to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 200

    # The throttle limit fields alone set the request-count condition and keep the custom one
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"throttleLimitCount":2,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should be 2
    When I send a "GET" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the JSON response array field "conditions" should have 2 items
    And the JSON response array "conditions" item with "id" equal to "requests" should have "params.limits[0].count" equal to "2"
    And the JSON response array "conditions" item with "id" equal to "region-limit" should have "type" equal to "custom"
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 429

    # [] removes every condition: the plan is unlimited
    When I send a "PUT" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}" with body:
      """
      {"conditions":[]}
      """
    Then the response status should be 200
    And the JSON response field "throttleLimitCount" should not exist
    And the JSON response array field "conditions" should have 0 items
    When I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

  @GWPLAN-002 @type:negative
  Scenario Outline: A plan with invalid conditions is refused with the reason
    Given I generate a unique value from "gw-bad-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}",<fields>}
      """
    Then the response status should be 400
    And the JSON response field "message" should be "<message>"

    Examples:
      | case                     | fields                                                                                                                                       | message                                                                                                    |
      | event-count was removed  | "conditions":[{"type":"event-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}]}}]                                                 | conditions[0]: type must be one of request-count, tool-call-count, token-count or custom                  |
      | count of zero            | "conditions":[{"type":"request-count","params":{"limits":[{"count":0,"timeUnit":"MINUTE"}]}}]                                                | conditions[0].params.limits[0]: count must be at least 1                                                   |
      | unknown window           | "conditions":[{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"WEEK"}]}}]                                                 | conditions[0].params.limits[0]: timeUnit must be one of MINUTE, HOUR, DAY or MONTH                         |
      | windows of equal length  | "conditions":[{"type":"request-count","params":{"limits":[{"count":10,"timeAmount":24,"timeUnit":"HOUR"},{"count":20,"timeUnit":"DAY"}]}}]   | conditions[0].params.limits[1]: the condition already has a limit over 1 DAY; each limit needs a window of its own length |
      | unknown onExceed         | "conditions":[{"type":"request-count","params":{"limits":[{"count":10,"timeUnit":"MINUTE"}],"onExceed":"throttle"}}]                         | conditions[0]: params.onExceed must be reject or allow                                                     |
      | two request-count        | "conditions":[{"type":"request-count","params":{"limits":[{"count":1,"timeUnit":"MINUTE"}]}},{"type":"request-count","params":{"limits":[{"count":2,"timeUnit":"DAY"}]}}] | conditions[1]: a plan can have only one request-count condition                     |
      | custom with built-in id  | "conditions":[{"id":"requests","type":"custom","params":{}}]                                                                                 | conditions[0]: requests is the id of a built-in condition                                                  |
      | custom without params    | "conditions":[{"id":"region-limit","type":"custom"}]                                                                                         | conditions[0]: params is required; send {} for a custom condition with no values                          |
      | conditions and throttle  | "conditions":[],"throttleLimitCount":5,"throttleLimitUnit":"Min"                                                                             | send conditions or throttleLimitCount and throttleLimitUnit, not both                                      |
      | conditions null          | "conditions":null                                                                                                                            | conditions must be an array; send [] for a plan with no conditions                                         |
