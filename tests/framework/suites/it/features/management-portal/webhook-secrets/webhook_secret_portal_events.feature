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

@webhook-secret-portal-events
Feature: Platform API finds the secret of a portal's events from the organization and the secret hash
  As an API Portal that holds a webhook secret issued by the control plane
  I want my signed events accepted with that secret, found by the hash I send
  So that each portal has a secret of its own, and a retired or foreign secret is never accepted

  Background:
    Given I generate a unique resource name from "wsec-evt" and store it as "secretId"
    And I generate a unique resource name from "wsec-meter" and store it as "meterId"
    And I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}","name":"Portal events"}
      """
    And the response status code should be 201
    And I store the JSON response field "secret" as "secretV1"

  Scenario: An event signed with a portal's own secret and naming its hash is accepted
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Per-portal meter","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    And the JSON response field "status" should be "processed"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Per-portal meter"

  Scenario: An event signed with a portal secret but naming no hash is still accepted
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" without a secret hash with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Legacy portal meter","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200

  Scenario: Every retained version verifies, and a deleted version stops verifying
    Given I send a "POST" request to the control plane at "/webhook-secrets/${CTX:secretId}/rotate"
    And I store the JSON response field "secret" as "secretV2"
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Signed with the older version","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When the API Portal sends the "meter.updated" event to the control plane signed with the secret "secretV2" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Signed with the newer version","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}/versions/1"
    Then the response status code should be 204
    When the API Portal sends the "meter.updated" event to the control plane signed with the secret "secretV1" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Signed with the deleted version","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "displayName" should be "Signed with the newer version"

  Scenario: A secret hash that names a different secret than the one that signed is rejected
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}-b","name":"Second portal"}
      """
    And I store the JSON response field "secret" as "secretB"
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" naming the secret "secretB" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Mismatched hash","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404

  Scenario: The portal that holds each secret is found by its own hash
    Given I send a "POST" request to the control plane at "/webhook-secrets" with body:
      """
      {"id":"${CTX:secretId}-b","name":"Second portal"}
      """
    And I store the JSON response field "secret" as "secretB"
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretB" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"From the second portal","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When the API Portal sends the "meter.updated" event to the control plane signed with the secret "secretV1" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"From the first portal","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the JSON response field "displayName" should be "From the first portal"

  Scenario Outline: A hash that names no held secret is rejected the same way as a bad signature: <case>
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" naming the secret hash "<hash>" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Unknown hash","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    And the response body should not contain "hash"
    And the response body should not contain "secret"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404

    Examples:
      | case                  | hash                                                             |
      | a well-formed unknown | 0000000000000000000000000000000000000000000000000000000000000000 |
      | not hexadecimal       | not-a-hash                                                       |
      | an upper-case hash       | DB2D454D8112DB2D454D8112DB2D454D8112DB2D454D8112DB2D454D8112DB2D |

  Scenario: An event whose body changed after it was signed is rejected
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" with a tampered body with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Tampered","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404

  Scenario: A deleted secret no longer verifies anything
    Given I send a "DELETE" request to the control plane at "/webhook-secrets/${CTX:secretId}"
    And the response status code should be 204
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"After delete","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "secretV1" without a secret hash with data:
      """
      {"handle":"${CTX:meterId}","display_name":"After delete, no hash","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401

  Scenario: Another organization's secret cannot sign an event for this organization
    Given a webhook secret "${CTX:secretId}-other" exists in the second organization and its value is stored as "orgBSecret"
    When the API Portal sends the "meter.created" event to the control plane signed with the secret "orgBSecret" with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Foreign secret","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 401
    And the JSON response field "code" should be "UNAUTHORIZED"
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 404

  Scenario: The shared webhook secret keeps working next to per-portal secrets
    When the API Portal sends the "meter.created" event to the control plane with data:
      """
      {"handle":"${CTX:meterId}","display_name":"Shared secret meter","type":"REQUEST_COUNT"}
      """
    Then the response status code should be 200
    When I send a "GET" request to the control plane at "/meters/${CTX:meterId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "Shared secret meter"
