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

@mtls @mtls-hostnames
Feature: Scoping the client certificate request to hostnames
  As a gateway operator
  I want the HTTPS listener to ask for a client certificate only on the hostnames of mutual TLS APIs
  So that callers of every other API are never asked and keep TLS session resumption

  The listener decides during the TLS handshake, from the server name the caller sends, so every
  scenario names the server name its HTTPS requests send and the Host header they carry
  separately. The listener serves one self-signed certificate whatever the server name, so the
  requests skip verifying it. Hostnames are generated for each scenario.

  A hostname is asked when an API that attaches mtls-auth is served on it. An API on a hostname the
  listener cannot match, such as the gateway default, or a relay entry in the pool makes the
  listener ask every connection instead; a deploy that causes the first carries the
  MTLS_HOSTNAME_NOT_SCOPED warning. The listener's state follows the configuration with a delay,
  so every check that a hostname is, or is no longer, asked waits for the listener to say so.

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

  Scenario: Only the hostnames of mutual TLS APIs are asked for a certificate
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:publicLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should not have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate

  Scenario: A caller without a certificate on a mutual TLS hostname is asked and refused
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with no client certificate
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    And the HTTPS connection should have been asked for a client certificate

  Scenario: A public server name never carries a caller past a mutual TLS API
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    And the HTTPS connection should not have been asked for a client certificate

  Scenario: A caller sending no server name is not asked and is refused by a mutual TLS API
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send no server name
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    And the HTTPS connection should not have been asked for a client certificate
    And a TLS connection with no server name should not be asked for a client certificate

  # The warning names spec.vhosts.main, and the listener asks the public hostname too.
  Scenario: An API on the default hostname makes the listener ask every connection until it is deleted
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:otherApiName} |
      | spec.displayName       | ${CTX:otherApiName} |
      | spec.version           | ${CTX:otherApiVersion} |
      | spec.context           | ${CTX:otherContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings[0].code" should be "MTLS_HOSTNAME_NOT_SCOPED"
    And the JSON response field "status.warnings[0].field" should be "spec.vhosts.main"
    And the JSON response field "status.warnings[0].message" should be "this API is served on a hostname the HTTPS listener cannot match (a gateway default, an IP address, or a pattern other than an exact name or a leading *.), so the listener asks every connection for a client certificate; give it its own vhosts.main to limit that to its hostname"
    And I set request host to "${CTX:otherLabel}.example"
    And I send a "GET" request to "${CTX:otherContext}/${CTX:otherApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    When I delete the API "${CTX:otherApiName}"
    Then the response should be successful
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate

  Scenario: A sandbox upstream on the default sandbox hostname warns and makes the listener ask every connection
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
    And the JSON response field "status.warnings[0].code" should be "MTLS_HOSTNAME_NOT_SCOPED"
    And the JSON response field "status.warnings[0].field" should be "spec.vhosts.sandbox"
    And the JSON response field "status.warnings[0].message" should be "this API is served on a hostname the HTTPS listener cannot match (a gateway default, an IP address, or a pattern other than an exact name or a leading *.), so the listener asks every connection for a client certificate; give it its own vhosts.sandbox to limit that to its hostname"
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate

  Scenario: A sandbox upstream with its own hostname is asked with the main hostname and the public hostname is not
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
      | spec.vhosts.sandbox       | ${CTX:sandboxLabel}.example |
      | spec.policies             | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations           | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
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
    And I set request host to "${CTX:sandboxLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:sandboxLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:sandboxLabel}.example"
    And I set request host to "${CTX:sandboxLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate

  Scenario: A wildcard hostname is asked for every name it covers and the public hostname is not
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | "*.${CTX:mtlsLabel}.example" |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:publicApiName} |
      | spec.displayName       | ${CTX:publicApiName} |
      | spec.version           | ${CTX:publicApiVersion} |
      | spec.context           | ${CTX:publicContext}/$version |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "a.${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And I set request host to "${CTX:publicLabel}.example"
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    And a TLS connection with server name "a.${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "b.${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "a.${CTX:mtlsLabel}.example"
    And I set request host to "a.${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate

  Scenario: A relay entry makes the listener ask every connection until it is removed
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    When the certificate fixture "edge-lb-ca" is pooled as "${CTX:relayName}" as a relay
    And I store the JSON response field "id" as "relayId"
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with no server name should be asked for a client certificate
    When I send a "DELETE" request to the "gateway-controller" service at "/certificates/${CTX:relayId}"
    Then the response should be successful
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    And a TLS connection with no server name should not be asked for a client certificate
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate

  Scenario: Connections to a public hostname resume TLS sessions and connections to a mutual TLS hostname never do
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
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:publicLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" with client certificate "client-valid" on a resumable TLS session
    Then the response status code should be 200
    And the HTTPS connection should not have been asked for a client certificate
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" on a new connection from the same TLS session cache
    Then the response status code should be 200
    And the gateway should have resumed the TLS session
    And the HTTPS connection should not have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid" on a resumable TLS session
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" on a new connection from the same TLS session cache
    Then the response status code should be 200
    And the gateway should have run a full TLS handshake
    And the HTTPS connection should have been asked for a client certificate

  Scenario: An operation that attaches mtls-auth makes its own hostname asked and no other
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/anything","policies":[{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}]}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with no client certificate
    Then the response status code should be 401
    And the HTTPS connection should have been asked for a client certificate

  Scenario: Two mutual TLS APIs on their own hostnames are each asked and each accepts only its own authority
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    Given the certificate fixture "ca-b" is pooled as "${CTX:caB}" with usage "downstream"
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
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:otherApiName} |
      | spec.displayName       | ${CTX:otherApiName} |
      | spec.version           | ${CTX:otherApiVersion} |
      | spec.context           | ${CTX:otherContext}/$version |
      | spec.vhosts.main       | ${CTX:otherLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caB}","thumbprints":["${CTX:fixture.client-wrong-ca.thumbprint}"]}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
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
    And I set request host to "${CTX:otherLabel}.example"
    And I send a "GET" request to "${CTX:otherContext}/${CTX:otherApiVersion}/anything" until the route answers 401
    And I set request host to "${CTX:publicLabel}.example"
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:otherLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    And the HTTPS connection should have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:otherLabel}.example"
    And I set request host to "${CTX:otherLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:otherContext}/${CTX:otherApiVersion}/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    Given HTTPS requests send server name "${CTX:otherLabel}.example"
    And I set request host to "${CTX:otherLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:otherContext}/${CTX:otherApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    And the HTTPS connection should have been asked for a client certificate

  Scenario: Forged certificate headers never reach the backend of a route that does not attach mtls-auth
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.operations        | [{"method":"GET","path":"/anything","policies":[{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}]},{"method":"GET","path":"/open"}] |
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    Given I set header "X-Forwarded-Client-Cert" to "Subject=CN=forged;URI=urn:partner-a:payments"
    And I set header "X-WSO2-Client-Certificate" to "forged"
    Given HTTPS requests send server name "${CTX:publicLabel}.example"
    And I set request host to "${CTX:publicLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should not have been asked for a client certificate
    And the response should not contain echoed header "x-forwarded-client-cert"
    And the response should not contain echoed header "x-wso2-client-certificate"
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/open" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    And the response should not contain echoed header "x-forwarded-client-cert"
    And the response should not contain echoed header "x-wso2-client-certificate"
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example"
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/open" with no client certificate
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate
    And the response should not contain echoed header "x-forwarded-client-cert"
    And the response should not contain echoed header "x-wso2-client-certificate"

  # The mechanism is a kept-alive connection: it is opened on the public hostname, the new hostname is
  # added to the asking filter chain, and the same connection then carries two more requests while no response announces a close.
  Scenario: An open connection to a public API survives deploying a mutual TLS API on a new hostname
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    When I open a kept-alive HTTPS connection for "${CTX:publicLabel}.example" and send a GET request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything"
    Then the response status code should be 200
    And the kept-alive HTTPS connection should have carried 1 responses and not be closing
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:otherApiName} |
      | spec.displayName       | ${CTX:otherApiName} |
      | spec.version           | ${CTX:otherApiVersion} |
      | spec.context           | ${CTX:otherContext}/$version |
      | spec.vhosts.main       | ${CTX:otherLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I set request host to "${CTX:otherLabel}.example"
    And I send a "GET" request to "${CTX:otherContext}/${CTX:otherApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:otherLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    When I send a GET request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" on the kept-alive HTTPS connection
    Then the response status code should be 200
    And the kept-alive HTTPS connection should have carried 2 responses and not be closing
    When I send a GET request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" on the kept-alive HTTPS connection
    Then the response status code should be 200
    And the kept-alive HTTPS connection should have carried 3 responses and not be closing

  Scenario: A server name in upper case is matched like the lower case hostname
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" in upper case should be asked for a client certificate
    Given HTTPS requests send server name "${CTX:mtlsLabel}.example" in upper case
    And I set request host to "${CTX:mtlsLabel}.example"
    When I send a "GET" request over HTTPS to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the HTTPS connection should have been asked for a client certificate

  Scenario: No hostname is asked while no deployed API attaches mtls-auth
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should not be asked for a client certificate
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
    And I set request host to "${CTX:mtlsLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate
    When I delete the API "${CTX:mtlsApiName}"
    Then the response should be successful
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should not be asked for a client certificate

  Scenario: Every exact name in a list of hostnames is asked and the public hostname is not
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example;${CTX:otherLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
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
    And I set request host to "${CTX:otherLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And I set request host to "${CTX:publicLabel}.example"
    And I send a "GET" request to "${CTX:publicContext}/${CTX:publicApiVersion}/anything" until the route answers 200
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:otherLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate

  Scenario: One hostname the listener cannot match among exact names makes the listener ask every connection and warns on vhosts.main
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:mtlsLabel}.example;192.0.2.10;${CTX:otherLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings[0].code" should be "MTLS_HOSTNAME_NOT_SCOPED"
    And the JSON response field "status.warnings[0].field" should be "spec.vhosts.main"
    And the JSON response field "status.warnings[0].message" should be "this API is served on a hostname the HTTPS listener cannot match (a gateway default, an IP address, or a pattern other than an exact name or a leading *.), so the listener asks every connection for a client certificate; give it its own vhosts.main to limit that to its hostname"
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:otherLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should be asked for a client certificate
    And a TLS connection with no server name should be asked for a client certificate

  Scenario: Updating the hostname of a mutual TLS API moves what is asked to the new hostname
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
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:otherLabel}.example" should not be asked for a client certificate
    When I update API "${CTX:mtlsApiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:mtlsApiName} |
      | spec.displayName       | ${CTX:mtlsApiName} |
      | spec.version           | ${CTX:mtlsApiVersion} |
      | spec.context           | ${CTX:mtlsContext}/$version |
      | spec.vhosts.main       | ${CTX:otherLabel}.example |
      | spec.upstream.main.url | http://testbench:3002 |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:caA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the JSON response field "status.warnings" should not exist
    And I set request host to "${CTX:otherLabel}.example"
    And I send a "GET" request to "${CTX:mtlsContext}/${CTX:mtlsApiVersion}/anything" until the route answers 401
    And a TLS connection with server name "${CTX:otherLabel}.example" should be asked for a client certificate
    And a TLS connection with server name "${CTX:mtlsLabel}.example" should not be asked for a client certificate
    And a TLS connection with server name "${CTX:publicLabel}.example" should not be asked for a client certificate

  Scenario: A relay entry raises no hostname warning on an API with its own hostname
    Given the certificate fixture "ca-a" is pooled as "${CTX:caA}" with usage "downstream"
    And the certificate fixture "edge-lb-ca" is pooled as "${CTX:relayName}" as a relay
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
