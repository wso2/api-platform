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

@policy-engine @admin
Feature: Policy Engine Admin API
  As a gateway administrator
  I want to access the policy engine admin API
  So that I can inspect the current configuration and debug issues

  Background:
    Given the gateway services are running

  # ============================================================
  # Basic Endpoint Tests
  # ============================================================

  Scenario: Config dump endpoint returns valid JSON
    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the response content type should be "application/json"
    And the response should be valid JSON

  Scenario: Config dump contains policy registry section
    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the response JSON should have key "policy_registry"
    And the response JSON at "policy_registry" should have key "total_policies"
    And the response JSON at "policy_registry" should have key "policies"

  Scenario: Config dump contains routes section
    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the response JSON should have key "routes"
    And the response JSON at "routes" should have key "total_routes"
    And the response JSON at "routes" should have key "route_configs"

  Scenario: Config dump contains lazy resources section
    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the response JSON should have key "lazy_resources"
    And the response JSON at "lazy_resources" should have key "total_resources"

  # ============================================================
  # Route Configuration Tests
  # ============================================================

  Scenario: Config dump reflects deployed API routes
    Given I deploy a test API with the following configuration:
      """
      name: admin-test-api
      version: v1
      basePath: /admin-test
      backend:
        url: http://sample-backend:9080
      operations:
        - method: GET
          path: /info
          policies:
            - name: modify-headers
              version: v0.1.0
              params:
                requestHeadersToAdd:
                  - name: X-Test-Header
                    value: test-value
      """
    And I wait for 3 seconds for xDS synchronization
    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the response JSON at "routes.total_routes" should be greater than 0
    And the config dump should contain route with basePath "/admin-test"
    And I delete the API "admin-test-api" version "v1"

  Scenario: Config dump reflects API deletion
    Given I deploy a test API with the following configuration:
      """
      name: admin-delete-test-api
      version: v1
      basePath: /admin-delete-test
      backend:
        url: http://sample-backend:9080
      operations:
        - method: GET
          path: /info
      """
    And I wait for 3 seconds for xDS synchronization
    When I send a GET request to the policy-engine config dump endpoint
    Then the config dump should contain route with basePath "/admin-delete-test"
    When I delete the API "admin-delete-test-api" version "v1"
    And I wait for 3 seconds for xDS synchronization
    And I send a GET request to the policy-engine config dump endpoint
    Then the config dump should not contain route with basePath "/admin-delete-test"

  Scenario: Config dump shows policy parameters
    Given I deploy a test API with the following configuration:
      """
      name: admin-policy-params-api
      version: v1
      basePath: /admin-policy-params
      backend:
        url: http://sample-backend:9080
      operations:
        - method: GET
          path: /test
          policies:
            - name: modify-headers
              version: v0.1.0
              params:
                requestHeadersToAdd:
                  - name: X-Custom-Header
                    value: custom-value
      """
    And I wait for 3 seconds for xDS synchronization
    When I send a GET request to the policy-engine config dump endpoint
    Then the response status code should be 200
    And the config dump should contain policy "modify-headers" for route "/admin-policy-params"
    And I delete the API "admin-policy-params-api" version "v1"

  # ============================================================
  # Method Validation Tests
  # ============================================================

  Scenario: POST request to config dump returns 405 Method Not Allowed
    When I send a POST request to the policy-engine config dump endpoint
    Then the response status code should be 405
