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
                version: v0
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

  # ==================== CONFIG DUMP WITH MCP PROXY ====================

  Scenario: Config dump includes deployed MCP proxy
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Mcp
      metadata:
        name: config-dump-mcp-v1.0
      spec:
        displayName: ConfigDump-MCP
        version: v1.0
        context: /config-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the MCP proxy "config-dump-mcp-v1.0"
    Then the response should be successful

  # ==================== CONFIG DUMP WITH LLM PROVIDER ====================

  Scenario: Config dump includes deployed LLM provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: config-dump-llm-provider
      spec:
        displayName: ConfigDump LLM Provider
        version: v1.0
        template: openai
        upstream:
          url: https://mock-openapi-https:9443/openai/v1
          auth:
            type: api-key
            header: Authorization
            value: Bearer sk-test-key
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the LLM provider "config-dump-llm-provider"
    Then the response status code should be 200

  # ==================== CONFIG DUMP WITH MULTIPLE RESOURCE TYPES ====================

  Scenario: Config dump includes mixed resource types - API, MCP, and LLM provider
    # Deploy API
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: mixed-resources-api
      spec:
        displayName: Mixed-Resources-API
        version: v1.0
        context: /mixed-api
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Deploy MCP
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Mcp
      metadata:
        name: mixed-resources-mcp-v1.0
      spec:
        displayName: Mixed-Resources-MCP
        version: v1.0
        context: /mixed-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    # Deploy LLM Provider
    When I create this LLM provider:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: LlmProvider
      metadata:
        name: mixed-resources-llm
      spec:
        displayName: Mixed Resources LLM
        version: v1.0
        template: openai
        upstream:
          url: https://mock-openapi-https:9443/openai/v1
        accessControl:
          mode: allow_all
      """
    Then the response status code should be 201
    # Get config dump
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "mixed-resources-api"
    # Cleanup
    When I delete the API "mixed-resources-api"
    Then the response should be successful
    When I delete the MCP proxy "mixed-resources-mcp-v1.0"
    Then the response should be successful
    When I delete the LLM provider "mixed-resources-llm"
    Then the response status code should be 200

  # ==================== CONFIG DUMP AFTER DELETION ====================

  Scenario: Config dump reflects removed API after deletion
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: deletion-test-api
      spec:
        displayName: Deletion-Test-API
        version: v1.0
        context: /deletion-test
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /data
      """
    Then the response should be successful
    # Verify API is in config dump
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response body should contain "deletion-test-api"
    # Delete the API
    When I delete the API "deletion-test-api"
    Then the response should be successful
    # Verify API is removed from config dump
    When I send a GET request to the "gateway-controller" service at "/config_dump"
    Then the response status should be 200
    And the response body should not contain "deletion-test-api"
