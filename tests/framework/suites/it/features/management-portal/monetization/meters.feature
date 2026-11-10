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

@platform-api-meters
Feature: Meters for usage-based plans
  As an administrator of the API platform
  I want to define what a usage-based plan counts, and which properties its prices can filter on
  So that each plan charges for exactly the calls or tokens it is meant to

  Scenario: An administrator defines a request meter, finds it, refines it and removes it
    Given I generate a unique resource name from "calls" and store it as "meterId"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"API calls ${CTX:meterId}","description":"Every call a subscriber makes","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"}]}
      """
    Then the response status code should be 201
    And the response header "Location" should be "/api/v0.9/meters/${CTX:meterId}"
    And the JSON response field "id" should be "${CTX:meterId}"
    And the JSON response field "displayName" should be "API calls ${CTX:meterId}"
    And the JSON response field "type" should be "REQUEST_COUNT"
    And the JSON response field "typeKey" should be "null"
    And the JSON response field "planCount" should be 0
    And the JSON response field "filterFields[1].field" should be "Method"
    And the JSON response field "filterFields[1].eventKey" should be "request.verb"

    When I send a "GET" request to the control plane at "/meters?query=${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response field "count" should be 1
    And the JSON response field "list[0].id" should be "${CTX:meterId}"

    # Replacing the meter keeps its id and takes the new set of filter properties.
    When I send a "PUT" request to the control plane at "/meters/${CTX:meterId}" with body:
      """
      {"displayName":"API calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"},{"field":"Method","eventKey":"request.verb"},{"field":"Resource","eventKey":"request.route"}]}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response array field "filterFields" should have 3 items
    And the JSON response field "filterFields[2].field" should be "Resource"
    And the JSON response field "filterFields[2].eventKey" should be "request.route"

    When I send a "DELETE" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "METER_NOT_FOUND"
    And the JSON response field "message" should be "The specified meter could not be found."

  Scenario: A meter named in words gets an id derived from its name, and ids are unique
    Given I generate a unique resource name from "ok" and store it as "name"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"displayName":"Successful calls ${CTX:name}","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 201
    And the JSON response field "id" should be "successful-calls-${CTX:name}"

    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"successful-calls-${CTX:name}","displayName":"Another meter","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "METER_EXISTS"
    And the JSON response field "message" should be:
      """
      A meter with id "successful-calls-${CTX:name}" already exists in this organization.
      """

    When I send a "PUT" request to the control plane at "/meters/successful-calls-${CTX:name}" with body:
      """
      {"id":"renamed-${CTX:name}","displayName":"Successful calls ${CTX:name}","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 400
    And the JSON response field "message" should be "The id of a meter cannot be changed"

  Scenario: A token meter counts the tokens under its type key
    Given I generate a unique resource name from "tokens" and store it as "meterId"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Input tokens ${CTX:meterId}","type":"TOKENS","typeKey":"aiTokenUsage.inputTokens","filterFields":[{"field":"Model","eventKey":"aiMetadata.model"}]}
      """
    Then the response status code should be 201
    And the JSON response field "type" should be "TOKENS"
    And the JSON response field "typeKey" should be "aiTokenUsage.inputTokens"

  Scenario Outline: A meter that cannot count anything sensible is refused with the reason
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      <meter>
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    And the JSON response field "message" should be "<message>"

    Examples:
      | meter                                                                                                                                            | message                                                         |
      | {"displayName":"Tokens","type":"TOKENS"}                                                                                                         | A TOKENS meter needs a typeKey                                  |
      | {"displayName":"Calls","type":"REQUEST_COUNT","typeKey":"usage.total"}                                                                           | typeKey applies only to a TOKENS meter                          |
      | {"displayName":"Bytes","type":"BYTES"}                                                                                                           | type must be one of REQUEST_COUNT, TOKENS                       |
      | {"id":"Calls_Meter","displayName":"Calls","type":"REQUEST_COUNT"}                                                                                | id must be 3 to 40 lowercase letters, digits or hyphens         |
      | {"displayName":"Calls","type":"REQUEST_COUNT","colour":"red"}                                                                                    | colour is not a known field                                     |
      | {"displayName":"Calls","type":"REQUEST_COUNT","planCount":3}                                                                                     | planCount is read-only                                          |
      | {"type":"REQUEST_COUNT"}                                                                                                                         | displayName is required                                         |
      | {"displayName":"Calls","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"},{"field":"status","eventKey":"x"}]} | The meter has the property status more than once                |
      | {"displayName":"Calls","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"code"},{"field":"Code","eventKey":"code"}]}           | Code and Status read the same event key code                    |
      | {"displayName":"Calls","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response status"}]}                                  | The event key of Status must be 1 to 255 characters with no spaces |

  Scenario: A meter a plan prices is locked until no plan prices it
    Given I generate a unique resource name from "locked" and store it as "meterId"
    And I generate a unique resource name from "payg" and store it as "planId"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Locked calls ${CTX:meterId}","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/subscription-plans" with body:
      """
      {"id":"${CTX:planId}","displayName":"Pay as you go ${CTX:planId}","status":"ACTIVE","pricing":{"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]}}
      """
    Then the response status code should be 201

    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response field "planCount" should be 1
    When I send a "PUT" request to the control plane at "/meters/${CTX:meterId}" with body:
      """
      {"displayName":"Renamed calls","type":"REQUEST_COUNT","filterFields":[{"field":"Status","eventKey":"response.status"}]}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "METER_IN_USE"
    And the JSON response field "message" should be "The meter is in use, so it can't be updated."
    When I send a "DELETE" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 409
    And the JSON response field "code" should be "METER_IN_USE"
    And the JSON response field "message" should be "The meter is in use, so it can't be deleted."

    # Once the plan is gone, so is the lock.
    When I send a "DELETE" request to the control plane at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 204
    When I send a "DELETE" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 204

  Scenario: Publishers can read meters, only administrators define them, and subscribers see none
    Given I generate a unique resource name from "roles" and store it as "meterId"
    When I send a "POST" request to the control plane at "/meters" with body:
      """
      {"id":"${CTX:meterId}","displayName":"Role check ${CTX:meterId}","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 201

    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}" as "publisher"
    Then the response status code should be 200
    When I send a "POST" request to the control plane at "/meters" as "publisher" with body:
      """
      {"displayName":"Publisher meter","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "GET" request to the control plane at "/meters" as "developer"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
