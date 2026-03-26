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
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-non-existing-policy-test
            spec:
              displayName: MCP Non-Existing Policy Test
              version: v1.0
              context: /mcpnonexistingpolicy
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: non-existing-policy
                  version: v0
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
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-auth-test
            spec:
              displayName: MCP Auth Test
              version: v1.0
              context: /mcpauth
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-auth
                  version: v0
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
        And I wait for 4 seconds

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
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-auth-valid-token-test
            spec:
              displayName: MCP Auth Valid Token Test
              version: v1.0
              context: /mcpvalidtoken
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-auth
                  version: v0
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
        And I wait for 4 seconds

        When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
        And I use the MCP Client to send an initialize request to "http://localhost:8080/mcpvalidtoken/mcp" with the JWT token
        Then the response should be successful
        # Cleanup
        And I clear all headers
        Given I authenticate using basic auth as "admin"
        When I delete the MCP proxy "mcp-auth-valid-token-test"
        Then the response should be successful

    Scenario: Deploy and MCP proxy with mcp-authz policy and verify whether 403 is returned for unauthorized access
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-authz-test
            spec:
              displayName: MCP AuthZ Test
              version: v1.0
              context: /mcpauthz
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-auth
                  version: v0
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v0
                  params:
                    rules:
                      - attribute:
                          type: "tool"
                          name: "add"
                        requiredScopes:
                          - "add-scope"
                      - attribute:
                          type: "tool"
                          name: "echo"
                        requiredScopes:
                          - "echo-scope"
            """
        
        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 4 seconds

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
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-authz-valid-token-test
            spec:
              displayName: MCP AuthZ Valid Token Test
              version: v1.0
              context: /mcpauthzvalidtoken
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-auth
                  version: v0
                  params:
                    issuers:
                      - mock-jwks
                - name: mcp-authz
                  version: v0
                  params:
                    rules:
                      - attribute:
                          type: "tool"
                          name: "add"
                        requiredScopes:
                          - "add-scope"
                      - attribute:
                          type: "tool"
                          name: "echo"
                        requiredScopes:
                          - "echo-scope"
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 4 seconds

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

    Scenario: Deploy an MCP Proxy with mcp-acl-list policy and verify modes with exceptions
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-acl-test
            spec:
              displayName: MCP ACL Test
              version: v1.0
              context: /mcpacl
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-acl-list
                  version: v0
                  params:
                    tools:
                      mode: deny
                      exceptions:
                        - add
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 4 seconds

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
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-acl-test
            spec:
              displayName: MCP ACL Test
              version: v1.0
              context: /mcpacl
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-acl-list
                  version: v0
                  params:
                    tools:
                      mode: allow
                      exceptions:
                        - add
            """

        Then the response should be successful
        And the response should be valid JSON
        And the JSON response field "status" should be "success"
        And I wait for 4 seconds

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
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-rewrite-test
            spec:
              displayName: MCP Rewrite Test
              version: v1.0
              context: /mcprewrite
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: mcp-rewrite
                  version: v0
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
        And I wait for 4 seconds

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

    Scenario: Deploy an MCP Proxy with cors policy and verify preflight and simple request behaviour
        Given I authenticate using basic auth as "admin"
        When I deploy this MCP configuration:
            """
            apiVersion: gateway.api-platform.wso2.com/v1alpha1
            kind: Mcp
            metadata:
              name: mcp-cors-test
            spec:
              displayName: MCP CORS Test
              version: v1.0
              context: /mcpcors
              specVersion: "2025-06-18"
              upstream:
                url: http://mcp-server-backend:3001
              policies:
                - name: cors
                  version: v0
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
        And I wait for 4 seconds

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
