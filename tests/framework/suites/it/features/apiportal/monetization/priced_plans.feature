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

@api-portal-priced-plans
Feature: Priced subscription plans in the API Portal
  As an API Portal administrator
  I want to give a plan a price, flat or by usage, when I create it
  So that the portal can sell it, and the price a subscriber agreed to never changes under them

  Background:
    Given I generate a unique resource name from "portal-plan" and store it as "planId"
    And I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with body and stored as "meterId":
      """
      {"id":"${CTX:meterId}","displayName":"API calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}]}
      """

  Scenario: A flat-rate plan is priced once, and renaming it keeps its price
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Starter ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":60,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"FLAT_RATE","currency":"usd","billingPeriod":"MONTH","flatAmount":"9.99"}}
      """
    Then the response status code should be 201
    And the JSON response field "paid" should be true
    And the JSON response field "pricing.currency" should be "USD"
    And the JSON response field "pricing.flatAmount" should be "9.99"

    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}/pricing" as "developer"
    Then the response status code should be 200
    And the JSON response field "pricingType" should be "FLAT_RATE"
    And the JSON response field "billingPeriod" should be "MONTH"

    # Sending a price for a plan that exists is refused, and nothing changes.
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON body:
      """
      {"id":"${CTX:planId}","displayName":"Starter ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":60,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"19.99"}}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "PLAN_PRICING_LOCKED"
    And the JSON response field "message" should be "The plan ${CTX:planId} already exists; its pricing can't be updated once a plan is created."

    # An update that sends no price keeps the price.
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON body:
      """
      {"id":"${CTX:planId}","displayName":"Starter plus ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":120,"timeUnit":"MINUTE"}]}
      """
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the JSON response field "displayName" should be "Starter plus ${CTX:planId}"
    And the JSON response field "pricing.flatAmount" should be "9.99"

  Scenario: A usage-based plan prices reads per unit and writes in graduated tiers
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Pay as you go ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":-1}],"pricing":{"pricingType":"USAGE_BASED","currency":"EUR","billingPeriod":"YEAR","prices":[
        {"meterId":"${CTX:meterId}","filters":[{"field":"method","operator":"EQUALS","values":["GET"]},{"field":"Status","operator":"GREATER_THAN_OR_EQUAL","values":["0200"]},{"field":"Status","operator":"LESS_THAN","values":["300"]}],"pricingModel":"PER_UNIT","unitAmount":"0.50","unitQuantity":1000},
        {"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"IN","values":["POST","PUT","POST"]}],"pricingModel":"GRADUATED","tiers":[{"upTo":1000,"unitAmount":"0"},{"upTo":null,"unitAmount":"0.002","flatAmount":"5"}]}
      ]}}
      """
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}/pricing" as "admin"
    Then the response status code should be 200
    And the JSON response field "pricingType" should be "USAGE_BASED"
    And the JSON response field "currency" should be "EUR"
    And the JSON response field "billingPeriod" should be "YEAR"
    And the JSON response array field "prices" should have 2 items
    # Each filter names the property as the meter spells it; a comparison is a plain whole number;
    # a repeated value is kept once.
    And the JSON response array "prices" item with "pricingModel" equal to "PER_UNIT" should have "unitQuantity" equal to "1000"
    And the JSON response array "prices" item with "pricingModel" equal to "PER_UNIT" should have "filters[0].field" equal to "Method"
    And the JSON response array "prices" item with "pricingModel" equal to "PER_UNIT" should have "filters[1].values[0]" equal to "200"
    And the JSON response array "prices" item with "pricingModel" equal to "GRADUATED" should have "filters[0].values" equal to "[POST PUT]"
    And the JSON response array "prices" item with "pricingModel" equal to "GRADUATED" should have "tiers[1].unitAmount" equal to "0.002"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the JSON response field "planCount" should be 1

  Scenario: A free plan has no price, its price can't be set later, and a price is never set or removed on its own
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Free ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":100,"timeUnit":"DAY"}]}
      """
    Then the JSON response field "paid" should be false
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}/pricing" as "admin"
    Then the response status code should be 404
    And the JSON response field "message" should be "The plan is free: it has no pricing"
    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON body:
      """
      {"id":"${CTX:planId}","displayName":"Free ${CTX:planId}","pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"5"}}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "PLAN_PRICING_LOCKED"
    When I send an authenticated API Portal "PUT" request to "/subscription-plans/${CTX:planId}/pricing" as "admin" with JSON body:
      """
      {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"5"}
      """
    Then the response status code should be 405
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}/pricing" as "admin"
    Then the response status code should be 405

  Scenario: A paid plan an API offers can't be deleted
    Given a unique API Portal resource is created at "/subscription-plans" as "admin" with body and stored as "planId":
      """
      {"id":"${CTX:planId}","displayName":"Pro ${CTX:planId}","limits":[{"limitType":"REQUEST_COUNT","limitCount":600,"timeUnit":"MINUTE"}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"49"}}
      """
    And a REST API is created in the API Portal and stored as "apiId"
    When I send an authenticated API Portal "PUT" multipart request to "/apis/${CTX:apiId}" as "publisher" with metadata only:
      """
      {"name":"Priced REST API","version":"v1.0","type":"REST","status":"PUBLISHED","endPoints":{"productionURL":"https://priced.example.invalid","sandboxURL":"https://priced-sandbox.example.invalid"},"subscriptionPlans":[{"id":"Gold"},{"id":"${CTX:planId}"}]}
      """
    Then the response status code should be 200
    And the JSON response array "subscriptionPlans" should contain an item with "id" equal to "${CTX:planId}"
    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 409
    And the JSON response field "code" should be "PLAN_IN_USE"
    And the JSON response field "message" should be "The plan Pro ${CTX:planId} is in use and can't be deleted."

  Scenario Outline: A price that can't be billed is refused with the reason, and no plan is created
    When I send an authenticated API Portal "POST" request to "/subscription-plans" as "admin" with JSON body:
      """
      {"id":"${CTX:planId}","displayName":"Refused ${CTX:planId}","pricing":<pricing>}
      """
    Then the response status code should be 400
    And the JSON response field "errors[0].message" should be "<message>"
    When I send an authenticated API Portal "GET" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 404

    Examples:
      | pricing | message |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"0"} | flatAmount must be more than 0. Create a free plan instead. |
      | {"pricingType":"FLAT_RATE","currency":"JPY","billingPeriod":"MONTH","flatAmount":"500"} | currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used. |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH"} | flatAmount is required |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","flatAmount":"5","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | A usage based plan has no flat amount |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1,"tiers":[{"upTo":null,"unitAmount":"1"}]}]} | A PER_UNIT price (price 1) does not take tiers |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"VOLUME","tiers":[{"upTo":10,"unitAmount":"1"},{"upTo":5,"unitAmount":"0.5"},{"upTo":null,"unitAmount":"0.1"}]}]} | Each tier of price 1 needs an upTo greater than the tier before it |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"GRADUATED","tiers":[{"upTo":10,"unitAmount":"1"}]}]} | The last tier of price 1 must have no upper bound (upTo null) |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET","POST"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: Operator EQUALS takes exactly one value |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN","values":["500"]},{"field":"Status","operator":"LESS_THAN","values":["100"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: the comparisons on Status can't all be true |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Country","operator":"EQUALS","values":["LK"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: the meter ${CTX:meterId} has no property Country |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"no-such-meter","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | Unknown meter: no-such-meter |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1},{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]} | Prices 1 and 2 both count the same requests of the meter ${CTX:meterId}; give them different filters. |
