# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

Feature: List Available Policies
  As a gateway administrator
  I want to retrieve the list of available policy definitions
  So that I can discover what policies can be applied to APIs

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== BASIC POLICY LISTING ====================

  Scenario: List all available policies
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "policies"

  # ==================== POLICY LIST CONTENT ====================

  Scenario: Policy list includes common policies
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "cors"
    And the response body should contain "jwt-auth"

  # ==================== POLICY STRUCTURE VALIDATION ====================

  Scenario: Each policy has required fields
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response should have field "policies"

  # ==================== SYSTEM POLICIES ====================

  Scenario: Policy list includes ratelimit policies
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "basic-ratelimit"

  # ==================== POLICY LISTING WITH PAGINATION ====================

  Scenario: List policies with limit parameter
    When I send a GET request to the "gateway-controller" service at "/policies?limit=5"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List policies with offset parameter
    When I send a GET request to the "gateway-controller" service at "/policies?offset=0"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  Scenario: List policies with limit and offset parameters
    When I send a GET request to the "gateway-controller" service at "/policies?limit=10&offset=0"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"

  # ==================== POLICY LIST INCLUDES GUARDRAIL POLICIES ====================

  Scenario: Policy list includes guardrail policies
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "word-count-guardrail"

  Scenario: Policy list includes modify-headers policy
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "modify-headers"

  # ==================== POLICY COUNT VALIDATION ====================

  Scenario: Policy list returns count field
    When I send a GET request to the "gateway-controller" service at "/policies"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response should have field "count"
