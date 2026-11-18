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

# The AuthContext is read from the traffic log, whose properties the block's overlay binds to the
# auth.* fields, and from the user id of the analytics event. Both read the latest AuthContext
# layer that is authenticated and has a subject, walking back through Previous. A traffic log line
# is found by its API's context, which a refused request's line carries without an operation path,
# and it lands about a second after its request, so each request's line is awaited before the next.
# An analytics event is matched to its request by the correlation id of the line found last.
@ACTX @auth-context
Feature: Typed AuthContext
  As a policy author
  I want each auth policy to describe the caller in a typed AuthContext
  So that later policies and analytics read who called and how

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  @ACTX-001 @ACTX-009 @ACTX-014
  Scenario: jwt-auth fills the AuthContext from the token's claims
    Given I generate a unique value from "actx-jwt" and store it as "api"
    And I generate a unique API context from "/actx-jwt" and store it as "ctx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:api}                        |
      | spec.displayName       | ACTX-JWT                          |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:ctx}                        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "orders:read orders:write" and claims "sub=actx-jwt-user" and store it as "token"
    And I clear all headers
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:ctx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:ctx}" with:
      | "authAuthenticated":true                     |
      | "authType":"jwt"                             |
      | "authSubject":"actx-jwt-user"                |
      | "authIssuer":"http://testbench:3001/token"   |
      | "authAudience":["test-audience"]             |
      | "authScopes":["orders:read","orders:write"]  |
    And the analytics event for the logged request should have user id "actx-jwt-user"

  @ACTX-002 @ACTX-009 @ACTX-014
  Scenario: basic-auth fills the AuthContext with the username only
    Given I generate a unique value from "actx-basic" and store it as "api"
    And I generate a unique API context from "/actx-basic" and store it as "ctx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:api}                        |
      | spec.displayName       | ACTX-Basic                        |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:ctx}                        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"basic-auth","version":"v1","params":{"username":"actx-user","password":"actx-secret"}}] |
    Then the response should be successful
    When I clear all headers
    And I set header "Authorization" to "Basic YWN0eC11c2VyOmFjdHgtc2VjcmV0"
    And I send a "GET" request to "${CTX:ctx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:ctx}" with:
      | "authAuthenticated":true  |
      | "authType":"basic"        |
      | "authSubject":"actx-user" |
      | "authIssuer":""           |
      | "authAudience":[]         |
      | "authScopes":[]           |
    And the analytics event for the logged request should have user id "actx-user"

  @ACTX-003 @ACTX-014 @known-issue
  Scenario: api-key-auth fills the AuthContext with the key's owner and credential
    Given I generate a unique value from "actx-key" and store it as "api"
    And I generate a unique API context from "/actx-key" and store it as "ctx"
    And I generate a unique resource name from "actx-key" and store it as "keyName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:api}                        |
      | spec.displayName       | ACTX-Key                          |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:ctx}                        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
    Then the response should be successful
    # A key generated on the gateway has no application, so its credential is the key's own id,
    # which the gateway's REST API does not return: the line can only show that one is set. A key
    # with an application, whose credential is the application id, needs platform-api to map it.
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:api}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "key"
    When I clear all headers
    And I set header "API-Key" to "${CTX:key}"
    And I send a "GET" request to "${CTX:ctx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:ctx}" with:
      | "authAuthenticated":true             |
      | "authType":"apikey"                  |
      | "authSubject":"admin"                |
      | "authCredentialId":"                 |
      | not "authCredentialId":""            |
      | "authIssuer":""                      |
      | "authAudience":[]                    |
      | "authScopes":[]                      |
    And the analytics event for the logged request should have user id "admin"

  # The 200 before each refusal is the positive control: the same API reports its caller once the
  # credentials are valid. For api-key-auth that report is ACTX-003, which waits on #4340.
  @ACTX-004
  Scenario: Missing or invalid credentials are refused and leave no authenticated subject
    Given I generate a unique value from "actx-fail-jwt" and store it as "jwtApi"
    And I generate a unique API context from "/actx-fail-jwt" and store it as "jwtCtx"
    And I generate a unique value from "actx-fail-basic" and store it as "basicApi"
    And I generate a unique API context from "/actx-fail-basic" and store it as "basicCtx"
    And I generate a unique value from "actx-fail-key" and store it as "keyApi"
    And I generate a unique API context from "/actx-fail-key" and store it as "keyCtx"
    And I generate a unique resource name from "actx-fail-key" and store it as "keyName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:jwtApi}                     |
      | spec.displayName       | ACTX-Fail-JWT                     |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:jwtCtx}                     |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:basicApi}                   |
      | spec.displayName       | ACTX-Fail-Basic                   |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:basicCtx}                   |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"basic-auth","version":"v1","params":{"username":"actx-user","password":"actx-secret"}}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:keyApi}                     |
      | spec.displayName       | ACTX-Fail-Key                     |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:keyCtx}                     |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"api-key-auth","version":"v1","params":{"key":"API-Key","in":"header"}}] |
    Then the response should be successful
    When I send a "POST" request to the "gateway-controller" service at "/rest-apis/${CTX:keyApi}/api-keys" with body:
      """
      {"name": "${CTX:keyName}"}
      """
    Then the response status should be 201
    And I store the JSON response field "apiKey.apiKey" as "key"
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"

    # jwt-auth
    When I clear all headers
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:jwtCtx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:jwtCtx}" with:
      | "authAuthenticated":true  |
      | "authSubject":"test-user" |
    When I clear all headers
    And I send a "GET" request to "${CTX:jwtCtx}/data"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:jwtCtx}" with:
      | "authAuthenticated":false |
      | "authSubject":""          |
    And the analytics event for the logged request should have user id "anonymous"
    When I set header "Authorization" to "Bearer not-a-jwt"
    And I send a "GET" request to "${CTX:jwtCtx}/data"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:jwtCtx}" with:
      | "authAuthenticated":false |
      | "authSubject":""          |
    And the analytics event for the logged request should have user id "anonymous"

    # basic-auth
    When I clear all headers
    And I set header "Authorization" to "Basic YWN0eC11c2VyOmFjdHgtc2VjcmV0"
    And I send a "GET" request to "${CTX:basicCtx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:basicCtx}" with:
      | "authAuthenticated":true  |
      | "authSubject":"actx-user" |
    When I clear all headers
    And I send a "GET" request to "${CTX:basicCtx}/data"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:basicCtx}" with:
      | "authAuthenticated":false |
      | "authSubject":""          |
    And the analytics event for the logged request should have user id "anonymous"
    When I set header "Authorization" to "Basic YWN0eC11c2VyOndyb25n"
    And I send a "GET" request to "${CTX:basicCtx}/data"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:basicCtx}" with:
      | "authAuthenticated":false |
      | "authSubject":""          |
    And the analytics event for the logged request should have user id "anonymous"

    # api-key-auth
    When I clear all headers
    And I set header "API-Key" to "${CTX:key}"
    And I send a "GET" request to "${CTX:keyCtx}/data" until status 200
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:keyCtx}" with:
      | "status":200 |
    When I clear all headers
    And I send a "GET" request to "${CTX:keyCtx}/data"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:keyCtx}" with:
      | "authAuthenticated":false |
      | "authSubject":""          |
    And the analytics event for the logged request should have user id "anonymous"
    When I set header "API-Key" to "actx-not-a-valid-key-0000000000000000000"
    And I send a "GET" request to "${CTX:keyCtx}/data"
    Then the response status code should be 401
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:keyCtx}" with:
      | "authAuthenticated":false |
      | "authSubject":""          |
    And the analytics event for the logged request should have user id "anonymous"

  @ACTX-005 @ACTX-014
  Scenario: mcp-auth authenticates an MCP tool call without authorizing it
    Given I generate a unique resource name from "actx-mcp-auth" and store it as "mcpName"
    And I generate a unique API version from "actx-mcp-auth" and store it as "mcpVersion"
    And I generate a unique API context from "/actx-mcp-auth" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                          |
      | name              | ${CTX:mcpName}                                     |
      | displayName       | ACTX-MCP-Auth                                      |
      | version           | ${CTX:mcpVersion}                                  |
      | context           | ${CTX:mcpContext}                                  |
      | specVersion       | 2025-06-18                                         |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}}] |
    Then the response should be successful
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I clear all headers
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    # Wait for the initialize call's own line, so the tool call's line is the next one.
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "authAuthenticated":true |
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "authAuthenticated":true  |
      | "authType":"mcp/oauth"    |
      | "authSubject":"test-user" |
      | "authAuthorized":false    |

  @ACTX-006 @ACTX-014
  Scenario: mcp-authz authorizes a tool call whose token has the tool's scope
    Given I generate a unique resource name from "actx-mcp-authz" and store it as "mcpName"
    And I generate a unique API version from "actx-mcp-authz" and store it as "mcpVersion"
    And I generate a unique API context from "/actx-mcp-authz" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                          |
      | name              | ${CTX:mcpName}                                     |
      | displayName       | ACTX-MCP-Authz                                     |
      | version           | ${CTX:mcpVersion}                                  |
      | context           | ${CTX:mcpContext}                                  |
      | specVersion       | 2025-06-18                                         |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","scopes":{"allOf":["mcp:add"]}}]}}] |
    Then the response should be successful
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "mcp:add" and store it as "token"
    And I clear all headers
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    # Wait for the initialize call's own line, so the tool call's line is the next one.
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "authAuthenticated":true |
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "authAuthenticated":true     |
      | "authType":"mcp/oauth+authz" |
      | "authSubject":"test-user"    |
      | "authAuthorized":true        |

  # The scoped call first is the positive control for the refusal.
  @ACTX-007
  Scenario: mcp-authz refuses a tool call whose token lacks the tool's scope
    Given I generate a unique resource name from "actx-mcp-deny" and store it as "mcpName"
    And I generate a unique API version from "actx-mcp-deny" and store it as "mcpVersion"
    And I generate a unique API context from "/actx-mcp-deny" and store it as "mcpContext"
    When I create MCP proxy from "resources/templates/mcp.yaml" with values:
      | apiVersion        | ${CTX:gatewaySpecVersion}                          |
      | name              | ${CTX:mcpName}                                     |
      | displayName       | ACTX-MCP-Deny                                      |
      | version           | ${CTX:mcpVersion}                                  |
      | context           | ${CTX:mcpContext}                                  |
      | specVersion       | 2025-06-18                                         |
      | spec.upstream.url | http://testbench:3009${CTX:gatewayMCPUpstreamPath} |
      | spec.policies     | [{"name":"mcp-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mcp-authz","version":"v1","params":{"tools":[{"name":"add","scopes":{"allOf":["mcp:add"]}}]}}] |
    Then the response should be successful
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "mcp:add" and store it as "scoped"
    And I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "mcp:other" and store it as "unscoped"
    And I clear all headers
    And I set header "Authorization" to "Bearer ${CTX:scoped}"
    And I set header "Content-Type" to "application/json"
    And I set header "Accept" to "application/json, text/event-stream"
    And I send a "POST" request to "${CTX:mcpContext}/mcp" until status 200 with body:
      """
      {"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"warmup","version":"1.0.0"}}}
      """
    # Wait for the initialize call's own line, so the tool call's line is the next one.
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "authAuthenticated":true |
    When I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response should be successful
    And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "authAuthorized":true |
    When I set header "Authorization" to "Bearer ${CTX:unscoped}"
    And I use the MCP Client to send "add" tools/call request to "${CTX:mcpContext}/mcp"
    Then the response status code should be 403
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:mcpContext}" with:
      | "status":403           |
      | "authAuthorized":false |
      | "authType":"mcp/oauth" |

  # basic-auth reads Authorization, so jwt-auth reads its token from X-JWT. The first request
  # passes both policies; the second sends no basic credentials, which basic-auth lets through
  # unauthenticated, so analytics walks back through Previous to the jwt-auth layer (ACTX-010).
  @ACTX-008 @ACTX-010
  Scenario: A second auth policy's AuthContext wraps the first one in Previous
    Given I generate a unique value from "actx-layered" and store it as "api"
    And I generate a unique API context from "/actx-layered" and store it as "ctx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:api}                        |
      | spec.displayName       | ACTX-Layered                      |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:ctx}                        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
      | spec.policies          | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"headerName":"X-JWT"}},{"name":"basic-auth","version":"v1","params":{"username":"actx-user","password":"actx-secret","allowUnauthenticated":true}}] |
    Then the response should be successful
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "orders:read" and claims "sub=actx-jwt-user" and store it as "token"
    And I clear all headers
    And I set header "X-JWT" to "Bearer ${CTX:token}"
    And I set header "Authorization" to "Basic YWN0eC11c2VyOmFjdHgtc2VjcmV0"
    And I send a "GET" request to "${CTX:ctx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:ctx}" with:
      | "authType":"basic"        |
      | "authSubject":"actx-user" |
      | "authIssuer":""           |
      | "authScopes":[]           |
    When I clear all headers
    And I set header "X-JWT" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:ctx}/data"
    Then the response status code should be 200
    And the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:ctx}" with:
      | "authAuthenticated":true                   |
      | "authType":"jwt"                           |
      | "authSubject":"actx-jwt-user"              |
      | "authIssuer":"http://testbench:3001/token" |
      | "authAudience":["test-audience"]           |
      | "authScopes":["orders:read"]               |
    And the analytics event for the logged request should have user id "actx-jwt-user"

  @ACTX-012
  Scenario: A request to an API with no auth policy is published as anonymous
    Given I generate a unique value from "actx-open" and store it as "api"
    And I generate a unique API context from "/actx-open" and store it as "ctx"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}         |
      | name                   | ${CTX:api}                        |
      | spec.displayName       | ACTX-Open                         |
      | spec.version           | v1.0                              |
      | spec.context           | ${CTX:ctx}                        |
      | spec.upstream.main.url | http://testbench:3000             |
      | spec.operations        | [{"method":"GET","path":"/data"}] |
    Then the response should be successful
    When I clear all headers
    And I send a "GET" request to "${CTX:ctx}/data" until status 200
    Then the "gateway-runtime" access log since the latest request should show a line for API context "${CTX:ctx}" with:
      | "status":200              |
      | "authAuthenticated":false |
      | "authSubject":""          |
      | "authType":""             |
    And the analytics event for the logged request should have user id "anonymous"
