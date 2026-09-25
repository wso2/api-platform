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

@mcp-rewrite-policy
Feature: MCP proxies rewritten by mcp-rewrite
  As an API developer
  I want mcp-rewrite to present an upstream tool under a name of my choosing
  So that the name a client sees is independent of the name the server implements

  # Both protocol eras are here, and deliberately so: this policy's whole behaviour should be
  # readable in one place. A scenario whose steps say "declaring MCP version" sends a 2026-07-28
  # request, which mirrors the method and capability name into headers; the others send
  # handshake-era requests, which mirror nothing.
  #
  # The runner is gated to gateway builds later than 1.2.0 because the modern scenarios need the
  # operation resolver, which no released 1.2.0 carries.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: An MCP proxy with mcp-rewrite renames a tool and its schema
    Given I generate a unique resource name from "mcp-rewrite" and store it as "mcpName"
    And I generate a unique value from "mcp-rewrite" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-rewrite" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-rewrite" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-rewrite","version":"v1","params":{"tools":[{"name":"sum","description":"Take the sum of two numbers","target":"add","inputSchema":"{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"additionalProperties\":false,\"properties\":{\"a\":{\"description\":\"First number\",\"type\":\"number\"},\"b\":{\"description\":\"Second number\",\"type\":\"number\"}},\"required\":[\"a\",\"b\"],\"type\":\"object\"}"}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I use the MCP Client to send "sum" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # This is the one scenario whose oracle is the upstream rather than an assertion. The policy
  # renames the tool in the body, and 2026-07-28 requires the mirrored header to name the same
  # capability - so a rewrite that changed only the body would be refused by the server with
  # -32020, and the call could not succeed. A successful result is therefore proof that the
  # policy restated the header as part of the same change.
  Scenario: mcp-rewrite renames a tool and restates the mirrored header with it
    Given I generate a unique resource name from "mcp-rw-modern" and store it as "mcpName"
    And I generate a unique value from "mcp-rw-modern" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-rw-modern" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-rw-modern" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-rewrite","version":"v1","params":{"tools":[{"name":"sum","description":"Take the sum of two numbers","target":"add","inputSchema":"{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"additionalProperties\":false,\"properties\":{\"a\":{\"description\":\"First number\",\"type\":\"number\"},\"b\":{\"description\":\"Second number\",\"type\":\"number\"}},\"required\":[\"a\",\"b\"],\"type\":\"object\"}"}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/call"
    And I set header "Mcp-Name" to "sum"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"sum","arguments":{"a":1,"b":2},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I use the MCP Client to send a "tools/call" request for "sum" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful
    And the response body should contain "The sum of 40 and 60 is 100."

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # Rewriting a header to match a body it disagrees with would turn the server's mandatory check
  # into a rubber stamp: a caller could present one capability for the gateway's policies to
  # govern and have another executed. So the policy verifies before it rewrites, and a request
  # that contradicts itself is refused whichever capability it names.
  Scenario: mcp-rewrite refuses a request whose mirrored name contradicts its body
    Given I generate a unique resource name from "mcp-rw-contradiction" and store it as "mcpName"
    And I generate a unique value from "mcp-rw-contradiction" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-rw-contradiction" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-rw-contradiction" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-rewrite","version":"v1","params":{"tools":[{"name":"sum","description":"Take the sum of two numbers","target":"add","inputSchema":"{\"$schema\":\"http://json-schema.org/draft-07/schema#\",\"additionalProperties\":false,\"properties\":{\"a\":{\"description\":\"First number\",\"type\":\"number\"},\"b\":{\"description\":\"Second number\",\"type\":\"number\"}},\"required\":[\"a\",\"b\"],\"type\":\"object\"}"}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/call"
    And I set header "Mcp-Name" to "sum"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"sum","arguments":{"a":1,"b":2},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers
    And I authenticate using basic auth as "admin"

    # The body still names "sum"; the header claims the backend name directly.
    When I set header "Mcp-Name" to "add"
    And I use the MCP Client to send a "tools/call" request for "sum" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 400
    And the JSON response field "error.code" should be "-32020"
    And the response body should contain "Mcp-Name does not match the capability in the request body"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
