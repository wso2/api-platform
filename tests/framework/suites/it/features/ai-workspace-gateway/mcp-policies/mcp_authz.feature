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

@mcp-authz-policy
Feature: MCP proxies governed by mcp-authz
  As an API developer
  I want mcp-authz to decide which identity may invoke which capability
  So that a token that authenticates is not automatically a token that authorizes

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

  Scenario: An MCP proxy with mcp-authz returns 403 for unauthorized access
    Given I generate a unique resource name from "mcp-authz" and store it as "mcpName"
    And I generate a unique value from "mcp-authz" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  @gateway-v1.2
  Scenario: mcp-authz new scopes format (allOf and anyOf) is enforced
    Given I generate a unique resource name from "mcp-authz-scopes" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-scopes" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-scopes" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-scopes" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  @gateway-v1.2
  Scenario: mcp-authz new claims format (allOf and anyOf) is enforced
    Given I generate a unique resource name from "mcp-authz-claims" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-claims" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-claims" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-claims" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  @gateway-v1.2
  Scenario: mcp-authz new scopes take precedence over deprecated requiredScopes
    Given I generate a unique resource name from "mcp-authz-scope-prec" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-scope-prec" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-scope-prec" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-scope-prec" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  @gateway-v1.2
  Scenario: mcp-authz new claims take precedence over deprecated requiredClaims
    Given I generate a unique resource name from "mcp-authz-claim-prec" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-claim-prec" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-claim-prec" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-claim-prec" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  @gateway-v1.2
  Scenario: mcp-authz requires all matching rules to pass (specific rule and wildcard rule)
    Given I generate a unique resource name from "mcp-authz-multirule" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-multirule" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-multirule" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-multirule" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  @gateway-v1.2
  Scenario: mcp-authz mixes new scopes with deprecated requiredClaims on one rule
    Given I generate a unique resource name from "mcp-authz-mixed" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-mixed" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-mixed" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-mixed" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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
  @gateway-v1.2
  Scenario: mcp-authz passes through capabilities no rule targets and fails closed on governed ones
    Given I generate a unique resource name from "mcp-authz-passthrough" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-passthrough" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-passthrough" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-passthrough" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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
  @gateway-v1.2
  Scenario: mcp-authz does not block an mcp-auth-excluded tool while still enforcing governed tools
    Given I generate a unique resource name from "mcp-authz-excluded" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-excluded" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-excluded" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-excluded" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
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

  # The rule targets a tool by name, and on a modern request that name comes from the header.
  # Both halves matter: refusing without the scope shows the rule was found, and allowing with
  # it shows the request still completes at the upstream.
  Scenario: mcp-authz governs a tool named by the mirrored header
    Given I generate a unique resource name from "mcp-authz-modern" and store it as "mcpName"
    And I generate a unique value from "mcp-authz-modern" and store it as "mcpDisplayName"
    And I generate a unique API version from "mcp-authz-modern" and store it as "mcpVersion"
    And I generate a unique API context from "/mcp-authz-modern" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion} |
      | name              | ${CTX:mcpName}            |
      | displayName       | ${CTX:mcpDisplayName}     |
      | version           | ${CTX:mcpVersion}         |
      | context           | ${CTX:mcpContext}         |
      | specVersion       | 2026-07-28                |
      | spec.upstream.url | http://testbench:3009/mcp |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","requiredScopes":["add-scope"]}]}}] |
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

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "plainToken"
    And I set header "Authorization" to "Bearer ${CTX:plainToken}"
    And I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response status code should be 403

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "add-scope" and store it as "scopedToken"
    And I set header "Authorization" to "Bearer ${CTX:scopedToken}"
    And I use the MCP Client to send a "tools/call" request for "add" to "${CTX:mcpContext}/mcp" declaring MCP version "2026-07-28"
    Then the response should be successful
    And the JSON response should have field "result"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the MCP proxy "${CTX:mcpName}"
    Then the response should be successful
