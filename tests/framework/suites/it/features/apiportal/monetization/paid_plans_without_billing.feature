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

@api-portal-paid-plans-without-billing
Feature: Paid plans before billing is set up
  As an API Portal administrator preparing to sell plans
  I want to price plans and offer them on APIs before billing is connected
  So that the catalog is ready, while nobody can subscribe to a paid plan they could not be charged for

  Background:
    Given I generate a unique resource name from "unbilled-plan" and store it as "planId"
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Business ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":1000,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"99"}}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata only:
      """
      {"name":"Business REST API","version":"v1.0","type":"REST","status":"PUBLISHED","endPoints":{"productionURL":"https://business.example.invalid","sandboxURL":"https://business-sandbox.example.invalid"},"subscriptionPlans":[{"id":"Gold"},{"id":"${CTX:planId}"}]}
      """
    Then the response status code should be 200

  Scenario: Without billing, a paid plan is offered on an API but can't be subscribed to or bought
    When I send an authenticated API Portal "GET" request to "/billing-configuration" as "admin"
    Then the response status code should be 404
    And the JSON response field "message" should be "Billing is not set up for this organization."

    # The API still lists the plan: the offer is recorded, ready for when billing is connected.
    When I send an authenticated API Portal "GET" request to "/apis/${CTX:apiId}" as "admin"
    Then the response status code should be 200
    And the JSON response array "subscriptionPlans" should contain an item with "id" equal to "${CTX:planId}"

    # A paid plan is only ever bought through checkout, never subscribed to directly.
    When I send an authenticated API Portal "POST" request to "/subscriptions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiId}","subscriptionPlanId":"${CTX:planId}"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "PAID_PLAN_REQUIRES_CHECKOUT"
    And the JSON response field "message" should be "This is a paid plan. Subscribe to it through checkout."
    # And without billing, checkout is not open.
    When I send an authenticated API Portal "POST" request to "/checkout-sessions" as "developer" with JSON body:
      """
      {"artifactId":"${CTX:apiId}","subscriptionPlanId":"${CTX:planId}"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "PLAN_NOT_AVAILABLE"
    And the JSON response field "message" should be "This plan cannot be bought right now."

    # The free plan on the same API is unaffected.
    When a developer subscription for API "${CTX:apiId}" using plan "Gold" is created and stored as "subscriptionId"
    Then the response status code should be 201

  Scenario: A paid plan taken off an API with no subscriptions is simply removed
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata only:
      """
      {"name":"Business REST API","version":"v1.0","type":"REST","status":"PUBLISHED","endPoints":{"productionURL":"https://business.example.invalid","sandboxURL":"https://business-sandbox.example.invalid"},"subscriptionPlans":[{"id":"Gold"}]}
      """
    Then the response status code should be 200
    And the JSON response array "subscriptionPlans" should not contain an item with "id" equal to "${CTX:planId}"
    # Not offered anywhere, the plan can be deleted.
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
