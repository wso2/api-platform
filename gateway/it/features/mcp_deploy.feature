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

Feature: Test MCP CRUD and connectivity
    As an API developer
    I want to deploy an MCP Proxy configuration and connect to it
    So that I can verify the gateway routes the MCP requests correctly

    Background:
        Given the gateway services are running
        
    Scenario: Deploy a sample MCP Server and do a tools/call
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: everything-mcp-v1.0
            spec:
              displayName: Everything
              version: v1.0
              context: /everything
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

        Given I authenticate using basic auth as "admin"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 1
        And I wait for 2 seconds
    
        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/everything/mcp"
        Then the response should be successful
        When I use the MCP Client to send a tools/call request to "http://127.0.0.1:8080/everything/mcp"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response should have field "result"
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
        
        Given I authenticate using basic auth as "admin"
        When I update the MCP proxy "everything-mcp-v1.0" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: everything-mcp-v1.0
            spec:
              displayName: Everything
              version: v1.0
              context: /everything
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "everything-mcp-v1.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 0

    # ==================== MCP PROXY ERROR CASES ====================
    
    Scenario: List MCP proxies when none exist
        Given I authenticate using basic auth as "admin"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 0

    Scenario: List MCP proxies with pagination parameters
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies?limit=10&offset=0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: Get non-existent MCP proxy returns 404
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies/non-existent-mcp-id"
        Then the response status should be 404
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Get MCP proxy with invalid ID format returns 404
        Given I authenticate using basic auth as "admin"
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies/invalid@mcp#id"
        Then the response status should be 404
        And the response should be valid JSON

    Scenario: Delete non-existent MCP proxy returns 404
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "non-existent-mcp-delete"
        Then the response status should be 404
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Update non-existent MCP proxy returns 404
        Given I authenticate using basic auth as "admin"
        When I update the MCP proxy "non-existent-mcp-update" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: non-existent-mcp-update
            spec:
              version: v1.0
              context: /test
              upstream:
                url: http://test:3001
            """
        Then the response status should be 404
        And the response should be valid JSON

    Scenario: Deploy an MCP Proxy with labels and verify they are stored
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: labeled-mcp-v1.0
              labels:
                environment: production
                team: mcp-team
                service: mcp-proxy
            spec:
              displayName: Labeled MCP
              version: v1.0
              context: /labeled-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds
        
        Given I authenticate using basic auth as "admin"
        When I get the MCP proxy "labeled-mcp-v1.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "mcp.configuration.metadata.labels.environment" should be "production"
        And the JSON response field "mcp.configuration.metadata.labels.team" should be "mcp-team"
        And the JSON response field "mcp.configuration.metadata.labels.service" should be "mcp-proxy"
        
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "labeled-mcp-v1.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"

    Scenario: Deploy an MCP Proxy with invalid labels (spaces in keys) should fail
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: invalid-mcp-labels-v1.0
              labels:
                "Invalid Key": value
            spec:
              displayName: Invalid MCP Labels
              version: v1.0
              context: /invalid-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        And the response body should contain "configuration validation failed"

    # ==================== MCP PROXY ADDITIONAL ERROR CASES ====================

    Scenario: Deploy MCP proxy with invalid JSON body returns error
        Given I authenticate using basic auth as "admin"
        When I send a POST request to the "gateway-controller" service at "/mcp-proxies" with body:
            """
            { this is not valid json content
            """
        Then the response should be a client error
        And the response should be valid JSON

    Scenario: Deploy MCP proxy with missing required fields returns error
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: incomplete-mcp-v1.0
            spec:
              displayName: Incomplete MCP
            """
        Then the response should be a client error
        And the response should be valid JSON
        And the JSON response field "status" should be "error"

    Scenario: Deploy duplicate MCP proxy returns conflict
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: duplicate-mcp-v1.0
            spec:
              displayName: Duplicate MCP
              version: v1.0
              context: /duplicate-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        # Try to create duplicate
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: duplicate-mcp-v1.0
            spec:
              displayName: Duplicate MCP
              version: v1.0
              context: /duplicate-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response status should be 409
        And the response should be valid JSON
        And the JSON response field "status" should be "error"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "duplicate-mcp-v1.0"
        Then the response should be successful

    Scenario: Update MCP proxy with invalid JSON body returns error
        Given I authenticate using basic auth as "admin"
        When I send a PUT request to the "gateway-controller" service at "/mcp-proxies/some-mcp" with body:
            """
            { invalid json body
            """
        Then the response should be a client error
        And the response should be valid JSON

    # ==================== MCP PROXY FILTER TESTS ====================

    Scenario: List MCP proxies with displayName filter
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: filter-test-mcp-v1.0
            spec:
              displayName: UniqueMCPFilterTest
              version: v1.0
              context: /filter-test-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies?displayName=UniqueMCPFilterTest"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the response body should contain "UniqueMCPFilterTest"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "filter-test-mcp-v1.0"
        Then the response should be successful

    Scenario: List MCP proxies with version filter
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: version-test-mcp-v99.0
            spec:
              displayName: Version Test MCP
              version: v99.0
              context: /version-test-mcp
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        When I send a GET request to the "gateway-controller" service at "/mcp-proxies?version=v99.0"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "version-test-mcp-v99.0"
        Then the response should be successful

