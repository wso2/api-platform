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

@mcp-auth-gateway-host
Feature: mcp-auth and mcp-authz honor the deprecated gatewayHost when gatewayUrl is unset
  As an operator who has not yet migrated to gatewayUrl
  I want the deprecated gatewayHost parameter to keep building correct challenges and metadata
  So that upgrading the mcp-auth/mcp-authz policies does not break my existing configuration
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: mcp-auth defaults to port 8080 when the request's own Host header carries no port
    Given I generate a unique resource name from "mcp-auth-noport" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-noport" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-noport" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-noport" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful

    Given I set request host to "mcp-e2e-noport-probe.example.com"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    And the response header "WWW-Authenticate" should contain "http://mcp-e2e-gatewayhost.example.com:8080${CTX:mcpContext}/.well-known/oauth-protected-resource"

    When I reset the request
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-auth passes an explicit port in the request's Host header through verbatim
    Given I generate a unique resource name from "mcp-auth-port" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-port" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-port" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-port" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful

    Given I set request host to "mcp-e2e-explicit-port-probe.example.com:9999"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    And the response header "WWW-Authenticate" should contain "http://mcp-e2e-gatewayhost.example.com:9999${CTX:mcpContext}/.well-known/oauth-protected-resource"

    When I reset the request
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-auth builds its 401 challenge and protected-resource metadata from gatewayHost
    Given I generate a unique resource name from "mcp-auth-gwhost" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-gwhost" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-gwhost" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-gwhost" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    And the response header "WWW-Authenticate" should contain "http://mcp-e2e-gatewayhost.example.com"
    And the response header "WWW-Authenticate" should contain "${CTX:mcpContext}/.well-known/oauth-protected-resource"

    When I send a "GET" request to "${CTX:mcpContext}/.well-known/oauth-protected-resource"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "resource" should contain "http://mcp-e2e-gatewayhost.example.com"
    And the JSON response field "authorization_servers[0]" should be "http://testbench:3001/token"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz builds its 403 insufficient-scope challenge from gatewayHost
    Given I generate a unique resource name from "mcp-authz-gwhost" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-gwhost" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-gwhost" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-gwhost" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","scopes":{"anyOf":["add-scope"]}}]}}] |
    Then the response should be successful

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
    And the response header "WWW-Authenticate" should contain "http://mcp-e2e-gatewayhost.example.com"
    And the response header "WWW-Authenticate" should contain "${CTX:mcpContext}/.well-known/oauth-protected-resource"
    And the response header "WWW-Authenticate" should contain "add-scope"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
