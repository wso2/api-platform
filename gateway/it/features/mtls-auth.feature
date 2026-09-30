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

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"

  # ==================== AN AUTHORITY NARROWED BY SAN ====================

  Scenario Outline: An API accepting one authority narrowed by URI SAN decides per certificate
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "auth-ca-b" with usage "downstream"
    And the certificate fixture "ca-b-same-dn" is pooled as "auth-lookalike" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
                  match:
                    uriSANs: ["urn:partner-a:payments"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-san/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                 | status |
      | with no client certificate                                 | 401    |
      | with client certificate "client-valid"                     | 200    |
      | with client certificate "client-valid-extra-sans"          | 200    |
      | with client certificate "client-multi-san"                 | 401    |
      | with client certificate "client-no-san"                    | 401    |
      | with client certificate "client-renewed"                   | 200    |
      | with client certificate "client-wrong-ca"                  | 401    |
      | with client certificate "client-same-cn-ca-b"              | 401    |
      | with client certificate "client-from-lookalike-ca"         | 401    |
      | with client certificate "client-selfsigned-b"              | 401    |
      | with client certificate "client-expired"                   | 401    |
      | with client certificate "client-not-yet-valid"             | 401    |
      | with client certificate "client-serverauth-only"           | 401    |
      | with client certificate "client-via-intermediate"          | 401    |
      | with client certificate "client-via-intermediate" and its chain | 401 |

  Scenario: Every rejection carries the same body and an accepted certificate is described to the backend
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
                  match:
                    uriSANs: ["urn:partner-a:payments"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-san/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with no client certificate
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-no-san"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-expired"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should contain echoed header "x-forwarded-client-cert" containing "Subject="
    And the response should contain echoed header "x-forwarded-client-cert" containing "URI=urn:partner-a:payments"
    And the response should contain echoed header "x-forwarded-client-cert" containing "Cert="

  Scenario: An API can keep the certificate header away from its backend
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-nofwd-api
      spec:
        displayName: mTLS No Forward API
        version: v1.0
        context: /mtls-nofwd/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
              forwardCertificate: false
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-nofwd/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-nofwd/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"
    When I send a GET request to "https://localhost:8443/mtls-nofwd/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401

  # ==================== EXACT CERTIFICATES BY THUMBPRINT ====================

  Scenario: An API accepting exact thumbprints admits those certificates and nothing else from the authority
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration with fixture values:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-thumb-api
      spec:
        displayName: mTLS Thumbprint API
        version: v1.0
        context: /mtls-thumb/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
                  thumbprints: ["{{thumbprint "client-valid"}}"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-thumb/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-thumb/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/mtls-thumb/v1.0/anything" with client certificate "client-renewed"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-thumb/v1.0/anything" with client certificate "client-no-san"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-thumb/v1.0/anything" with no client certificate
    Then the response status code should be 401

  Scenario: A renewed certificate is admitted once its thumbprint is listed alongside the old one
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration with fixture values:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-thumb-api
      spec:
        displayName: mTLS Thumbprint API
        version: v1.0
        context: /mtls-thumb/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
                  thumbprints: ["{{thumbprint "client-valid"}}", "{{thumbprint "client-renewed"}}"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-thumb/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-thumb/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/mtls-thumb/v1.0/anything" with client certificate "client-renewed"
    Then the response status code should be 200

  # ==================== THE ACCEPT LIST IS EVALUATED ON EVERY REQUEST ====================

  Scenario: Narrowing the accept list takes effect on the caller's next request
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-san/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-no-san"
    Then the response status code should be 200
    When I update the API "mtls-san-api" with this configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
                  match:
                    uriSANs: ["urn:partner-a:payments"]
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for policy snapshot sync
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-no-san"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: A client that caches its TLS session is accepted again on a new connection
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-resume-api
      spec:
        displayName: mTLS Resume API
        version: v1.0
        context: /mtls-resume/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-resume/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-resume/v1.0/anything" with client certificate "client-valid" on a resumable TLS session
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/mtls-resume/v1.0/anything" on a new connection from the same TLS session cache
    Then the gateway should have run a full TLS handshake
    And the response status code should be 200

  Scenario: Adding an authority to the pool grants no access to an API that does not accept it
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-san/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    Given the certificate fixture "ca-b" is pooled as "auth-ca-b" with usage "downstream"
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200

  Scenario: An API that inherits the whole pool follows the pool as it changes
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-pool-api
      spec:
        displayName: mTLS Pool API
        version: v1.0
        context: /mtls-pool/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-pool/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-pool/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/mtls-pool/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 401
    Given the certificate fixture "ca-b" is pooled as "auth-ca-b" with usage "downstream"
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/mtls-pool/v1.0/anything" with client certificate "client-wrong-ca"
    Then the response status code should be 200

  # ==================== PUBLIC APIS AND THE FORWARDED-CERTIFICATE HEADER ====================

  Scenario Outline: A public API ignores whatever certificate arrives and its backend never sees the header
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
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
        name: mtls-public-api
      spec:
        displayName: Public Echo API
        version: v1.0
        context: /mtls-public/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-public/v1.0/anything" to be ready
    When I send a GET request to "https://localhost:8443/mtls-public/v1.0/anything" <presenting>
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"

    Examples:
      | presenting                                          |
      | with no client certificate                          |
      | with client certificate "client-valid"              |
      | with client certificate "client-wrong-ca"           |
      | with client certificate "client-expired"            |
      | with client certificate "client-selfsigned-b"       |
      | with client certificate "client-serverauth-only"    |

  Scenario: A forged forwarded-certificate header never reaches a backend or stands in for a handshake
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-san-api
      spec:
        displayName: mTLS SAN API
        version: v1.0
        context: /mtls-san/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
                  match:
                    uriSANs: ["urn:partner-a:payments"]
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
        name: mtls-public-api
      spec:
        displayName: Public Echo API
        version: v1.0
        context: /mtls-public/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-public/v1.0/anything" to be ready
    Given I set header "X-Forwarded-Client-Cert" to "Subject=CN=forged;URI=urn:partner-a:payments"
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with no client certificate
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-no-san"
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-san/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should contain echoed header "x-forwarded-client-cert" containing "URI=urn:partner-a:payments"
    And the response body should not contain "CN=forged"
    When I send a GET request to "https://localhost:8443/mtls-public/v1.0/anything" with no client certificate
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"
    When I send a GET request to "http://localhost:8080/mtls-public/v1.0/anything"
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"

  # ==================== WHICH SHAPES OF POOL ENTRY ANCHOR WHICH CERTIFICATES ====================

  Scenario Outline: A pool entry holding only the root anchors everything the root signed, when the client sends its intermediate
    Given the certificate fixture "ca-a" is pooled as "anchor-entry" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-anchor-api
      spec:
        displayName: mTLS Anchor API
        version: v1.0
        context: /mtls-anchor/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: anchor-entry
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-anchor/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-anchor/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                            | status |
      | with client certificate "client-valid"                                | 200    |
      | with client certificate "client-via-intermediate"                     | 401    |
      | with client certificate "client-via-intermediate" and its chain       | 200    |
      | with client certificate "client-via-intermediate-2" and its chain     | 200    |
      | with client certificate "client-via-other-intermediate" and its chain | 200    |
      | with client certificate "client-wrong-ca"                             | 401    |

  Scenario Outline: A pool entry holding the root and its issuing intermediate supplies the intermediate itself
    Given I upload the certificate fixtures "ca-a-intermediate,ca-a" as "anchor-entry" with usage "downstream"
    And the response status should be 201
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-anchor-api
      spec:
        displayName: mTLS Anchor API
        version: v1.0
        context: /mtls-anchor/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: anchor-entry
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-anchor/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-anchor/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                            | status |
      | with client certificate "client-via-intermediate"                     | 200    |
      | with client certificate "client-via-intermediate" and its chain       | 200    |
      | with client certificate "client-via-intermediate-2" and its chain     | 200    |
      | with client certificate "client-via-intermediate-2"                   | 401    |
      | with client certificate "client-via-other-intermediate" and its chain | 200    |

  Scenario Outline: A pool entry holding only an issuing intermediate trusts exactly what that intermediate signed
    Given the certificate fixture "ca-a-intermediate" is pooled as "anchor-entry" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-anchor-api
      spec:
        displayName: mTLS Anchor API
        version: v1.0
        context: /mtls-anchor/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: anchor-entry
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-anchor/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-anchor/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                            | status |
      | with client certificate "client-via-intermediate"                     | 200    |
      | with client certificate "client-via-intermediate" and its chain       | 200    |
      | with client certificate "client-via-intermediate-2" and its chain     | 401    |
      | with client certificate "client-via-other-intermediate" and its chain | 401    |
      | with client certificate "client-valid"                                | 401    |

  Scenario Outline: A self-signed certificate pooled as its own authority admits exactly itself
    Given the certificate fixture "client-selfsigned" is pooled as "anchor-entry" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-anchor-api
      spec:
        displayName: mTLS Anchor API
        version: v1.0
        context: /mtls-anchor/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: anchor-entry
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-anchor/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-anchor/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                        | status |
      | with client certificate "client-selfsigned"                       | 200    |
      | with client certificate "client-selfsigned-b"                     | 401    |
      | with client certificate "client-selfsigned-renewed"               | 401    |
      | with client certificate "client-signed-by-leaf" and its chain     | 401    |

  Scenario Outline: With the root and the intermediate pooled separately, the named entry decides how wide the trust is
    Given the certificate fixture "ca-a" is pooled as "anchor-root" with usage "downstream"
    And the certificate fixture "ca-a-intermediate" is pooled as "anchor-intermediate" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-anchor-api
      spec:
        displayName: mTLS Anchor API
        version: v1.0
        context: /mtls-anchor/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: <entry>
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-anchor/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/mtls-anchor/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | entry               | presenting                                                            | status |
      | anchor-intermediate | with client certificate "client-via-intermediate"                     | 200    |
      | anchor-root         | with client certificate "client-via-intermediate"                     | 200    |
      | anchor-root         | with client certificate "client-via-other-intermediate" and its chain | 200    |
      | anchor-intermediate | with client certificate "client-via-other-intermediate" and its chain | 401    |

  # ==================== COMPOSITION AND SCOPE ====================

  Scenario: A certificate and a token are both required when both policies are attached
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-and-jwt-api
      spec:
        displayName: mTLS and JWT API
        version: v1.0
        context: /mtls-and-jwt/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: auth-ca-a
          - name: jwt-auth
            version: v1
            params:
              issuers:
                - mock-jwks
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-and-jwt/v1.0/anything" to respond with status 401
    When I get a JWT token from the mock JWKS server with issuer "http://mock-jwks:8080/token"
    And I send a GET request to "https://localhost:8443/mtls-and-jwt/v1.0/anything" with the JWT token and client certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/mtls-and-jwt/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    When I send a GET request to "https://localhost:8443/mtls-and-jwt/v1.0/anything" with the JWT token and no client certificate
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """
    When I send a GET request to "https://localhost:8443/mtls-and-jwt/v1.0/anything" with no client certificate
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """

  Scenario: Attached to one operation, the policy protects that operation only
    Given the certificate fixture "ca-a" is pooled as "auth-ca-a" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: mtls-op-api
      spec:
        displayName: mTLS Operation API
        version: v1.0
        context: /mtls-op/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /anything
          - method: GET
            path: /anything/protected
            policies:
              - name: mtls-auth
                version: v1
                params:
                  accept:
                    - ca: auth-ca-a
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/mtls-op/v1.0/anything" to be ready
    When I send a GET request to "https://localhost:8443/mtls-op/v1.0/anything" with no client certificate
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"
    When I send a GET request to "https://localhost:8443/mtls-op/v1.0/anything/protected" with no client certificate
    Then the response status code should be 401
    When I send a GET request to "https://localhost:8443/mtls-op/v1.0/anything/protected" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should contain echoed header "x-forwarded-client-cert" containing "Subject="
    When I send a GET request to "https://localhost:8443/mtls-op/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-forwarded-client-cert"
