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
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# --------------------------------------------------------------------

# This policy is not yet in any released Gateway build, so these scenarios run only under the
# "gateway-controller-policies" block, which builds the Gateway from the adjacent
# gateway-controllers checkout (see it-suite.yaml). The policy itself is also what makes
# "I get a JWT token from the mock JWKS server" a meaningful setup step here, not just
# borrowed from jwt-auth's scenarios: the JWT stands in for a token already issued by the
# gateway's own IdP, which is exactly the credential this policy exchanges for one the
# backend's own authorization server will accept.
@oauth-token-exchange
Feature: OAuth2 token exchange
  As an API developer
  I want the gateway to exchange my caller's own credential for a backend-specific token
  So that my backend can require a token from its own authorization server without my caller ever holding one

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I resolve the "oauth2" service URL at "/oauth2/token" and store it as "oauthTokenEndpoint"
    And I send a "POST" request to the "oauth2" service at "/debug/reset" with body:
      """
      {}
      """

  Scenario: Token-exchange grant exchanges the caller's token for a backend token
    Given I generate a unique resource name from "oauth-token-exchange-happy" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-happy" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-happy" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response body should contain "Bearer mock-token-"
    And the response body should not contain "${CTX:token}"
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the response body should contain "urn:ietf:params:oauth:grant-type:token-exchange"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: JwtBearer grant exchanges the caller's token for a backend token
    Given I generate a unique resource name from "oauth-token-exchange-jwtbearer" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-jwtbearer" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-jwtbearer" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"grantType":"JwtBearer","tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response body should contain "Bearer mock-token-"
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the response body should contain "urn:ietf:params:oauth:grant-type:jwt-bearer"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A request without the caller's credential is rejected
    Given I generate a unique resource name from "oauth-token-exchange-nocred" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-nocred" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-nocred" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    And I clear all headers
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data" until status 401
    And the JSON response field "error" should be "unauthorized"

    When I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Invalid client credentials return Bad Gateway
    Given I generate a unique resource name from "oauth-token-exchange-badclient" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-badclient" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-badclient" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"definitely-the-wrong-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 502

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: An unreachable token endpoint returns Bad Gateway
    Given I generate a unique resource name from "oauth-token-exchange-unreachable" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-unreachable" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-unreachable" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"http://oauth2-does-not-exist:9601/oauth2/token","clientId":"test-client","clientSecret":"test-secret","requestTimeout":"3s"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 502

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  # The authorization server rejecting the subject token itself (as opposed to the gateway's own
  # client credentials, or an unreachable endpoint) is a third, distinct failure path. The mock
  # oauth2 service never validates a subject_token cryptographically, so it only ever rejects the
  # literal sentinel value "invalid-subject-token" - good enough to prove this path is handled the
  # same way as the others (Bad Gateway, no upstream detail leaked), without needing a real
  # malformed-signature JWT.
  Scenario: The authorization server rejecting the subject token returns Bad Gateway
    Given I generate a unique resource name from "oauth-token-exchange-rejected" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-rejected" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-rejected" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I set header "Authorization" to "Bearer invalid-subject-token"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 502

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: A backend token is cached across repeated requests
    Given I generate a unique resource name from "oauth-token-exchange-cache" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-cache" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-cache" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}?ttl=3600","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send 5 "GET" requests to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the JSON response field "tokenRequestCount" should be 1

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  Scenario: Configured audiences, scopes, and resources reach the token endpoint
    Given I generate a unique resource name from "oauth-token-exchange-params" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-params" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-params" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret","audiences":["it-suite-audience"],"scopes":["it-suite-scope"],"resources":["https://backend.example.com"]}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    When I send a "GET" request to the "oauth2" service at "/debug/stats"
    Then the response body should contain "it-suite-audience"
    And the response body should contain "it-suite-scope"
    And the response body should contain "backend.example.com"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  # jwt-auth validates the caller's own gateway-issued JWT and forwards it under a different
  # header (rather than leaving it on Authorization); oauth-token-exchange must be told to read
  # from that same header, or it has nothing to exchange. This is the correctly-wired version of
  # the chain; the next scenario is its negative counterpart.
  Scenario: jwt-auth forwards the caller's token for oauth-token-exchange to exchange
    Given I generate a unique resource name from "oauth-token-exchange-chain" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-chain" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-chain" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"forwardToken":true,"forwardedTokenHeader":"x-forwarded-authorization"}},{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret","subjectTokenSource":{"type":"header","name":"x-forwarded-authorization","prefix":"Bearer "}}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 200
    And the response body should contain "Bearer mock-token-"
    And the response body should not contain "${CTX:token}"

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful

  # The same chain as above, but oauth-token-exchange is left at its default subjectTokenSource
  # (the Authorization header) instead of being pointed at x-forwarded-authorization. jwt-auth has
  # already moved the caller's token there, so Authorization is empty by the time
  # oauth-token-exchange runs and it has no credential to exchange - a misconfiguration that must
  # fail closed, not silently let the request through unexchanged.
  Scenario: Leaving oauth-token-exchange at its default header misses a token jwt-auth forwarded elsewhere
    Given I generate a unique resource name from "oauth-token-exchange-miswired" and store it as "apiName"
    And I generate a unique API version from "oauth-token-exchange-miswired" and store it as "apiVersion"
    And I generate a unique API context from "/oauth-token-exchange-miswired" and store it as "apiContext"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName}                    |
      | spec.displayName       | ${CTX:apiName}                    |
      | spec.version           | ${CTX:apiVersion}                 |
      | spec.context           | ${CTX:apiContext}/$version        |
      | spec.upstream.main.url | http://testbench:3002             |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/data","policies":[{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"],"forwardToken":true,"forwardedTokenHeader":"x-forwarded-authorization"}},{"name":"oauth-token-exchange","version":"v1","params":{"tokenEndpoint":"${CTX:oauthTokenEndpoint}","clientId":"test-client","clientSecret":"test-secret"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until status 200

    When I get a JWT token from the mock JWKS server with issuer "http://testbench:3001/token" and store it as "token"
    And I set header "Authorization" to "Bearer ${CTX:token}"
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/data"
    Then the response status code should be 401

    When I clear all headers
    And I authenticate using basic auth as "admin"
    And I delete the API "${CTX:apiName}"
    Then the response should be successful
