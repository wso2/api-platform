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

@subscription-analytics
Feature: Subscription analytics monetized billing metadata
  As a platform administrator
  I want billing fields from a monetized subscription to appear in analytics events
  So that usage can be tracked and correlated with billing records
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Monetized subscription billing fields appear in analytics events
    Given I generate a unique value from "subscription-analytics-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"

    Given I generate a unique value from "subscription-analytics" and store it as "apiName"
    And I generate a unique API context from "/subscription-analytics" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion              | ${CTX:gatewaySpecVersion}         |
      | name                    | ${CTX:apiName}                    |
      | spec.displayName        | ${CTX:apiName}                    |
      | spec.version            | v1.0                                |
      | spec.context            | ${CTX:apiContext}/$version          |
      | spec.subscriptionPlans  | ["${CTX:planName}"]                 |
      | spec.upstream.main.url  | http://testbench:3002                |
      | spec.operations         | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful

    # A fresh route protected by subscription-validation rejects an unrecognized key with 403 -
    # this doubles as the route-readiness fold, exactly as the plain-200 idiom does elsewhere.
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    And I reset the analytics collector

    Given I generate a unique value from "subscription-analytics-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}","subscriptionPlanId":"${CTX:planId}","billingCustomerId":"cust-123","billingSubscriptionId":"billing-sub-456"}
      """
    Then the response status should be 201

    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200

    And the analytics collector should have received at least 1 event
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "billingCustomerId" with value "cust-123"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "billingSubscriptionId" with value "billing-sub-456"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "subscriptionStatus" with value "ACTIVE"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "subscriptionPlanName" with value "${CTX:planName}"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 404
    When I send a "DELETE" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response should be successful

  Scenario: Refused, throttled and free calls are not attributed to a billing customer
    Given I generate a unique value from "subscription-analytics-limited-plan" and store it as "limitedPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:limitedPlanName}","throttleLimitCount":1,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "limitedPlanId"
    And I register the "subscription-plan" "${CTX:limitedPlanId}" for cleanup
    Given I generate a unique value from "subscription-analytics-open-plan" and store it as "openPlanName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:openPlanName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "openPlanId"
    And I register the "subscription-plan" "${CTX:openPlanId}" for cleanup

    Given I generate a unique value from "subscription-analytics-unbilled" and store it as "apiName"
    And I generate a unique API context from "/subscription-analytics-unbilled" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                          |
      | name                   | ${CTX:apiName}                                     |
      | spec.displayName       | ${CTX:apiName}                                     |
      | spec.version           | v1.0                                               |
      | spec.context           | ${CTX:apiContext}/$version                         |
      | spec.subscriptionPlans | ["${CTX:limitedPlanName}","${CTX:openPlanName}"]   |
      | spec.upstream.main.url | http://testbench:3002                              |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 403

    # A paid subscription on the limited plan, and a free one (no billing identifiers) on the open plan.
    Given I generate a unique value from "subscription-analytics-unbilled-customer" and store it as "customerId"
    And I generate a unique value from "subscription-analytics-unbilled-paid-token" and store it as "paidToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:paidToken}","subscriptionPlanId":"${CTX:limitedPlanId}","billingCustomerId":"${CTX:customerId}","billingSubscriptionId":"billing-${CTX:customerId}"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "paidSubId"
    And I register the "subscription" "${CTX:paidSubId}" for cleanup
    Given I generate a unique value from "subscription-analytics-unbilled-free-token" and store it as "freeToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:freeToken}","subscriptionPlanId":"${CTX:openPlanId}"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "freeSubId"
    And I register the "subscription" "${CTX:freeSubId}" for cleanup

    # Refused by the subscription policy: no billing customer.
    When I wait for the analytics collector to settle
    And I reset the analytics collector
    And I set header "Subscription-Key" to "not-a-subscription-token"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 403
    And the analytics collector should have received 1 event
    # A call the policy refuses never reaches the operation, so it is recorded under the API context.
    And the latest analytics event for path "${CTX:apiContext}/v1.0" should have response status 403
    And the latest analytics event for path "${CTX:apiContext}/v1.0" should not have company id
    And the latest analytics event for path "${CTX:apiContext}/v1.0" should not have metadata field "billingCustomerId"

    # A free subscription's call: served, but not attributed to a customer.
    When I set header "Subscription-Key" to "${CTX:freeToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should not have company id
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should not have metadata field "billingCustomerId"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should not have metadata field "billingSubscriptionId"

    # Throttled by the plan (1 per minute): no billing customer.
    When I set header "Subscription-Key" to "${CTX:paidToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 429
    And the analytics collector should have received 1 event
    And the latest analytics event for path "${CTX:apiContext}/v1.0" should have response status 429
    And the latest analytics event for path "${CTX:apiContext}/v1.0" should not have company id
    And the latest analytics event for path "${CTX:apiContext}/v1.0" should not have metadata field "billingCustomerId"
