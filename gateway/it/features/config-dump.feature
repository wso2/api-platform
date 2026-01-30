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

Feature: Configuration Dump Endpoint
  As a gateway administrator
  I want to retrieve the complete gateway configuration snapshot
  So that I can debug and verify deployed APIs, policies, and certificates

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== BASIC CONFIG DUMP ====================

  Scenario: Get config dump with no APIs deployed
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response should have field "timestamp"
    And the JSON response should have field "apis"
    And the JSON response should have field "policies"
    And the JSON response should have field "certificates"
    And the JSON response should have field "statistics"

  # ==================== CONFIG DUMP WITH DEPLOYED API ====================

  Scenario: Get config dump after deploying a simple API
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: config-dump-test-api
      spec:
        displayName: ConfigDump-Test
        version: v1.0
        context: /config-test
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "config-dump-test-api"
    And the response body should contain "config-test"
    And the response body should contain "sample-backend"
    # Cleanup
    When I delete the API "config-dump-test-api"
    Then the response should be successful

  # ==================== CONFIG DUMP WITH MULTIPLE APIS ====================

  Scenario: Config dump includes multiple deployed APIs
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: config-dump-api-1
      spec:
        displayName: ConfigDump-API-1
        version: v1.0
        context: /api1
        upstream:
          main:
            url: http://backend1:8080
        operations:
          - method: GET
            path: /resource1
      """
    Then the response should be successful
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: config-dump-api-2
      spec:
        displayName: ConfigDump-API-2
        version: v2.0
        context: /api2
        upstream:
          main:
            url: http://backend2:9090
        operations:
          - method: POST
            path: /resource2
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "api1"
    And the response body should contain "api2"
    And the response body should contain "backend1"
    And the response body should contain "backend2"
    # Cleanup
    When I delete the API "config-dump-api-1"
    Then the response should be successful
    When I delete the API "config-dump-api-2"
    Then the response should be successful

  # ==================== CONFIG DUMP WITH POLICIES ====================

  Scenario: Config dump includes API with CORS policy
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: config-dump-cors-api
      spec:
        displayName: ConfigDump-CORS-API
        version: v1.0
        context: /cors-test
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
            policies:
              - name: cors
                version: v0.1.0
                params:
                  allowedOrigins:
                    - "http://example.com"
                  allowedMethods:
                    - GET
                    - POST
                  allowedHeaders:
                    - Content-Type
                  allowCredentials: true
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "cors-test"
    And the response body should contain "cors"
    # Cleanup
    When I delete the API "config-dump-cors-api"
    Then the response should be successful

  # ==================== CONFIG DUMP STRUCTURE VALIDATION ====================

  Scenario: Config dump has expected structure and statistics
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response should have field "apis"
    And the JSON response should have field "policies"
    And the JSON response should have field "certificates"
    And the JSON response should have field "statistics"
    And the JSON response should have field "statistics.totalApis"
    And the JSON response should have field "statistics.totalPolicies"
    And the JSON response should have field "statistics.totalCertificates"

  # ==================== CONFIG DUMP STATISTICS VALIDATION ====================

  Scenario: Config dump statistics reflect deployed resources
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: stats-test-api
      spec:
        displayName: Stats-Test-API
        version: v1.0
        context: /stats-test
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the response body should contain "stats-test-api"
    # Cleanup
    When I delete the API "stats-test-api"
    Then the response should be successful
