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


@APIPORTAL @apiportal-meter
Feature: Meters in the API Portal
  As an API Portal administrator who sells usage-based plans
  I want to define what a plan counts, and which properties its prices can filter on
  So that each plan I sell charges for exactly the calls or tokens it is meant to

  # Scenarios APIPORTAL (scenarios/APIPORTAL.yaml) at the API level.
  # Meters are an administrator-only resource. Every refused create or change is followed by a
  # read that shows nothing was stored or changed.

  @APIP-040 @type:smoke
  Scenario: An administrator defines request and token meters, which get ids from their names
    # An administrator defines a meter, finds it, replaces it and removes it
    Given I generate a unique resource name from "portal-calls" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                                                                         |
      | displayName  | API calls ${CTX:meterId}                                                                               |
      | description  | Every call a subscriber makes                                                                          |
      | type         | REQUEST_COUNT                                                                                          |
      | filterFields | [{"field":"Status code","eventKey":"response.status"},{"field":"HTTP method","eventKey":"request.verb"}] |
    Then the response status code should be 201
    And the JSON response field "id" should be "${CTX:meterId}"
    And the JSON response field "type" should be "REQUEST_COUNT"
    And the JSON response field "typeKey" should be null
    And the JSON response field "planCount" should be 0
    And the JSON response field "filterFields[0].field" should be "Status code"

    When I send an authenticated API Portal "GET" request to "/meters?limit=100" as "admin"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:meterId}"

    # A replacement that leaves out the filter properties clears them.
    When I send an authenticated API Portal "PUT" request to "/meters/${CTX:meterId}" as "admin" with JSON values:
      | displayName | All calls ${CTX:meterId} |
      | type        | REQUEST_COUNT            |
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "All calls ${CTX:meterId}"
    And the JSON response field "description" should be null
    And the JSON response array field "filterFields" should have 0 items

    When I send an authenticated API Portal "DELETE" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404
    And the JSON response field "message" should be "Meter not found"

    # A meter named in words gets an id from its name, and a second one with that name gets another id
    Given I generate a unique resource name from "ok" and store it as "name"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "firstMeter":
      | displayName | Successful calls ${CTX:name} |
      | type        | REQUEST_COUNT                |
    Then the JSON response field "id" should be "successful-calls-${CTX:name}"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "secondMeter":
      | displayName | Successful calls ${CTX:name} |
      | type        | REQUEST_COUNT                |
    Then the JSON response field "id" should contain "successful-calls-${CTX:name}-"

    # An id the administrator chooses is not changed: a taken one is refused.
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | successful-calls-${CTX:name} |
      | displayName | Another meter                |
      | type        | REQUEST_COUNT                |
    Then the response status code should be 409
    And the JSON response field "message" should be:
      """
      A meter with id "successful-calls-${CTX:name}" already exists in this organization.
      """
    When I send an authenticated API Portal "PUT" request to "/meters/successful-calls-${CTX:name}" as "admin" with JSON values:
      | id          | renamed-${CTX:name}          |
      | displayName | Successful calls ${CTX:name} |
      | type        | REQUEST_COUNT                |
    Then the response status code should be 400
    And the JSON response field "message" should be "The id of a meter cannot be changed"
    When I send an authenticated API Portal "GET" request to "/meters/successful-calls-${CTX:name}" as "admin"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Successful calls ${CTX:name}"
    When I send an authenticated API Portal "GET" request to "/meters/renamed-${CTX:name}" as "admin"
    Then the response status code should be 404

    # A token meter counts the tokens under its type key
    Given I generate a unique resource name from "portal-tokens" and store it as "meterId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                               |
      | displayName  | Prompt tokens ${CTX:meterId}                                 |
      | type         | TOKENS                                                       |
      | typeKey      | metadata.aiTokenUsage.promptTokens                           |
      | filterFields | [{"field":"Model","eventKey":"metadata.aiMetadata.model"}] |
    Then the JSON response field "type" should be "TOKENS"
    And the JSON response field "typeKey" should be "metadata.aiTokenUsage.promptTokens"

  @APIP-040 @type:negative @type:security
  Scenario: Invalid meters are refused, a priced meter is locked, and only administrators can define meters
    # A meter that sets the read-only plan count is refused
    Given I generate a unique resource name from "portal-plan-count-meter" and store it as "meterId"
    And I register API Portal resource "${CTX:meterId}" at "/meters" for cleanup
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | REQUEST_COUNT  |
      | planCount   | 3              |
    Then the response status code should be 400

    # A meter that counts tokens without a type key is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-refused-meter" and store it as "meterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | TOKENS         |
      | description     | Counts tokens        |
    Then the response status code should be 400
    And the JSON response field "message" should be "A TOKENS meter needs a typeKey"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404

    # A meter that counts calls with a type key is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-refused-meter" and store it as "meterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | REQUEST_COUNT         |
      | typeKey     | usage.total        |
    Then the response status code should be 400
    And the JSON response field "message" should be "typeKey applies only to a TOKENS meter"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404

    # A meter that has a field the portal does not know is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-refused-meter" and store it as "meterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | REQUEST_COUNT         |
      | colour     | red        |
    Then the response status code should be 400
    And the JSON response field "message" should be "colour is not a known field"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404

    # A meter that names a property twice is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-refused-meter" and store it as "meterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | REQUEST_COUNT         |
      | filterFields     | [{"field":"Status","eventKey":"response.status"},{"field":"status","eventKey":"x"}]        |
    Then the response status code should be 400
    And the JSON response field "message" should be "The meter has the property status more than once"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404

    # A meter that reads one event key under two properties is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-refused-meter" and store it as "meterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | REQUEST_COUNT         |
      | filterFields     | [{"field":"Status","eventKey":"code"},{"field":"Code","eventKey":"code"}]        |
    Then the response status code should be 400
    And the JSON response field "message" should be "Code and Status read the same event key code"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404

    # A meter that has an event key with a space is refused with the reason and nothing is stored
    Given I generate a unique resource name from "portal-refused-meter" and store it as "meterId"
    When I send an authenticated API Portal "POST" request to "/meters" as "admin" with JSON values:
      | id          | ${CTX:meterId} |
      | displayName | Calls          |
      | type        | REQUEST_COUNT         |
      | filterFields     | [{"field":"Status","eventKey":"response status"}]        |
    Then the response status code should be 400
    And the JSON response field "message" should be "The event key of Status must be 1 to 255 characters with no spaces"
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404

    # A meter a plan prices is locked until the plan is deleted
    Given I generate a unique resource name from "portal-locked" and store it as "meterId"
    And I generate a unique resource name from "portal-payg" and store it as "planId"
    And a unique API Portal resource is created at "/meters" as "admin" with values and stored as "meterId":
      | id           | ${CTX:meterId}                                     |
      | displayName  | Locked calls ${CTX:meterId}                        |
      | type         | REQUEST_COUNT                                      |
      | filterFields | [{"field":"Status","eventKey":"response.status"}] |
    And a unique API Portal resource is created at "/subscription-plans" as "admin" with values and stored as "planId":
      | id          | ${CTX:planId}                                                                                                                                                                                                  |
      | displayName | Pay as you go ${CTX:planId}                                                                                                                                                                                    |
      | limits      | [{"limitType":"REQUEST_COUNT","limitCount":1000,"timeUnit":"MINUTE"}]                                                                                                                                       |
      | pricing     | {"pricingType":"USAGE_BASED","currency":"USD","billingPeriod":"MONTH","prices":[{"meterId":"${CTX:meterId}","filters":[{"field":"Status","operator":"EQUALS","values":["200"]}],"pricingModel":"PER_UNIT","unitAmount":"0.01","unitQuantity":1}]} |

    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the JSON response field "planCount" should be 1
    When I send an authenticated API Portal "PUT" request to "/meters/${CTX:meterId}" as "admin" with JSON values:
      | displayName  | Renamed calls                                      |
      | type         | REQUEST_COUNT                                      |
      | filterFields | [{"field":"Status","eventKey":"response.status"}] |
    Then the response status code should be 409
    And the JSON response field "code" should be "METER_IN_USE"
    And the JSON response field "message" should be "The meter is in use, so it can't be updated."
    When I send an authenticated API Portal "DELETE" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 409
    And the JSON response field "code" should be "METER_IN_USE"
    And the JSON response field "message" should be "The meter is in use, so it can't be deleted."
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the JSON response field "displayName" should be "Locked calls ${CTX:meterId}"

    When I send an authenticated API Portal "DELETE" request to "/subscription-plans/${CTX:planId}" as "admin"
    Then the response status code should be 204
    When I send an authenticated API Portal "DELETE" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 204

    # Only administrators can see and define meters
    Given I generate a unique resource name from "portal-developer-meter" and store it as "meterId"
    When I send an authenticated API Portal "GET" request to "/meters" as "admin"
    Then the response status code should be 200
    When I send an authenticated API Portal "GET" request to "/meters" as "publisher"
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/meters" as "developer"
    Then the response status code should be 403
    When I send an unauthenticated API Portal "GET" request to "/meters"
    Then the response status code should be 401
    When I send an authenticated API Portal "POST" request to "/meters" as "developer" with JSON values:
      | id          | ${CTX:meterId}  |
      | displayName | Developer meter |
      | type        | REQUEST_COUNT   |
    Then the response status code should be 403
    When I send an authenticated API Portal "GET" request to "/meters/${CTX:meterId}" as "admin"
    Then the response status code should be 404
