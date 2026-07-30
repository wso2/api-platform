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

Feature: Test how MCP Proxies behave when various policies are applied.
    As an API developer
    I want to deploy an MCP Proxy configuration with policies attached to it
    So that I can verify that the proxy behaves according to the policy.

    Background:
        Given the gateway services are running

    Scenario: Deploy an MCP Proxy with non-existing policy and verify whether the deployment fails
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-non-existing-policy-test
            spec:
              displayName: MCP Non-Existing Policy Test
              version: v1.0
              context: /mcpnonexistingpolicy
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: non-existing-policy
                  version: v1
                  params: {}
              tools: []
              resources: []
              prompts: []
            """

        Then the response status code should be 400
        And the response should be valid JSON

    Scenario: Deploy an MCP Proxy with mcp-auth policy
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-test
            spec:
              displayName: MCP Auth Test
              version: v1.0
              context: /mcpauth
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
              tools: []
              resources: []
              prompts: []
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcpauth/mcp"
        Then the response status code should be 401
        And the response header "WWW-Authenticate" should contain "http://localhost:8080/mcpauth/.well-known/oauth-protected-resource"
        And I send a GET request to "http://localhost:8080/mcpauth/.well-known/oauth-protected-resource"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "authorization_servers[0]" should be "http://mock-jwks:8080/token"
        # Cleanup
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-auth and verify with a valid token
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-valid-token-test
            spec:
              displayName: MCP Auth Valid Token Test
              version: v1.0
              context: /mcpvalidtoken
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
              tools: []
              resources: []
              prompts: []
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpvalidtoken/mcp" with the JWT token
        Then the response should be successful
        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-valid-token-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-auth policy restricting only tools
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-tools-only-test
            spec:
              displayName: MCP Auth Tools Only Test
              version: v1.0
              context: /mcptoolsonly
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                    methods:
                      enabled: false
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcptoolsonly/mcp"
        Then the response should be successful

        And I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcptoolsonly/mcp"
        Then the response status code should be 401
        And the response header "WWW-Authenticate" should contain "http://localhost:8080/mcptoolsonly/.well-known/oauth-protected-resource"
        
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send a tools/call request to "http://127.0.0.1:8080/mcptoolsonly/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-tools-only-test"
        Then the response should be successful

    # Issue #2867: mcp-auth exposes jwt-auth's token-forwarding parameters (forwardToken,
    # forwardedTokenHeader, forwardTokenStripScheme, userIdClaim). This verifies those parameters are
    # accepted by the policy and the end-to-end authentication flow is preserved when they are set.
    # (The mock MCP backend does not echo request headers, so what actually reaches the upstream under
    # forwardedTokenHeader is covered by the policy's unit tests; here we guard config acceptance and
    # that forwarding does not break authentication.)
    Scenario: mcp-auth accepts token-forwarding parameters and preserves the auth flow
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-forward-token-test
            spec:
              displayName: MCP Auth Forward Token Test
              version: v1.0
              context: /mcpforwardtoken
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                    forwardToken: true
                    forwardedTokenHeader: x-forwarded-authorization
                    forwardTokenStripScheme: true
                    userIdClaim: email
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Authentication is still enforced with the forwarding parameters configured.
        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcpforwardtoken/mcp"
        Then the response status code should be 401

        # A valid token authenticates successfully — the forwarding configuration does not break the flow.
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "email=alice@example.com"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpforwardtoken/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpforwardtoken/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-forward-token-test"
        Then the response should be successful

    # Issue #2866: a peer policy (set-headers) that overwrites the live Authorization header during the
    # header phase must not break mcp-auth. mcp-auth validates the client's ORIGINAL Authorization from
    # the downstream request snapshot, not the peer-mutated live value. Here set-headers replaces
    # Authorization with a non-token backend credential; the client's valid JWT must still authenticate.
    Scenario: mcp-auth authenticates the client token even when set-headers overwrites Authorization
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-set-headers-test
            spec:
              displayName: MCP Auth Set-Headers Test
              version: v1.0
              context: /mcpsetheaders
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: set-headers
                  version: v1
                  params:
                    request:
                      headers:
                        - name: Authorization
                          value: "Bearer backend-service-credential"
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # The client's valid JWT authenticates even though set-headers overwrites the live Authorization
        # header with a non-token value — mcp-auth validates the client's snapshot token, not the live one.
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpsetheaders/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpsetheaders/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-set-headers-test"
        Then the response should be successful

    # Issue #2866 (the exact failing combination): mcp-auth token forwarding AND set-headers together.
    # set-headers injects a backend Authorization in the header phase; mcp-auth (forwardToken: true)
    # validates the client's snapshot token and forwards it under a DIFFERENT header
    # (x-forwarded-authorization) while leaving the peer-owned Authorization in place. Exercises the
    # preserveTokenHeader + snapshot-validation path with forwarding enabled; the client's valid JWT
    # must authenticate. (The backend does not echo headers, so this asserts the client-observable
    # outcome — the forwarded/preserved header values themselves are covered by the policy unit tests.)
    Scenario: mcp-auth forwardToken coexists with set-headers injecting Authorization
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-forward-setheaders-test
            spec:
              displayName: MCP Auth Forward and Set-Headers Test
              version: v1.0
              context: /mcpforwardsetheaders
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                    forwardToken: true
                    forwardedTokenHeader: x-forwarded-authorization
                - name: set-headers
                  version: v1
                  params:
                    request:
                      headers:
                        - name: Authorization
                          value: "Bearer backend-service-credential"
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # The client's valid JWT authenticates: mcp-auth validates the snapshot token (not the
        # set-headers value) and forwards it under x-forwarded-authorization, leaving the peer-owned
        # Authorization header untouched.
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpforwardsetheaders/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpforwardsetheaders/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-forward-setheaders-test"
        Then the response should be successful

    # Issue #2866 (collision variant): forwardedTokenHeader names the SAME header set-headers owns
    # (Authorization). mcp-auth must not forward the validated token over a peer-claimed header — it
    # skips forwarding (logging a warning) and must not strip the peer's value. Because mcp-auth still
    # validates the client's snapshot token, the client's valid JWT must authenticate.
    Scenario: mcp-auth skips forwarding when forwardedTokenHeader collides with a set-headers-owned header
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-auth-forward-collision-test
            spec:
              displayName: MCP Auth Forward Collision Test
              version: v1.0
              context: /mcpforwardcollision
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                    forwardToken: true
                    forwardedTokenHeader: Authorization
                - name: set-headers
                  version: v1
                  params:
                    request:
                      headers:
                        - name: Authorization
                          value: "Bearer backend-service-credential"
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Authorization is owned by set-headers, so mcp-auth skips forwarding under that name; the
        # client's valid JWT still authenticates via the snapshot token.
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpforwardcollision/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpforwardcollision/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-forward-collision-test"
        Then the response should be successful

    Scenario: Deploy an MCP proxy with mcp-authz policy and verify whether 403 is returned for unauthorized access
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-test
            spec:
              displayName: MCP AuthZ Test
              version: v1.0
              context: /mcpauthz
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        requiredScopes:
                          - "add-scope"
                      - name: "echo"
                        requiredScopes:
                          - "echo-scope"
            """
        
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthz/mcp" with the JWT token
        Then the response should be successful

        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthz/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "http://localhost:8080/mcpauthz/.well-known/oauth-protected-resource"
        And the response header "WWW-Authenticate" should contain "add-scope"

        And I send a GET request to "http://localhost:8080/mcpauthz/.well-known/oauth-protected-resource"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "authorization_servers[0]" should be "http://mock-jwks:8080/token"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-authz and verify access with a valid token having required scopes
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-valid-token-test
            spec:
              displayName: MCP AuthZ Valid Token Test
              version: v1.0
              context: /mcpauthzvalidtoken
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        requiredScopes:
                          - "add-scope"
                      - name: "echo"
                        requiredScopes:
                          - "echo-scope"
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "add-scope"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzvalidtoken/mcp" with the JWT token
        Then the response should be successful

        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzvalidtoken/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-valid-token-test"
        Then the response should be successful

    Scenario: mcp-authz new scopes format (allOf + anyOf) is enforced
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-scopes
            spec:
              displayName: MCP AuthZ Scopes
              version: v1.0
              context: /mcpauthzscopes
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        scopes:
                          allOf:
                            - "s-read"
                            - "s-deploy"
                          anyOf:
                            - "s-write"
                            - "s-update"
            """
        Then the response should be successful
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # allOf satisfied AND one anyOf present → authorized
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "s-read s-deploy s-write"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzscopes/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzscopes/mcp" with the JWT token
        Then the response should be successful

        # allOf satisfied but no anyOf scope present → 403 (challenge advertises an anyOf scope)
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "s-read s-deploy"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzscopes/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "s-write"

        # anyOf present but allOf incomplete (missing s-deploy) → 403 (challenge advertises s-deploy)
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "s-read s-write"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzscopes/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "s-deploy"

        # neither allOf nor anyOf satisfied → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "unrelated"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzscopes/mcp" with the JWT token
        Then the response status code should be 403

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-scopes"
        Then the response should be successful

    Scenario: mcp-authz new claims format (allOf + anyOf) is enforced
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-claims
            spec:
              displayName: MCP AuthZ Claims
              version: v1.0
              context: /mcpauthzclaims
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        claims:
                          allOf:
                            - claim: department
                              values:
                                - platform
                          anyOf:
                            - claim: role
                              values:
                                - admin
                                - superadmin
            """
        Then the response should be successful
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # department=platform AND role in {admin, superadmin} → authorized
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=platform,role=admin"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzclaims/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzclaims/mcp" with the JWT token
        Then the response should be successful

        # anyOf claim not matched (role=viewer) → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=platform,role=viewer"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzclaims/mcp" with the JWT token
        Then the response status code should be 403

        # allOf claim not matched (department=sales) → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=sales,role=admin"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzclaims/mcp" with the JWT token
        Then the response status code should be 403

        # required claim entirely absent (no department) → 403 (fail-closed)
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "role=admin"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzclaims/mcp" with the JWT token
        Then the response status code should be 403

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-claims"
        Then the response should be successful

    Scenario: mcp-authz new scopes take precedence over deprecated requiredScopes
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-scope-prec
            spec:
              displayName: MCP AuthZ Scope Precedence
              version: v1.0
              context: /mcpauthzscopeprec
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        requiredScopes:
                          - "old-scope"
                        scopes:
                          allOf:
                            - "new-scope"
            """
        Then the response should be successful
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Token has the new scope → authorized (new format enforced)
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "new-scope"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzscopeprec/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzscopeprec/mcp" with the JWT token
        Then the response should be successful

        # Token satisfies only the deprecated requiredScopes → 403 (deprecated is ignored, new wins)
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "old-scope"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzscopeprec/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "new-scope"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-scope-prec"
        Then the response should be successful

    Scenario: mcp-authz new claims take precedence over deprecated requiredClaims
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-claim-prec
            spec:
              displayName: MCP AuthZ Claim Precedence
              version: v1.0
              context: /mcpauthzclaimprec
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        requiredClaims:
                          department: platform
                        claims:
                          allOf:
                            - claim: department
                              values:
                                - engineering
            """
        Then the response should be successful
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Token satisfies the new claim (department=engineering) → authorized
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=engineering"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzclaimprec/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzclaimprec/mcp" with the JWT token
        Then the response should be successful

        # Token satisfies only the deprecated requiredClaims (department=platform) → 403 (new wins)
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=platform"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzclaimprec/mcp" with the JWT token
        Then the response status code should be 403

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-claim-prec"
        Then the response should be successful

    Scenario: mcp-authz requires all matching rules to pass (specific rule AND wildcard rule)
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-multirule
            spec:
              displayName: MCP AuthZ Multi Rule
              version: v1.0
              context: /mcpauthzmultirule
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "*"
                        scopes:
                          allOf:
                            - "base-scope"
                      - name: "add"
                        scopes:
                          allOf:
                            - "add-scope"
            """
        Then the response should be successful
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Both the wildcard rule and the specific rule are satisfied → authorized
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "base-scope add-scope"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzmultirule/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzmultirule/mcp" with the JWT token
        Then the response should be successful

        # Specific rule passes but the wildcard rule fails (no base-scope) → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "add-scope"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzmultirule/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "base-scope"

        # Wildcard rule passes but the specific rule fails (no add-scope) → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "base-scope"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzmultirule/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "add-scope"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-multirule"
        Then the response should be successful

    Scenario: mcp-authz mixes new scopes with deprecated requiredClaims on one rule
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-mixed
            spec:
              displayName: MCP AuthZ Mixed
              version: v1.0
              context: /mcpauthzmixed
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        scopes:
                          allOf:
                            - "api:read"
                        requiredClaims:
                          department: platform
            """
        Then the response should be successful
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Both the new scope and the deprecated claim are satisfied → authorized
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read" and claims "department=platform"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzmixed/mcp" with the JWT token
        Then the response should be successful
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzmixed/mcp" with the JWT token
        Then the response should be successful

        # Scope satisfied but the deprecated claim fails (department=sales) → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read" and claims "department=sales"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzmixed/mcp" with the JWT token
        Then the response status code should be 403

        # Claim satisfied but the new scope fails (no api:read) → 403
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "unrelated" and claims "department=platform"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzmixed/mcp" with the JWT token
        Then the response status code should be 403
        And the response header "WWW-Authenticate" should contain "api:read"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-mixed"
        Then the response should be successful

    # Issue #2842: mcp-authz must decide governance by rule matching BEFORE consulting identity. A
    # capability that no rule targets is not governed and passes through untouched — even with no
    # authenticated context — while a governed capability still fails closed. Here mcp-authz alone
    # (no mcp-auth) governs "add" only.
    Scenario: mcp-authz passes through capabilities no rule targets and fails closed on governed ones
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-passthrough
            spec:
              displayName: MCP AuthZ Passthrough
              version: v1.0
              context: /mcpauthzpassthrough
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        requiredScopes:
                          - "add-scope"
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Handshake and an untargeted capability pass through without any authentication.
        When I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzpassthrough/mcp"
        Then the response should be successful
        And I use the MCP Client to send "echo" tools/call request to "http://localhost:8080/mcpauthzpassthrough/mcp"
        Then the response should be successful

        # The governed capability ("add") still fails closed with 401 when no identity is present.
        And I use the MCP Client to send "add" tools/call request to "http://localhost:8080/mcpauthzpassthrough/mcp"
        Then the response status code should be 401

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-passthrough"
        Then the response should be successful

    # Issue #2842: a capability the mcp-auth policy excludes from authentication (tools.exceptions)
    # and that mcp-authz does not govern must not be blocked by mcp-authz — it legitimately arrives
    # with no AuthContext. The protected, governed capability must still be enforced (backward compat).
    Scenario: mcp-authz does not block an mcp-auth-excluded tool while still enforcing governed tools
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-authz-excluded-tool
            spec:
              displayName: MCP AuthZ Excluded Tool
              version: v1.0
              context: /mcpauthzexcluded
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-auth
                  version: v1
                  params:
                    issuers:
                      - mock-jwks
                    methods:
                      enabled: false
                    tools:
                      exceptions:
                        - echo
                - name: mcp-authz
                  version: v1
                  params:
                    tools:
                      - name: "add"
                        requiredScopes:
                          - "add-scope"
              tools: []
              resources: []
              prompts: []
            """
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Handshake works unauthenticated (methods.enabled: false).
        When I use the MCP Client to send an initialize request to "http://localhost:8080/mcpauthzexcluded/mcp"
        Then the response should be successful

        # "echo" is auth-excluded in mcp-auth and not governed by mcp-authz → passes through unauthenticated.
        And I use the MCP Client to send "echo" tools/call request to "http://localhost:8080/mcpauthzexcluded/mcp"
        Then the response should be successful

        # "add" is protected → 401 without a token (backward compatibility preserved).
        And I use the MCP Client to send "add" tools/call request to "http://localhost:8080/mcpauthzexcluded/mcp"
        Then the response status code should be 401

        # "add" with a token carrying the required scope → authorized.
        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "add-scope"
        And I use the MCP Client to send a tools/call request to "http://localhost:8080/mcpauthzexcluded/mcp" with the JWT token
        Then the response should be successful

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-authz-excluded-tool"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-acl-list policy and verify modes with exceptions
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-acl-test
            spec:
              displayName: MCP ACL Test
              version: v1.0
              context: /mcpacl
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-acl-list
                  version: v1
                  params:
                    tools:
                      mode: deny
                      exceptions:
                        - add
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcpacl/mcp"
        Then the response should be successful
        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpacl/mcp"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response should have field "result"
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

        When I use the MCP Client to send "echo" tools/call request to "http://127.0.0.1:8080/mcpacl/mcp"
        Then the response status code should be 400

        Given I authenticate using basic auth as "admin"
        When I update the MCP proxy "mcp-acl-test" with:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-acl-test
            spec:
              displayName: MCP ACL Test
              version: v1.0
              context: /mcpacl
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-acl-list
                  version: v1
                  params:
                    tools:
                      mode: allow
                      exceptions:
                        - add
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send "echo" tools/call request to "http://localhost:8080/mcpacl/mcp"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response should have field "result"
        And the JSON response field "result.content[0].text" should contain "Hello, World!"

        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpacl/mcp"
        Then the response status code should be 400

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-acl-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-rewrite policy and verify the behaviour
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-rewrite-test
            spec:
              displayName: MCP Rewrite Test
              version: v1.0
              context: /mcprewrite
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-rewrite
                  version: v1
                  params:
                    tools:
                      - name: sum
                        description: Take the sum of two numbers
                        target: add
                        inputSchema: |
                          {
                            "$schema": "http://json-schema.org/draft-07/schema#",
                            "additionalProperties": false,
                            "properties": {
                              "a": {
                                "description": "First number",
                                "type": "number"
                              },
                              "b": {
                                "description": "Second number",
                                "type": "number"
                              }
                            },
                            "required": [
                              "a",
                              "b"
                            ],
                            "type": "object"
                          }
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcprewrite/mcp"
        Then the response should be successful
        When I use the MCP Client to send "sum" tools/call request to "http://127.0.0.1:8080/mcprewrite/mcp"
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response should have field "result"
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-rewrite-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-ratelimit policy and verify a specific tool is rate limited
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-ratelimit-tool-test
            spec:
              displayName: MCP Rate Limit Tool Test
              version: v1.0
              context: /mcpratelimittool
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-ratelimit
                  version: v1
                  params:
                    tools:
                      - name: add
                        limits:
                          - limit: 2
                            duration: "1m"
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcpratelimittool/mcp"
        Then the response should be successful

        # First two "add" calls are within the limit and carry rate-limit headers
        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpratelimittool/mcp"
        Then the response should be successful
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."
        And the response header "X-RateLimit-Limit" should be "2"
        And the response header "X-RateLimit-Remaining" should be "1"
        And the response header "RateLimit-Policy" should exist
        And the response header "Retry-After" should not exist

        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpratelimittool/mcp"
        Then the response should be successful
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

        # Third "add" call exceeds the limit and is throttled
        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpratelimittool/mcp"
        Then the response status code should be 429
        And the response should be valid JSON
        And the JSON response field "error.code" should be "-32000"
        And the response header "X-RateLimit-Limit" should be "2"
        And the response header "X-RateLimit-Remaining" should be "0"
        And the response header "Retry-After" should exist

        # A different tool ("echo") has its own counter and is not affected
        When I use the MCP Client to send "echo" tools/call request to "http://127.0.0.1:8080/mcpratelimittool/mcp"
        Then the response should be successful
        And the JSON response field "result.content[0].text" should contain "Hello, World!"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-ratelimit-tool-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with mcp-ratelimit policy and verify a JSON-RPC method (tools/list) is rate limited
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-ratelimit-method-test
            spec:
              displayName: MCP Rate Limit Method Test
              version: v1.0
              context: /mcpratelimitmethod
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: mcp-ratelimit
                  version: v1
                  params:
                    methods:
                      - name: tools/list
                        limits:
                          - limit: 2
                            duration: "1m"
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        When I use the MCP Client to send an initialize request to "http://127.0.0.1:8080/mcpratelimitmethod/mcp"
        Then the response should be successful

        # First two tools/list calls are within the limit and carry rate-limit headers
        When I use the MCP Client to send a tools/list request to "http://127.0.0.1:8080/mcpratelimitmethod/mcp"
        Then the response should be successful
        And the JSON response should have field "result"
        And the response header "X-RateLimit-Limit" should be "2"
        And the response header "X-RateLimit-Remaining" should be "1"
        And the response header "RateLimit-Policy" should exist
        And the response header "Retry-After" should not exist

        When I use the MCP Client to send a tools/list request to "http://127.0.0.1:8080/mcpratelimitmethod/mcp"
        Then the response should be successful
        And the JSON response should have field "result"

        # Third tools/list call exceeds the limit and is throttled
        When I use the MCP Client to send a tools/list request to "http://127.0.0.1:8080/mcpratelimitmethod/mcp"
        Then the response status code should be 429
        And the response should be valid JSON
        And the JSON response field "error.code" should be "-32000"
        And the response header "X-RateLimit-Limit" should be "2"
        And the response header "X-RateLimit-Remaining" should be "0"
        And the response header "Retry-After" should exist

        # A different method (tools/call) is not affected by the tools/list limit
        When I use the MCP Client to send "add" tools/call request to "http://127.0.0.1:8080/mcpratelimitmethod/mcp"
        Then the response should be successful
        And the JSON response field "result.content[0].text" should contain "The sum of 40 and 60 is 100."

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-ratelimit-method-test"
        Then the response should be successful

    Scenario: Deploy an MCP Proxy with cors policy and verify preflight and simple request behaviour
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1
            kind: Mcp
            metadata:
              name: mcp-cors-test
            spec:
              displayName: MCP CORS Test
              version: v1.0
              context: /mcpcors
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001/mcp
              policies:
                - name: cors
                  version: v1
                  params:
                    allowedOrigins:
                      - "http://example.com"
                    allowedMethods:
                      - "GET"
                      - "POST"
                    allowedHeaders:
                      - "Content-Type"
                    exposedHeaders:
                      - "X-Custom-Header"
              tools: []
              resources: []
              prompts: []
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 2 seconds

        # Preflight request from allowed origin
        When I set header "Origin" to "http://example.com"
        And I set header "Access-Control-Request-Method" to "POST"
        And I set header "Access-Control-Request-Headers" to "Content-Type"
        And I send an OPTIONS request to "http://localhost:8080/mcpcors/mcp"
        Then the response status code should be 204
        And the response header "Access-Control-Allow-Origin" should be "http://example.com"
        And the response header "Access-Control-Allow-Methods" should contain "POST"
        And the response header "Access-Control-Allow-Headers" should contain "Content-Type"

        # Preflight request from disallowed origin should not return CORS headers
        When I set header "Origin" to "http://evil.com"
        And I set header "Access-Control-Request-Method" to "POST"
        And I set header "Access-Control-Request-Headers" to "Content-Type"
        And I send an OPTIONS request to "http://localhost:8080/mcpcors/mcp"
        Then the response status code should be 204
        And the response header "Access-Control-Allow-Origin" should not exist

        # Simple request from allowed origin gets CORS response headers
        # When I clear all headers
        # And I set header "Origin" to "http://example.com"
        # And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpcors/mcp"
        # Then the response should be successful
        # And the response header "Access-Control-Allow-Origin" should be "http://example.com"

        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-cors-test"
        Then the response should be successful
