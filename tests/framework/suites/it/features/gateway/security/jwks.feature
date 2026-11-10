# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the License at
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

# The block runs the gateway with the gateway-signing overlay: an active EC P-256 key whose kid
# defaults to its RFC 7638 thumbprint, and a verify-only RSA key with an explicit kid. The
# thumbprint below is that of the fixed test key in core/catalog/platformgateway/signing-keys.
@gateway-jwks
Feature: Gateway JWKS endpoint
  As the owner of a backend or client that receives gateway-signed tokens
  I want the gateway to publish its signing keys as a JWK Set
  So that I can verify its signatures by key ID instead of copying keys out of band

  Background:
    Given the gateway services are running

  @type:smoke
  Scenario: The gateway publishes every configured signing key without private members
    When I send a "GET" request to "/.well-known/jwks" until status 200
    Then the response header "Content-Type" should contain "application/jwk-set+json"
    And the response header "Cache-Control" should be "public, max-age=300"
    And the response header "ETag" should exist
    And the response header "Access-Control-Allow-Origin" should be "*"
    And the response should be valid JSON
    And the JSON response array field "keys" should have 2 items
    And the JSON response array "keys" item with "kid" equal to "GqWj6qPTMk3medznt-SHvWjANJNmVkHUtMMUhRWhV3I" should have "kty" equal to "EC"
    And the JSON response array "keys" item with "kid" equal to "GqWj6qPTMk3medznt-SHvWjANJNmVkHUtMMUhRWhV3I" should have "crv" equal to "P-256"
    And the JSON response array "keys" item with "kid" equal to "GqWj6qPTMk3medznt-SHvWjANJNmVkHUtMMUhRWhV3I" should have "alg" equal to "ES256"
    And the JSON response array "keys" item with "kid" equal to "GqWj6qPTMk3medznt-SHvWjANJNmVkHUtMMUhRWhV3I" should not have field "d"
    And the JSON response array "keys" item with "kid" equal to "it-gateway-previous" should have "kty" equal to "RSA"
    And the JSON response array "keys" item with "kid" equal to "it-gateway-previous" should have "alg" equal to "RS256"
    And the JSON response array "keys" item with "kid" equal to "it-gateway-previous" should not have field "d"
    And the JSON response array "keys" item with "kid" equal to "it-gateway-previous" should not have field "p"
    And the JSON response array "keys" item with "kid" equal to "it-gateway-previous" should not have field "q"

  Scenario: A conditional request with the current ETag is answered without a body
    When I send a "GET" request to "/.well-known/jwks" until status 200
    And I store the response header "ETag" as "etag"
    And I set header "If-None-Match" to "${CTX:etag}"
    And I send a "GET" request to "/.well-known/jwks"
    Then the response status code should be 304
    And the response body should be empty
    And the response header "Cache-Control" should be "public, max-age=300"

  Scenario: A HEAD request returns the JWK Set headers
    When I send a "GET" request to "/.well-known/jwks" until status 200
    And I store the response header "ETag" as "etag"
    And I send a "HEAD" request to "/.well-known/jwks"
    Then the response status code should be 200
    And the response header "ETag" should be "${CTX:etag}"
    And the response header "Content-Type" should contain "application/jwk-set+json"

  @type:negative
  Scenario: Methods other than GET and HEAD are rejected
    When I send a "POST" request to "/.well-known/jwks"
    Then the response status code should be 405

  Scenario: The JWK Set is served on any host the gateway accepts
    When I set request host to "jwks-check.localhost"
    And I send a "GET" request to "/.well-known/jwks" until status 200
    Then the response header "Content-Type" should contain "application/jwk-set+json"
    And the JSON response array field "keys" should have 2 items

  @type:negative
  Scenario: An API cannot claim the JWKS path while the endpoint is enabled
    Given I authenticate using basic auth as "admin"
    And I generate a unique value from "jwks-shadow-api" and store it as "apiName"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}             |
      | name                   | ${CTX:apiName}                        |
      | spec.displayName       | JWKS-Shadow-API                       |
      | spec.version           | v1.0                                  |
      | spec.context           | /.well-known                          |
      | spec.upstream.main.url | http://testbench:3000/api/v2          |
      | spec.operations        | [{"method":"GET","path":"/jwks"}]     |
    Then the response should be a client error
    And the response should be valid JSON
    And the JSON response field "status" should be "error"
    And the response body should contain "Configuration validation failed"
    And the response body should contain "reserved gateway JWKS endpoint"
