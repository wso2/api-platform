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

@platform-api-mcp-server
Feature: The Platform API's MCP tools
  As an AI agent acting for a signed-in user
  I want to create, deploy and remove APIs with the Platform API's MCP tools
  So that what the tools do is what the REST API and the gateway do

  Background:
    Given the gateway services are running
    And I generate a unique value from "mcp-project" and store it as "project"
    And I create a project "${CTX:project}" on the control plane

  Scenario Outline: Each user sees the tools their role allows
    When I list the MCP tools as "<user>"
    Then the MCP tools should be "<tools>"

    Examples:
      | user      | tools |
      | admin     | list_apis, get_api, get_api_schema, list_api_deployments, create_api, update_api, deploy_api, undeploy_api, delete_api, list_projects, create_project, delete_project, list_gateways, get_gateway |
      | publisher | list_apis, get_api, get_api_schema, list_api_deployments, create_api, update_api, deploy_api, undeploy_api, delete_api, list_projects, create_project, delete_project, list_gateways, get_gateway |
      | developer | get_api, get_api_schema, list_apis, list_projects |

  Scenario: A project is created, listed and deleted through the tools
    Given I generate a unique value from "mcp-created" and store it as "projectName"
    When I call the MCP tool "create_project" with arguments:
      """
      {"name": "${CTX:projectName}"}
      """
    Then the MCP tool call should succeed
    And I store the JSON response field "result.structuredContent.id" as "createdId"
    When I call the MCP tool "list_projects" with arguments:
      """
      {"limit": 100}
      """
    Then the JSON response array "result.structuredContent.items" should contain an item with "id" equal to "${CTX:createdId}"
    When I call the MCP tool "delete_project" with arguments:
      """
      {"projectId": "${CTX:createdId}", "confirm": true}
      """
    Then the JSON response field "result.structuredContent.deleted" should be "${CTX:createdId}"
    When I send a "GET" request to the control plane at "/projects/${CTX:createdId}"
    Then the response status code should be 404

  Scenario: A REST API is created, deployed, served and removed through the tools, and REST agrees
    Given I generate a unique resource name from "mcp-orders" and store it as "apiName"
    And I generate a unique API context from "/mcp-orders" and store it as "apiContext"
    When I call the MCP tool "create_api" with arguments:
      """
      {"apiType": "rest-api", "payload": {"displayName": "${CTX:apiName}", "context": "${CTX:apiContext}",
       "version": "v1", "projectId": "${CTX:project}", "upstream": {"main": {"url": "http://testbench:3000"}},
       "operations": [{"request": {"method": "GET", "path": "/health"}}]}}
      """
    Then the MCP tool call should succeed
    And I store the JSON response field "result.structuredContent.id" as "apiId"
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiId}"
    Then the response status code should be 200
    And the JSON response field "displayName" should be "${CTX:apiName}"
    And the JSON response field "context" should be "${CTX:apiContext}"
    When I call the MCP tool "update_api" with arguments:
      """
      {"apiType": "rest-api", "apiId": "${CTX:apiId}", "payload": {"description": "changed by MCP"}}
      """
    Then the MCP tool call should succeed
    When I send a "GET" request to the control plane at "/rest-apis/${CTX:apiId}"
    Then the JSON response field "description" should be "changed by MCP"
    When I call the MCP tool "list_apis" with arguments:
      """
      {"apiType": "rest-api", "projectId": "${CTX:project}"}
      """
    Then the JSON response array "result.structuredContent.items" should contain an item with "id" equal to "${CTX:apiId}"
    When I call the MCP tool "list_gateways" with arguments:
      """
      {"limit": 100}
      """
    Then the JSON response array "result.structuredContent.items" should contain an item with "id" equal to "it-gateway"
    When I call the MCP tool "get_gateway" with arguments:
      """
      {"gatewayId": "it-gateway"}
      """
    Then the JSON response field "result.structuredContent.isActive" should be true
    When I call the MCP tool "deploy_api" with arguments:
      """
      {"apiType": "rest-api", "apiId": "${CTX:apiId}", "gatewayId": "it-gateway"}
      """
    Then the JSON response field "result.structuredContent.deployments[0].status" should be "DEPLOYING"
    When I call the MCP tool "list_api_deployments" until the JSON field "result.structuredContent.items[0].status" is "DEPLOYED" with arguments:
      """
      {"apiType": "rest-api", "apiId": "${CTX:apiId}"}
      """
    Then I send a "GET" request to "${CTX:apiContext}/health" until status 200
    When I call the MCP tool "undeploy_api" with arguments:
      """
      {"apiType": "rest-api", "apiId": "${CTX:apiId}", "gatewayId": "it-gateway", "confirm": true}
      """
    Then the JSON response array field "result.structuredContent.undeployed" should have 1 item
    And I send a "GET" request to "${CTX:apiContext}/health" until status 404
    When I call the MCP tool "delete_api" with arguments:
      """
      {"apiType": "rest-api", "apiId": "${CTX:apiId}", "confirm": true}
      """
    Then the JSON response field "result.structuredContent.deleted" should be "${CTX:apiId}"
    When I call the MCP tool "get_api" with arguments:
      """
      {"apiType": "rest-api", "apiId": "${CTX:apiId}"}
      """
    Then the MCP tool call should fail with "The specified REST API could not be found."

  Scenario: The LLM provider, LLM proxy and MCP proxy types are wired
    Given I generate a unique resource name from "mcp-provider" and store it as "providerId"
    And I generate a unique resource name from "mcp-llm-proxy" and store it as "proxyId"
    And I generate a unique resource name from "mcp-mcp-proxy" and store it as "mcpProxyId"
    And I generate a unique API context from "/mcp-provider" and store it as "providerContext"
    And I generate a unique API context from "/mcp-llm-proxy" and store it as "proxyContext"
    And I generate a unique API context from "/mcp-mcp-proxy" and store it as "mcpProxyContext"
    When I call the MCP tool "get_api_schema" with arguments:
      """
      {"apiType": "llm-provider"}
      """
    Then the MCP tool call should succeed
    When I call the MCP tool "create_api" with arguments:
      """
      {"apiType": "llm-provider", "payload": {"id": "${CTX:providerId}", "displayName": "${CTX:providerId}",
       "version": "v1.0", "context": "${CTX:providerContext}", "template": "openai",
       "upstream": {"main": {"url": "http://testbench:3000",
        "auth": {"type": "api-key", "header": "Authorization", "value": "mcp-test-key"}}},
       "accessControl": {"mode": "allow_all"}}}
      """
    Then the MCP tool call should succeed
    When I call the MCP tool "get_api" with arguments:
      """
      {"apiType": "llm-provider", "apiId": "${CTX:providerId}"}
      """
    Then the MCP tool call should succeed
    And the JSON response field "result.structuredContent.api.upstream.main.auth.header" should be "Authorization"
    And the JSON response field "result.structuredContent.api.upstream.main.auth.value" should not exist
    When I call the MCP tool "create_api" with arguments:
      """
      {"apiType": "llm-proxy", "payload": {"id": "${CTX:proxyId}", "displayName": "${CTX:proxyId}",
       "version": "v1.0", "projectId": "${CTX:project}", "context": "${CTX:proxyContext}",
       "provider": {"id": "${CTX:providerId}"}}}
      """
    Then the MCP tool call should succeed
    When I call the MCP tool "get_api" with arguments:
      """
      {"apiType": "llm-proxy", "apiId": "${CTX:proxyId}"}
      """
    Then the JSON response field "result.structuredContent.api.provider.id" should be "${CTX:providerId}"
    When I call the MCP tool "create_api" with arguments:
      """
      {"apiType": "mcp-proxy", "payload": {"id": "${CTX:mcpProxyId}", "displayName": "${CTX:mcpProxyId}",
       "version": "v1.0", "projectId": "${CTX:project}", "context": "${CTX:mcpProxyContext}",
       "mcpSpecVersion": "2025-06-18", "upstream": {"main": {"url": "http://testbench:3009${CTX:gatewayMCPUpstreamPath}"}}}}
      """
    Then the MCP tool call should succeed
    When I call the MCP tool "get_api" with arguments:
      """
      {"apiType": "mcp-proxy", "apiId": "${CTX:mcpProxyId}"}
      """
    Then the JSON response field "result.structuredContent.api.context" should be "${CTX:mcpProxyContext}"
    When I call the MCP tool "delete_api" with arguments:
      """
      {"apiType": "mcp-proxy", "apiId": "${CTX:mcpProxyId}", "confirm": true}
      """
    Then the JSON response field "result.structuredContent.deleted" should be "${CTX:mcpProxyId}"
    When I call the MCP tool "delete_api" with arguments:
      """
      {"apiType": "llm-proxy", "apiId": "${CTX:proxyId}", "confirm": true}
      """
    Then the JSON response field "result.structuredContent.deleted" should be "${CTX:proxyId}"
    When I call the MCP tool "delete_api" with arguments:
      """
      {"apiType": "llm-provider", "apiId": "${CTX:providerId}", "confirm": true}
      """
    Then the JSON response field "result.structuredContent.deleted" should be "${CTX:providerId}"

  Scenario Outline: A tool that cannot do what it was asked answers with one sentence
    When I call the MCP tool "<tool>" with arguments:
      """
      <arguments>
      """
    Then the MCP tool call should fail with "<sentence>"

    Examples:
      | tool        | arguments                                                  | sentence                                                                                                              |
      | get_api     | {"apiType": "rest-api", "apiId": "nope"}                   | The specified REST API could not be found.                                                                            |
      | delete_api  | {"apiType": "rest-api", "apiId": "nope", "confirm": false} | refusing to run delete_api without confirm=true. Set it only for the exact target the user asked for, then call again |
      | get_gateway | {"gatewayId": "no-such-gw"}                                | The specified gateway could not be found.                                                                             |

  Scenario Outline: A tool the user's role does not allow is refused
    When I call the MCP tool "<tool>" as "<user>" with arguments:
      """
      <arguments>
      """
    Then the MCP tool call should fail with "<sentence>"

    Examples:
      | user      | tool           | arguments                                  | sentence |
      | publisher | create_api     | {"apiType": "llm-provider", "payload": {}} | missing permission: this needs ap:llm_provider:create or ap:llm_provider:manage |
      | developer | create_api     | {"apiType": "rest-api", "payload": {}}     | missing permission: this needs ap:rest_api:create or ap:rest_api:manage |
      | developer | create_project | {"name": "${UNIQUE:refused-project}"}      | missing permission: this needs ap:project:create or ap:project:manage |

  Scenario Outline: A request without a valid token is refused before the tools
    When I list the MCP tools with the token "<token>"
    Then the response status code should be 401

    Examples:
      | token   |
      |         |
      | garbage |

  Scenario: A request from another site is refused before the tools
    When I call the MCP tool "list_projects" with header "Sec-Fetch-Site" set to "cross-site"
    Then the response status code should be 403
