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

@mcp-spec-validation-policy
Feature: MCP requests checked by mcp-spec-validation
  As an operator
  I want mcp-spec-validation to reject a request whose headers and body disagree
  So that the mirrored headers every other MCP policy reads can be trusted

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

  # A legacy request mirrors nothing, so there is nothing for this policy to compare. It must
  # forward rather than fault, or attaching it would break every client that has not migrated.
  Scenario: mcp-spec-validation forwards a legacy request that mirrors nothing
    Given I generate a unique resource name from "mcp-val-legacy" and store it as "mcpName"
    And I generate a unique value from "mcp-val-legacy" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-val-legacy" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-val-legacy" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-spec-validation","version":"v0","params":{}}] |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The header says one operation and the body another. Whichever the upstream would have
  # executed, the gateway's own policies would have governed the other one - so the request is
  # refused before it travels.
  Scenario: mcp-spec-validation refuses a modern request whose header contradicts its body
    Given I generate a unique resource name from "mcp-val-mismatch" and store it as "mcpName"
    And I generate a unique value from "mcp-val-mismatch" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-val-mismatch" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-val-mismatch" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-spec-validation","version":"v0","params":{}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/list"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """

    # The header is set last and wins over the one the step would have mirrored, which is how a
    # scenario describes a request no conformant client would send.
    When I set header "Mcp-Method" to "tools/list"
    And I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 400
    And the JSON response field "error.code" should be "-32020"
    # The gateway's wording. The upstream's for the same request reads "header mismatch: ...",
    # so this is what says the request never left.
    And the response body should contain "Mcp-Method does not match the method in the request body"
    And the response body should not contain "header mismatch:"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The body names a member twice, under spellings that encoding/json folds together. The
  # gateway reads one and the upstream may read the other, so the request cannot be governed.
  # mcp_resolver.feature sends this same body with no policy attached and it is forwarded; the
  # difference between the two is entirely this policy.
  Scenario: mcp-spec-validation refuses a body that names a member more than once
    Given I generate a unique resource name from "mcp-val-ambiguous" and store it as "mcpName"
    And I generate a unique value from "mcp-val-ambiguous" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-val-ambiguous" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-val-ambiguous" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2025-06-18                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-spec-validation","version":"v0","params":{}}] |
    Then the response should be successful
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp" until successful

    When I use the MCP Client to send this request to "${CTX:mcpContext}/mcp":
      """
      {"jsonrpc":"2.0","id":1,"method":"tools/list","Method":"tools/call","params":{}}
      """
    Then the response status code should be 400
    And the JSON response field "error.code" should be "-32600"
    And the response body should contain "Request body names a member more than once"

    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The reason this policy exists is to be attached AHEAD of the policies that read the mirrored
  # headers, so those headers can be trusted by the time they are governed on. That makes the
  # chain, not the policy alone, the thing worth asserting - and chain order is invisible to a
  # unit test, which only ever sees one policy.
  #
  # The pair below is one request shape answered two ways. mcp-auth's challenge carries a
  # WWW-Authenticate header and this policy's rejection does not, so the header says which policy
  # answered, without relying on a status code both could have produced.
  Scenario: mcp-spec-validation refuses a contradictory request before mcp-auth can challenge it
    Given I generate a unique resource name from "mcp-val-chain" and store it as "mcpName"
    And I generate a unique value from "mcp-val-chain" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-val-chain" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-val-chain" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-spec-validation","version":"v0","params":{}},{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/call"
    And I set header "Mcp-Name" to "echo"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"echo","arguments":{"message":"warmup"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers

    # The header names a tool no rule governs; the body invokes the one that is governed. Left to
    # itself, a governing policy on this route decides from the header and never sees the lie.
    When I set header "Mcp-Name" to "echo"
    And I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 400
    And the JSON response field "error.code" should be "-32020"
    # This policy's own wording. mcp-rewrite says "the capability" for the same condition, so the
    # phrase also says which of the two answered.
    And the response body should contain "Mcp-Name does not match the capability name in the request body"
    # Not mcp-auth's 401 and not the upstream's own check: this policy answered first.
    And the response header "WWW-Authenticate" should not exist
    And the response body should not contain "header mismatch:"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The control for the scenario above. Without it, that one would pass just as well if this
  # policy rejected every modern request, which is the failure mode a "rejects correctly" test
  # cannot tell apart from correctness.
  Scenario: A request whose headers agree with its body reaches the policies behind it
    Given I generate a unique resource name from "mcp-val-chain-ok" and store it as "mcpName"
    And I generate a unique value from "mcp-val-chain-ok" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-val-chain-ok" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-val-chain-ok" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-spec-validation","version":"v0","params":{}},{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/call"
    And I set header "Mcp-Name" to "add"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"add","arguments":{"a":1,"b":2},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    And I clear all headers

    # Unauthenticated, so mcp-auth answers - which it can only do if this policy forwarded.
    When I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 401
    And the response header "WWW-Authenticate" should exist

    # And with a token carrying the scope the rule demands, the call completes at the upstream,
    # so all three policies in the chain had their turn.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "add-scope" and store it as "scopedToken"
    And I set header "Authorization" to "Bearer ${CTX:scopedToken}"
    And I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful
    And the response body should contain "The sum of 40 and 60 is 100."

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
