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

@subscription-analytics-billing-customer
Feature: Subscription analytics attribute paid calls to the billing customer
  As a platform administrator
  I want a paid subscription's calls to carry its billing customer and the operation's path template in analytics events
  So that usage can be billed to the right customer

  # Runs on gateways newer than 1.2.0 only: older gateways don't send the path template, and don't
  # attribute calls over a plan's quota to the billing customer.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A paid subscription's calls are attributed to its billing customer under the operation's path template
    Given I generate a unique value from "subscription-analytics-template-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup

    Given I generate a unique value from "subscription-analytics-template" and store it as "apiName"
    And I generate a unique API context from "/subscription-analytics-template" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | v1.0                       |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.subscriptionPlans | ["${CTX:planName}"]        |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.operations        | [{"method":"GET","path":"/items/{id}","policies":[{"name":"subscription-validation","version":"v1","params":{"subscriptionKeyHeader":"Subscription-Key"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items/42" until status 403

    Given I generate a unique value from "subscription-analytics-template-customer" and store it as "customerId"
    And I generate a unique value from "subscription-analytics-template-billing" and store it as "billingSubId"
    And I generate a unique value from "subscription-analytics-template-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}","subscriptionPlanId":"${CTX:planId}","billingCustomerId":"${CTX:customerId}","billingSubscriptionId":"${CTX:billingSubId}"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "subId"
    And I register the "subscription" "${CTX:subId}" for cleanup

    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items/42" until status 200
    And I wait for an analytics event for path "${CTX:apiContext}/v1.0/items/42" with response status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector
    And I send a "GET" request to "${CTX:apiContext}/v1.0/items/42"
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    # Billing meters filter on the template, never the concrete path.
    And the latest analytics event for path "${CTX:apiContext}/v1.0/items/42" should have metadata field "apiResourceTemplate" with value "/items/{id}"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/items/42" should have company id "${CTX:customerId}"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/items/42" should have metadata field "billingCustomerId" with value "${CTX:customerId}"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/items/42" should have metadata field "billingSubscriptionId" with value "${CTX:billingSubId}"

  Scenario: Calls over the quota of a plan that does not stop at it are still attributed to the billing customer
    Given I generate a unique value from "subscription-analytics-overage-plan" and store it as "planName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":1,"throttleLimitUnit":"Min","stopOnQuotaReach":false}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup

    Given I generate a unique value from "subscription-analytics-overage" and store it as "apiName"
    And I generate a unique API context from "/subscription-analytics-overage" and store it as "apiContext"
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

    Given I generate a unique value from "subscription-analytics-overage-customer" and store it as "customerId"
    And I generate a unique value from "subscription-analytics-overage-token" and store it as "subToken"
    When I send a "POST" request to the "gateway-controller" service at "/subscriptions" with body:
      """
      {"apiId":"${CTX:apiName}","subscriptionToken":"${CTX:subToken}","subscriptionPlanId":"${CTX:planId}","billingCustomerId":"${CTX:customerId}","billingSubscriptionId":"billing-${CTX:customerId}"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "subId"
    And I register the "subscription" "${CTX:subId}" for cleanup

    # The first call uses the quota of 1; the next is over it, served and billed.
    When I set header "Subscription-Key" to "${CTX:subToken}"
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health" until status 200
    And I wait for an analytics event for path "${CTX:apiContext}/v1.0/health" with response status 200
    And I wait for the analytics collector to settle
    And I reset the analytics collector
    And I send a "GET" request to "${CTX:apiContext}/v1.0/health"
    Then the response status code should be 200
    And the analytics collector should have received 1 event
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have company id "${CTX:customerId}"
    And the latest analytics event for path "${CTX:apiContext}/v1.0/health" should have metadata field "billingCustomerId" with value "${CTX:customerId}"
