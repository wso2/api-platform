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

@mtls @mtls-auth
Feature: Authenticating API callers with a client certificate
  As an API developer
  I want callers of my API to be authenticated by the certificate they present
  So that only certificates from the authorities I accept, narrowed the way I choose, reach my backend

  The gateway validates every presented certificate against the pool and reports the verdict; the
  mtls-auth policy denies on a negative verdict and otherwise applies the API's accept list: the
  certificate must chain to the named authority (verified cryptographically, never by comparing
  names), and match every narrowing the entry carries. Entries are tried in order, first match
  wins. Every rejection is the same 401 body. A public API ignores whatever certificate arrives and
  its backend never sees the forwarded-certificate header; an mtls-auth API forwards it only for a
  certificate it accepted.

  Every scenario starts from an empty client authority pool, and every HTTPS request waits until
  the gateway has applied the pool the scenario built.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "mtls-auth" and store it as "apiName"
    And I generate a unique API version from "mtls-auth" and store it as "apiVersion"
    And I generate a unique API context from "/mtls-auth" and store it as "apiContext"
    And I generate a unique resource name from "auth-ca-a" and store it as "caA"
    And I generate a unique resource name from "auth-ca-b" and store it as "caB"
    And I generate a unique resource name from "auth-lookalike" and store it as "lookalike"
    And I generate a unique resource name from "anchor-entry" and store it as "anchorEntry"
    And I generate a unique resource name from "anchor-root" and store it as "anchorRoot"
    And I generate a unique resource name from "anchor-intermediate" and store it as "anchorIntermediate"

  # ==================== THE ACCEPT LIST IS EVALUATED ON EVERY REQUEST ====================

  Scenario: A client that caches its TLS session is accepted again on a new connection
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | ${CTX:apiVersion}          |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.policies          | [{"name":"mtls-auth","version":"v0","params":{"accept":[{"ca":"${CTX:caA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/anything" until the route answers 401
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" with client certificate "client-valid" on a resumable TLS session
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/anything" on a new connection from the same TLS session cache
    Then the gateway should have run a full TLS handshake
    And the response status code should be 200

  # ==================== PUBLIC APIS AND THE FORWARDED-CERTIFICATE HEADER ====================

  Scenario Outline: A public API ignores whatever certificate arrives and its backend never sees the header
    Given I generate a unique value from "mtls-public" and store it as "publicApiName"
    And I generate a unique API version from "mtls-public" and store it as "publicApiVersion"
    And I generate a unique API context from "/mtls-public" and store it as "publicApiContext"
    And the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}  |
      | name                   | ${CTX:apiName}             |
      | spec.displayName       | ${CTX:apiName}             |
      | spec.version           | ${CTX:apiVersion}          |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002      |
      | spec.policies          | [{"name":"mtls-auth","version":"v0"}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:publicApiName}             |
      | spec.displayName       | ${CTX:publicApiName}             |
      | spec.version           | ${CTX:publicApiVersion}          |
      | spec.context           | ${CTX:publicApiContext}/$version |
      | spec.upstream.main.url | http://testbench:3002            |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:publicApiContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    When I send a "GET" request over HTTPS to "${CTX:publicApiContext}/${CTX:publicApiVersion}/anything" <presenting>
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"

    Examples:
      | presenting                                       |
      | with no client certificate                       |
      | with client certificate "client-valid"           |
      | with client certificate "client-wrong-ca"        |
      | with client certificate "client-expired"         |
      | with client certificate "client-selfsigned-b"    |
      | with client certificate "client-serverauth-only" |
