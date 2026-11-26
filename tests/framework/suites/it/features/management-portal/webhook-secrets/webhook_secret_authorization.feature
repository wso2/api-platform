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

@webhook-secret-authorization
Feature: Webhook secrets are managed only by an authorized administrator of the owning organization
  As a platform operator
  I want every webhook secret operation to need its own permission and stay inside one organization
  So that a publisher, a developer or another organization can never read or change a portal's secret

  Background:
    Given I generate a unique resource name from "wsec-auth" and store it as "secretId"
    And I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Guarded secret"}
      """
    And the response status code should be 201
    And I store the JSON response field "secret" as "secretV1"

  Scenario Outline: A user without the webhook secret permissions is refused: <actor> <method> <path>
    When I send a "<method>" request to the control plane at "<path>" as "<actor>"
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    And the response body should not contain "${CTX:secretV1}"

    Examples:
      | actor     | method | path                                          |
      | publisher | GET    | /webhook-secrets                              |
      | publisher | GET    | /webhook-secrets/${CTX:secretId}              |
      | publisher | POST   | /webhook-secrets/${CTX:secretId}/rotate       |
      | publisher | DELETE | /webhook-secrets/${CTX:secretId}/versions/1   |
      | publisher | DELETE | /webhook-secrets/${CTX:secretId}              |
      | developer | GET    | /webhook-secrets                              |
      | developer | GET    | /webhook-secrets/${CTX:secretId}              |
      | developer | POST   | /webhook-secrets/${CTX:secretId}/rotate       |
      | developer | DELETE | /webhook-secrets/${CTX:secretId}              |

  Scenario: A user without the create permission cannot create a secret
    When I send a "POST" request to the control plane at "/webhook-secrets" as "publisher" with body:
      """
      {"id":"${CTX:secretId}-pub","name":"Publisher secret"}
      """
    Then the response status code should be 403
    And the JSON response field "code" should be "FORBIDDEN"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}-pub"
    Then the response status code should be 404
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}-pub","name":"Owner secret"}
      """
    Then the response status code should be 201
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}-pub"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:secretId}-pub"

  Scenario Outline: Nobody manages webhook secrets without signing in: <method> <path>
    When I send a "<method>" request to the control plane at "<path>" without signing in
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"

    Examples:
      | method | path                                          |
      | GET    | /webhook-secrets                              |
      | POST   | /webhook-secrets                              |
      | GET    | /webhook-secrets/${CTX:secretId}              |
      | POST   | /webhook-secrets/${CTX:secretId}/rotate       |
      | DELETE | /webhook-secrets/${CTX:secretId}/versions/1   |
      | DELETE | /webhook-secrets/${CTX:secretId}              |

  Scenario: Another organization cannot see a secret and sees it as not found
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}" as "org B admin"
    Then the response status code should be 404
    And the JSON response field "code" should be "WEBHOOK_SECRET_NOT_FOUND"
    And the response body should not contain "${CTX:secretId}"
    When I send a "GET" request to the control plane at "/webhook-secrets" as "org B admin"
    Then the response status code should be 200
    And the JSON response array "list" should not contain an item with "id" equal to "${CTX:secretId}"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the response status code should be 200
    And the JSON response field "id" should be "${CTX:secretId}"
    When I send a "GET" request to the control plane at "/webhook-secrets"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:secretId}"

  Scenario: Another organization cannot rotate or delete a secret
    When I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate" as "org B admin"
    Then the response status code should be 404
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/1" as "org B admin"
    Then the response status code should be 404
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}" as "org B admin"
    Then the response status code should be 404
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the response status code should be 200
    And the JSON response array field "versions" should have 1 item

  Scenario: Two organizations can each hold a secret with the same id
    Given a webhook secret "${CTX:secretId}" exists in the second organization and its value is stored as "orgBSecret"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the response status code should be 200
    And the JSON response field "name" should be "Guarded secret"

  Scenario: Error responses never reveal internals or a secret value
    When I send a "GET" request to the control plane at "/webhook-secrets/does-not-exist-${CTX:secretId}"
    Then the response status code should be 404
    And the JSON response field "message" should be "The specified webhook secret could not be found."
    And the response body should not contain "sql"
    And the response body should not contain "goroutine"
    And the response body should not contain "does-not-exist"
