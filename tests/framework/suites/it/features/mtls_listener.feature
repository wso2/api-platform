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


@mtls @mtls-listener
Feature: HTTPS listener derived from the APIs that use mutual TLS
  As an API developer
  I want attaching the mtls-auth policy to be the only thing I do to require a client certificate
  So that the gateway asks for certificates only while some API needs them, and never turns callers
  of other APIs away

  The listener has no switch of its own. While at least one deployed API attaches mtls-auth, the
  HTTPS listener requests a client certificate from every connection and validates it against the
  pool of client authorities; it never closes a connection over the certificate it received. When
  the last such API is removed the listener stops asking. Deploying an API that attaches mtls-auth
  is refused when its parameters could never authenticate anyone.

  Every scenario starts from an empty client authority pool, and every HTTPS request waits until
  the gateway has applied the pool the scenario built.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "mtls-listener" and store it as "apiName"
    And I generate a unique API version from "mtls-listener" and store it as "apiVersion"
    And I generate a unique API context from "/mtls-listener" and store it as "apiContext"
    And I generate a unique resource name from "listener-partner-a" and store it as "partnerA"
    And I generate a unique resource name from "listener-partner-b" and store it as "partnerB"
    And I generate a unique resource name from "listener-edge-lb" and store it as "edgeLb"
    And I generate a unique resource name from "listener-backend-trust" and store it as "backendTrust"

  # ==================== ASKING FOR A CERTIFICATE IS DERIVED, NOT CONFIGURED ====================

  Scenario: The listener asks for a client certificate only while an API attaches mtls-auth
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the HTTPS listener should not request a client certificate
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Listener API               |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"mtls-auth","version":"v1"}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until the route answers 200
    And the HTTPS listener should request a client certificate
    When I delete the API "${CTX:apiName}"
    Then the response should be successful
    And the HTTPS listener should stop requesting a client certificate
    And the HTTPS listener should not request a client certificate

  Scenario: Callers of other APIs are not affected while the listener asks for certificates
    Given I generate a unique value from "plain-neighbour" and store it as "neighbourName"
    And I generate a unique API version from "plain-neighbour" and store it as "neighbourVersion"
    And I generate a unique API context from "/plain-neighbour" and store it as "neighbourContext"
    And the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Listener API               |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"mtls-auth","version":"v1"}]}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}        |
      | name                   | ${CTX:neighbourName}             |
      | spec.displayName       | Plain Neighbour API              |
      | spec.version           | ${CTX:neighbourVersion}          |
      | spec.context           | ${CTX:neighbourContext}/$version |
      | spec.upstream.main.url | http://testbench:3000/api/v1     |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:neighbourContext}/${CTX:neighbourVersion}/health" until the route answers 200
    And the HTTPS listener should request a client certificate
    When I send a "GET" request over HTTPS to "${CTX:neighbourContext}/${CTX:neighbourVersion}/health" with no client certificate
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:neighbourContext}/${CTX:neighbourVersion}/health" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:neighbourContext}/${CTX:neighbourVersion}/health" with client certificate "client-wrong-ca"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:neighbourContext}/${CTX:neighbourVersion}/health" with client certificate "client-expired"
    Then the response status code should be 200
    When I send a "GET" request over HTTPS to "${CTX:neighbourContext}/${CTX:neighbourVersion}/health" with client certificate "client-selfsigned-b"
    Then the response status code should be 200
    When I delete the API "${CTX:neighbourName}"
    And I delete the API "${CTX:apiName}"

  Scenario: A protected operation denies a caller who presents no certificate with the uniform body
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Listener API               |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.operations        | [{"method":"GET","path":"/health"},{"method":"GET","path":"/protected","policies":[{"name":"mtls-auth","version":"v1"}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until the route answers 200
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/protected" with no client certificate
    Then the response status code should be 401
    And the response header "Content-Type" should contain "application/json"
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    And the response header "WWW-Authenticate" should not exist
    When I send a "GET" request over HTTPS to "${CTX:apiContext}/${CTX:apiVersion}/health" with no client certificate
    Then the response status code should be 200
    When I delete the API "${CTX:apiName}"

  # ==================== THE LISTENER'S OWN CONFIGURATION ====================

  Scenario: The listener validates against the pool without requiring a certificate and never drops a connection over it
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Listener API               |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"mtls-auth","version":"v1"}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/${CTX:apiVersion}/health" until the route answers 401
    And the gateway has applied the client authority pool
    When I send a "GET" request to the "envoy-admin" service at "/config_dump?resource=dynamic_listeners"
    Then the response status code should be 200
    And the response body should contain "downstream_client_ca"
    And the response body should match pattern "require_client_certificate\W+false"
    When I send a "GET" request to the "envoy-admin" service at "/config_dump?resource=dynamic_active_secrets"
    Then the response status code should be 200
    And the response body should contain "downstream_client_ca"
    And the response body should contain "ACCEPT_UNTRUSTED"
    When I delete the API "${CTX:apiName}"

  Scenario: The listener's private key is delivered as a secret, not inlined in the listener
    When I send a "GET" request to the "envoy-admin" service at "/config_dump?resource=dynamic_listeners"
    Then the response status code should be 200
    And the response body should contain "tls_certificate_sds_secret_configs"
    And the response body should not contain "private_key"
    And the response body should not contain "tls_certificates"
    And the HTTPS listener should present the certificate in "gateway/gateway-controller/listener-certs/default-listener.crt"

  # ==================== DEPLOYMENTS THAT COULD NEVER AUTHENTICATE ANYONE ARE REFUSED ====================

  Scenario: Attaching mtls-auth while the client authority pool is empty is refused
    Given the client authority pool is empty
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response status should be 400
    And the JSON response field "status" should be "error"
    And the response should list a validation error for field "spec.policies[0]" with message "mtls-auth requires at least one client authority; add one with POST /certificates and usage: downstream"

  Scenario Outline: An accept list that cannot select anyone is refused with the offending path
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And I upload the certificate fixture "ca-b" as "${CTX:edgeLb}" with usage "downstream" and role "relay"
    And the response status should be 201
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendTrust}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":<params>}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response status should be 400
    And the response should list a validation error for field "<field>" with message "<message>"

    Examples:
      | params                                                                          | field                                              | message                                                                                                                     |
      | {"accept":[]}                                                                   | spec.policies[0].params.accept                     | omit accept to inherit every pooled authority, or list at least one entry                                                   |
      | {"accept":[{"match":{"uriSANs":["urn:x"]}}]}                                    | spec.policies[0].params.accept[0].ca               | ca is required and must name an authority in this gateway's client-CA pool                                                  |
      | {"accept":[{"ca":"${CTX:partnerB}"}]}                                           | spec.policies[0].params.accept[0].ca               | no client-CA authority named ${CTX:partnerB} exists on this gateway                                                         |
      | {"accept":[{"ca":"${CTX:edgeLb}"}]}                                             | spec.policies[0].params.accept[0].ca               | ${CTX:edgeLb} is a relay (front proxy) entry and cannot be accepted as a client                                             |
      | {"accept":[{"ca":"${CTX:backendTrust}"}]}                                       | spec.policies[0].params.accept[0].ca               | ${CTX:backendTrust} is a backend trust certificate (usage: upstream); accept takes usage: downstream authorities            |
      | {"accept":[{"ca":"${CTX:partnerA}","match":{"uriSANs":[]}}]}                    | spec.policies[0].params.accept[0].match.uriSANs    | list at least one non-empty SAN, or remove match to accept any certificate from this authority                              |
      | {"accept":[{"ca":"${CTX:partnerA}","match":{"dnsSANs":["a",""]}}]}              | spec.policies[0].params.accept[0].match.dnsSANs[1] | list at least one non-empty SAN, or remove match to accept any certificate from this authority                              |
      | {"accept":[{"ca":"${CTX:partnerA}","thumbprints":[]}]}                          | spec.policies[0].params.accept[0].thumbprints      | list at least one thumbprint, or remove thumbprints to accept any certificate from this authority                           |
      | {"accept":[{"ca":"${CTX:partnerA}","thumbprints":["zz"]}]}                      | spec.policies[0].params.accept[0].thumbprints[0]   | a thumbprint is the SHA-256 of the certificate as 64 hex characters (colons and a sha256: prefix are accepted)             |
      | {"accept":[{"ca":"${CTX:partnerA}","thumbprint":"9f86d081"}]}                   | spec.policies[0].params.accept[0].thumbprint       | unknown parameter thumbprint; the field is thumbprints                                                                      |
      | {"mode":"strict"}                                                               | spec.policies[0].params.mode                       | unknown parameter mode                                                                                                      |
      | {"forwardCertificate":"no"}                                                     | spec.policies[0].params.forwardCertificate         | forwardCertificate must be true or false                                                                                    |
      | {"accept":{"ca":"${CTX:partnerA}"}}                                             | spec.policies[0].params.accept                     | accept must be a list of entries; omit it to inherit every pooled authority                                                 |
      | {"accept":["${CTX:partnerA}"]}                                                  | spec.policies[0].params.accept[0]                  | each accept entry must be an object naming ca                                                                               |
      | {"accept":[{"ca":"${CTX:partnerA}","thumbprints":"9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"}]} | spec.policies[0].params.accept[0].thumbprints | thumbprints must be a list                                                               |
      | {"accept":[{"ca":"${CTX:partnerA}","match":{"uriSANs":"urn:x"}}]}               | spec.policies[0].params.accept[0].match.uriSANs    | uriSANs must be a list                                                                                                      |
      | {"accept":[{"ca":"${CTX:partnerA}","match":{}}]}                                | spec.policies[0].params.accept[0].match            | match must list uriSANs or dnsSANs; remove it to accept any certificate from this authority                                 |
      | {"accept":[{"ca":"${CTX:partnerA}","match":"urn:x"}]}                           | spec.policies[0].params.accept[0].match            | match must be an object listing uriSANs or dnsSANs                                                                          |

  Scenario: mtls-auth cannot be made conditional
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3002           |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","executionCondition":"request.Method == \"POST\"","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.policies[0].executionCondition" with message "mtls-auth runs on every request and cannot carry an executionCondition"

  Scenario: Attaching mtls-auth while the pool holds only relay entries is refused
    Given I upload the certificate fixture "edge-lb-ca" as "${CTX:edgeLb}" with usage "downstream" and role "relay"
    And the response status should be 201
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3002           |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.policies[0]" containing "requires at least one client authority"

  Scenario: mtls-auth may appear once per scope
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"},{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.policies[1]" with message "mtls-auth may appear once per scope; use several accept entries instead"

  Scenario: mtls-auth attached at both API and operation level is refused
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/health","policies":[{"name":"mtls-auth","version":"v1"}]}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.operations[0].policies[0]" with message "mtls-auth is already attached at API level; attach it at one level only"

  Scenario: A refused deployment leaves the listener alone
    Given the client authority pool is empty
    And the HTTPS listener should not request a client certificate
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Refused API                |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response status should be 400
    And the HTTPS listener should not request a client certificate

  # ==================== WARNINGS ON THE DEPLOY RESPONSE ====================

  Scenario: Omitting accept while the pool holds several authorities warns and echoes the resolved list
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Warned API                 |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And the response should include a warning with code "MTLS_ACCEPT_INHERITS_POOL" for field "spec.policies[0].params.accept"
    And the JSON response array field "spec.policies[0].params.accept" should have 2 items
    When I delete the API "${CTX:apiName}"

  Scenario: An entry with neither match nor thumbprints warns
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "${CTX:partnerB}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Warned API                 |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And the response should include a warning with code "MTLS_ACCEPT_UNNARROWED" for field "spec.policies[0].params.accept[0]"
    When I delete the API "${CTX:apiName}"

  Scenario: An explicit entry is warned as unnarrowed even when the pool holds one authority
    Given the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Warned API                 |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}"}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And the response should include a warning with code "MTLS_ACCEPT_UNNARROWED" for field "spec.policies[0].params.accept[0]"
    When I delete the API "${CTX:apiName}"

  Scenario: A single-authority pool produces no inheritance warning
    Given the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    And I generate a unique resource name from "listener-host" and store it as "listenerLabel"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Warned API                 |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.vhosts.main       | ${CTX:listenerLabel}.example    |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1"}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And the response should include no warnings
    When I delete the API "${CTX:apiName}"

  Scenario: Another authentication policy ahead of mtls-auth in the chain warns
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Warned API                 |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"jwt-auth","version":"v1","params":{"issuers":["mock-jwks"]}},{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","match":{"uriSANs":["urn:partner-a:payments"]}}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And the response should include a warning with code "MTLS_AUTH_NOT_FIRST" for field "spec.policies[0]"
    When I delete the API "${CTX:apiName}"

  Scenario: A thumbprint written with colons or a prefix is accepted and normalised with a warning
    Given the certificate fixture "ca-a" is pooled as "${CTX:partnerA}" with usage "downstream"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion}       |
      | name                   | ${CTX:apiName}                  |
      | spec.displayName       | mTLS Warned API                 |
      | spec.version           | ${CTX:apiVersion}               |
      | spec.context           | ${CTX:apiContext}/$version      |
      | spec.upstream.main.url | http://testbench:3000/api/v1    |
      | spec.policies          | [{"name":"mtls-auth","version":"v1","params":{"accept":[{"ca":"${CTX:partnerA}","thumbprints":["sha256:9F:86:D0:81:88:4C:7D:65:9A:2F:EA:A0:C5:5A:D0:15:A3:BF:4F:1B:2B:0B:82:2C:D1:5D:6C:15:B0:F0:0A:08"]}]}}] |
      | spec.operations        | [{"method":"GET","path":"/health"}] |
    Then the response should be successful
    And the response should include a warning with code "MTLS_THUMBPRINT_NORMALISED" for field "spec.policies[0].params.accept[0].thumbprints[0]"
    And the JSON response field "spec.policies[0].params.accept[0].thumbprints[0]" should be "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
    When I delete the API "${CTX:apiName}"
