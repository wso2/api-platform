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

@mcp-gateway-url
Feature: MCP gateway URL precedence
  As an MCP client
  I want protected-resource metadata to use the configured gateway URL
  So that authentication challenges advertise the externally reachable resource

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: gatewayurl takes precedence over vhost and gatewayhost in mcp-auth's 401 challenge
    Given I generate a unique resource name from "mcp-auth-gwurl" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-gwurl" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-gwurl" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-gwurl" and store it as "mcpContext"
    And I generate a unique resource name from "mcp-auth-gwurl-vhost" and store it as "vhostName"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                 |
      | name              | ${CTX:mcpName}                            |
      | displayName       | ${CTX:mcpDisplayName}                     |
      | version           | ${CTX:mcpVersion}                         |
      | context           | ${CTX:mcpContext}                         |
      | specVersion       | 2025-06-18                                |
      | spec.gatewayurl   | https://mcp-e2e-gatewayurl.example.com:7777 |
      | spec.vhost        | ${CTX:vhostName}.example.com              |
      | spec.upstream.url | http://testbench:3009/mcp                 |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful

    Given I set request host to "${CTX:vhostName}.example.com"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    And the response header "WWW-Authenticate" should contain "https://mcp-e2e-gatewayurl.example.com:7777${CTX:mcpContext}/.well-known/oauth-protected-resource"

    When I send a "GET" request to "${CTX:mcpContext}/.well-known/oauth-protected-resource"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "resource" should be "https://mcp-e2e-gatewayurl.example.com:7777${CTX:mcpContext}/mcp"
    And the JSON response field "authorization_servers[0]" should be "http://testbench:3001/token"

    When I reset the request
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: gatewayurl takes precedence over vhost and gatewayhost in mcp-authz's 403 challenge
    Given I generate a unique resource name from "mcp-authz-gwurl" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-gwurl" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-gwurl" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-gwurl" and store it as "mcpContext"
    And I generate a unique resource name from "mcp-authz-gwurl-vhost" and store it as "vhostName"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                                                                                                                                                        |
      | name              | ${CTX:mcpName}                                                                                                                                                                   |
      | displayName       | ${CTX:mcpDisplayName}                                                                                                                                                            |
      | version           | ${CTX:mcpVersion}                                                                                                                                                                |
      | context           | ${CTX:mcpContext}                                                                                                                                                                |
      | specVersion       | 2025-06-18                                                                                                                                                                       |
      | spec.gatewayurl   | https://mcp-e2e-gatewayurl.example.com:7777                                                                                                                                      |
      | spec.vhost        | ${CTX:vhostName}.example.com                                                                                                                                                     |
      | spec.upstream.url | http://testbench:3009/mcp                                                                                                                                                        |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","scopes":{"anyOf":["add-scope"]}}]}}] |
    Then the response should be successful

    Given I set request host to "${CTX:vhostName}.example.com"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "https://mcp-e2e-gatewayurl.example.com:7777${CTX:mcpContext}/.well-known/oauth-protected-resource"
    And the response header "WWW-Authenticate" should contain "add-scope"

    When I reset the request
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
