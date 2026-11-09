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


@APIPORTAL @apiportal-monetization-events
Feature: The API Portal reports its meters and priced plans to webhook subscribers
  As the control plane that enforces the plans the API Portal sells
  I want every meter and plan the portal creates, changes or deletes to be delivered to me
  So that the gateway always enforces the plans the portal is selling

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml), IDs APIP-WEBHOOKS-021..023, at the API level. The
  # subscriber points at this scenario's partition of the testbench webhook sink. A refused create
  # publishes nothing, which is checked after a valid create has been delivered.

  Background:
    Given I generate a unique resource name from "monetization-events-subscriber" and store it as "subscriberId"
    And a unique API Portal resource is created at "/webhook-subscribers" as "admin" with values and stored as "subscriberId":
      | id          | ${CTX:subscriberId}                  |
      | displayName | Monetization Subscriber               |
      | targetUrl   | http://testbench:3012/${CTX:apiPortalRunnerPartition}/webhook |
      | events      | ["meter.*","subscription_plan.*"]     |
    And I generate a unique resource name from "event-calls" and store it as "meterId"
    And I generate a unique resource name from "event-plan" and store it as "planId"

  @APIP-WEBHOOKS-021
  Scenario: A meter's whole life is reported with its definition
    Given a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                     |
      | displayName  | Event calls ${CTX:meterId}                              |
      | type         | REQUEST_COUNT                                      |
      | filterFields | [{"field":"Status","eventKey":"response.status"}] |
    When I wait for an API Portal webhook event "meter.created" containing "${CTX:meterId}"
    Then the JSON response field "data.handle" should be "${CTX:meterId}"
    And the JSON response field "data.display_name" should be "Event calls ${CTX:meterId}"
    And the JSON response field "data.type" should be "REQUEST_COUNT"
    And the JSON response field "data.type_key" should be "null"
    And the JSON response field "data.filter_fields[0].field" should be "Status"
    And the JSON response field "data.filter_fields[0].event_key" should be "response.status"

    When I send an authenticated API Portal "PUT" request to "/meters/${CTX:meterId}" as "admin" with JSON values:
      | displayName  | Renamed calls ${CTX:meterId}                      |
      | type         | REQUEST_COUNT                                      |
      | filterFields | [{"field":"Status","eventKey":"response.status"}] |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "meter.updated" containing "${CTX:meterId}"
    Then the JSON response field "data.display_name" should be "Renamed calls ${CTX:meterId}"

    When I send an authenticated API Portal "DELETE" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 204
    When I wait for an API Portal webhook event "meter.deleted" containing "${CTX:meterId}"
    Then the JSON response field "data.handle" should be "${CTX:meterId}"
    And the JSON response field "data.display_name" should not exist

  @APIP-WEBHOOKS-022
  Scenario: A priced plan is reported with its price in the control plane's terms, through its life
    Given a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                     |
      | displayName  | Event calls ${CTX:meterId}                              |
      | type         | REQUEST_COUNT                                      |
      | filterFields | [{"field":"Status","eventKey":"response.status"}] |
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                       |
      | displayName | Event plan ${CTX:planId}                                            |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":500,"timeUnit":"HOUR"}] |
      | pricing     | {"pricingType":"USAGE_BASED","currency":"EUR","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.25","unitQuantity":100}]} |
    When I wait for an API Portal webhook event "subscription_plan.created" containing "${CTX:planId}"
    Then the JSON response field "data.handle" should be "${CTX:planId}"
    And the JSON response field "data.display_name" should be "Event plan ${CTX:planId}"
    And the JSON response field "data.limits[0].limit_count" should be 500
    And the JSON response field "data.limits[0].time_unit" should be "HOUR"
    And the JSON response field "data.pricing.pricing_type" should be "USAGE_BASED"
    And the JSON response field "data.pricing.currency" should be "EUR"
    And the JSON response field "data.pricing.prices[0].meter_handle" should be "${CTX:meterId}"
    And the JSON response field "data.pricing.prices[0].unit_amount" should be "0.25"
    And the JSON response field "data.pricing.prices[0].unit_quantity" should be 100
    And the JSON response field "data.pricing.prices[0].filters[0].field" should be "Status"

    When I send an authenticated API Portal "PUT" request to "/subscription-plans" as "admin" with JSON values:
      | id          | ${CTX:planId}                                                       |
      | displayName | Renamed plan ${CTX:planId}                                          |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":500,"timeUnit":"HOUR"}] |
    Then the response status code should be 200
    When I wait for an API Portal webhook event "subscription_plan.updated" containing "${CTX:planId}"
    Then the JSON response field "data.display_name" should be "Renamed plan ${CTX:planId}"

    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
    When I wait for an API Portal webhook event "subscription_plan.deleted" containing "${CTX:planId}"
    Then the JSON response field "data.handle" should be "${CTX:planId}"

  @APIP-WEBHOOKS-023 @type:negative
  Scenario: A refused meter is not reported, while a valid one is
    Given I generate a unique resource name from "refused-event-meter" and store it as "refusedMeterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:refusedMeterId} |
      | displayName | Refused meter         |
      | type        | TOKENS                |
    Then the response status code should be 400
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                     |
      | displayName  | Event calls ${CTX:meterId}                              |
      | type         | REQUEST_COUNT                                      |
      | filterFields | [{"field":"Status","eventKey":"response.status"}] |
    When I wait for an API Portal webhook event "meter.created" containing "${CTX:meterId}"
    Then the JSON response field "data.handle" should be "${CTX:meterId}"
    And I verify no API Portal webhook event "meter.created" containing "${CTX:refusedMeterId}" is delivered
