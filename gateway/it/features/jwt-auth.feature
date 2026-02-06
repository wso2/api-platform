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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
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
                version: v0
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
