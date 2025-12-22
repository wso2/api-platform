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
        
    Scenario: Deploy a sample MCP Server
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

    Scenario: List all MCP proxies
        Given I authenticate using basic auth as "admin"
        When I list all MCP proxies
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And the JSON response field "count" should be 1
        
    Scenario: Update an existing MCP proxy
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

    Scenario: Delete an MCP proxy
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

