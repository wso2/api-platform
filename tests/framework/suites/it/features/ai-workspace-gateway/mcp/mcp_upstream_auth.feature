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
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

@mcp-upstream-auth
Feature: MCP proxy upstream API key authentication
  As an API developer
  I want the gateway to send my MCP proxy's upstream API key on every forwarded request
  So that an MCP server protected by an API key accepts traffic through the proxy

  Each proxy's upstream is the testbench capture service, which answers with the request the
  gateway actually forwarded, so every scenario asserts the credential the upstream received.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I resolve the "capture" service URL at "" and store it as "captureUpstream"

  Scenario: An api-key upstream credential is sent with every forwarded MCP request
    Given I generate a unique resource name from "mcp-apikey" and store it as "mcpName"
    And I generate a unique value from "mcp-apikey" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-apikey" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-apikey" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"api-key","header":"X-API-Key","value":"mcp-apikey-credential"} |
    Then the response should be successful
    And the JSON response field "status.state" should be "deployed"

    When I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it-suite","version":"1.0.0"}}}
      """
    Then the response should contain echoed header "X-API-Key" with value "mcp-apikey-credential"

  Scenario: A proxy using the legacy "header" auth type is accepted, stored as api-key, and sends its credential
    Given I generate a unique resource name from "mcp-legacy" and store it as "mcpName"
    And I generate a unique value from "mcp-legacy" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-legacy" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-legacy" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"header","header":"X-API-Key","value":"mcp-legacy-credential"} |
    Then the response should be successful
    And the JSON response field "status.state" should be "deployed"

    When I get the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
    And the JSON response field "spec.upstream.auth.type" should be "api-key"
    And the JSON response field "spec.upstream.auth.header" should be "X-API-Key"

    When I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it-suite","version":"1.0.0"}}}
      """
    Then the response should contain echoed header "X-API-Key" with value "mcp-legacy-credential"

  Scenario: The legacy "header" auth type still requires a credential value
    Given I generate a unique resource name from "mcp-legacy-noval" and store it as "mcpName"
    And I generate a unique value from "mcp-legacy-noval" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-legacy-noval" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-legacy-noval" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"header","header":"X-API-Key"}                           |
    Then the response status code should be 400

  Scenario: An update that omits the credential keeps sending the stored one
    Given I generate a unique resource name from "mcp-inherit" and store it as "mcpName"
    And I generate a unique value from "mcp-inherit" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-inherit" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-inherit" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"api-key","header":"X-API-Key","value":"mcp-inherit-credential"} |
    Then the response should be successful

    When I update MCP proxy "${CTX:mcpName}" from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"header","header":"X-API-Key"}                           |
    Then the response should be successful
    When I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it-suite","version":"1.0.0"}}}
      """
    Then the response should contain echoed header "X-API-Key" with value "mcp-inherit-credential"

    When I update MCP proxy "${CTX:mcpName}" from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"api-key"}                                               |
    Then the response should be successful
    When I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it-suite","version":"1.0.0"}}}
      """
    Then the response should contain echoed header "X-API-Key" with value "mcp-inherit-credential"

  Scenario: Switching a header and value credential to policyParams sends the new credential
    Given I generate a unique resource name from "mcp-policyparams" and store it as "mcpName"
    And I generate a unique value from "mcp-policyparams" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-policyparams" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-policyparams" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"api-key","header":"X-API-Key","value":"mcp-original-credential"} |
    Then the response should be successful

    When I update MCP proxy "${CTX:mcpName}" from "resources/templates/mcp.yaml" with values:
      | apiVersion         | ${CTX:gatewaySpecVersion}                                        |
      | name               | ${CTX:mcpName}                                                   |
      | displayName        | ${CTX:mcpDisplayName}                                            |
      | version            | ${CTX:mcpVersion}                                                |
      | context            | ${CTX:mcpContext}                                                |
      | specVersion        | 2025-06-18                                                       |
      | spec.upstream.url  | ${CTX:captureUpstream}/${CTX:mcpName}${CTX:gatewayMCPUpstreamPath} |
      | spec.upstream.auth | {"type":"api-key","policyParams":{"request":{"headers":[{"name":"X-API-Key","value":"mcp-policyparams-credential"}]}}} |
    Then the response should be successful
    When I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until the response body contains "mcp-policyparams-credential" with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"it-suite","version":"1.0.0"}}}
      """
    Then the response should contain echoed header "X-API-Key" with value "mcp-policyparams-credential"
