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
Feature: Asking every connection for a client certificate
  As a gateway operator
  I want to have the listener ask every connection for a client certificate
  So that callers that send no server name, or one other than the API's, authenticate

  With router.downstream_tls.client_certificate_request set to all_connections the listener asks
  every connection whatever its hostname, once a deployed API attaches mtls-auth. No hostname
  warning is raised, since no hostname decides what is asked.

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

  Scenario: Every hostname is asked once an API attaches mtls-auth and none before
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:publicApiName} |
      | spec.displayName       | ${CTX:publicApiName} |
      | spec.version           | ${CTX:publicApiVersion} |
      | spec.context           | ${CTX:publicContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "${CTX:publicLabel}.example"
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with no server name should be asked for a client certificate
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:publicLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    When I delete the API "${CTX:mtlsApiName}"
    Then the response should be successful
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate

  Scenario: An API on the default hostname deploys without a hostname warning
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:publicApiName} |
      | spec.displayName       | ${CTX:publicApiName} |
      | spec.version           | ${CTX:publicApiVersion} |
      | spec.context           | ${CTX:publicContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "${CTX:publicLabel}.example"
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate

  Scenario: A sandbox upstream on the default sandbox hostname deploys without a hostname warning
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
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:sandboxLabel}.example" should be asked for a client certificate

  Scenario: A caller sending no server name, or another one, is asked and authenticates
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with no server name should be asked for a client certificate
    Given HTTPS requests send no server name
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate

  Scenario: A second connection never resumes a TLS session on any hostname while every connection is asked
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:publicApiName} |
      | spec.displayName       | ${CTX:publicApiName} |
      | spec.version           | ${CTX:publicApiVersion} |
      | spec.context           | ${CTX:publicContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And I set request host to "${CTX:publicLabel}.example"
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:publicLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" with client certificate "client-valid" on a resumable TLS session
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" on a new connection from the same TLS session cache
    Then the response status code should be 200
    And the gateway should have run a full TLS handshake
    And the HTTPS connection should have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid" on a resumable TLS session
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" on a new connection from the same TLS session cache
    Then the response status code should be 200
    And the gateway should have run a full TLS handshake
    And the HTTPS connection should have been asked for a client certificate
