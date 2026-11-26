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

@webhook-secret-validation
Feature: Webhook secret requests are validated
  As an organization administrator
  I want a malformed webhook secret request refused clearly
  So that nothing half-formed is stored and a mistake is easy to correct

  Background:
    Given I generate a unique resource name from "wsec-val" and store it as "secretId"

  Scenario Outline: A secret with an invalid id or name is refused: <case>
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      <body>
      """
    Then the response status code should be 400
    And the JSON response field "code" should be "VALIDATION_FAILED"
    When I send a "GET" request to the control plane at "/webhook-secrets"
    Then the response status code should be 200

    Examples:
      | case                        | body                                                                    |
      | missing id                  | {"name":"No id"}                                                        |
      | id with upper case and space | {"id":"Bad Id!","name":"Bad id"}                                       |
      | id longer than 40 characters | {"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"Long id"}    |
      | missing name                | {"id":"no-name-${CTX:secretId}"}                                        |
      | empty name                  | {"id":"empty-name-${CTX:secretId}","name":""}                           |
      | unknown type                | {"id":"bad-type-${CTX:secretId}","name":"Bad type","type":"OTHER"}      |

  Scenario: A secret of the PORTAL type may be named explicitly
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Typed secret","type":"PORTAL"}
      """
    Then the response status code should be 201
    And the JSON response field "type" should be "PORTAL"

  Scenario: A request body that is not JSON is refused
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      this is not json
      """
    Then the response status code should be 400

  Scenario Outline: An unknown secret or version is reported as not found: <method> <path>
    When I send a "<method>" request to the control plane at "<path>"
    Then the response status code should be <status>
    And the JSON response field "code" should be "WEBHOOK_SECRET_NOT_FOUND"

    Examples:
      | method | path                                                  | status |
      | GET    | /webhook-secrets/missing-${CTX:secretId}              | 404    |
      | POST   | /webhook-secrets/missing-${CTX:secretId}/rotate       | 404    |
      | DELETE | /webhook-secrets/missing-${CTX:secretId}/versions/1   | 404    |
      | DELETE | /webhook-secrets/missing-${CTX:secretId}              | 404    |

  Scenario: An unknown version of an existing secret is reported as not found
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Two versions"}
      """
    And I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/9"
    Then the response status code should be 404
    And the JSON response field "code" should be "WEBHOOK_SECRET_NOT_FOUND"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the JSON response array field "versions" should have 2 items

  Scenario Outline: A version that is not a positive whole number is refused: <version>
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Version check"}
      """
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/<version>"
    Then the response status code should be 400

    Examples:
      | version |
      | abc     |
      | 0       |
      | -1      |
