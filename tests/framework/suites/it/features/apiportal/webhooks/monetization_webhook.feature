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

@devportal-monetization-webhook
Feature: Meters and priced plans made in the API Portal reach the control plane
  As an API Portal administrator who prices plans in the portal
  I want the meters and priced plans I make there to appear in the control plane on their own
  So that the gateway enforces the plans I sell without anyone copying them across

  Background:
    Given the gateway services are running
    And a webhook subscriber targeting the control plane is registered in the API Portal for events "meter.*, subscription_plan.*"
    And I generate a unique resource name from "portal-made-calls" and store it as "meterId"
    And I generate a unique resource name from "portal-made-plan" and store it as "planId"

  Scenario: A meter and a usage-priced plan made in the API Portal appear in the control plane, and leave with them
    Given a unique API Portal resource is created at "/meters" as "admin" with body and stored as "meterId":
      """
      {"id":"${CTX:meterId}","displayName":"Portal-made calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    And the control plane meter "${CTX:meterId}" is registered for cleanup
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Portal-made ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":300,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.03","unitQuantity":1}]}}
      """
    And the control plane subscription_plan "${CTX:planId}" is registered for cleanup

    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}" until the JSON field "displayName" is "Portal-made calls ${CTX:meterId}"
    Then the JSON response field "filterFields[0].eventKey" should be "response.status"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}" until the JSON field "displayName" is "Portal-made ${CTX:planId}"
    Then the JSON response field "limits[0].limitCount" should be 300
    And the JSON response field "pricing.pricingType" should be "USAGE_BASED"
    And the JSON response field "pricing.prices[0].meterId" should be "${CTX:meterId}"
    And the JSON response field "pricing.prices[0].unitAmount" should be "0.03"

    # Renaming the plan in the portal renames it in the control plane; the price stays.
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON body:
      """
      {"id":"${CTX:planId}","displayName":"Portal-renamed ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":300,"timeUnit":"MINUTE"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}" until the JSON field "displayName" is "Portal-renamed ${CTX:planId}"
    Then the JSON response field "pricing.prices[0].unitAmount" should be "0.03"

    # Deleting the plan in the portal deletes it in the control plane.
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}" until the JSON field "planCount" is "0"
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 404
