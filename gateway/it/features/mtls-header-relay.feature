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

@mtls @mtls-header-relay
Feature: Client certificates relayed in a header by a front proxy
  As a gateway administrator running the gateway behind a load balancer that terminates TLS
  I want the client certificate the proxy relays in a header to authenticate the client
  So that partners keep their certificates while the topology changes, without letting anyone
  who can write a header impersonate anyone else

  The same mtls-auth policy handles it and API definitions do not change. A header is text anyone
  can send, so it is believed only when the connection that carries it authenticated as a pool entry
  marked as a relay. Otherwise the header is ignored, never trusted, and the connection's own
  certificate is evaluated as itself. The relayed header never reaches a backend: when the policy
  believed it, the backend receives the certificate it carried as X-Forwarded-Client-Cert instead,
  unless the API sets forwardCertificate to false. The gateway's own default configuration applies
  here: no bypass.

  Background:
    Given the gateway services are running
    And I authenticate using basic auth as "admin"
    And the client authority pool is empty
    And the certificate fixture "ca-a" is pooled as "relay-partner-a" with usage "downstream"
    And the certificate fixture "ca-b" is pooled as "relay-partner-b" with usage "downstream"
    And I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: relay-api
      spec:
        displayName: Relay API
        version: v1.0
        context: /relay/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: relay-partner-a
        operations:
          - method: GET
            path: /anything
      """
    And the response should be successful
    And I wait for the endpoint "http://localhost:8080/relay/v1.0/anything" to respond with status 401

  # ==================== HEADER MODE OFF: THE HEADER IS TEXT ====================

  Scenario: Without a relay entry the header is ignored and deleted, and the connection decides
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 401

  # ==================== HEADER MODE ON: A RELAY ENTRY VOUCHES ====================

  Scenario Outline: With a relay entry, only a connection authenticated as the relay can make the header believed
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" <presenting>
    Then the response status code should be <status>

    Examples:
      | presenting                                                                                                                      | status |
      | with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"                            | 401    |
      | with client certificate "client-expired" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"              | 401    |
      | with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"             | 200    |
      | with client certificate "client-wrong-ca" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"             | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"                     | 200    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"                  | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-expired"                   | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-selfsigned-b"              | 401    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "pem"    | 200    |
      | with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid" encoded as "base64" | 200    |
      | with client certificate "edge-lb"                                                                                               | 401    |

  Scenario: A header that is not a certificate is rejected when the relay vouched for it
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    And I set header "X-WSO2-CLIENT-CERTIFICATE" to "this-is-not-a-certificate"
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb"
    Then the response status code should be 401
    And the response body should be:
      """
      {"error":"Unauthorized","message":"Authentication failed"}
      """

  Scenario: A relay entry narrowed by SAN vouches only for the proxy carrying that SAN
    Given I upload the certificate fixture "corp-ca" as "relay-corp" with usage "downstream" and role "relay" and dns SAN "lb.corp.test"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb-corp" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "corp-other-service" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 401

  Scenario: A believed header reaches the backend as X-Forwarded-Client-Cert where the policy evaluated it, and never on a public route
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"
    And the backend's X-Forwarded-Client-Cert should not name certificate "edge-lb"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: relay-public-api
      spec:
        displayName: Relay Public API
        version: v1.0
        context: /relay-public/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/relay-public/v1.0/anything" to be ready
    When I send a GET request to "https://localhost:8443/relay-public/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a GET request to "https://localhost:8443/relay-public/v1.0/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"

  Scenario: A header from a connection that is not the relay never reaches a backend
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: relay-public-api
      spec:
        displayName: Relay Public API
        version: v1.0
        context: /relay-public/$version
        upstream:
          main:
            url: http://echo-backend:80
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/relay-public/v1.0/anything" to be ready
    When I send a GET request to "https://localhost:8443/relay-public/v1.0/anything" with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a GET request to "https://localhost:8443/relay-public/v1.0/anything" with no client certificate and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "client-valid" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-wrong-ca"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the backend's X-Forwarded-Client-Cert should name certificate "client-valid"
    And the backend's X-Forwarded-Client-Cert should not name certificate "client-wrong-ca"

  Scenario: An API that opts out of the certificate header receives neither header, even one it believed
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: relay-nofwd-api
      spec:
        displayName: Relay Opt-out API
        version: v1.0
        context: /relay-nofwd/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: relay-partner-a
              forwardCertificate: false
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And I wait for the endpoint "http://localhost:8080/relay-nofwd/v1.0/anything" to respond with status 401
    When I send a GET request to "https://localhost:8443/relay-nofwd/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the response should not contain echoed header "x-wso2-client-certificate"
    And the response should not contain echoed header "x-forwarded-client-cert"

  Scenario: A relay entry cannot be accepted as a client
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: relay-refused-api
      spec:
        displayName: Relay Refused API
        version: v1.0
        context: /relay-refused/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: relay-edge-lb
        operations:
          - method: GET
            path: /anything
      """
    Then the response status should be 400
    And the response should list a validation error for field "spec.policies[0].params.accept[0].ca" with message "relay-edge-lb is a relay (front proxy) entry and cannot be accepted as a client"

  Scenario: One load balancer serves an API that accepts it and an API that accepts the clients it relays
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the certificate fixture "edge-lb-ca" is pooled as "relay-edge-lb-client" with usage "downstream"
    When I deploy this API configuration:
      """
      apiVersion: gateway.api-platform.wso2.com/v1
      kind: RestApi
      metadata:
        name: relay-lb-api
      spec:
        displayName: Relay LB API
        version: v1.0
        context: /relay-lb/$version
        upstream:
          main:
            url: http://echo-backend:80
        policies:
          - name: mtls-auth
            version: v1
            params:
              accept:
                - ca: relay-edge-lb-client
        operations:
          - method: GET
            path: /anything
      """
    Then the response should be successful
    And the response should include a warning with code "MTLS_ACCEPT_NAMES_RELAY_AUTHORITY" for field "spec.policies[0].params.accept[0].ca"
    And I wait for the endpoint "http://localhost:8080/relay-lb/v1.0/anything" to respond with status 401
    Given I reset the analytics collector
    When I send a GET request to "https://localhost:8443/relay-lb/v1.0/anything" with client certificate "edge-lb"
    Then the response status code should be 200
    When I send a GET request to "https://localhost:8443/relay-lb/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the analytics collector should receive at least 2 events within 10 seconds
    And the analytics collector should have received at least 2 events
    And the latest analytics event should have the user id of fixture "edge-lb"
    Given I reset the analytics collector
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    And the analytics collector should receive at least 1 event within 10 seconds
    And the latest analytics event should have the user id of fixture "client-valid"
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb"
    Then the response status code should be 401

  Scenario: Removing the last relay entry turns header mode off again
    Given I upload the certificate fixture "edge-lb-ca" as "relay-edge-lb" with usage "downstream" and role "relay"
    And the response status should be 201
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 200
    When I delete the certificate named "relay-edge-lb"
    Then the response should be successful
    And the gateway has applied the client authority pool
    When I send a GET request to "https://localhost:8443/relay/v1.0/anything" with client certificate "edge-lb" and header "X-WSO2-CLIENT-CERTIFICATE" carrying certificate "client-valid"
    Then the response status code should be 401
