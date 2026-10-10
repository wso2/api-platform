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

@mcp-ratelimit-policy
Feature: MCP proxies throttled by mcp-ratelimit
  As an API developer
  I want mcp-ratelimit to count calls per capability rather than per route
  So that one heavily used tool cannot exhaust the quota of every other one

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

  @gateway-v1.2
  Scenario: An MCP proxy with mcp-ratelimit throttles a specific tool
    Given I generate a unique resource name from "mcp-ratelimit-tool" and store it as "mcpName"
    And I generate a unique value from "mcp-ratelimit-tool" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-ratelimit-tool" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-ratelimit-tool" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-ratelimit","version":"v1","params":{"tools":[{"name":"add","limits":[{"limit":2,"duration":"1m"}]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # First two "add" calls are within the limit and carry rate-limit headers
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
    And the response header "X-RateLimit-Limit" should be "2"
    And the response header "X-RateLimit-Remaining" should be "1"
    And the response header "RateLimit-Policy" should exist
    And the response header "Retry-After" should not exist

    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    # Third "add" call exceeds the limit and is throttled
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 429
    And the response should be valid JSON
    And the JSON response field "error.code" should be "-32000"
    And the response header "X-RateLimit-Limit" should be "2"
    And the response header "X-RateLimit-Remaining" should be "0"
    And the response header "Retry-After" should exist

    # A different tool ("echo") has its own counter and is not affected
    When I use the MCP Client to send "echo" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "Hello, World!"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  @gateway-v1.2
  Scenario: An MCP proxy with mcp-ratelimit throttles a JSON-RPC method
    Given I generate a unique resource name from "mcp-ratelimit-method" and store it as "mcpName"
    And I generate a unique value from "mcp-ratelimit-method" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-ratelimit-method" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-ratelimit-method" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-ratelimit","version":"v1","params":{"methods":[{"name":"tools/list","limits":[{"limit":2,"duration":"1m"}]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # First two tools/list calls are within the limit and carry rate-limit headers
    When I use the MCP Client to send a tools/list request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"
    And the response header "X-RateLimit-Limit" should be "2"
    And the response header "X-RateLimit-Remaining" should be "1"
    And the response header "RateLimit-Policy" should exist
    And the response header "Retry-After" should not exist

    When I use the MCP Client to send a tools/list request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"

    # Third tools/list call exceeds the limit and is throttled
    When I use the MCP Client to send a tools/list request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 429
    And the response should be valid JSON
    And the JSON response field "error.code" should be "-32000"
    And the response header "X-RateLimit-Limit" should be "2"
    And the response header "X-RateLimit-Remaining" should be "0"
    And the response header "Retry-After" should exist

    # A different method (tools/call) is not affected by the tools/list limit
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The limit is per capability, so the policy has to know which one was invoked before it can
  # count. Counting is engine state carried across requests, which is why the third call is the
  # assertion rather than the first.
  Scenario: mcp-ratelimit counts a tool named by the mirrored header
    Given I generate a unique resource name from "mcp-rl-modern" and store it as "mcpName"
    And I generate a unique value from "mcp-rl-modern" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-rl-modern" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-rl-modern" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-ratelimit","version":"v1","params":{"tools":[{"name":"add","limits":[{"limit":2,"duration":"1m"}]}]}}] |
    Then the response should be successful

    # The warm-up is an echo call, which no rule targets, so waiting for the route to come live
    # does not spend the add quota the scenario is about to measure.
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/call"
    And I set header "Mcp-Name" to "echo"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"echo","arguments":{"message":"warmup"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers
    And I authenticate using basic auth as "admin"

    When I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful
    When I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful
    When I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 429

    # A rule naming one tool must not count another, or the limit would be the route's rather
    # than the capability's.
    When I use the MCP Client to send a "tools/call" request for "echo" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
