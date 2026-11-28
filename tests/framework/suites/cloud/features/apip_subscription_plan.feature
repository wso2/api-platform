# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

@apip-cloud @PLAN
Feature: APIP cloud subscription plans
  As an APIP cloud API publisher
  I want the plans I offer to keep their limits and prices as I set them
  So that what a consumer subscribes to is what the gateway enforces and the portal shows

  # A scenario's plans carry one generated word in their display names, so a search for it finds
  # them and nothing else of the shared organisation.
  @ID_APIP_PLAN_001
  Scenario: A subscription plan keeps its limits through list, update and duplicate attempts
    Given I obtain an APIP cloud console token
    When I submit the synthetic subscription plan "A" with:
      | status | ACTIVE                                                               |
      | limits | [{"limitType":"REQUEST_COUNT","timeUnit":"MINUTE","limitCount":10}] |
    Then the cloud API response status code should be 201
    And the subscription plan answer should be located at synthetic subscription plan "A"
    And the subscription plan answer should name the console user as its creator
    When I submit the synthetic subscription plan "B" with:
      | status | ACTIVE                                                              |
      | limits | [{"limitType":"REQUEST_COUNT","timeUnit":"HOUR","limitCount":100}] |
    Then the cloud API response status code should be 201
    When I submit the synthetic subscription plan "C" with:
      | status | ACTIVE |
    Then the cloud API response status code should be 201
    When I submit another subscription plan with the id of synthetic subscription plan "A" with:
      | status | ACTIVE |
    Then the cloud API response status code should be 409
    And the cloud API error code should be "SUBSCRIPTION_PLAN_EXISTS"
    When I list the scenario's synthetic subscription plans with a limit of 2
    Then the cloud API response status code should be 200
    And the cloud API response should list 2 of 3 subscription plans
    When I list the scenario's synthetic subscription plans with a limit of 100
    Then the cloud API response should list 3 of 3 subscription plans
    And the listed synthetic subscription plan "A" should allow 10 requests per MINUTE
    And the listed synthetic subscription plan "B" should allow 100 requests per HOUR
    When I update the synthetic subscription plan "A" with:
      | status | ACTIVE |
      | limits | []     |
    Then the cloud API response status code should be 200
    When I read the synthetic subscription plan "A"
    Then the cloud API response status code should be 200
    And the subscription plan answer should have 0 limits

  # The cloud refuses to create a priced plan while monetization is off, but not an update, so the
  # platform's own rule that a free plan stays free is what answers it.
  @ID_APIP_PLAN_002 @type:negative
  Scenario: A free plan has no price to read, and a price can't be added later
    Given I obtain an APIP cloud console token
    When I submit the synthetic subscription plan "free" with:
      | status | ACTIVE                                                             |
      | limits | [{"limitType":"REQUEST_COUNT","timeUnit":"DAY","limitCount":100}] |
    Then the cloud API response status code should be 201
    And the subscription plan answer should carry no pricing
    When I read the pricing of the synthetic subscription plan "free"
    Then the cloud API response status code should be 404
    And the cloud API error code should be "SUBSCRIPTION_PLAN_PRICING_NOT_FOUND"
    And the cloud API error message should be "This subscription plan has no pricing."
    When I update the synthetic subscription plan "free" with:
      | status  | ACTIVE                                                                                  |
      | pricing | {"pricingType":"FLAT_RATE","currency":"USD","billingPeriod":"MONTH","flatAmount":"5"} |
    Then the cloud API response status code should be 409
    And the cloud API error code should be "SUBSCRIPTION_PLAN_PRICING_LOCKED"
    When I update the synthetic subscription plan "free" with:
      | status | ACTIVE                                                             |
      | limits | [{"limitType":"REQUEST_COUNT","timeUnit":"DAY","limitCount":100}] |
    Then the cloud API response status code should be 200
    And the subscription plan answer should carry no pricing
