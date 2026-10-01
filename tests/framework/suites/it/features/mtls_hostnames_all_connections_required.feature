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

@mtls @mtls-hostnames @mtls-all-connections
Feature: Asking every connection while a dedicated hostname is required
  As a gateway operator
  I want to know that asking every connection takes precedence over requiring a dedicated hostname
  So that no deploy is refused for a hostname that decides nothing

  With client_certificate_request set to all_connections, mtls_requires_dedicated_hostname has no
  effect: no deploy is refused and none carries a hostname warning.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "hn-mtls-api" and store it as "mtlsApiName"
    And I generate a unique API version from "hn-mtls-api" and store it as "mtlsApiVersion"
    And I generate a unique API context from "/hn-mtls" and store it as "mtlsContext"
    And I generate a unique resource name from "hn-mtls-host" and store it as "mtlsLabel"
    And I generate a unique value from "hn-public-api" and store it as "publicApiName"
    And I generate a unique API version from "hn-public-api" and store it as "publicApiVersion"
    And I generate a unique API context from "/hn-public" and store it as "publicContext"
    And I generate a unique resource name from "hn-public-host" and store it as "publicLabel"
    And I generate a unique value from "hn-other-api" and store it as "otherApiName"
    And I generate a unique API version from "hn-other-api" and store it as "otherApiVersion"
    And I generate a unique API context from "/hn-other" and store it as "otherContext"
    And I generate a unique resource name from "hn-other-host" and store it as "otherLabel"
    And I generate a unique resource name from "hn-sandbox-host" and store it as "sandboxLabel"
    And I generate a unique resource name from "hn-ca-a" and store it as "caA"
    And I generate a unique resource name from "hn-ca-b" and store it as "caB"
    And I generate a unique resource name from "hn-relay" and store it as "relayName"

  Scenario: A deploy without its own hostname succeeds because every connection is asked
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response status code should be 201
    And the JSON response field "status.warnings" should not exist
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate
    And a TLS connection with no server name should be asked for a client certificate

  Scenario: A sandbox upstream without its own hostname succeeds because every connection is asked
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion                | ${CTX:gatewaySpecVersion} |
      | name                      | ${CTX:mtlsApiName} |
      | spec.displayName          | ${CTX:mtlsApiName} |
      | spec.version              | ${CTX:mtlsApiVersion} |
      | spec.context              | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main          | ${CTX:mtlsLabel}.example |
      | spec.upstream.main.url    | http://testbench:3002 |
      | spec.upstream.sandbox.url | http://testbench:3002 |
      | spec.policies             | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations           | [{"method":"GET","path":"/anything"}] |
    Then the response status code should be 201
    And the JSON response field "status.warnings" should not exist
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:sandboxLabel}.example" should be asked for a client certificate
