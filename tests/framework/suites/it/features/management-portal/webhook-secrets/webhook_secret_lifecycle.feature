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

@webhook-secret-lifecycle
Feature: Webhook secrets are issued once, rotated through a bounded set of versions and removed
  As an organization administrator
  I want to issue a webhook secret for an API Portal and rotate it without cutting off the portal
  So that a secret can be replaced while the old one still verifies, and is never shown again

  Background:
    Given I generate a unique resource name from "wsec" and store it as "secretId"

  Scenario: Creating a webhook secret returns its value once and only a masked value afterwards
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Portal secret"}
      """
    Then the response status code should be 201
    And the JSON response field "id" should be "${CTX:secretId}"
    And the JSON response field "type" should be "PORTAL"
    And the JSON response field "secret" should match pattern "^whsec_[0-9a-f]{64}$"
    And the JSON response field "versions[0].version" should be 1
    And the JSON response field "versions[0].maskedSecret" should match pattern "^whsec_\*{4}[0-9a-f]{4}$"
    And I store the JSON response field "secret" as "secretV1"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the response status code should be 200
    And the JSON response field "secret" should not exist
    And the JSON response array field "versions" should have 1 item
    And the JSON response field "versions[0].maskedSecret" should match pattern "^whsec_\*{4}[0-9a-f]{4}$"
    And the response body should not contain "${CTX:secretV1}"

  Scenario: The hash listed for a version is the hash a portal carries for that secret
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Portal secret"}
      """
    Then the response status code should be 201
    And I store the JSON response field "secret" as "secretV1"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then I store the JSON response field "versions[0].hash" as "listedHash"
    And the hash of the secret "secretV1" should equal the stored "listedHash"

  Scenario: A created secret is listed with versions but never its value
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Listed secret"}
      """
    Then the response status code should be 201
    And I store the JSON response field "secret" as "secretV1"
    When I send a "GET" request to the control plane at "/webhook-secrets"
    Then the response status code should be 200
    And the JSON response array "list" should contain an item with "id" equal to "${CTX:secretId}"
    And the JSON response array "list" item with "id" equal to "${CTX:secretId}" should have "name" equal to "Listed secret"
    And the response body should not contain "${CTX:secretV1}"

  Scenario: Creating a secret with an id that is already taken is refused
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"First"}
      """
    And the response status code should be 201
    When I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Second"}
      """
    Then the response status code should be 409
    And the JSON response field "code" should be "WEBHOOK_SECRET_EXISTS"

  Scenario: Rotating issues a new version and keeps the previous one valid
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Rotated secret"}
      """
    And I store the JSON response field "secret" as "secretV1"
    When I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    Then the response status code should be 201
    And the JSON response field "versions[0].version" should be 2
    And the JSON response field "secret" should match pattern "^whsec_[0-9a-f]{64}$"
    And the JSON response field "secret" should not equal "${CTX:secretV1}"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the JSON response array field "versions" should have 2 items
    And the JSON response field "versions[0].version" should be 2
    And the JSON response field "versions[1].version" should be 1

  Scenario: Rotating past the version limit is refused until a version is deleted
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Limited secret"}
      """
    And I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    And I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    And the JSON response field "versions[0].version" should be 3
    When I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    Then the response status code should be 409
    And the JSON response field "code" should be "WEBHOOK_SECRET_VERSION_LIMIT"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the JSON response array field "versions" should have 3 items
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/1"
    Then the response status code should be 204
    When I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    Then the response status code should be 201
    And the JSON response field "versions[0].version" should be 4

  Scenario: A version number is never handed out twice
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Counted secret"}
      """
    And I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    And I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/3"
    Then the response status code should be 204
    When I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    Then the response status code should be 201
    And the JSON response field "versions[0].version" should be 4
    # A delete that still names the removed version finds nothing and cannot touch the new one.
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/3"
    Then the response status code should be 404
    And the JSON response field "code" should be "WEBHOOK_SECRET_NOT_FOUND"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the JSON response array field "versions" should have 3 items
    And the JSON response field "versions[0].version" should be 4

  Scenario: The last remaining version cannot be deleted on its own
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Single version"}
      """
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/1"
    Then the response status code should be 409
    And the JSON response field "code" should be "WEBHOOK_SECRET_LAST_VERSION"
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the JSON response array field "versions" should have 1 item

  Scenario: Deleting a secret removes every version
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Removed secret"}
      """
    And I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the response status code should be 204
    When I send a "GET" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    Then the response status code should be 404
    And the JSON response field "code" should be "WEBHOOK_SECRET_NOT_FOUND"
    When I send a "GET" request to the control plane at "/webhook-secrets"
    Then the JSON response array "list" should not contain an item with "id" equal to "${CTX:secretId}"
