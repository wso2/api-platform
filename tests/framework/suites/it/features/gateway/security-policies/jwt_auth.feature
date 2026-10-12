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

@jwt-auth
Feature: JWT authentication
  As an API developer
  I want to secure my APIs with JWT authentication
  So that only authorized requests with valid tokens can access my resources
  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  Scenario: A request with a valid JWT token is authorized
    Given I generate a unique value from "jwt-auth-basic" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-basic" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-basic" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: A request without an authorization header is rejected
    Given I generate a unique value from "jwt-auth-no-header" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-no-header" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-no-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401
    And the response body should contain "Authentication failed"

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: A request with an invalid JWT token is rejected
    Given I generate a unique value from "jwt-auth-invalid-token" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-invalid-token" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-invalid-token" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I set header "Authorization" to "Bearer invalid.jwt.token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: A request with a malformed Bearer header is rejected
    Given I generate a unique value from "jwt-auth-malformed" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-malformed" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-malformed" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I set header "Authorization" to "NotBearer sometoken"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: A request with the wrong issuer is rejected
    Given I generate a unique value from "jwt-auth-wrong-issuer" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-wrong-issuer" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-wrong-issuer" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["wrong-issuer-km"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # Token issued for "mock-jwks" but the operation only trusts "wrong-issuer-km"
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: JWT authentication with audience validation succeeds
    Given I generate a unique value from "jwt-auth-audience" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-audience" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-audience" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"audiences":["test-audience"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: JWT authentication rejects the wrong audience
    Given I generate a unique value from "jwt-auth-wrong-audience" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-wrong-audience" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-wrong-audience" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"audiences":["expected-audience"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: Multiple key managers are matched by issuer
    Given I generate a unique value from "jwt-auth-multi-km" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-multi-km" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-multi-km" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks","wrong-issuer-km"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: JWT auth does not affect unprotected endpoints
    Given I generate a unique value from "jwt-auth-partial" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-partial" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-partial" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/public"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/public" until status 200
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: An empty Bearer token is rejected
    Given I generate a unique value from "jwt-auth-empty-bearer" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-empty-bearer" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-empty-bearer" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I set header "Authorization" to "Bearer "
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404

  Scenario: A Bearer scheme without a token is rejected
    Given I generate a unique value from "jwt-auth-bearer-only" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-bearer-only" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-bearer-only" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    When I set header "Authorization" to "Bearer"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 404
