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

@api-portal-usage-billing
Feature: Billing a plan by usage in the API Portal
  As an API Portal administrator selling a plan charged by usage
  I want the plan's meter and price created in Moesif when the plan goes on sale, and its usage billed
  So that what a developer sees as used is what Stripe charges, and a plan Moesif can't create is never sold

  Background:
    Given I send a "POST" request to the "stripe" service at "/test/reset"
    And I send a "POST" request to the "moesif" service at "/test/reset"
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "usage-billing-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "billed-calls" and store it as "meterId"
    And I generate a unique resource name from "usage-plan" and store it as "planHandle"
    And I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Billed calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    And the response status code should be 201
    And I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planHandle}","displayName":"Metered ${CTX:planHandle}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]}}
      """
    And the response status code should be 201
    And I generate a unique resource name from "usage-billing-api" and store it as "apiHandle"
    And I generate a unique API context from "/usage-billing" and store it as "apiContext"
    And I generate a unique API version from "v" and store it as "apiVersion"
    And I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planHandle}"
    And I generate a unique resource name from "usage-billing-portal" and store it as "portalHandle"
    And I register the API Portal "${CTX:portalHandle}" with the control plane
    And I set up billing in the API Portal
    And the response status code should be 201
    And I save a publishable draft of the "RestApi" "${CTX:apiHandle}" on the API Portal "${CTX:portalHandle}" with values:
      | displayName         | ${CTX:apiHandle}      |
      | version             | ${CTX:apiVersion}     |
      | subscriptionPlanIds | ["${CTX:planHandle}"] |

  Scenario: Usage Moesif counts is shown to the developer and billed by Stripe when they unsubscribe
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 201
    And I register API Portal resource "${CTX:planHandle}" at "/subscription-plans" for cleanup
    And I register API Portal resource "${CTX:meterId}" at "/meters" for cleanup
    # The plan's usage price and the billing meter that counts it are in Moesif.
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the JSON response array field "prices" should have 1 item
    And the JSON response field "prices[0].pricing_model" should be "per_unit"
    And the JSON response field "prices[0].price_in_decimal" should be "0.02"
    And the JSON response array field "meters" should have 1 item

    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiHandle}","subscriptionPlanId":"${CTX:planHandle}"}
      """
    Then the response status code should be 201
    And I store the JSON response field "id" as "checkoutId"
    When the checkout "${CTX:checkoutId}" is paid in Stripe by "developer" and the subscription is stored as "subscriptionId"
    And I send an authenticated API Portal "GET" request to "/subscriptions/${CTX:subscriptionId}/billing" as "developer"
    Then the JSON response field "byUsage" should be true

    # Moesif counts 250 calls, and reports them to Stripe against the price.
    When I send a "POST" request to the "moesif" service at "/test/usage" with body:
      """
      {"subscription":"${CTX:checkoutId}","value":250}
      """
    Then the response status should be 201
    When I send an authenticated API Portal "GET" request to "/usage?subscriptionId=${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    And the JSON response field "subscriptionId" should be "${CTX:subscriptionId}"
    And the JSON response field "usageAvailable" should be true
    And the JSON response field "usage.prices[0].total" should be 250
    And the JSON response field "estimate.total" should be "5.00"

    # A plan charged by usage ends now, with a final invoice for the usage so far.
    When I send an authenticated API Portal "DELETE" request to "/subscriptions/${CTX:subscriptionId}" as "developer"
    Then the response status code should be 200
    When I send a "GET" request to the "stripe" service at "/test/state"
    Then the JSON response array "subscriptions" item with "metadata.wso2_checkout" equal to "${CTX:checkoutId}" should have "status" equal to "canceled"
    And the JSON response array "invoices" should contain an item with "total" equal to "500"
    When I send an authenticated API Portal "GET" request to "/billing-reports/invoices" as "admin"
    Then the response status code should be 200
    And the JSON response array "invoices" should contain an item with "total" equal to "5.00"

  @type:negative
  Scenario: A plan Moesif can't create is not put on sale, and nothing made for it is left in Moesif
    Given I send a "POST" request to the "moesif" service at "/test/fail" with body:
      """
      {"method":"POST","path":"/v1/~/billing/meters","status":503}
      """
    When I publish the "RestApi" "${CTX:apiHandle}" to the API Portal "${CTX:portalHandle}" through the control plane
    Then the response status code should be 409
    And the JSON response field "code" should be "PUBLICATION_PORTAL_CONFLICT"
    And the JSON response field "message" should contain "its paid plans can be created with the billing provider"
    And I register API Portal resource "${CTX:planHandle}" at "/subscription-plans" for cleanup
    And I register API Portal resource "${CTX:meterId}" at "/meters" for cleanup
    And the API Portal should not list the API "${CTX:apiHandle}"
    # The plan and price made before the meter failed are taken back.
    When I send a "GET" request to the "moesif" service at "/test/state"
    Then the JSON response array field "prices" should have 0 items
    And the JSON response array field "meters" should have 0 items
    And the JSON response array "plans" should not contain an item with "status" equal to "active"
