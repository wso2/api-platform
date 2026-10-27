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

@platform-api-portal-monetization-events
Feature: Meters and priced plans made in the API Portal reach the control plane
  As an API Portal administrator who prices plans in the portal
  I want the meters and plans I make there to become the control plane's meters and plans
  So that the gateway enforces the same plans the portal sells, and neither side silently changes a price

  Background:
    Given I generate a unique resource name from "portal-calls" and store it as "meterId"
    And I generate a unique resource name from "portal-plan" and store it as "planId"

  Scenario: A meter and a usage plan made in the API Portal become control-plane resources
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Portal calls ${CTX:meterId}","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    Then the response status code should be 200
    And the JSON response field "status" should be "processed"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Portal calls ${CTX:meterId}"
    And the JSON response field "filterFields[0].field" should be "Status"
    And the JSON response field "filterFields[0].eventKey" should be "response.status"

    When the API Portal sends the "subscription_plan.created" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Portal usage ${CTX:planId}","limits":[{"limit_type":"REQUEST_COUNT","limit_count":100,"time_unit":"MINUTE","time_amount":1}],"pricing":{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[{"meter_handle":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricing_model":"PER_UNIT","unit_amount":"0.01","unit_quantity":1}]}}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Portal usage ${CTX:planId}"
    And the JSON response field "limits[0].limitCount" should be 100
    And the JSON response field "limits[0].timeUnit" should be "MINUTE"
    And the JSON response field "pricing.pricingType" should be "USAGE_BASED"
    And the JSON response field "pricing.prices[0].meterId" should be "${CTX:meterId}"
    And the JSON response field "pricing.prices[0].unitAmount" should be "0.01"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "planCount" should be 1

  Scenario: An older change from the API Portal never overwrites a newer one
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Current name","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    # A change the portal made before the current state arrives late: it is accepted and ignored.
    When the API Portal sends the "meter.updated" event dated "2020-01-01T00:00:00Z" to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Stale name","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "displayName" should be "Current name"

    When the API Portal sends the "meter.updated" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Newer name","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "displayName" should be "Newer name"

  Scenario: The API Portal can rename a priced plan, but its price and the meter it prices can't be updated
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Priced calls","type":"REQUEST_COUNT","filter_fields":[{"field":"Status","event_key":"response.status"}]}
      """
    And the API Portal sends the "subscription_plan.created" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Metered ${CTX:planId}","pricing":{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[{"meter_handle":"${CTX:meterId}","pricing_model":"PER_UNIT","unit_amount":"0.01","unit_quantity":1}]}}
      """
    Then the response status code should be 200

    When the API Portal sends the "subscription_plan.updated" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Metered ${CTX:planId}","pricing":{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[{"meter_handle":"${CTX:meterId}","pricing_model":"PER_UNIT","unit_amount":"0.02","unit_quantity":1}]}}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "SUBSCRIPTION_PLAN_PRICING_LOCKED"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}/pricing"
    Then the JSON response field "prices[0].unitAmount" should be "0.01"

    When the API Portal sends the "subscription_plan.updated" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Metered renamed ${CTX:planId}","pricing":{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[{"meter_handle":"${CTX:meterId}","pricing_model":"PER_UNIT","unit_amount":"0.01","unit_quantity":1}]}}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the JSON response field "displayName" should be "Metered renamed ${CTX:planId}"

    # A change to a meter a plan prices is accepted and ignored.
    When the API Portal sends the "meter.updated" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Changed calls","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "displayName" should be "Priced calls"
    And the JSON response array field "filterFields" should have 1 item

  Scenario: A plan that prices a meter the control plane doesn't know is refused and not created
    When the API Portal sends the "subscription_plan.created" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Orphan ${CTX:planId}","pricing":{"pricing_type":"USAGE_BASED","currency":"USD","billing_period":"MONTH","prices":[{"meter_handle":"${CTX:meterId}","pricing_model":"PER_UNIT","unit_amount":"0.01","unit_quantity":1}]}}
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "Unknown meter: ${CTX:meterId}"
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 404

  Scenario: Deleting a plan and then its meter in the API Portal deletes them in the control plane
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Short-lived calls","type":"REQUEST_COUNT"}
      """
    And the API Portal sends the "subscription_plan.created" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Short-lived ${CTX:planId}","pricing":{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"5"}}
      """
    Then the response status code should be 200

    When the API Portal sends the "subscription_plan.deleted" event to the control plane with data:
      """
      {"handle":"${CTX:planId}"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 404
    When the API Portal sends the "meter.deleted" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404

  Scenario: Deleting in the API Portal a plan a control-plane API offers takes it off that API
    Given I generate a unique value from "portal-delete-project" and store it as "projectHandle"
    And I create a project "${CTX:projectHandle}" on the control plane
    And I generate a unique resource name from "portal-delete-api" and store it as "apiHandle"
    And I generate a unique API context from "/portal-delete" and store it as "apiContext"
    When the API Portal sends the "subscription_plan.created" event to the control plane with data:
      """
      {"handle":"${CTX:planId}","display_name":"Offered ${CTX:planId}","limits":[{"limit_type":"REQUEST_COUNT","limit_count":50,"time_unit":"MINUTE","time_amount":1}],"pricing":{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"12"}}
      """
    Then the response status code should be 200
    When I create a secured REST API "${CTX:apiHandle}" via the control plane in project "${CTX:projectHandle}" with context "${CTX:apiContext}" offering plan "${CTX:planId}"
    And I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the JSON response field "usage.restApis" should be 1

    # The portal is the owner of a plan it made: deleting it there deletes it here, in use or not.
    When the API Portal sends the "subscription_plan.deleted" event to the control plane with data:
      """
      {"handle":"${CTX:planId}"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiHandle}"
    Then the response status code should be 200
    And the JSON response field "subscriptionPlans" should not exist

  Scenario Outline: A meter or plan from the API Portal that the control plane could not enforce is refused
    When the API Portal sends the "<event>" event to the control plane with data:
      """
      <data>
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "Failed to process event"

    Examples:
      | event                     | data |
      | meter.created             | {"handle":"${CTX:meterId}","display_name":"Tokens","type":"TOKENS"} |
      | meter.created             | {"handle":"${CTX:meterId}","display_name":"Calls","type":"REQUEST_COUNT","type_key":"usage.total"} |
      | meter.created             | {"handle":"${CTX:meterId}","display_name":"Calls","type":"BYTES"} |
      | meter.created             | {"handle":"${CTX:meterId}","display_name":"Calls","type":"TOKENS","type_key":"usage total"} |
      | meter.created             | {"handle":"${CTX:meterId}","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"","event_key":"a"}]} |
      | meter.created             | {"handle":"${CTX:meterId}","display_name":"Calls","type":"REQUEST_COUNT","filter_fields":[{"field":"a","event_key":"a"},{"field":"b","event_key":"b"},{"field":"c","event_key":"c"},{"field":"d","event_key":"d"},{"field":"e","event_key":"e"},{"field":"f","event_key":"f"},{"field":"g","event_key":"g"},{"field":"h","event_key":"h"},{"field":"i","event_key":"i"},{"field":"j","event_key":"j"},{"field":"k","event_key":"k"}]} |
      | subscription_plan.created | {"handle":"${CTX:planId}","display_name":"Odd","pricing":{"pricing_type":"FLAT_RATE","currency":"USD","billing_period":"MONTH","flat_amount":"5","prices":[{"meter_handle":"m","pricing_model":"PER_UNIT","unit_amount":"1","unit_quantity":1}]}} |
      | subscription_plan.created | {"handle":"${CTX:planId}","display_name":"Odd","pricing":{"pricing_type":"BARTER","currency":"USD","billing_period":"MONTH","flat_amount":"5"}} |
