# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

@jwt-auth
Feature: JWT Authentication
  As an API developer
  I want to secure my APIs with JWT authentication
  So that only authorized requests with valid tokens can access my resources

  Background:
    Given the gateway services are running

  Scenario: Request with valid JWT token is authorized
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-basic-api
      spec:
        displayName: JWT Auth Basic API
        version: v1.0
        context: /jwt-auth-basic/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-basic/v1.0/health" to be ready

    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I send a GET request to "http://localhost:8080/jwt-auth-basic/v1.0/protected" with the JWT token
    Then the response status code should be 200

  Scenario: Request without authorization header is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-no-header-api
      spec:
        displayName: JWT Auth No Header API
        version: v1.0
        context: /jwt-auth-no-header/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-no-header/v1.0/health" to be ready

    When I clear all headers
    And I send a GET request to "http://localhost:8080/jwt-auth-no-header/v1.0/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

  Scenario: Request with invalid JWT token is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-invalid-token-api
      spec:
        displayName: JWT Auth Invalid Token API
        version: v1.0
        context: /jwt-auth-invalid-token/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-invalid-token/v1.0/health" to be ready

    When I clear all headers
    And I set header "Authorization" to "Bearer invalid.jwt.token"
    And I send a GET request to "http://localhost:8080/jwt-auth-invalid-token/v1.0/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

  Scenario: Request with malformed Bearer header is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-malformed-header-api
      spec:
        displayName: JWT Auth Malformed Header API
        version: v1.0
        context: /jwt-auth-malformed-header/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-malformed-header/v1.0/health" to be ready

    When I clear all headers
    And I set header "Authorization" to "NotBearer sometoken"
    And I send a GET request to "http://localhost:8080/jwt-auth-malformed-header/v1.0/protected"
    Then the response status code should be 401

  Scenario: Request with wrong issuer is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-wrong-issuer-api
      spec:
        displayName: JWT Auth Wrong Issuer API
        version: v1.0
        context: /jwt-auth-wrong-issuer/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - wrong-issuer-km
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-wrong-issuer/v1.0/health" to be ready

    # Token has issuer "http://mock-jwks:8080/token" but API expects "http://expected-issuer.com"
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I send a GET request to "http://localhost:8080/jwt-auth-wrong-issuer/v1.0/protected" with the JWT token
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

  Scenario: JWT authentication with audience validation
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-audience-api
      spec:
        displayName: JWT Auth Audience API
        version: v1.0
        context: /jwt-auth-audience/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  audiences:
                    - "test-audience"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-audience/v1.0/health" to be ready

    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I send a GET request to "http://localhost:8080/jwt-auth-audience/v1.0/protected" with the JWT token
    Then the response status code should be 200

  Scenario: JWT authentication rejects wrong audience
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-wrong-audience-api
      spec:
        displayName: JWT Auth Wrong Audience API
        version: v1.0
        context: /jwt-auth-wrong-audience/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  audiences:
                    - "expected-audience"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-wrong-audience/v1.0/health" to be ready

    # Token has audience "test-audience" but API expects "expected-audience"
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I send a GET request to "http://localhost:8080/jwt-auth-wrong-audience/v1.0/protected" with the JWT token
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

  Scenario: Multiple key managers with issuer matching
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-multi-keymanager-api
      spec:
        displayName: JWT Auth Multi KeyManager API
        version: v1.0
        context: /jwt-auth-multi-km/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                    - wrong-issuer-km
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-multi-km/v1.0/health" to be ready

    # Token from mock-jwks should work
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I send a GET request to "http://localhost:8080/jwt-auth-multi-km/v1.0/protected" with the JWT token
    Then the response status code should be 200

  Scenario: JWT auth does not affect unprotected endpoints
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-partial-api
      spec:
        displayName: JWT Auth Partial API
        version: v1.0
        context: /jwt-auth-partial/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /public
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-partial/v1.0/public" to be ready

    # Public endpoint should work without token
    When I clear all headers
    And I send a GET request to "http://localhost:8080/jwt-auth-partial/v1.0/public"
    Then the response status code should be 200

    # Protected endpoint should require token
    When I send a GET request to "http://localhost:8080/jwt-auth-partial/v1.0/protected"
    Then the response status code should be 401

  Scenario: Empty Bearer token is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-empty-bearer-api
      spec:
        displayName: JWT Auth Empty Bearer API
        version: v1.0
        context: /jwt-auth-empty-bearer/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-empty-bearer/v1.0/health" to be ready

    When I clear all headers
    And I set header "Authorization" to "Bearer "
    And I send a GET request to "http://localhost:8080/jwt-auth-empty-bearer/v1.0/protected"
    Then the response status code should be 401

  Scenario: Bearer-only without token is rejected
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-bearer-only-api
      spec:
        displayName: JWT Auth Bearer Only API
        version: v1.0
        context: /jwt-auth-bearer-only/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-bearer-only/v1.0/health" to be ready

    When I clear all headers
    And I set header "Authorization" to "Bearer"
    And I send a GET request to "http://localhost:8080/jwt-auth-bearer-only/v1.0/protected"
    Then the response status code should be 401

  Scenario: scopes allOf requires every listed scope
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-scopes-allof-api
      spec:
        displayName: JWT Auth Scopes AllOf API
        version: v1.0
        context: /jwt-auth-scopes-allof/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  scopes:
                    allOf:
                      - "api:read"
                      - "api:deploy"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-scopes-allof/v1.0/health" to be ready

    # Token has both required scopes → authorized
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "api:read api:deploy api:write"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-allof/v1.0/protected" with the JWT token
    Then the response status code should be 200

    # Token missing one of the allOf scopes → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "api:read"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-allof/v1.0/protected" with the JWT token
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

  Scenario: scopes allOf and anyOf are combined
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-scopes-combined-api
      spec:
        displayName: JWT Auth Scopes Combined API
        version: v1.0
        context: /jwt-auth-scopes-combined/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  scopes:
                    allOf:
                      - "api:read"
                      - "api:deploy"
                    anyOf:
                      - "api:write"
                      - "api:update"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-scopes-combined/v1.0/health" to be ready

    # allOf satisfied AND one anyOf scope present → authorized
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "api:read api:deploy api:update"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-combined/v1.0/protected" with the JWT token
    Then the response status code should be 200

    # allOf satisfied but no anyOf scope present → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "api:read api:deploy"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-combined/v1.0/protected" with the JWT token
    Then the response status code should be 401

  Scenario: scopes takes precedence over deprecated requiredScopes
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-scopes-precedence-api
      spec:
        displayName: JWT Auth Scopes Precedence API
        version: v1.0
        context: /jwt-auth-scopes-precedence/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  requiredScopes:
                    - "api:read"
                  scopes:
                    allOf:
                      - "api:deploy"
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-scopes-precedence/v1.0/health" to be ready

    # Token satisfies the deprecated requiredScopes ("api:read") but not the new scopes
    # (allOf "api:deploy"). The new param wins → rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and scope "api:read"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-precedence/v1.0/protected" with the JWT token
    Then the response status code should be 401

  Scenario: claims allOf and anyOf are combined
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-claims-api
      spec:
        displayName: JWT Auth Claims API
        version: v1.0
        context: /jwt-auth-claims/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  claims:
                    anyOf:
                      - claim: department
                        values:
                          - platform
                          - engineering
                    allOf:
                      - claim: status
                        values:
                          - suspended
                      - claim: role
                        values:
                          - internal
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-claims/v1.0/health" to be ready

    # department in {platform, engineering} AND status=suspended AND role=internal → authorized
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=platform,status=suspended,role=internal"
    And I send a GET request to "http://localhost:8080/jwt-auth-claims/v1.0/protected" with the JWT token
    Then the response status code should be 200

    # role=external fails the allOf matcher → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=platform,status=suspended,role=external"
    And I send a GET request to "http://localhost:8080/jwt-auth-claims/v1.0/protected" with the JWT token
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

    # department not in the anyOf set → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "department=sales,status=suspended,role=internal"
    And I send a GET request to "http://localhost:8080/jwt-auth-claims/v1.0/protected" with the JWT token
    Then the response status code should be 401

  Scenario: claims takes precedence over deprecated requiredClaims
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-claims-precedence-api
      spec:
        displayName: JWT Auth Claims Precedence API
        version: v1.0
        context: /jwt-auth-claims-precedence/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  requiredClaims:
                    role: admin
                  claims:
                    allOf:
                      - claim: role
                        values:
                          - superadmin
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-claims-precedence/v1.0/health" to be ready

    # Token satisfies the deprecated requiredClaims (role=admin) but not the new claims
    # (role must be superadmin). The new param wins → rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token" and claims "role=admin"
    And I send a GET request to "http://localhost:8080/jwt-auth-claims-precedence/v1.0/protected" with the JWT token
    Then the response status code should be 401

  Scenario: scopes and claims (both new params, allOf + anyOf) are enforced together
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-scopes-claims-both-api
      spec:
        displayName: JWT Auth Scopes And Claims API
        version: v1.0
        context: /jwt-auth-scopes-claims-both/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  # SCOPES = (api:read AND api:deploy) AND (api:write OR api:update)
                  scopes:
                    allOf:
                      - "api:read"
                      - "api:deploy"
                    anyOf:
                      - "api:write"
                      - "api:update"
                  # CLAIMS = status=active AND (department in {platform, engineering})
                  claims:
                    allOf:
                      - claim: status
                        values:
                          - active
                    anyOf:
                      - claim: department
                        values:
                          - platform
                          - engineering
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-scopes-claims-both/v1.0/health" to be ready

    # All satisfied: scope allOf+anyOf and claim allOf+anyOf → authorized
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:deploy api:write" and claims "status=active,department=platform"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-claims-both/v1.0/protected" with the JWT token
    Then the response status code should be 200

    # Scope allOf satisfied but anyOf (write/update) absent → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:deploy" and claims "status=active,department=engineering"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-claims-both/v1.0/protected" with the JWT token
    Then the response status code should be 401

    # Scope anyOf satisfied but allOf incomplete (api:deploy missing) → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:write" and claims "status=active,department=platform"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-claims-both/v1.0/protected" with the JWT token
    Then the response status code should be 401

    # Scopes fully satisfied but claim anyOf (department) not in set → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:deploy api:update" and claims "status=active,department=sales"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-claims-both/v1.0/protected" with the JWT token
    Then the response status code should be 401

    # Scopes fully satisfied and claim anyOf ok but claim allOf (status) fails → rejected
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:deploy api:write" and claims "status=inactive,department=platform"
    And I send a GET request to "http://localhost:8080/jwt-auth-scopes-claims-both/v1.0/protected" with the JWT token
    Then the response status code should be 401

  Scenario: mixed old/new params across two operations on one API (allOf + anyOf)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-mixed-ops-api
      spec:
        displayName: JWT Auth Mixed Ops API
        version: v1.0
        context: /jwt-auth-mixed-ops/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          # op1: new scopes (allOf + anyOf) + deprecated requiredClaims
          - method: GET
            path: /op1
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  scopes:
                    allOf:
                      - "api:read"
                    anyOf:
                      - "api:write"
                      - "api:update"
                  requiredClaims:
                    role: admin
          # op2: deprecated requiredScopes + new claims (allOf + anyOf)
          - method: GET
            path: /op2
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  requiredScopes:
                    - "api:read"
                  claims:
                    allOf:
                      - claim: status
                        values:
                          - active
                    anyOf:
                      - claim: department
                        values:
                          - platform
                          - engineering
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-mixed-ops/v1.0/health" to be ready

    # Token A: scope "api:read api:write", role=admin, status=inactive, department=sales.
    # op1: scope allOf(api:read) + anyOf(api:write) + requiredClaims role=admin → authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:write" and claims "role=admin,status=inactive,department=sales"
    And I send a GET request to "http://localhost:8080/jwt-auth-mixed-ops/v1.0/op1" with the JWT token
    Then the response status code should be 200

    # Same Token A on op2: old requiredScopes ok, but new claim allOf status=inactive fails → rejected.
    When I send a GET request to "http://localhost:8080/jwt-auth-mixed-ops/v1.0/op2" with the JWT token
    Then the response status code should be 401

    # Token B: scope "api:read" (no anyOf scope), role=user, status=active, department=engineering.
    # op2: old requiredScopes ok, new claim allOf(status=active) + anyOf(department) → authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read" and claims "role=user,status=active,department=engineering"
    And I send a GET request to "http://localhost:8080/jwt-auth-mixed-ops/v1.0/op2" with the JWT token
    Then the response status code should be 200

    # Same Token B on op1: scope allOf(api:read) ok but anyOf(write/update) absent → rejected.
    When I send a GET request to "http://localhost:8080/jwt-auth-mixed-ops/v1.0/op1" with the JWT token
    Then the response status code should be 401

  Scenario: new params win over deprecated formats on one operation (allOf + anyOf)
    Given I authenticate using basic auth as "admin"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: jwt-auth-both-formats-api
      spec:
        displayName: JWT Auth Both Formats API
        version: v1.0
        context: /jwt-auth-both-formats/$version
        upstream:
          main:
            url: http://sample-backend:9080/api/v1
        operations:
          - method: GET
            path: /health
          - method: GET
            path: /protected
            policies:
              - name: jwt-auth
                version: v1
                params:
                  issuers:
                    - mock-jwks
                  requiredScopes:
                    - "api:legacy"
                  requiredClaims:
                    role: legacy
                  scopes:
                    allOf:
                      - "api:read"
                    anyOf:
                      - "api:write"
                      - "api:update"
                  claims:
                    allOf:
                      - claim: status
                        values:
                          - active
                    anyOf:
                      - claim: department
                        values:
                          - platform
                          - engineering
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/jwt-auth-both-formats/v1.0/health" to be ready

    # Satisfies the NEW params (scope allOf+anyOf, claim allOf+anyOf); fails the deprecated ones,
    # which are ignored → authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:update" and claims "status=active,department=engineering"
    And I send a GET request to "http://localhost:8080/jwt-auth-both-formats/v1.0/protected" with the JWT token
    Then the response status code should be 200

    # Satisfies only the deprecated params (scope api:legacy, role=legacy); fails the new ones,
    # which take precedence → rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:legacy" and claims "role=legacy"
    And I send a GET request to "http://localhost:8080/jwt-auth-both-formats/v1.0/protected" with the JWT token
    Then the response status code should be 401

    # New scope allOf ok but anyOf (write/update) absent → rejected (proves anyOf enforced here too).
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read" and claims "status=active,department=platform"
    And I send a GET request to "http://localhost:8080/jwt-auth-both-formats/v1.0/protected" with the JWT token
    Then the response status code should be 401

    # New scopes fully satisfied but claim anyOf (department) not in set → rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token", scope "api:read api:write" and claims "status=active,department=sales"
    And I send a GET request to "http://localhost:8080/jwt-auth-both-formats/v1.0/protected" with the JWT token
    Then the response status code should be 401
