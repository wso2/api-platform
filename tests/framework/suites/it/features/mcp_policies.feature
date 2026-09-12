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

@mcp-policies
Feature: MCP proxy behavior under attached policies
  As an API developer
  I want to attach policies to an MCP proxy
  So that I can verify the proxy enforces them correctly
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: Deploying an MCP proxy with a non-existing policy fails
    Given I generate a unique resource name from "mcp-nonexistent-policy" and store it as "mcpName"
    And I generate a unique API context from "/mcp-nonexistent-policy" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | MCP Non-Existing Policy Test        |
      | version           | v1.0                                |
      | context           | ${CTX:mcpContext}                  |
      | specVersion       | 2025-06-18                          |
      | spec.upstream.url | http://testbench:3009/mcp           |
      | spec.policies     | [{"name":"non-existing-policy","version":"v1","params":{}}] |
    Then the response status code should be 400
    And the response should be valid JSON

  Scenario: An MCP proxy with mcp-auth rejects an unauthenticated request
    Given I generate a unique resource name from "mcp-auth" and store it as "mcpName"
    And I generate a unique value from "mcp-auth" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth" and store it as "mcpContext"
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
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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

  Scenario: mcp-auth accepts token-forwarding parameters and preserves the auth flow
    Given I generate a unique resource name from "mcp-auth-forward" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-forward" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-forward" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-forward" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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
  Scenario: mcp-auth authenticates the client token even when set-headers overwrites Authorization
    Given I generate a unique resource name from "mcp-auth-setheaders" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-setheaders" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-setheaders" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-setheaders" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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
  Scenario: mcp-auth forwardToken coexists with set-headers injecting Authorization
    Given I generate a unique resource name from "mcp-auth-forward-setheaders" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-forward-setheaders" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-forward-setheaders" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-forward-setheaders" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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
  Scenario: mcp-auth skips forwarding when forwardedTokenHeader collides with a set-headers-owned header
    Given I generate a unique resource name from "mcp-auth-collision" and store it as "mcpName"
    And I generate a unique value from "mcp-auth-collision" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-auth-collision" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-auth-collision" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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

  Scenario: An MCP proxy with mcp-authz returns 403 for unauthorized access
    Given I generate a unique resource name from "mcp-authz" and store it as "mcpName"
    And I generate a unique value from "mcp-authz" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]},{"name":"echo","requiredScopes":["echo-scope"]}]}}] |
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
    And the response header "WWW-Authenticate" should contain "${CTX:mcpContext}/.well-known/oauth-protected-resource"
    And the response header "WWW-Authenticate" should contain "add-scope"

    When I clear all headers
    And I send a "GET" request to "${CTX:mcpContext}/.well-known/oauth-protected-resource"
    Then the response should be successful
    And the JSON response field "authorization_servers[0]" should be "http://testbench:3001/token"

    Given I authenticate using basic auth as "admin"
    When I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: An MCP proxy with mcp-authz allows access with a token holding the required scope
    Given I generate a unique resource name from "mcp-authz-valid" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-valid" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-valid" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-valid" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]},{"name":"echo","requiredScopes":["echo-scope"]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "add-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz new scopes format (allOf and anyOf) is enforced
    Given I generate a unique resource name from "mcp-authz-scopes" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-scopes" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-scopes" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-scopes" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","scopes":{"allOf":["s-read","s-deploy"],"anyOf":["s-write","s-update"]}}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # allOf satisfied AND one anyOf present -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "s-read s-deploy s-write" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # allOf satisfied but no anyOf scope present -> 403 (challenge advertises an anyOf scope)
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "s-read s-deploy" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "s-write"

    # anyOf present but allOf incomplete (missing s-deploy) -> 403 (challenge advertises s-deploy)
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "s-read s-write" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "s-deploy"

    # neither allOf nor anyOf satisfied -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "unrelated" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz new claims format (allOf and anyOf) is enforced
    Given I generate a unique resource name from "mcp-authz-claims" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-claims" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-claims" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-claims" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","claims":{"allOf":[{"claim":"department","values":["platform"]}],"anyOf":[{"claim":"role","values":["admin","superadmin"]}]}}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # department=platform AND role in {admin, superadmin} -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=platform,role=admin" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # anyOf claim not matched (role=viewer) -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=platform,role=viewer" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403

    # allOf claim not matched (department=sales) -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=sales,role=admin" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403

    # required claim entirely absent (no department) -> 403 (fail-closed)
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "role=admin" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz new scopes take precedence over deprecated requiredScopes
    Given I generate a unique resource name from "mcp-authz-scope-prec" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-scope-prec" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-scope-prec" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-scope-prec" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["old-scope"],"scopes":{"allOf":["new-scope"]}}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Token has the new scope -> authorized (new format enforced)
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "new-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # Token satisfies only the deprecated requiredScopes -> 403 (deprecated is ignored, new wins)
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "old-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "new-scope"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz new claims take precedence over deprecated requiredClaims
    Given I generate a unique resource name from "mcp-authz-claim-prec" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-claim-prec" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-claim-prec" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-claim-prec" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredClaims":{"department":"platform"},"claims":{"allOf":[{"claim":"department","values":["engineering"]}]}}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Token satisfies the new claim (department=engineering) -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=engineering" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # Token satisfies only the deprecated requiredClaims (department=platform) -> 403 (new wins)
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz requires all matching rules to pass (specific rule and wildcard rule)
    Given I generate a unique resource name from "mcp-authz-multirule" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-multirule" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-multirule" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-multirule" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"*","scopes":{"allOf":["base-scope"]}},{"name":"add","scopes":{"allOf":["add-scope"]}}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Both the wildcard rule and the specific rule are satisfied -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "base-scope add-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # Specific rule passes but the wildcard rule fails (no base-scope) -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "add-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "base-scope"

    # Wildcard rule passes but the specific rule fails (no add-scope) -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "base-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "add-scope"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: mcp-authz mixes new scopes with deprecated requiredClaims on one rule
    Given I generate a unique resource name from "mcp-authz-mixed" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-mixed" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-mixed" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-mixed" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","scopes":{"allOf":["api:read"]},"requiredClaims":{"department":"platform"}}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 401 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Both the new scope and the deprecated claim are satisfied -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read" and claims "department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # Scope satisfied but the deprecated claim fails (department=sales) -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read" and claims "department=sales" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403

    # Claim satisfied but the new scope fails (no api:read) -> 403
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "unrelated" and claims "department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the response header "WWW-Authenticate" should contain "api:read"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # mcp-authz must decide governance by rule matching BEFORE consulting identity. A capability
  # no rule targets is not governed and passes through untouched, even with no authenticated
  # context, while a governed capability still fails closed. Here mcp-authz alone (no
  # mcp-auth) governs "add" only.
  Scenario: mcp-authz passes through capabilities no rule targets and fails closed on governed ones
    Given I generate a unique resource name from "mcp-authz-passthrough" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-passthrough" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-passthrough" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-passthrough" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Handshake and an untargeted capability pass through without any authentication.
    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And I use the MCP Client to send "echo" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # The governed capability ("add") still fails closed with 401 when no identity is present.
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  # A capability the mcp-auth policy excludes from authentication (tools.exceptions) and that
  # mcp-authz does not govern must not be blocked by mcp-authz - it legitimately arrives with
  # no auth context. The protected, governed capability must still be enforced.
  Scenario: mcp-authz does not block an mcp-auth-excluded tool while still enforcing governed tools
    Given I generate a unique resource name from "mcp-authz-excluded" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-excluded" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-excluded" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-excluded" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"],"methods":{"enabled":false},"tools":{"exceptions":["echo"]}}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]}]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Handshake works unauthenticated (methods.enabled: false).
    When I use the MCP Client to send an initialize request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # "echo" is auth-excluded in mcp-auth and not governed by mcp-authz -> passes through unauthenticated.
    And I use the MCP Client to send "echo" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    # "add" is protected -> 401 without a token (backward compatibility preserved).
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 401

    # "add" with a token carrying the required scope -> authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "add-scope" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: An MCP proxy with mcp-acl-list enforces mode and exceptions
    Given I generate a unique resource name from "mcp-acl" and store it as "mcpName"
    And I generate a unique value from "mcp-acl" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-acl" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-acl" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-acl-list","version":"v1","params":{"tools":{"mode":"deny","exceptions":["add"]}}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

    When I use the MCP Client to send "echo" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 400

    Given I authenticate using basic auth as "admin"
    When I update MCP proxy "${CTX:mcpName}" from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"mcp-acl-list","version":"v1","params":{"tools":{"mode":"allow","exceptions":["add"]}}}] |
    Then the response should be successful

    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"message":"Hello, World!"}}}
      """

    When I use the MCP Client to send "echo" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response should have field "result"
    And the JSON response field "result.content[0].text" should contain "Hello, World!"

    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 400

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful

  Scenario: An MCP proxy with mcp-rewrite renames a tool and its schema
    Given I generate a unique resource name from "mcp-rewrite" and store it as "mcpName"
    And I generate a unique value from "mcp-rewrite" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-rewrite" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-rewrite" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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

  Scenario: An MCP proxy with mcp-ratelimit throttles a specific tool
    Given I generate a unique resource name from "mcp-ratelimit-tool" and store it as "mcpName"
    And I generate a unique value from "mcp-ratelimit-tool" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-ratelimit-tool" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-ratelimit-tool" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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

  Scenario: An MCP proxy with mcp-ratelimit throttles a JSON-RPC method
    Given I generate a unique resource name from "mcp-ratelimit-method" and store it as "mcpName"
    And I generate a unique value from "mcp-ratelimit-method" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-ratelimit-method" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-ratelimit-method" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
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

  Scenario: An MCP proxy with cors handles preflight and disallowed-origin requests
    Given I generate a unique resource name from "mcp-cors" and store it as "mcpName"
    And I generate a unique value from "mcp-cors" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-cors" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-cors" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | gateway.api-platform.wso2.com/v1 |
      | name              | ${CTX:mcpName}                    |
      | displayName       | ${CTX:mcpDisplayName}             |
      | version           | ${CTX:mcpVersion}                 |
      | context           | ${CTX:mcpContext}                 |
      | specVersion       | 2025-06-18                         |
      | spec.upstream.url | http://testbench:3009/mcp          |
      | spec.policies     | [{"name":"cors","version":"v1","params":{"allowedOrigins":["http://example.com"],"allowedMethods":["GET","POST"],"allowedHeaders":["Content-Type"],"exposedHeaders":["X-Custom-Header"]}}] |
    Then the response should be successful

    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """

    # Preflight request from allowed origin
    When I clear all headers
    And I set header "Origin" to "http://example.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should be "http://example.com"
    And the response header "Access-Control-Allow-Methods" should contain "POST"
    And the response header "Access-Control-Allow-Headers" should contain "Content-Type"

    # Preflight request from disallowed origin should not return CORS headers
    When I set header "Origin" to "http://evil.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist

    # Preflight request from an origin with a disallowed suffix should not return CORS headers
    When I set header "Origin" to "http://example.com.evil.com"
    And I set header "Access-Control-Request-Method" to "POST"
    And I set header "Access-Control-Request-Headers" to "Content-Type"
    And I send a "OPTIONS" request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 204
    And the response header "Access-Control-Allow-Origin" should not exist

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
