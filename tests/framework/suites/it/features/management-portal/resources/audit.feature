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

@platform-api-audit
Feature: Audit records of control-plane changes
  As a platform operator
  I want every change recorded with who made it
  So that I can tell who changed what in the control plane, and when

  Scenario: Every change is recorded in the audit table and every output
    Given I generate a unique resource name from "audit-gw" and store it as "gateway"
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:gateway}", "displayName": "${CTX:gateway}", "endpoints": ["http://audit.example.com"], "functionalityType": "regular"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the response status code should be 201
    And I store the JSON response field "id" as "token"
    When I send a "DELETE" request to the control plane at "/gateways/${CTX:gateway}/tokens/${CTX:token}"
    Then the response status code should be 200
    And the platform-api audit table should have exactly these audit records for "${CTX:token}":
      | action | resourceType  | organizationId                       |
      | CREATE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 |
      | REVOKE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 |
    And the platform-api log should have exactly these audit records for "${CTX:token}":
      | action | resourceType  | organizationId                       | performedBy |
      | CREATE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 | admin       |
      | REVOKE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 | admin       |
    And the platform-api audit file should have exactly these audit records for "${CTX:token}":
      | action | resourceType  | organizationId                       | performedBy |
      | CREATE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 | admin       |
      | REVOKE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 | admin       |
    And the platform-api audit receiver should have exactly these audit records for "${CTX:token}":
      | action | resourceType  | organizationId                       | performedBy |
      | CREATE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 | admin       |
      | REVOKE | gateway_token | 8f3c2a1e-6d4b-4e7a-9c1f-2b5d7e9a0c31 | admin       |

  Scenario: Reads and rejected requests are not recorded
    Given I generate a unique resource name from "audit-gw" and store it as "gateway"
    When I send a "POST" request to the control plane at "/gateways" with body:
      """
      {"id": "${CTX:gateway}", "displayName": "${CTX:gateway}", "endpoints": ["http://audit.example.com"], "functionalityType": "regular"}
      """
    Then the response status code should be 201
    When I send a "POST" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the response status code should be 201
    And I store the JSON response field "id" as "token"
    When I send a "GET" request to the control plane at "/gateways/${CTX:gateway}/tokens"
    Then the response status code should be 200
    When I send a "DELETE" request to the control plane at "/gateways/${CTX:gateway}-missing/tokens/${CTX:token}"
    Then the response status code should be 404
    When I send a "DELETE" request to the control plane at "/gateways/${CTX:gateway}/tokens/${CTX:token}"
    Then the response status code should be 200
    And the platform-api audit table should have exactly these audit records for "${CTX:token}":
      | action |
      | CREATE |
      | REVOKE |
    And the platform-api log should have exactly these audit records for "${CTX:token}":
      | action |
      | CREATE |
      | REVOKE |
    And the platform-api audit file should have exactly these audit records for "${CTX:token}":
      | action |
      | CREATE |
      | REVOKE |
    And the platform-api audit receiver should have exactly these audit records for "${CTX:token}":
      | action |
      | CREATE |
      | REVOKE |
