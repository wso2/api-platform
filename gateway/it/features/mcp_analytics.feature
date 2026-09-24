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

Feature: Analytics published for MCP proxies
    As an operator
    I want each MCP request recorded with the operation it invoked
    So that usage can be attributed to a tool rather than to one shared route

    # Every JSON-RPC method reaches the same route, so the request URI cannot say which
    # operation ran: without these properties an MCP event is indistinguishable from any other
    # on the proxy. The analytics policy takes them from the operation resolver, which reads the
    # body once for the whole chain, and falls back to parsing the body itself where no resolver
    # ran. Both paths must publish the same properties, so this asserts the published result.

    Background:
        Given the gateway services are running

    Scenario: MCP request properties name the method and the tool invoked
        Given I authenticate using basic auth as "admin"
        And I reset the analytics collector
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-analytics-v1.0
            spec:
              displayName: MCP Analytics
              version: v1.0
              context: /mcpanalytics
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcpanalytics/mcp"
        And I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpanalytics/mcp"
        And I wait 5 seconds for analytics to be published
        Then the analytics collector should have received at least 1 event
        And the latest analytics event should have MCP field "jsonRpcMethod" with value "tools/call"
        And the latest analytics event should have MCP field "capabilityName" with value "add"
        And the latest analytics event should have MCP field "capability" with value "TOOL"
