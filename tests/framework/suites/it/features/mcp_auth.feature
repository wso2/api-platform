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

@mcp-auth-policy
Feature: MCP proxies protected by mcp-auth
  As an API developer
  I want mcp-auth to decide which MCP calls need a token
  So that an operator can open selected capabilities without opening the whole proxy

  # The three scenarios that also attach set-headers belong here rather than with that policy:
  # what they assert is that mcp-auth still reads the client's own Authorization header after a
  # peer policy has overwritten it, which is mcp-auth's behaviour, not set-headers'.

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

  Scenario: An MCP proxy with mcp-auth rejects an unauthenticated request
    Given I generate a unique resource name from "mcp-auth" and store it as "mcpName"
    And I generate a unique value from "mcp-auth" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful
    And the JSON response field "status.state" should be "deployed"

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    And the response header "WWW-Authenticate" should contain "${CTX:mcpContext}/.well-known/oauth-protected-resource"

    When I send a "GET" request to "${CTX:mcpContext}/.well-known/oauth-protected-resource"
    Then the response should be successful
    And the response should be valid JSON
    And the JSON response field "authorization_servers[0]" should be "http://testbench:3001/token"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: An MCP proxy with mcp-auth accepts a valid token
    Given I generate a unique resource name from "mcp-auth-valid" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-valid" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-valid" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-valid" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-auth restricting only tools leaves the handshake unauthenticated
    Given I generate a unique resource name from "mcp-auth-tools-only" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-tools-only" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-tools-only" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-tools-only" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"methods":{"enabled":false}}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 401
    And the response header "WWW-Authenticate" should contain "${CTX:mcpContext}/.well-known/oauth-protected-resource"

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  @gateway-v1.2
  Scenario: mcp-auth accepts token-forwarding parameters and preserves the auth flow
    Given I generate a unique resource name from "mcp-auth-forward" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-forward" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-forward" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-forward" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"forwardToken":true,"forwardedTokenHeader":"x-forwarded-authorization","forwardTokenStripScheme":true,"userIdClaim":"email"}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "email=alice@example.com" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # A peer policy (set-headers) overwriting the live Authorization header during the header
  # phase must not break mcp-auth: it validates the client's ORIGINAL Authorization from the
  # downstream request snapshot, not the peer-mutated live value.
  @gateway-v1.2
  Scenario: mcp-auth authenticates the client token even when set-headers overwrites Authorization
    Given I generate a unique resource name from "mcp-auth-setheaders" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-setheaders" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-setheaders" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-setheaders" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"Authorization","value":"Bearer backend-service-credential"}]}}}] |
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
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The exact failing combination from a past regression: token forwarding AND set-headers
  # together, forwarding under a DIFFERENT header than the one set-headers owns.
  @gateway-v1.2
  Scenario: mcp-auth forwardToken coexists with set-headers injecting Authorization
    Given I generate a unique resource name from "mcp-auth-forward-setheaders" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-forward-setheaders" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-forward-setheaders" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-forward-setheaders" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"forwardToken":true,"forwardedTokenHeader":"x-forwarded-authorization"}},{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"Authorization","value":"Bearer backend-service-credential"}]}}}] |
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
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # Collision variant: forwardedTokenHeader names the SAME header set-headers owns
  # (Authorization). mcp-auth must skip forwarding rather than overwrite the peer's value, but
  # must still validate the client's snapshot token.
  @gateway-v1.2
  Scenario: mcp-auth skips forwarding when forwardedTokenHeader collides with a set-headers-owned header
    Given I generate a unique resource name from "mcp-auth-collision" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-collision" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-collision" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-collision" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"forwardToken":true,"forwardedTokenHeader":"Authorization"}},{"name":"set-headers","version":"v1","params":{"request":{"headers":[{"name":"Authorization","value":"Bearer backend-service-credential"}]}}}] |
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
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The exception list matches on the capability the request names, and on a modern request that
  # name arrives in a header rather than in the body the policy parses.
  Scenario: mcp-auth exempts a capability named by the mirrored header
    Given I generate a unique resource name from "mcp-auth-modern" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-modern" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-modern" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-modern" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"methods":{"enabled":false},"tools":{"exceptions":["echo"]}}}] |
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
    # No basic-auth header is restored here: this route authenticates with mcp-auth, and a
    # Basic credential on the data path would be read as a bearer token and rejected, making
    # the exempt call below fail for a reason that has nothing to do with the exception list.
    And I clear all headers

    When I use the MCP Client to send a "tools/call" request for "echo" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful
    And the JSON response should have field "result"

    When I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # An absent Mcp-Name would leave the policy matching its exception list against an empty
  # string, which matches nothing - and under methods.enabled false a non-match is an exemption.
  # So the request is refused rather than allowed through on a name nobody stated.
  Scenario: mcp-auth refuses a modern request that mirrors no capability name
    Given I generate a unique resource name from "mcp-auth-noname" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-noname" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-noname" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-noname" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"methods":{"enabled":false},"tools":{"exceptions":["echo"]}}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I set header "MCP-Protocol-Version" to "2026-07-28"
    And I set header "Mcp-Method" to "tools/call"
    # No Mcp-Name, which a conformant client would always send for tools/call.
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 400 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/call","params":{"name":"echo","arguments":{"message":"hi"},"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"warmup","version":"1.0.0"},"io.modelcontextprotocol/clientCapabilities":{}}}}
      """
    Then the JSON response field "error.code" should be "-32020"
    # The gateway's own wording, and it names the policy's reason rather than the upstream's.
    And the response body should contain "Mcp-Name header is required for tools/call"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # The list is otherwise inert - the gateway stores it and validates its shape - except here:
  # the OAuth protected-resource route is synthesised when ANY declared revision is 2025-06-18
  # or later. So this is the one scenario where declaring a second version changes what the
  # data plane serves, and it is asserted by asking for the route rather than by reading config.
  #
  # It lives in this file because the route means nothing without mcp-auth: that policy is what
  # answers it, and this block is where a key manager is configured for it.
  Scenario Outline: The protected-resource route follows the declared spec versions (<reason>)
    Given I generate a unique resource name from "mcp-prm-<slug>" and store it as "mcpName"
    And I generate a unique value from "mcp-prm-<slug>" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-prm-<slug>" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-prm-<slug>" and store it as "mcpContext"
    # A whole-spec override, because the canonical template carries the deprecated scalar and
    # declaring both forms is itself an error.
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion  | ${CTX:gatewaySpecVersion} |
      | name        | ${CTX:mcpName}            |
      | displayName | Replaced                  |
      | version     | v1.0                      |
      | context     | /replaced                 |
      | specVersion | 2025-06-18                |
      | spec        | {"displayName":"${CTX:mcpDisplayName}","version":"${CTX:mcpVersion}","context":"${CTX:mcpContext}","specVersions":<versions>,"upstream":{"url":"http://testbench:3009/mcp"},"tools":[],"resources":[],"prompts":[],"policies":[{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]} |
    Then the response should be successful

    # The proxy has to be live before an absent route means anything: a 404 is also what a
    # deployment that has not landed yet looks like. The /mcp route answering 401 - mcp-auth
    # challenging an unauthenticated call - is the proof that this proxy's routes are in place.
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"tools/list","params":{}}
      """
    And I clear all headers

    When I send a "GET" request to "${CTX:mcpContext}/.well-known/oauth-protected-resource" until status <status>
    Then the response status code should be <status>

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

    Examples:
      | slug   | versions                       | status | reason                                     |
      | absent | ["2025-03-26"]                 | 404    | every declared revision predates the model |
      | one    | ["2025-03-26","2026-07-28"]    | 200    | one declared revision implements it        |
