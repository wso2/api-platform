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

@platform-api-priced-plans
Feature: Priced subscription plans
  As an administrator of the API platform
  I want to give a subscription plan a price, flat or by usage
  So that the API Portal can sell it, and the price a subscriber agreed to never changes under them

  Background:
    Given I generate a unique resource name from "plan" and store it as "planId"
    And I generate a unique resource name from "calls" and store it as "meterId"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"API calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}]}
      """
    Then the response status code should be 201

  Scenario: A flat-rate plan is priced once, its price can be read, and only its name and limit can change
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Starter ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":60}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"9.99"}}
      """
    Then the response status code should be 201
    And the JSON response field "pricing.pricingType" should be "FLAT_RATE"
    And the JSON response field "pricing.currency" should be "USD"
    And the JSON response field "pricing.billingPeriod" should be "MONTH"
    And the JSON response field "pricing.flatAmount" should be "9.99"

    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}/pricing"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlanId" should be "${CTX:planId}"
    And the JSON response field "flatAmount" should be "9.99"

    # The price is fixed once the plan exists: neither a new price nor removing it is accepted.
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Starter ${CTX:planId}","status":"ACTIVE","pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"19.99"}}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_PRICING_LOCKED"
    And the JSON response field "message" should be "Pricing can't be updated once a plan is created. Create a new plan with the new pricing."
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Starter ${CTX:planId}","status":"ACTIVE","pricing":null}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_PRICING_LOCKED"

    # While no API offers it, its name and limit can change, and the price stays.
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Starter plus ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":120}]}
      """
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Starter plus ${CTX:planId}"
    And the JSON response field "limits[0].limitCount" should be 120
    And the JSON response field "pricing.flatAmount" should be "9.99"

  Scenario: A usage-based plan prices reads per unit and writes in graduated tiers
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Pay as you go ${CTX:planId}","status":"ACTIVE","pricing":{"pricingType":"USAGE_BASED","currency":"EUR","billingPeriod":"YEAR","prices":[
        {"meterId":"${CTX:meterId}","filters":[{"field":"method","operator":"EQUALS","values":["GET"]},{"field":"Status","operator":"GREATER_THAN_OR_EQUAL","values":["0200"]},{"field":"Status","operator":"LESS_THAN","values":[300]}],"pricingModel":"PER_UNIT","unitAmount":"0.50","unitQuantity":1000},
        {"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"IN","values":["POST","PUT","POST"]}],"pricingModel":"GRADUATED","tiers":[{"upTo":1000,"unitAmount":"0"},{"upTo":null,"unitAmount":"0.002","flatAmount":"5"}]}
      ]}}
      """
    Then the response status code should be 201

    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}/pricing"
    Then the response status code should be 200
    And the JSON response field "pricingType" should be "USAGE_BASED"
    And the JSON response field "currency" should be "EUR"
    And the JSON response field "billingPeriod" should be "YEAR"
    And the JSON response field "flatAmount" should be "null"
    And the JSON response array field "prices" should have 2 items
    # A filter names the property as the meter spells it, and a comparison is a plain whole number.
    And the JSON response field "prices[0].meterId" should be "${CTX:meterId}"
    And the JSON response field "prices[0].filters[0].field" should be "Method"
    And the JSON response field "prices[0].filters[1].values[0]" should be "200"
    And the JSON response field "prices[0].pricingModel" should be "PER_UNIT"
    And the JSON response field "prices[0].unitAmount" should be "0.50"
    And the JSON response field "prices[0].unitQuantity" should be 1000
    # Repeated values are kept once.
    And the JSON response array field "prices[1].filters[0].values" should have 2 items
    And the JSON response field "prices[1].pricingModel" should be "GRADUATED"
    And the JSON response field "prices[1].tiers[0].upTo" should be 1000
    And the JSON response field "prices[1].tiers[1].upTo" should be "null"
    And the JSON response field "prices[1].tiers[1].unitAmount" should be "0.002"

    # The meter now knows a plan prices it.
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "planCount" should be 1

  Scenario: A free plan has no price to read, and a price can't be added later
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Free ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"DAY","limitCount":100}]}
      """
    Then the response status code should be 201
    And the JSON response field "pricing" should not exist
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}/pricing"
    Then the response status code should be 404
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_PRICING_NOT_FOUND"
    And the JSON response field "message" should be "This subscription plan has no pricing."
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Free ${CTX:planId}","status":"ACTIVE","pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"5"}}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_PRICING_LOCKED"

  Scenario: A plan an API offers keeps its name, limit and price, and can't be deleted
    Given I generate a unique value from "priced-plan-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "priced-api" and store it as "apiHandle"
    And I generate a unique API context from "/priced-api" and store it as "apiContext"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Pro ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}],"pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"49"}}
      """
    Then the response status code should be 201
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planId}"
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 200
    And the JSON response field "usage.inUse" should be true
    And the JSON response field "usage.restApis" should be 1

    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Pro renamed ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}]}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    And the JSON response field "message" should be "The plan Pro ${CTX:planId} is in use. To change its name or limit, create a new plan."
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Pro ${CTX:planId}","status":"ACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":1200}]}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"

    # Retiring the plan is allowed while it is offered; deleting it is not.
    When I send a "PUT" request to the control plane at "/subscription-plans/${CTX:planId}" with body:
      """
      {"id":"${CTX:planId}","displayName":"Pro ${CTX:planId}","status":"INACTIVE","limits":[{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":600}]}
      """
    Then the response status code should be 200
    And the JSON response field "status" should be "INACTIVE"
    And the JSON response field "pricing.flatAmount" should be "49"
    When I send a "DELETE" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_IN_USE"
    And the JSON response field "message" should be "The plan Pro ${CTX:planId} is in use and can't be deleted."

  Scenario: Subscribers and publishers can read prices, and only administrators set them
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Team ${CTX:planId}","status":"ACTIVE","pricing":{"pricingType":"FLAT_RATE","currency":"GBP","billingPeriod":"YEAR","flatAmount":"120.00"}}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}/pricing" as "developer"
    Then the response status code should be 200
    And the JSON response field "currency" should be "GBP"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}/pricing" as "publisher"
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/subscription-plans" as "publisher" with body:
      """
      {"id":"${CTX:planId}-pub","displayName":"Publisher plan","status":"ACTIVE","pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"1"}}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"

  Scenario Outline: A price that can't be billed is refused with the reason, and no plan is created
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Refused ${CTX:planId}","status":"ACTIVE","pricing":<pricing>}
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should be "<message>"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 404

    Examples: Flat rate
      | pricing                                                                                     | message                                                                                                                             |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"0"}       | flatAmount must be more than 0. Create a free plan instead.                                                                         |
      | {"pricingType":"FLAT_RATE","currency":"JPY","billingPeriod":"MONTH","flatAmount":"500"}     | currency must be the code of a currency with two decimal places, such as USD or EUR. Currencies with no or three decimal places cannot be used. |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"WEEK","flatAmount":"5"}        | billingPeriod must be one of MONTH, YEAR                                                                                            |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"9.9.9"}   | flatAmount must be a decimal amount such as 9.99                                                                                    |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":9.99}      | The request body is not a valid plan pricing                                                                                        |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH"}                        | flatAmount is required                                                                                                              |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"5","discount":"10"} | discount is not a known field                                                                                             |

    Examples: Usage based
      | pricing | message |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[]} | A usage based plan needs 1 to 20 prices |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","flatAmount":"5","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | A usage based plan has no flat amount |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1,"tiers":[{"upTo":null,"unitAmount":"1"}]}]} | A PER_UNIT price (price 1) does not take tiers |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"VOLUME","tiers":[{"upTo":10,"unitAmount":"1"},{"upTo":5,"unitAmount":"0.5"},{"upTo":null,"unitAmount":"0.1"}]}]} | Each tier of price 1 needs an upTo greater than the tier before it |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"GRADUATED","tiers":[{"upTo":10,"unitAmount":"1"}]}]} | The last tier of price 1 must have no upper bound (upTo null) |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":0}]} | unitQuantity of price 1 must be a whole number from 1 to 1000000000 |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"BETWEEN","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | The operator of filter 1 of price 1 must be one of EQUALS, NOT_EQUALS, IN, NOT_IN, STARTS_WITH, GREATER_THAN, GREATER_THAN_OR_EQUAL, LESS_THAN, LESS_THAN_OR_EQUAL |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET","POST"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: Operator EQUALS takes exactly one value |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN","values":["500"]},{"field":"Status","operator":"LESS_THAN","values":["100"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: the comparisons on Status can't all be true |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Country","operator":"EQUALS","values":["LK"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: the meter ${CTX:meterId} has no property Country |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"no-such-meter","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | Unknown meter: no-such-meter |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1},{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]} | Prices 1 and 2 both count the same requests of the meter ${CTX:meterId}; give them different filters. |
      | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"5","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | A flat rate plan has no usage prices |
      | {"pricingType":"SUBSCRIPTION","currency":"USD","billingPeriod":"MONTH","flatAmount":"5"} | pricingType must be one of FLAT_RATE, USAGE_BASED |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"STAIRSTEP","unitAmount":"0.01","unitQuantity":1}]} | pricingModel of price 1 must be one of PER_UNIT, VOLUME, GRADUATED |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"","pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | meterId of price 1 is required |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"PER_UNIT","unitQuantity":1}]} | unitAmount of price 1 is required |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"VOLUME","unitAmount":"0.01","tiers":[{"upTo":null,"unitAmount":"1"}]}]} | A VOLUME price (price 1) does not take unitAmount |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"VOLUME","tiers":[]}]} | price 1 needs 1 to 10 tiers |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","pricingModel":"GRADUATED","tiers":[{"upTo":10,"unitAmount":"1"},{"upTo":null}]}]} | Tier 2 of price 1 needs a unitAmount, a flatAmount or both |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"EQUALS","values":["GET"]},{"field":"Method","operator":"NOT_EQUALS","values":["PUT"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1 filters on Method more than once |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN","values":["100"]},{"field":"Status","operator":"LESS_THAN","values":["500"]},{"field":"Status","operator":"NOT_EQUALS","values":["404"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1 filters on Status more than once |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN","values":["100"]},{"field":"Status","operator":"LESS_THAN","values":["500"]},{"field":"Status","operator":"LESS_THAN_OR_EQUAL","values":["400"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: a price puts at most 2 comparisons on Status |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Method","operator":"IN","values":[]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: A filter on Method needs 1 to 20 values |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN","values":["two hundred"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} | price 1: A comparison on Status takes a whole number from 0 to 1000000000000 |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN_OR_EQUAL","values":["200"]},{"field":"Status","operator":"LESS_THAN","values":["300"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1},{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN_OR_EQUAL","values":["250"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]} | Prices 1 and 2 both count the same requests of the meter ${CTX:meterId}; give them different filters. |
      | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN_OR_EQUAL","values":["500"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1},{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"IN","values":["404","503"]}],"pricingModel":"PER_UNIT","unitAmount":"0.02","unitQuantity":1}]} | Prices 1 and 2 both count the same requests of the meter ${CTX:meterId}; give them different filters. |

  Scenario: Prices on separate ranges of one property, or on values outside a range, are accepted
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Banded ${CTX:planId}","status":"ACTIVE","pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[
        {"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"LESS_THAN_OR_EQUAL","values":["399"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1},
        {"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"GREATER_THAN","values":["399"]},{"field":"Status","operator":"LESS_THAN","values":["500"]}],"pricingModel":"VOLUME","tiers":[{"upTo":100,"unitAmount":"0.05"},{"upTo":null,"unitAmount":"0.02"}]},
        {"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"IN","values":["503","504"]}],"pricingModel":"PER_UNIT","unitAmount":"0","unitQuantity":1}
      ]}}
      """
    Then the response status code should be 201
    And the JSON response array field "pricing.prices" should have 3 items

  Scenario: Plans can be listed by status and found by name
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Retired ${CTX:planId}","status":"INACTIVE","pricing":{"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"15"}}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/subscription-plans?status=INACTIVE&query=${CTX:planId}"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "list[0].pricing.flatAmount" should be "15"
    When I send a "GET" request to the control plane at "/subscription-plans?status=ACTIVE&query=${CTX:planId}"
    Then the response status code should be 200
    And the JSON response field "count" should be 0
    When I send a "GET" request to the control plane at "/subscription-plans?status=PAUSED"
    Then the response status code should be 400
    And the JSON response field "message" should be "status must be ACTIVE or INACTIVE"
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"a-plan-id-that-is-longer-than-forty-characters","displayName":"Too long","status":"ACTIVE"}
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "id must be at most 40 characters"
