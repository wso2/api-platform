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
  three TLS backends: mtls-backend-a accepts client certificates from partner A's authority,
  mtls-backend-b from partner B's, and mtls-backend-wronghost serves a certificate whose name does not
  match its host. Each backend reports the client subject it saw in the X-Client-Subject header.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== GATEWAY IDENTITIES ====================

  Scenario: An identity is uploaded with its key and the key is never returned
    When I upload the gateway identity fixture "gw-identity-a" as "out-identity-a"
    Then the response status should be 201
    And the JSON response field "status" should be "success"
    And the JSON response field "name" should be "out-identity-a"
    And the JSON response field "usage" should be "identity"
    And the JSON response should have field "id"
    And the JSON response should have field "subject"
    And the JSON response should have field "issuer"
    And the JSON response should have field "notAfter"
    And the JSON response should have field "keyAlgorithm"
    And the JSON response field "privateKey" should not exist
    And the response body should not contain "PRIVATE KEY"
    And the JSON response field "warnings" should not exist
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=identity"
    Then the response status should be 200
    And the response body should contain "out-identity-a"
    And the response body should not contain "PRIVATE KEY"
    And the response body should not contain "privateKey"
    Given I authenticate using basic auth as "developer"
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=identity"
    Then the response status should be 200
    And the response body should contain "out-identity-a"
    And the response body should not contain "privateKey"

  Scenario: An identity may include its intermediate chain
    When I upload the gateway identity fixture "gw-identity-via-intermediate" with its chain as "out-identity-chain"
    Then the response status should be 201
    And the JSON response field "chainLength" should be 2

  Scenario: An identity whose certificate lacks the clientAuth usage is accepted with a warning
    When I upload the gateway identity fixture "gw-identity-serverauth" as "out-identity-serverauth"
    Then the response status should be 201
    And the response should include a warning with code "IDENTITY_NO_CLIENTAUTH_EKU" for field "certificate"
    When I upload the gateway identity fixture "gw-identity-no-eku" as "out-identity-no-eku"
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
      | body                                                                                                                                 | field       | message                                                                                                                     |
      | {"name":"out-bad","usage":"identity","certificate":"{{pem "gw-identity-a"}}","privateKey":"{{key "key-mismatch"}}"}                                    | privateKey  | the private key does not match the certificate                                                                              |
      | {"name":"out-bad","usage":"identity","certificate":"{{pem "gw-identity-a"}}","privateKey":"{{encryptedkey "gw-identity-a"}}"}                          | privateKey  | passphrase-protected private keys are not supported; upload an unencrypted key (it is encrypted at rest by the gateway)     |
      | {"name":"out-bad","usage":"identity","certificate":"{{pem "gw-identity-a"}}"}                                                                           | privateKey  | both certificate and privateKey are required for usage: identity                                                            |
      | {"name":"out-bad","usage":"downstream","certificate":"{{pem "ca-a"}}","privateKey":"{{key "gw-identity-a"}}"}                                             | privateKey  | privateKey applies only to usage: identity certificates                                                                     |
      | {"name":"out-bad","usage":"identity","privateKey":"{{key "gw-identity-a"}}"}                                                                            | certificate | both certificate and privateKey are required for usage: identity                                                            |
      | {"name":"out-bad","usage":"client","certificate":"{{pem "ca-a"}}"}                                                                                      | usage       | usage must be upstream, downstream or identity                                                                              |
      | {"name":"out bad","usage":"identity","certificate":"{{pem "gw-identity-a"}}","privateKey":"{{key "gw-identity-a"}}"}                                    | name        | name may contain only letters, digits, ., _ and -                                                                           |
      | {"name":"out-bad","usage":"identity","certificate":"not a certificate","privateKey":"{{key "gw-identity-a"}}"}                                          | certificate | the value is not a PEM-encoded certificate                                                                                  |
      | {"name":"out-bad","usage":"identity","certificate":"{{pem "gw-identity-a"}}\\n{{key "gw-identity-a"}}","privateKey":"{{key "gw-identity-a"}}"} | certificate | the certificate field takes certificates only; the private key belongs in privateKey |

  Scenario: An expired identity certificate is refused
    When I upload to the certificates endpoint the identity body:
      """
      {"name":"out-expired","usage":"identity","certificate":"{{pem "client-expired"}}","privateKey":"{{key "client-expired"}}"}
      """
    Then the response status should be 400
    And the response should list a validation error for field "certificate" containing "the certificate expired on"

  Scenario: A duplicate identity name is a conflict
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    When I upload the gateway identity fixture "gw-identity-b" as "out-identity-a"
    Then the response status should be 409
    And the JSON response field "message" should be "a gateway identity named out-identity-a already exists"

  Scenario: A developer cannot create or remove identities
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And I authenticate using basic auth as "developer"
    When I upload the gateway identity fixture "gw-identity-b" as "out-identity-dev"
    Then the response status should be 403
    When I delete the gateway identity named "out-identity-a"
    Then the response status should be 403

  Scenario: An identity that no upstream references can be removed
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    When I delete the gateway identity named "out-identity-a"
    Then the response should be successful
    When I send a GET request to the "gateway-controller" service at "/certificates?usage=identity"
    Then the response body should not contain "out-identity-a"

  # ==================== THE TLS BLOCK AT DEPLOY TIME ====================

  Scenario Outline: A tls block that could never work is refused with the offending path
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-refused-api
      spec:
        displayName: Outbound Refused API
        version: v1.0
        context: /out-refused/$version
        upstreamDefinitions:
          - name: partner
            upstreams:
              - url: <url>
            tls:
              <tls>
        upstream:
          main:
            ref: partner
        operations:
          - method: GET
            path: /anything
      """
    Then the response status should be 400
    And the response should list a validation error for field "<field>" with message "<message>"

    Examples:
      | url                        | tls                                              | field                                          | message                                                                                                   |
      | https://mtls-backend-a:8443 | identity: out-missing                            | spec.upstreamDefinitions[0].tls.identity        | no gateway identity named out-missing exists on this gateway                                              |
      | https://mtls-backend-a:8443 | trustedCAs: [out-missing-ca]                     | spec.upstreamDefinitions[0].tls.trustedCAs[0]   | no certificate named out-missing-ca exists on this gateway                                                |
      | https://mtls-backend-a:8443 | trustedCAs: []                                   | spec.upstreamDefinitions[0].tls.trustedCAs      | omit trustedCAs to use the gateway trust bundle, or list at least one certificate                         |
      | http://echo-backend:80      | identity: out-identity-a                         | spec.upstreamDefinitions[0].upstreams[0].url    | tls is configured but this target is http://; every target of a definition with tls must be https://    |
      | https://mtls-backend-a:8443 | mode: strict                                     | spec.upstreamDefinitions[0].tls.mode            | unknown parameter mode                                                                                    |

  Scenario: A tls block on an inline upstream is refused
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-refused-api
      spec:
        displayName: Outbound Refused API
        version: v1.0
        context: /out-refused/$version
        upstream:
          main:
            url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstream.main.tls" with message "tls is not supported on an inline upstream; move it to upstreamDefinitions and reference it"

  Scenario: A trust certificate meant for clients cannot be used as backend trust
    Given the certificate fixture "ca-a" is pooled as "out-client-authority" with usage "downstream"
    And the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-refused-api
      spec:
        displayName: Outbound Refused API
        version: v1.0
        context: /out-refused/$version
        upstreamDefinitions:
          - name: partner
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-client-authority]
        upstream:
          main:
            ref: partner
        operations:
          - method: GET
            path: /anything
      """
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" with message "out-client-authority is a client authority (usage: downstream); trustedCAs takes usage: upstream certificates"

  Scenario: Only an identity can be presented, and an identity is not backend trust
    Given the certificate fixture "ca-a" is pooled as "out-client-authority" with usage "downstream"
    And the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-refused-api
      spec:
        displayName: Outbound Refused API
        version: v1.0
        context: /out-refused/$version
        upstreamDefinitions:
          - name: partner
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-client-authority
              trustedCAs: [out-identity-a]
        upstream:
          main:
            ref: partner
        operations:
          - method: GET
            path: /anything
      """
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" with message "out-client-authority is not a gateway identity (usage: identity)"
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" with message "out-identity-a is a gateway identity (usage: identity); trustedCAs takes usage: upstream certificates"

  Scenario: Disabling hostname verification deploys with a warning
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-warned-api
      spec:
        displayName: Outbound Warned API
        version: v1.0
        context: /out-warned/$version
        upstreamDefinitions:
          - name: partner
            upstreams:
              - url: https://mtls-backend-wronghost:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
              verifyHostName: false
        upstream:
          main:
            ref: partner
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And the response should include a warning with code "TLS_VERIFY_HOSTNAME_DISABLED" for field "spec.upstreamDefinitions[0].tls.verifyHostName"

  # ==================== PRESENTING THE IDENTITY ====================

  Scenario: The gateway presents the named identity and the backend accepts it
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-partner/v1.0/anything" to be ready
    When I send a GET request to "http://localhost:8080/out-partner/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should contain "gateway-a"

  Scenario: Without an identity a backend that requires one refuses the request and sees no client subject
    Given the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              trustedCAs: [out-backend-ca]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-partner/v1.0/anything" to respond with status 400
    When I send a GET request to "http://localhost:8080/out-partner/v1.0/anything"
    Then the response status code should be 400
    And the response header "X-Client-Subject" should not exist

  Scenario: Two definitions with two identities keep each backend seeing only its own
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the gateway identity fixture "gw-identity-b" is stored as "out-identity-b"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    And the certificate fixture "backend-ca-b" is pooled as "out-backend-ca-b"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-two-partners-api
      spec:
        displayName: Outbound Two Partners API
        version: v1.0
        context: /out-two/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
          - name: partner-b
            upstreams:
              - url: https://mtls-backend-b:8443
            tls:
              identity: out-identity-b
              trustedCAs: [out-backend-ca-b]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
          - method: GET
            path: /anything/b
            policies:
              - name: dynamic-endpoint
                version: v1
                params:
                  targetUpstream: partner-b
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-two/v1.0/anything" to be ready
    When I send a GET request to "http://localhost:8080/out-two/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should contain "gateway-a"
    When I send a GET request to "http://localhost:8080/out-two/v1.0/anything/b"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should contain "gateway-b"

  Scenario: Two APIs to the same backend each present their own identity
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-anonymous-api
      spec:
        displayName: Outbound Anonymous API
        version: v1.0
        context: /out-anonymous/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              trustedCAs: [out-backend-ca]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-partner/v1.0/anything" to be ready
    And I wait for the endpoint "http://localhost:8080/out-anonymous/v1.0/anything" to respond with status 400
    When I send a GET request to "http://localhost:8080/out-partner/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should contain "gateway-a"
    When I send a GET request to "http://localhost:8080/out-anonymous/v1.0/anything"
    Then the response status code should be 400
    And the response header "X-Client-Subject" should not exist

  Scenario: Per-upstream trust replaces the gateway bundle for that upstream only
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca-b" is pooled as "out-backend-ca-b"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca-b]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-partner/v1.0/anything" to respond with status 503
    When I send a GET request to "http://localhost:8080/out-partner/v1.0/anything"
    Then the response status code should be 503
    And the response body should contain "upstream connect error"

  Scenario: Hostname verification is on by default and rejects a certificate for another host
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-wronghost:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-partner/v1.0/anything" to respond with status 503
    When I update the API "out-partner-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-wronghost:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
              verifyHostName: false
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/out-partner/v1.0/anything" to be ready
    When I send a GET request to "http://localhost:8080/out-partner/v1.0/anything"
    Then the response status code should be 200
    And the response header "X-Client-Subject" should contain "gateway-a"

  Scenario: An identity is rotated in place, and only identities can be updated
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I update the gateway identity "out-identity-a" with the fixture "gw-identity-via-intermediate" and its chain
    Then the response status should be 200
    And the JSON response field "chainLength" should be 2
    And the response body should not contain "PRIVATE KEY"
    When I update the certificate "out-backend-ca" with the identity fixture "gw-identity-a"
    Then the response status should be 400
    And the response should list a validation error for field "usage" with message "only usage: identity certificates can be updated; delete and re-upload other certificates"

  # ==================== REFERENCES ====================

  Scenario: An identity or trust certificate named by a deployed upstream cannot be removed
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: out-partner-api
      spec:
        displayName: Outbound Partner API
        version: v1.0
        context: /out-partner/$version
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
        upstream:
          main:
            ref: partner-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    When I delete the gateway identity named "out-identity-a"
    Then the response status should be 409
    And the JSON response field "message" should contain "gateway identity 'out-identity-a' is named by 1 deployed API"
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" containing "out-partner-api"
    When I delete the certificate named "out-backend-ca"
    Then the response status should be 409
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" containing "out-partner-api"

  Scenario: An Agent whose tls block names a missing identity is refused with the offending path
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: out-refused-agent
      spec:
        displayName: Outbound Refused Agent
        version: v1.0
        context: /out-refused-agent
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-missing
        upstream:
          ref: partner-a
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: JSONRPC
                pathPrefix: /
          agentCard:
            public:
              mode: passthrough
      """
    Then the response status should be 400
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" with message "no gateway identity named out-missing exists on this gateway"

  Scenario: An identity or trust certificate named by a deployed Agent cannot be removed
    Given the gateway identity fixture "gw-identity-a" is stored as "out-identity-a"
    And the certificate fixture "backend-ca" is pooled as "out-backend-ca"
    When I deploy this Agent configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: Agent
      metadata:
        name: out-partner-agent
      spec:
        displayName: Outbound Partner Agent
        version: v1.0
        context: /out-partner-agent
        upstreamDefinitions:
          - name: partner-a
            upstreams:
              - url: https://mtls-backend-a:8443
            tls:
              identity: out-identity-a
              trustedCAs: [out-backend-ca]
        upstream:
          ref: partner-a
        a2a:
          protocolVersion: "1.0"
          operationConfigs:
            transports:
              - protocolBinding: JSONRPC
                pathPrefix: /
          agentCard:
            public:
              mode: passthrough
      """
    Then the response should be successful
    When I delete the gateway identity named "out-identity-a"
    Then the response status should be 409
    And the JSON response field "message" should contain "gateway identity 'out-identity-a' is named by 1 deployed API"
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.identity" containing "out-partner-agent"
    When I delete the certificate named "out-backend-ca"
    Then the response status should be 409
    And the response should list a validation error for field "spec.upstreamDefinitions[0].tls.trustedCAs[0]" containing "out-partner-agent"
