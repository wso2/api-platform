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

@mtls @mtls-outbound
Feature: Presenting a gateway identity to backends that require a client certificate
  As a gateway administrator and an API developer
  I want the gateway to identify itself to a backend with a certificate of my choosing, and to
  trust exactly the authority that backend uses
  So that partner services requiring mutual TLS can sit behind the gateway

  A gateway identity (certificate chain plus private key) is uploaded once through the certificates
  endpoint with usage "identity"; the key is encrypted at rest and never returned. An upstream definition names the identity to present and, optionally,
  the authorities to trust for that backend in place of the gateway-wide bundle. The test stack runs
  three TLS backends on the host tls-backend: port 8443 accepts client certificates from partner A's
  authority, port 8444 from partner B's, and port 8445 serves a certificate whose name does not
  match its host. Each backend reports the client subject it saw in the X-Client-Subject header
  and in the client field of its body.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And I generate a unique value from "out-api" and store it as "apiName"
    And I generate a unique API context from "/out-partner" and store it as "apiContext"
    And I generate a unique value from "out-second-api" and store it as "secondApiName"
    And I generate a unique API context from "/out-second" and store it as "secondApiContext"
    And I generate a unique value from "out-agent" and store it as "agentName"
    And I generate a unique API context from "/out-agent" and store it as "agentContext"
    And I generate a unique resource name from "out-identity-a" and store it as "identityA"
    And I generate a unique resource name from "out-identity-b" and store it as "identityB"
    And I generate a unique resource name from "out-backend-ca" and store it as "backendCA"
    And I generate a unique resource name from "out-backend-ca-b" and store it as "backendCAB"
    And I generate a unique resource name from "out-client-authority" and store it as "clientAuthority"
    And I generate a unique resource name from "out-bad" and store it as "badName"

  # ==================== GATEWAY IDENTITIES ====================

  Scenario: An identity is uploaded with its key and the key is never returned
    When I upload the gateway identity fixture "gw-identity-a" as "${CTX:identityA}"
    Then the response status should be 201
    And the JSON response field "status" should be "success"
    And the JSON response field "name" should be "${CTX:identityA}"
    And the JSON response field "usage" should be "identity"
    And the JSON response should have field "id"
    And the JSON response should have field "subject"
    And the JSON response should have field "issuer"
    And the JSON response should have field "notAfter"
    And the JSON response should have field "keyAlgorithm"
    And the JSON response field "privateKey" should not exist
    And the response body should not contain "PRIVATE KEY"
    And the JSON response field "warnings" should not exist
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=identity"
    Then the response status should be 200
    And the response body should contain "${CTX:identityA}"
    And the response body should not contain "PRIVATE KEY"
    And the response body should not contain "privateKey"
    Given I authenticate using basic auth as "developer"
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=identity"
    Then the response status should be 200
    And the response body should contain "${CTX:identityA}"
    And the response body should not contain "privateKey"

  Scenario: An identity may include its intermediate chain
    When I upload the gateway identity fixture "gw-identity-via-intermediate" with its chain as "${CTX:identityA}"
    Then the response status should be 201
    And the JSON response field "chainLength" should be 2

  Scenario: An identity whose certificate lacks the clientAuth usage is accepted with a warning
    When I upload the gateway identity fixture "gw-identity-serverauth" as "${CTX:identityA}"
    Then the response status should be 201
    And the response should include a warning with code "IDENTITY_NO_CLIENTAUTH_EKU" for field "certificate"
    When I upload the gateway identity fixture "gw-identity-no-eku" as "${CTX:identityB}"
    Then the response status should be 201
    And the JSON response field "warnings" should not exist

  Scenario Outline: Uploads that could never work are refused
    When I upload to the certificates endpoint the identity body:
      """
      <body>
      """
    Then the response status should be 400
    And the JSON response field "status" should be "error"
    And the response should list a validation error for field "<field>" with message "<message>"

    Examples:
      | body | field | message |
      | {"name":"${CTX:badName}","usage":"identity","certificate":"${CTX:fixture.gw-identity-a.pem}","privateKey":"${CTX:fixture.key-mismatch.key}"} | privateKey | the private key does not match the certificate |
      | {"name":"${CTX:badName}","usage":"identity","certificate":"${CTX:fixture.gw-identity-a.pem}","privateKey":"${CTX:fixture.gw-identity-a.encryptedKey}"} | privateKey | passphrase-protected private keys are not supported; upload an unencrypted key (it is encrypted at rest by the gateway) |
      | {"name":"${CTX:badName}","usage":"identity","certificate":"${CTX:fixture.gw-identity-a.pem}"} | privateKey | both certificate and privateKey are required for usage: identity |
      | {"name":"${CTX:badName}","usage":"downstream","certificate":"${CTX:fixture.ca-a.pem}","privateKey":"${CTX:fixture.gw-identity-a.key}"} | privateKey | privateKey applies only to usage: identity certificates |
      | {"name":"${CTX:badName}","usage":"identity","privateKey":"${CTX:fixture.gw-identity-a.key}"} | certificate | both certificate and privateKey are required for usage: identity |
      | {"name":"${CTX:badName}","usage":"client","certificate":"${CTX:fixture.ca-a.pem}"} | usage | usage must be upstream, downstream or identity |
      | {"name":"out bad","usage":"identity","certificate":"${CTX:fixture.gw-identity-a.pem}","privateKey":"${CTX:fixture.gw-identity-a.key}"} | name | name may contain only letters, digits, ., _ and - |
      | {"name":"${CTX:badName}","usage":"identity","certificate":"not a certificate","privateKey":"${CTX:fixture.gw-identity-a.key}"} | certificate | the value is not a PEM-encoded certificate |
      | {"name":"${CTX:badName}","usage":"identity","certificate":"${CTX:fixture.gw-identity-a.pem}\\n${CTX:fixture.gw-identity-a.key}","privateKey":"${CTX:fixture.gw-identity-a.key}"} | certificate | the certificate field takes certificates only; the private key belongs in privateKey |

  Scenario: An expired identity certificate is refused
    When I upload to the certificates endpoint the identity body:
      """
      {"name":"${CTX:badName}","usage":"identity","certificate":"${CTX:fixture.client-expired.pem}","privateKey":"${CTX:fixture.client-expired.key}"}
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" containing "the certificate expired on"

  Scenario: A duplicate identity name is a conflict
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    When I upload the gateway identity fixture "gw-identity-b" as "${CTX:identityA}"
    Then the response status should be 409
    And the JSON response field "message" should be "a certificate named ${CTX:identityA} already exists"

  Scenario: A developer cannot create or remove identities
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And I authenticate using basic auth as "developer"
    When I upload the gateway identity fixture "gw-identity-b" as "${CTX:identityB}"
    Then the response status should be 403
    When I delete the gateway identity named "${CTX:identityA}"
    Then the response status should be 403

  Scenario: An identity that no upstream references can be removed
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    When I delete the gateway identity named "${CTX:identityA}"
    Then the response should be successful
    When I send a "GET" request to the "gateway-controller" service at "/certificates?usage=identity"
    Then the response body should not contain "${CTX:identityA}"

  # ==================== THE TLS BLOCK AT DEPLOY TIME ====================

  Scenario Outline: A tls block that could never work is refused with the offending path
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Refused API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner","upstreams":[{"url":"<url>"}],"tls":<tls>}] |
      | spec.upstream.main.ref   | partner |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response status should be 400
    And the response should list a validation error for field "<field>" with message "<message>"

    Examples:
      | url | tls | field | message |
      | https://tls-backend:8443 | {"identity":"out-missing"} | spec.upstreamDefinitions[0].tls.identity | no gateway identity named out-missing exists on this gateway |
      | https://tls-backend:8443 | {"trustedCAs":["out-missing-ca"]} | spec.upstreamDefinitions[0].tls.trustedCAs[0] | no certificate named out-missing-ca exists on this gateway |
      | https://tls-backend:8443 | {"trustedCAs":[]} | spec.upstreamDefinitions[0].tls.trustedCAs | omit trustedCAs to use the gateway trust bundle, or list at least one certificate |
      | http://testbench:3002 | {"identity":"${CTX:identityA}"} | spec.upstreamDefinitions[0].upstreams[0].url | tls is configured but this target is http://; every target of a definition with tls must be https:// |
      | https://tls-backend:8443 | {"mode":"strict"} | spec.upstreamDefinitions[0].tls.mode | unknown parameter mode |

  Scenario: A tls block on an inline upstream is refused
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion             | ${CTX:gatewaySpecVersion} |
      | name                   | ${CTX:apiName} |
      | spec.displayName       | Outbound Refused API |
      | spec.version           | v1.0 |
      | spec.context           | ${CTX:apiContext}/$version |
      | spec.upstream.main.url | https://tls-backend:8443 |
      | spec.upstream.main.tls | {"identity":"${CTX:identityA}"} |
      | spec.operations        | [{"method":"GET","path":"/anything"}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstream.main.tls" with message "tls is not supported on an inline upstream; move it to upstreamDefinitions and reference it"

  Scenario: A trust certificate meant for clients cannot be used as backend trust
    Given the certificate fixture "ca-a" is pooled as "${CTX:clientAuthority}" with usage "downstream"
    And the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Refused API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:clientAuthority}"]}}] |
      | spec.upstream.main.ref   | partner |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" with message "${CTX:clientAuthority} is a client authority (usage: downstream); trustedCAs takes usage: upstream certificates"

  Scenario: Only an identity can be presented, and an identity is not backend trust
    Given the certificate fixture "ca-a" is pooled as "${CTX:clientAuthority}" with usage "downstream"
    And the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Refused API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:clientAuthority}","trustedCAs":["${CTX:identityA}"]}}] |
      | spec.upstream.main.ref   | partner |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" with message "${CTX:clientAuthority} is not a gateway identity (usage: identity)"
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" with message "${CTX:identityA} is a gateway identity (usage: identity); trustedCAs takes usage: upstream certificates"

  Scenario: Disabling hostname verification deploys with a warning
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Warned API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner","upstreams":[{"url":"https://tls-backend:8445"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"],"verifyHostName":false}}] |
      | spec.upstream.main.ref   | partner |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And the response should include a warning with code "TLS_VERIFY_HOSTNAME_DISABLED" for field "spec.upstreamDefinitions[0].tls.verifyHostName"

  # ==================== PRESENTING THE IDENTITY ====================

  Scenario: The gateway presents the named identity and the backend accepts it
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the route answers 200
    And the gateway has applied its configuration
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should be "CN=gateway-a"
    And the JSON response field "backend" should be "a"
    And the JSON response field "client" should be "CN=gateway-a"

  Scenario: Without an identity a backend that requires one refuses the request and sees no client subject
    Given the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the backend refuses the request with 400
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything"
    Then the response status code should be 400
    And the response header "X-Client-Subject" should not exist
    And the JSON response field "backend" should be "a"
    And the JSON response field "client" should not exist

  Scenario: Two definitions with two identities keep each backend seeing only its own
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the gateway identity fixture "gw-identity-b" is stored as "${CTX:identityB}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    And the certificate fixture "backend-ca-b" is pooled as "${CTX:backendCAB}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Two Partners API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"]}},{"name":"partner-b","upstreams":[{"url":"https://tls-backend:8444"}],"tls":{"identity":"${CTX:identityB}","trustedCAs":["${CTX:backendCAB}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"},{"method":"GET","path":"/anything/b","policies":[{"name":"dynamic-endpoint","version":"v1","params":{"targetUpstream":"partner-b"}}]}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the route answers 200
    And I wait for policy snapshot sync
    And the gateway has applied its configuration
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should be "CN=gateway-a"
    And the JSON response field "backend" should be "a"
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything/b"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should be "CN=gateway-b"
    And the JSON response field "backend" should be "b"

  Scenario: Two APIs to the same backend each present their own identity
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:secondApiName} |
      | spec.displayName         | Outbound Anonymous API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:secondApiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the route answers 200
    And I send a "GET" request to "${CTX:secondApiContext}/v1.0/anything" until the backend refuses the request with 400
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should be "CN=gateway-a"
    And the JSON response field "client" should be "CN=gateway-a"
    When I send a "GET" request to "${CTX:secondApiContext}/v1.0/anything"
    Then the response status code should be 400
    And the response header "X-Client-Subject" should not exist
    And the JSON response field "client" should not exist

  Scenario: Per-upstream trust replaces the gateway bundle for that upstream only
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca-b" is pooled as "${CTX:backendCAB}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCAB}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the upstream TLS connection fails with 503
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything"
    Then the response status code should be 503
    And the response body should contain "upstream connect error"
    And the "gateway-runtime" log since the latest request should contain "\"upTlsFail\":\"TLS_error:|268435581:SSL_routines:OPENSSL_internal:CERTIFICATE_VERIFY_FAILED"

  Scenario: Hostname verification is on by default and rejects a certificate for another host
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8445"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the upstream TLS connection fails with 503
    When I update API "${CTX:apiName}" from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8445"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"],"verifyHostName":false}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    And I send a "GET" request to "${CTX:apiContext}/v1.0/anything" until the route answers 200
    And the gateway has applied its configuration
    When I send a "GET" request to "${CTX:apiContext}/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should be "CN=gateway-a"
    And the JSON response field "backend" should be "wronghost"

  Scenario: An identity is rotated in place, and only identities can be updated
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I update the gateway identity "${CTX:identityA}" with the fixture "gw-identity-via-intermediate" and its chain
    Then the response status should be 200
    And the JSON response field "chainLength" should be 2
    And the response body should not contain "PRIVATE KEY"
    When I update the certificate "${CTX:backendCA}" with the identity fixture "gw-identity-a"
    Then the response status should be 400
    And the response should list a validation error for field "usage" with message "only usage: identity certificates can be updated; delete and re-upload other certificates"

  # ==================== REFERENCES ====================

  Scenario: An identity or trust certificate named by a deployed upstream cannot be removed
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create API from "resources/templates/rest-api.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:apiName} |
      | spec.displayName         | Outbound Partner API |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:apiContext}/$version |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.main.ref   | partner-a |
      | spec.operations          | [{"method":"GET","path":"/anything"}] |
    Then the response should be successful
    When I delete the gateway identity named "${CTX:identityA}"
    Then the response status should be 409
    And the JSON response field "message" should contain "gateway identity '${CTX:identityA}' is named by 1 deployed API"
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" containing "${CTX:apiName}"
    When I delete the certificate named "${CTX:backendCA}"
    Then the response status should be 409
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" containing "${CTX:apiName}"

  Scenario: An Agent whose tls block names a missing identity is refused with the offending path
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:agentName} |
      | spec.displayName         | Outbound Refused Agent |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:agentContext} |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"out-missing"}}] |
      | spec.upstream.ref        | partner-a |
      | spec.a2a                 | {"protocolVersion":"1.0","operationConfigs":{"transports":[{"protocolBinding":"JSONRPC","pathPrefix":"/"}]},"agentCard":{"public":{"mode":"passthrough"}}} |
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" with message "no gateway identity named out-missing exists on this gateway"

  Scenario: An identity or trust certificate named by a deployed Agent cannot be removed
    Given the gateway identity fixture "gw-identity-a" is stored as "${CTX:identityA}"
    And the certificate fixture "backend-ca" is pooled as "${CTX:backendCA}"
    When I create Agent from "resources/templates/agent.yaml" with values:
      | apiVersion               | ${CTX:gatewaySpecVersion} |
      | name                     | ${CTX:agentName} |
      | spec.displayName         | Outbound Partner Agent |
      | spec.version             | v1.0 |
      | spec.context             | ${CTX:agentContext} |
      | spec.upstreamDefinitions | [{"name":"partner-a","upstreams":[{"url":"https://tls-backend:8443"}],"tls":{"identity":"${CTX:identityA}","trustedCAs":["${CTX:backendCA}"]}}] |
      | spec.upstream.ref        | partner-a |
      | spec.a2a                 | {"protocolVersion":"1.0","operationConfigs":{"transports":[{"protocolBinding":"JSONRPC","pathPrefix":"/"}]},"agentCard":{"public":{"mode":"passthrough"}}} |
    Then the response should be successful
    When I delete the gateway identity named "${CTX:identityA}"
    Then the response status should be 409
    And the JSON response field "message" should contain "gateway identity '${CTX:identityA}' is named by 1 deployed API"
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" containing "${CTX:agentName}"
    When I delete the certificate named "${CTX:backendCA}"
    Then the response status should be 409
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" containing "${CTX:agentName}"
