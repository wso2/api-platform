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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@audit
Feature: Audit records of management API changes
  As a gateway operator
  I want every change made through the management API recorded
  So that I can tell who changed what, when, and whether it was applied

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "audit-api" and store it as "apiName"
    And I generate a unique API version from "audit" and store it as "apiVersion"
    And I generate a unique API context from "/audit" and store it as "apiContext"
    And I generate a unique value from "audit-correlation" and store it as "correlationId"
    And I set header "X-Correlation-ID" to "${CTX:correlationId}"

  Scenario: Creating, updating and deleting an API is recorded in the table, the log and the receiver
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | ${CTX:apiVersion}                   |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}                                         |
      | name                   | ${CTX:apiName}                                                    |
      | spec.displayName       | ${CTX:apiName}                                                    |
      | spec.version           | ${CTX:apiVersion}                                                 |
      | spec.context           | ${CTX:apiContext}/$version                                        |
      | spec.upstream.main.url | http://testbench:3000/api/v2                                      |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/test"}] |
    Then the response should be successful
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And the gateway audit table should have exactly these audit records for "${CTX:apiName}":
      | action | resourceType | resourceName   | outcome   | performedBy | correlationId        |
      | CREATE | rest_api     | ${CTX:apiName} | succeeded | admin       | ${CTX:correlationId} |
      | UPDATE | rest_api     | ${CTX:apiName} | succeeded | admin       | ${CTX:correlationId} |
      | DELETE | rest_api     | ${CTX:apiName} | succeeded | admin       | ${CTX:correlationId} |
    And the gateway-controller log should have exactly these audit records for "${CTX:apiName}":
      | action | resourceType | outcome   | performedBy | correlationId        |
      | CREATE | rest_api     | succeeded | admin       | ${CTX:correlationId} |
      | UPDATE | rest_api     | succeeded | admin       | ${CTX:correlationId} |
      | DELETE | rest_api     | succeeded | admin       | ${CTX:correlationId} |
    And the audit receiver should have exactly these audit records for "${CTX:apiName}":
      | action | resourceType | outcome   | performedBy | correlationId        |
      | CREATE | rest_api     | succeeded | admin       | ${CTX:correlationId} |
      | UPDATE | rest_api     | succeeded | admin       | ${CTX:correlationId} |
      | DELETE | rest_api     | succeeded | admin       | ${CTX:correlationId} |

  Scenario: Subscription plan and API key changes are recorded
    Given I generate a unique value from "audit-plan" and store it as "planName"
    And I generate a unique value from "audit-key" and store it as "keyName"
    When I send a "POST" request to the "gateway-controller" service at "/subscription-plans" with body:
      """
      {"planName":"${CTX:planName}","throttleLimitCount":100,"throttleLimitUnit":"Min"}
      """
    Then the response status should be 201
    And I store the JSON response field "id" as "planId"
    And I register the "subscription-plan" "${CTX:planId}" for cleanup
    When I send a "DELETE" request to the "gateway-controller" service at "/subscription-plans/${CTX:planId}"
    Then the response status code should be 204
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | ${CTX:apiVersion}                   |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys" with body:
      """
      {"name":"${CTX:keyName}"}
      """
    Then the response status should be 201
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}/api-keys/${CTX:keyName}"
    Then the response status should be 200
    And the gateway audit table should have exactly these audit records for "${CTX:planId}":
      | action | resourceType      | resourceName    | outcome   | performedBy |
      | CREATE | subscription_plan | ${CTX:planName} | succeeded | admin       |
      | DELETE | subscription_plan |                 | succeeded | admin       |
    And the gateway audit table should have exactly these audit records for "${CTX:keyName}":
      | action | resourceType | outcome   | performedBy |
      | CREATE | api_key      | succeeded | admin       |
      | REVOKE | api_key      | succeeded | admin       |
    And the audit receiver should have exactly these audit records for "${CTX:keyName}":
      | action | resourceType | outcome   | performedBy |
      | CREATE | api_key      | succeeded | admin       |
      | REVOKE | api_key      | succeeded | admin       |

  Scenario: Reads and rejected requests are not recorded
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}           |
      | name                   | ${CTX:apiName}                      |
      | spec.displayName       | ${CTX:apiName}                      |
      | spec.version           | ${CTX:apiVersion}                   |
      | spec.context           | ${CTX:apiContext}/$version          |
      | spec.upstream.main.url | http://testbench:3000/api/v2        |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}"
    Then the response status should be 200
    When I send a "PUT" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}" with body:
      """
      {"kind":"RestApi"}
      """
    Then the response status should be 400
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    When I send a "DELETE" request to the "gateway-controller" service at "/rest-apis/${CTX:apiName}"
    Then the response status should be 404
    And the gateway audit table should have exactly these audit records for "${CTX:apiName}":
      | action | outcome   |
      | CREATE | succeeded |
      | DELETE | succeeded |
    And the gateway-controller log should have exactly these audit records for "${CTX:apiName}":
      | action | outcome   |
      | CREATE | succeeded |
      | DELETE | succeeded |
    And the audit receiver should have exactly these audit records for "${CTX:apiName}":
      | action | outcome   |
      | CREATE | succeeded |
      | DELETE | succeeded |
