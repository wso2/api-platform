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
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: A request without an authorization header is rejected
    Given I generate a unique value from "jwt-auth-no-header" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-no-header" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-no-header" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401
    And the response body should contain "Authentication failed"

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A request with an invalid JWT token is rejected
    Given I generate a unique value from "jwt-auth-invalid-token" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-invalid-token" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-invalid-token" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: A request with a malformed Bearer header is rejected
    Given I generate a unique value from "jwt-auth-malformed" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-malformed" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-malformed" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: A request with the wrong issuer is rejected
    Given I generate a unique value from "jwt-auth-wrong-issuer" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-wrong-issuer" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-wrong-issuer" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: JWT authentication with audience validation succeeds
    Given I generate a unique value from "jwt-auth-audience" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-audience" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-audience" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: JWT authentication rejects the wrong audience
    Given I generate a unique value from "jwt-auth-wrong-audience" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-wrong-audience" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-wrong-audience" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: Multiple key managers are matched by issuer
    Given I generate a unique value from "jwt-auth-multi-km" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-multi-km" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-multi-km" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: JWT auth does not affect unprotected endpoints
    Given I generate a unique value from "jwt-auth-partial" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-partial" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-partial" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: An empty Bearer token is rejected
    Given I generate a unique value from "jwt-auth-empty-bearer" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-empty-bearer" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-empty-bearer" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: A Bearer scheme without a token is rejected
    Given I generate a unique value from "jwt-auth-bearer-only" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-bearer-only" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-bearer-only" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
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

  Scenario: scopes allOf requires every listed scope
    Given I generate a unique value from "jwt-auth-scopes-allof" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-scopes-allof" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-scopes-allof" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"scopes":{"allOf":["api:read","api:deploy"]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # Token has both required scopes -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "api:read api:deploy api:write" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    # Token missing one of the allOf scopes -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "api:read" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: scopes allOf and anyOf are combined
    Given I generate a unique value from "jwt-auth-scopes-combined" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-scopes-combined" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-scopes-combined" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"scopes":{"allOf":["api:read","api:deploy"],"anyOf":["api:write","api:update"]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # allOf satisfied AND one anyOf scope present -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "api:read api:deploy api:update" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    # allOf satisfied but no anyOf scope present -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "api:read api:deploy" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: scopes takes precedence over deprecated requiredScopes
    Given I generate a unique value from "jwt-auth-scopes-prec" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-scopes-prec" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-scopes-prec" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"requiredScopes":["api:read"],"scopes":{"allOf":["api:deploy"]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # Token satisfies the deprecated requiredScopes ("api:read") but not the new scopes
    # (allOf "api:deploy"). The new param wins -> rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and scope "api:read" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: claims allOf and anyOf are combined
    Given I generate a unique value from "jwt-auth-claims" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-claims" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-claims" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"claims":{"anyOf":[{"claim":"department","values":["platform","engineering"]}],"allOf":[{"claim":"status","values":["suspended"]},{"claim":"role","values":["internal"]}]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # department in {platform, engineering} AND status=suspended AND role=internal -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=platform,status=suspended,role=internal" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    # role=external fails the allOf matcher -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=platform,status=suspended,role=external" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401
    And the response body should contain "Authentication failed"

    # department not in the anyOf set -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "department=sales,status=suspended,role=internal" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: claims takes precedence over deprecated requiredClaims
    Given I generate a unique value from "jwt-auth-claims-prec" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-claims-prec" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-claims-prec" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"requiredClaims":{"role":"admin"},"claims":{"allOf":[{"claim":"role","values":["superadmin"]}]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # Token satisfies the deprecated requiredClaims (role=admin) but not the new claims
    # (role must be superadmin). The new param wins -> rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and claims "role=admin" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: scopes and claims (both new params, allOf and anyOf) are enforced together
    Given I generate a unique value from "jwt-auth-scopes-claims" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-scopes-claims" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-scopes-claims" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"scopes":{"allOf":["api:read","api:deploy"],"anyOf":["api:write","api:update"]},"claims":{"allOf":[{"claim":"status","values":["active"]}],"anyOf":[{"claim":"department","values":["platform","engineering"]}]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # All satisfied: scope allOf+anyOf and claim allOf+anyOf -> authorized
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:deploy api:write" and claims "status=active,department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    # Scope allOf satisfied but anyOf (write/update) absent -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:deploy" and claims "status=active,department=engineering" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    # Scope anyOf satisfied but allOf incomplete (api:deploy missing) -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:write" and claims "status=active,department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    # Scopes fully satisfied but claim anyOf (department) not in set -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:deploy api:update" and claims "status=active,department=sales" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    # Scopes fully satisfied and claim anyOf ok but claim allOf (status) fails -> rejected
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:deploy api:write" and claims "status=inactive,department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Mixed old and new params across two operations on one API
    Given I generate a unique value from "jwt-auth-mixed-ops" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-mixed-ops" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-mixed-ops" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/op1","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"scopes":{"allOf":["api:read"],"anyOf":["api:write","api:update"]},"requiredClaims":{"role":"admin"}}}]},{"method":"GET","path":"/op2","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"requiredScopes":["api:read"],"claims":{"allOf":[{"claim":"status","values":["active"]}],"anyOf":[{"claim":"department","values":["platform","engineering"]}]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/op1" until status 401
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/op2" until status 401

    # Token A: scope "api:read api:write", role=admin, status=inactive, department=sales.
    # op1: scope allOf(api:read) + anyOf(api:write) + requiredClaims role=admin -> authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:write" and claims "role=admin,status=inactive,department=sales" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/op1"
    Then the response status code should be 200

    # Same Token A on op2: old requiredScopes ok, but new claim allOf status=inactive fails -> rejected.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/op2"
    Then the response status code should be 401

    # Token B: scope "api:read" (no anyOf scope), role=user, status=active, department=engineering.
    # op2: old requiredScopes ok, new claim allOf(status=active) + anyOf(department) -> authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read" and claims "role=user,status=active,department=engineering" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/op2"
    Then the response status code should be 200

    # Same Token B on op1: scope allOf(api:read) ok but anyOf(write/update) absent -> rejected.
    When I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/op1"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: New params win over deprecated formats on one operation
    Given I generate a unique value from "jwt-auth-both-formats" and store it as "apiName"
    And I generate a unique API version from "jwt-auth-both-formats" and store it as "apiVersion"
    And I generate a unique API context from "/jwt-auth-both-formats" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | gateway.api-platform.wso2.com/v1 |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002              |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"requiredScopes":["api:legacy"],"requiredClaims":{"role":"legacy"},"scopes":{"allOf":["api:read"],"anyOf":["api:write","api:update"]},"claims":{"allOf":[{"claim":"status","values":["active"]}],"anyOf":[{"claim":"department","values":["platform","engineering"]}]}}}]}] |
    Then the response should be successful

    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected" until status 401

    # Satisfies the NEW params (scope allOf+anyOf, claim allOf+anyOf); fails the deprecated ones,
    # which are ignored -> authorized.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:update" and claims "status=active,department=engineering" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 200

    # Satisfies only the deprecated params (scope api:legacy, role=legacy); fails the new ones,
    # which take precedence -> rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:legacy" and claims "role=legacy" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    # New scope allOf ok but anyOf (write/update) absent -> rejected (proves anyOf enforced here too).
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read" and claims "status=active,department=platform" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    # New scopes fully satisfied but claim anyOf (department) not in set -> rejected.
    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token", scope "api:read api:write" and claims "status=active,department=sales" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/protected"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
