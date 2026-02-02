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

Feature: Search Deployments API
  As an API administrator
  I want to search for deployed APIs and MCP proxies with filters
  So that I can find specific deployments easily

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== SEARCH APIs (using /apis endpoint with filters) ====================

  Scenario: Search APIs with no filters returns all APIs
    # First deploy some APIs
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: search-api-1
      spec:
        displayName: Search-API-One
        version: v1.0
        context: /search-one
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: search-api-2
      spec:
        displayName: Search-API-Two
        version: v2.0
        context: /search-two
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # List APIs (no filter = returns all)
    When I send a GET request to the "gateway-controller" service at "/apis"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "search-api-1"
    Then the response should be successful
    When I delete the API "search-api-2"
    Then the response should be successful

  Scenario: Search APIs by displayName filter
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: displayname-search-api
      spec:
        displayName: UniqueDisplayName
        version: v1.0
        context: /displayname-search
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Search by displayName via /apis endpoint
    When I send a GET request to the "gateway-controller" service at "/apis?displayName=UniqueDisplayName"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "UniqueDisplayName"
    # Cleanup
    When I delete the API "displayname-search-api"
    Then the response should be successful

  Scenario: Search APIs by version filter
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: version-search-api
      spec:
        displayName: Version-Search-API
        version: v3.0.0
        context: /version-search
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Search by version via /apis endpoint
    When I send a GET request to the "gateway-controller" service at "/apis?version=v3.0.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "version-search-api"
    Then the response should be successful

  Scenario: Search APIs by context filter
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: context-search-api
      spec:
        displayName: Context-Search-API
        version: v1.0
        context: /unique-context-path
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Search by context via /apis endpoint
    When I send a GET request to the "gateway-controller" service at "/apis?context=/unique-context-path"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "context-search-api"
    Then the response should be successful

  Scenario: Search APIs by status filter
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: status-search-api
      spec:
        displayName: Status-Search-API
        version: v1.0
        context: /status-search
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    And I wait for 2 seconds
    # Search by status via /apis endpoint
    When I send a GET request to the "gateway-controller" service at "/apis?status=DEPLOYED"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "status-search-api"
    Then the response should be successful

  Scenario: Search APIs with multiple filters
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: RestApi
      metadata:
        name: multi-filter-api
      spec:
        displayName: MultiFilterAPI
        version: v5.0
        context: /multi-filter
        upstream:
          main:
            url: http://sample-backend:9080
        operations:
          - method: GET
            path: /test
      """
    Then the response should be successful
    # Search with multiple filters via /apis endpoint
    When I send a GET request to the "gateway-controller" service at "/apis?displayName=MultiFilterAPI&version=v5.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the API "multi-filter-api"
    Then the response should be successful

  Scenario: Search APIs with non-matching filter returns empty
    When I send a GET request to the "gateway-controller" service at "/apis?displayName=NonExistentAPIName12345"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0

  # ==================== SEARCH MCP PROXIES (using /mcp-proxies endpoint with filters) ====================

  Scenario: Search MCP proxies with no filters
    # Deploy an MCP proxy first
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Mcp
      metadata:
        name: search-mcp-v1.0
      spec:
        displayName: SearchMCP
        version: v1.0
        context: /search-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    # List MCP proxies via /mcp-proxies endpoint
    When I send a GET request to the "gateway-controller" service at "/mcp-proxies"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "mcp_proxies"
    # Cleanup
    When I delete the MCP proxy "search-mcp-v1.0"
    Then the response should be successful

  Scenario: Search MCP proxies by displayName filter
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Mcp
      metadata:
        name: displayname-mcp-v1.0
      spec:
        displayName: UniqueMCPDisplayName
        version: v1.0
        context: /displayname-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    # Search by displayName via /mcp-proxies endpoint
    When I send a GET request to the "gateway-controller" service at "/mcp-proxies?displayName=UniqueMCPDisplayName"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the response body should contain "UniqueMCPDisplayName"
    # Cleanup
    When I delete the MCP proxy "displayname-mcp-v1.0"
    Then the response should be successful

  Scenario: Search MCP proxies by version filter
    When I deploy this MCP configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Mcp
      metadata:
        name: version-mcp-v2.0
      spec:
        displayName: VersionMCP
        version: v2.0
        context: /version-mcp
        specVersion: "2025-06-18"
        upstream:
          url: http://mcp-server-backend:3001
        tools: []
        resources: []
        prompts: []
      """
    Then the response should be successful
    # Search by version via /mcp-proxies endpoint
    When I send a GET request to the "gateway-controller" service at "/mcp-proxies?version=v2.0"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    # Cleanup
    When I delete the MCP proxy "version-mcp-v2.0"
    Then the response should be successful

  Scenario: Search MCP proxies with non-matching filter
    When I send a GET request to the "gateway-controller" service at "/mcp-proxies?displayName=NonExistentMCP12345"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "status" should be "success"
    And the JSON response field "count" should be 0

